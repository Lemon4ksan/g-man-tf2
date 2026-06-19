// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"github.com/lemon4ksan/miyako/generic"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logCfg := log.DefaultConfig(log.LevelInfo)
	logCfg.FullPath = true

	logger := log.New(logCfg)
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	logger.Info("Initializing inventory dumper...")

	username, password := os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS")
	if username == "" || password == "" {
		return errors.New("STEAM_USER and STEAM_PASS environment variables are required")
	}

	storagePath := generic.Coalesce(os.Getenv("STEAM_STORAGE_PATH"), "storage.json")

	store, err := jsonfile.New(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close session storage", log.Err(err))
		}
	}()

	clientCfg := steam.DefaultConfig()
	clientCfg.Storage = store

	opts := []steam.Option{
		steam.WithLogger(logger),
		apps.WithModule(),
		gc.WithModule(),
		tf2.WithModule(),
		schema.WithModule(schema.DefaultConfig()),
		backpack.WithModule(),
	}

	client, err := steam.NewClient(clientCfg, opts...)
	if err != nil {
		return fmt.Errorf("failed to create Steam client: %w", err)
	}

	defer func() {
		if err := client.Close(); err != nil {
			logger.Error("Error closing client session", log.Err(err))
		}
	}()

	if err := client.Run(); err != nil {
		return fmt.Errorf("failed to run client transport: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			logger.Warn("System signal received, starting graceful shutdown...", log.String("signal", sig.String()))
			cancel()
		case <-ctx.Done():
		}
	}()

	eventBus := client.Bus()

	sub := eventBus.Subscribe(&auth.LoggedOnEvent{}, &tf2.BackpackLoadedEvent{})
	defer sub.Unsubscribe()

	dir := directory.New(client.Service())

	server, err := dir.GetOptimalCMServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover optimal CM server: %w", err)
	}

	logger.Info("Optimal CM server discovered", log.String("endpoint", server.Endpoint))

	details := &auth.LogOnDetails{
		AccountName: username,
		Password:    password,
	}

	if err := client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("failed to connect and login to Steam: %w", err)
	}

	backpackLoaded := false
	for !backpackLoaded {
		select {
		case <-ctx.Done():
			return errors.New("timeout or shutdown requested before backpack loaded")
		case ev := <-sub.C():
			switch ev.(type) {
			case *auth.LoggedOnEvent:
				logger.Info("Steam session established")
			case *tf2.BackpackLoadedEvent:
				logger.Info("TF2 backpack loaded and synchronized successfully!")
				PrintDetailedInventory(backpack.From(client))

				backpackLoaded = true
			}
		}
	}

	return nil
}

func PrintDetailedInventory(bpMod *backpack.Backpack) {
	if bpMod == nil {
		return
	}

	s := bpMod.Schema().Get()
	items := bpMod.Cache().GetItems()

	slices.SortFunc(items, func(a, b *tf2.Item) int {
		if a.Position() != b.Position() {
			if a.Position() < b.Position() {
				return -1
			}

			return 1
		}

		if a.ID < b.ID {
			return -1
		} else if a.ID > b.ID {
			return 1
		}

		return 0
	})

	fmt.Println("=========================================================================================")
	fmt.Printf("DETAILED INVENTORY DUMP (Total Items: %d)\n", len(items))
	fmt.Println("=========================================================================================")

	for _, item := range items {
		name := "Unknown Item"
		if s != nil {
			if item.SKU != "" {
				skuItem, err := sku.FromString(item.SKU)
				if err == nil {
					name = s.ItemName(skuItem, true, false, false)
				}
			}

			if name == "Unknown Item" || name == "" {
				sch := s.ItemByDef(int(item.DefIndex))
				if sch != nil {
					if sch.ItemName != "" {
						name = sch.ItemName
					} else {
						name = sch.Name
					}
				}
			}
		}

		page := int((item.Position()-1)/50) + 1
		slot := int((item.Position()-1)%50) + 1

		positionStr := fmt.Sprintf("Page %d, Slot %d", page, slot)
		if item.Position() == 0 {
			positionStr = "Unplaced"
		}

		fmt.Printf("Item: %s\n", name)
		fmt.Printf("  • ID: %d (Original ID: %d)\n", item.ID, item.OriginalID)
		fmt.Printf("  • Defindex: %d\n", item.DefIndex)

		if s != nil {
			fmt.Printf("  • SKU: %s\n", item.GetSKU(s))
		}

		fmt.Printf("  • Quality: %s (%d)\n", getQualityName(item.Quality), item.Quality)
		fmt.Printf("  • Position: %s\n", positionStr)
		fmt.Printf(
			"  • Flags: Tradable: %t, Craftable: %t, Marketable: %t\n",
			item.IsTradable,
			item.IsCraftable,
			item.IsMarketable,
		)

		if item.CustomName != "" {
			fmt.Printf("  • Custom Name Tag: %q\n", item.CustomName)
		}

		if item.CustomDesc != "" {
			fmt.Printf("  • Custom Description Tag: %q\n", item.CustomDesc)
		}

		if item.Australium {
			fmt.Printf("  • Type: Australium weapon\n")
		}

		if item.Festivized {
			fmt.Printf("  • Festivized: Yes\n")
		}

		if item.Effect != 0 {
			effectName := "Unknown Effect"
			if s != nil {
				effectName = s.EffectByID(int(item.Effect))
			}

			fmt.Printf("  • Unusual Effect: %s (ID: %d)\n", effectName, item.Effect)
		}

		if item.KillstreakTier != 0 {
			fmt.Printf(
				"  • Killstreak Tier: %s (Tier: %d)\n",
				getKillstreakTierName(item.KillstreakTier),
				item.KillstreakTier,
			)

			if item.Sheen != 0 {
				fmt.Printf("    - Sheen ID: %d\n", item.Sheen)
			}

			if item.Killstreaker != 0 {
				fmt.Printf("    - Killstreaker (Eye Effect) ID: %d\n", item.Killstreaker)
			}
		}

		if item.Paintkit != 0 {
			skinName := "Unknown Skin"
			if s != nil {
				skinName = s.SkinByID(int(item.Paintkit))
			}

			fmt.Printf("  • Weapon Skin: %s (ID: %d) [Wear: %.2f%%]\n", skinName, item.Paintkit, item.Wear*100)
		}

		if item.CrateSeries != 0 {
			fmt.Printf("  • Crate Series: #%d\n", item.CrateSeries)
		}

		if item.Paint != 0 {
			paintName := "Unknown Paint Color"
			if s != nil {
				paintName = s.PaintNameByDecimal(int(item.Paint))
			}

			fmt.Printf("  • Applied Paint: %s (Color Decimal: %d)\n", paintName, item.Paint)
		}

		if len(item.Spells) > 0 {
			fmt.Println("  • Applied Halloween Spells:")

			for _, spell := range item.Spells {
				spellName := "Unknown Spell"
				if s != nil {
					spellName = s.SpellNameFromSKU(spell)
				}

				fmt.Printf("    - %s (Attribute: %d, Value: %d)\n", spellName, spell.Attribute, spell.Value)
			}
		}

		if len(item.Parts) > 0 {
			fmt.Println("  • Applied Strange Parts:")

			for _, partID := range item.Parts {
				partName := "Unknown Counter"
				if s != nil {
					for _, p := range s.Raw.Schema.KillEaterScoreTypes {
						if p.Type == int(partID) {
							partName = p.TypeName
							break
						}
					}
				}

				fmt.Printf("    - %s (Part Type ID: %d)\n", partName, partID)
			}
		}

		fmt.Println("-----------------------------------------------------------------------------------------")
	}

	fmt.Println("=========================================================================================")
}

func getQualityName(quality uint32) string {
	switch quality {
	case schema.QualityNormal:
		return "Normal"
	case schema.QualityGenuine:
		return "Genuine"
	case schema.QualityVintage:
		return "Vintage"
	case schema.QualityUnusual:
		return "Unusual"
	case schema.QualityUnique:
		return "Unique"
	case schema.QualityCommunity:
		return "Community"
	case schema.QualityValve:
		return "Valve"
	case schema.QualitySelfMade:
		return "Self-Made"
	case schema.QualityStrange:
		return "Strange"
	case schema.QualityHaunted:
		return "Haunted"
	case schema.QualityCollectors:
		return "Collector's"
	case schema.QualityDecorated:
		return "Decorated"
	default:
		return "Unknown"
	}
}

func getKillstreakTierName(tier uint32) string {
	switch tier {
	case tf2.KillstreakTierBasic:
		return "Basic"
	case tf2.KillstreakTierSpecialized:
		return "Specialized"
	case tf2.KillstreakTierProfessional:
		return "Professional"
	default:
		return "None"
	}
}
