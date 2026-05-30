// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
}

// run encapsulates the setup and execution lifecycle to ensure deferred cleanups run before exit.
func run() error {
	// 1. Initialize logging configuration
	logCfg := log.DefaultConfig(log.LevelInfo)
	logCfg.FullPath = true

	logger := log.New(logCfg)
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	logger.Info("Initializing inventory maintenance example...")

	// 2. Read authorization credentials from environment variables
	username := os.Getenv("STEAM_USER")

	password := os.Getenv("STEAM_PASS")
	if username == "" || password == "" {
		return errors.New("STEAM_USER and STEAM_PASS environment variables are required")
	}

	storagePath := os.Getenv("STEAM_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage.json"
	}

	// 3. Initialize session storage for auth tokens
	store, err := jsonfile.New(storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close session storage", log.Err(err))
		}
	}()

	// 4. Configure Steam client with necessary TF2 modules
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

	// Start background network transport services
	if err := client.Run(); err != nil {
		return fmt.Errorf("failed to run client transport: %w", err)
	}

	// 5. Define execution context supporting graceful shutdown on interrupt signals
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

	// Subscribe to key events to track inventory readiness
	eventBus := client.Bus()

	sub := eventBus.Subscribe(&auth.LoggedOnEvent{}, &tf2.BackpackLoadedEvent{})
	defer sub.Unsubscribe()

	// 6. Discover the optimal Steam Connection Manager (CM) Server
	dir := directory.New(client.Service())

	server, err := dir.GetOptimalCMServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover optimal CM server: %w", err)
	}

	logger.Info("Optimal CM server discovered", log.String("endpoint", server.Endpoint))

	// 7. Establish connection and logon
	details := &auth.LogOnDetails{
		AccountName: username,
		Password:    password,
	}

	if err := client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("failed to connect and login to Steam: %w", err)
	}

	logger.Info("Logon successful. Waiting for backpack to load and sync with Game Coordinator...")

	// Wait for BackpackLoadedEvent to confirm inventory is synchronized and cached
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

				backpackLoaded = true
			}
		}
	}

	// 8. Run inventory maintenance procedure
	err = RunInventoryMaintenance(ctx, client, logger)
	if err != nil {
		logger.Error("Inventory maintenance finished with error", log.Err(err))
		return err
	}

	logger.Info("Inventory maintenance completed successfully!")
	logger.Info("Closing session and exiting...")

	return nil
}

// RunInventoryMaintenance performs duplicate weapon smelting, metal condensing, and sorting.
func RunInventoryMaintenance(ctx context.Context, client *steam.Client, logger log.Logger) error {
	// Extract registered modules from client
	tf2Mod := tf2.From(client)
	if tf2Mod == nil {
		return errors.New("module tf2 not found or not registered")
	}

	bpMod := backpack.From(client)
	if bpMod == nil {
		return errors.New("module backpack not found or not registered")
	}

	// Verify Game Coordinator connection is ready
	if !tf2Mod.Connected() {
		return errors.New("no active connection to TF2 Game Coordinator (GC)")
	}

	logger.InfoContext(ctx, "Starting inventory maintenance...")

	// Initialize crafting manager
	craftMgr := crafting.NewManager(bpMod, tf2Mod)

	// --- STEP 1: Smelt Duplicate Weapons ---
	logger.InfoContext(ctx, "Scanning for weapons to smelt...")

	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
	s := bpMod.Schema().Get()

	type SmeltPair struct {
		Class string
		Item1 *tf2.Item
		Item2 *tf2.Item
	}

	var plan []SmeltPair

	for _, class := range classes {
		weapons := bpMod.FindWeaponsByClassForSmelting(class)
		if len(weapons) < 2 {
			// No duplicates left for this class (requires at least 2 weapons)
			continue
		}

		// We copy the slice to simulate sequential removal
		var available []*tf2.Item

		available = append(available, weapons...)

		for len(available) >= 2 {
			if !available[0].IsTradable || !available[1].IsTradable {
				logger.ErrorContext(ctx, "CRITICAL ERROR: Selected non-tradable weapon for smelting!",
					log.Uint64("id1", available[0].ID),
					log.Uint64("id2", available[1].ID),
				)
				available = available[2:]

				continue
			}

			plan = append(plan, SmeltPair{
				Class: class,
				Item1: available[0],
				Item2: available[1],
			})
			available = available[2:]
		}
	}

	if len(plan) > 0 {
		fmt.Println("\n===========================================================")
		fmt.Println("PLANNED WEAPON SMELTING OPERATIONS:")
		fmt.Println("===========================================================")
		fmt.Printf("%-10s | %-30s | %-30s\n", "Class", "Item 1 (ID)", "Item 2 (ID)")
		fmt.Println("-----------------------------------------------------------")

		for _, pair := range plan {
			name1 := "Unknown Item"

			name2 := "Unknown Item"
			if s != nil {
				sch1 := s.ItemByDef(int(pair.Item1.DefIndex))
				if sch1 != nil {
					name1 = sch1.ItemName
				}

				sch2 := s.ItemByDef(int(pair.Item2.DefIndex))
				if sch2 != nil {
					name2 = sch2.ItemName
				}
			}

			fmt.Printf("%-10s | %-30.30s | %-30.30s\n",
				pair.Class,
				fmt.Sprintf("%s (%d)", name1, pair.Item1.ID),
				fmt.Sprintf("%s (%d)", name2, pair.Item2.ID),
			)
		}

		fmt.Println("===========================================================")
		fmt.Printf(
			"Total smelt operations: %d (will smelt %d weapons into %d scrap metal).\n",
			len(plan),
			len(plan)*2,
			len(plan),
		)
		fmt.Print("Do you want to proceed with smelting? (y/N): ")

		scanner := bufio.NewScanner(os.Stdin)

		var response string
		if scanner.Scan() {
			response = scanner.Text()
		}

		response = strings.TrimSpace(response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			logger.InfoContext(ctx, "Weapon smelting cancelled by user.")
		} else {
			logger.InfoContext(ctx, "Weapon smelting confirmed. Dispatching GC commands...")

			totalSmelted := 0

			timer := time.NewTimer(500 * time.Millisecond)
			defer timer.Stop()

			for _, pair := range plan {
				// Verify items still exist and are unlocked before smelting (just in case)
				curWeapons := bpMod.FindWeaponsByClassForSmelting(pair.Class)
				hasItem1 := false

				hasItem2 := false
				for _, w := range curWeapons {
					if w.ID == pair.Item1.ID {
						hasItem1 = true
					}

					if w.ID == pair.Item2.ID {
						hasItem2 = true
					}
				}

				if !hasItem1 || !hasItem2 {
					logger.WarnContext(ctx, "Planned items are no longer available for smelting, skipping pair",
						log.Uint64("item_1", pair.Item1.ID),
						log.Uint64("item_2", pair.Item2.ID),
					)

					continue
				}

				logger.DebugContext(ctx, "Smelting class weapons...",
					log.String("class", pair.Class),
					log.Uint64("item_1", pair.Item1.ID),
					log.Uint64("item_2", pair.Item2.ID),
				)

				_, err := craftMgr.SmeltClassWeapons(ctx, pair.Class)
				if err != nil {
					logger.ErrorContext(ctx, "Failed to smelt weapons", log.String("class", pair.Class), log.Err(err))
					break
				}

				totalSmelted++

				// Reset and wait on reusable timer to avoid spamming Steam GC
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}

				timer.Reset(500 * time.Millisecond)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
				}
			}

			logger.InfoContext(ctx, "Weapon smelting completed", log.Int("smelt_cycles", totalSmelted))
		}
	} else {
		logger.InfoContext(ctx, "No weapons to smelt (fewer than 2 weapons per class).")
	}

	// --- STEP 2: Condense Low-Grade Metals ---
	logger.InfoContext(ctx, "Condensing excess low-grade metals...")

	crafts, err := craftMgr.CondenseMetal(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to condense low-grade metal", log.Err(err))
	} else {
		logger.InfoContext(ctx, "Low-grade metals successfully condensed", log.Int("combined_operations", crafts))
	}

	// --- STEP 3: Execute Complex Custom Sorting ---
	logger.InfoContext(ctx, "Executing complex custom sorting algorithm...")

	if err := SortInventoryByComplexRules(ctx, client, logger); err != nil {
		logger.ErrorContext(ctx, "Failed to apply complex custom sorting", log.Err(err))
	} else {
		logger.InfoContext(ctx, "Complex custom sorting completed successfully!")
	}

	return nil
}

// SortInventoryByComplexRules performs hierarchical sorting of the backpack:
// 1. Pure currency (Keys -> Refined -> Reclaimed -> Scrap) goes first.
// 2. Other items are grouped by character class (Scout -> Soldier -> ... -> Spy -> Multiclass -> Misc).
// 3. Identical items (by DefIndex) are grouped consecutively.
// 4. Within a group of identical items, Unique (6) quality items come first, followed by other qualities (Strange, Unusual, etc.).
func SortInventoryByComplexRules(ctx context.Context, client *steam.Client, logger log.Logger) error {
	tf2Mod := tf2.From(client)

	bpMod := backpack.From(client)
	if tf2Mod == nil || bpMod == nil || !tf2Mod.Connected() {
		return errors.New("TF2 modules are not ready or not connected to GC")
	}

	s := bpMod.Schema().Get()
	if s == nil {
		return errors.New("item schema is not loaded yet")
	}

	logger.InfoContext(ctx, "Starting complex hierarchical inventory sorting...")

	// Build map of locked items (active trades) to preserve their positions
	lockedMap := make(map[uint64]bool)
	for _, id := range bpMod.GetLockedAssetIDs() {
		lockedMap[id] = true
	}

	allItems := bpMod.Cache().GetItems()

	var unlockedItems []*tf2.Item

	// Filter out locked items
	for _, item := range allItems {
		if !lockedMap[item.ID] {
			unlockedItems = append(unlockedItems, item)
		}
	}

	// Sort unlockedItems in memory using a chain of rules
	slices.SortFunc(unlockedItems, func(a, b *tf2.Item) int {
		// Category Priority: Normal Tradable (1) -> Crate/Case Tradable (2) -> Untradable (3)
		aCat := getCategoryPriority(a, s)

		bCat := getCategoryPriority(b, s)
		if aCat != bCat {
			return aCat - bCat
		}

		// Rule 1: Pure currency checks (Keys, Refined, Reclaimed, Scrap)
		aPure := getPurePriority(a.DefIndex, s)

		bPure := getPurePriority(b.DefIndex, s)
		if aPure != bPure {
			return aPure - bPure
		}

		if aPure != 5 { // Both items are currency
			if a.DefIndex != b.DefIndex {
				return int(a.DefIndex) - int(b.DefIndex)
			}

			return int(a.ID - b.ID)
		}

		// Rule 2: Group by class (Scout -> ... -> Spy -> Multiclass -> Misc)
		aClassPri := getClassPriority(a, s)

		bClassPri := getClassPriority(b, s)
		if aClassPri != bClassPri {
			return aClassPri - bClassPri
		}

		// Rule 3: Group by weapon slot (Primary -> Secondary -> Melee -> PDA -> Misc)
		aSlotPri := getSlotPriority(a, s)

		bSlotPri := getSlotPriority(b, s)
		if aSlotPri != bSlotPri {
			return aSlotPri - bSlotPri
		}

		// Rule 4: Group identical items (by base DefIndex)
		if a.DefIndex != b.DefIndex {
			return int(a.DefIndex) - int(b.DefIndex)
		}

		// Rule 5: Unique (Unique/6) items first, followed by others (Strange, Unusual, etc.)
		aQualPri := getQualityPriority(a.Quality)

		bQualPri := getQualityPriority(b.Quality)
		if aQualPri != bQualPri {
			return aQualPri - bQualPri
		}

		// If qualities are different, sort by quality ID
		if a.Quality != b.Quality {
			return int(a.Quality) - int(b.Quality)
		}

		// Stable sort by asset ID
		if a.ID < b.ID {
			return -1
		} else if a.ID > b.ID {
			return 1
		}

		return 0
	})

	// Generate moves, skipping slots occupied by locked items to preserve their positions
	var moves []tf2.ItemPos

	currentSlot := 1
	currentPage := 1

	for _, item := range unlockedItems {
		for {
			targetPos := backpack.PositionOf(currentPage, currentSlot)

			// Check if the slot is currently occupied by a locked item
			if !isSlotOccupiedByLockedItem(targetPos, allItems, lockedMap) {
				if item.Position() != targetPos {
					moves = append(moves, tf2.ItemPos{
						ID:       item.ID,
						Position: targetPos,
					})
				}

				// Move to next slot
				currentSlot++
				if currentSlot > backpack.ItemsPerPage {
					currentSlot = 1
					currentPage++
				}

				break
			}

			// Slot is occupied by a locked item - skip it to preserve its position
			currentSlot++
			if currentSlot > backpack.ItemsPerPage {
				currentSlot = 1
				currentPage++
			}
		}
	}

	if len(moves) == 0 {
		logger.InfoContext(ctx, "Backpack is already sorted according to the specified rules")
		return nil
	}

	logger.InfoContext(ctx, "Applying backpack positions via GC...", log.Int("total_moves", len(moves)))

	return tf2Mod.MoveItems(ctx, moves)
}

// getCategoryPriority determines an item's high-level category priority:
// 1: Tradable ordinary items (first)
// 2: Tradable cases/crates (after normal items)
// 3: Untradable items (at the very end of the backpack)
func getCategoryPriority(item *tf2.Item, s *schema.Schema) int {
	if !item.IsTradable {
		return 3
	}

	sch := s.ItemByDef(int(item.DefIndex))
	if sch != nil && sch.ItemClass == "supply_crate" {
		return 2
	}

	return 1
}

// getPurePriority determines the priority of pure currency (Keys -> Refined -> Reclaimed -> Scrap).
func getPurePriority(defIndex uint32, s *schema.Schema) int {
	norm := s.NormalizeDefindex(int(defIndex))
	switch norm {
	case schema.DefKey:
		return 1 // Keys (5021)
	case schema.DefRefined:
		return 2 // Refined Metal (5002)
	case schema.DefReclaimed:
		return 3 // Reclaimed Metal (5001)
	case schema.DefScrap:
		return 4 // Scrap Metal (5000)
	default:
		return 5 // Ordinary items
	}
}

// getClassPriority determines the class grouping priority.
func getClassPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil || len(sch.UsedByClasses) == 0 {
		return 12 // Misc
	}

	if len(sch.UsedByClasses) > 1 {
		return 10 // Multiclass
	}

	switch sch.UsedByClasses[0] {
	case "Scout":
		return 1
	case "Soldier":
		return 2
	case "Pyro":
		return 3
	case "Demoman":
		return 4
	case "Heavy":
		return 5
	case "Engineer":
		return 6
	case "Medic":
		return 7
	case "Sniper":
		return 8
	case "Spy":
		return 9
	default:
		return 11
	}
}

func getSlotPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return 5
	}

	if sch.CraftClass != "weapon" {
		return 4
	}

	switch sch.ItemClass {
	case "tf_weapon_scattergun", "tf_weapon_rocketlauncher", "tf_weapon_flamethrower",
		"tf_weapon_grenadelauncher", "tf_weapon_minigun", "tf_weapon_syringegun",
		"tf_weapon_sniperrifle", "tf_weapon_revolver":
		return 1 // Primary

	case "tf_weapon_pistol", "tf_weapon_shotgun", "tf_weapon_pipebomblauncher",
		"tf_weapon_smg", "tf_weapon_medigun", "tf_weapon_buff_item", "tf_weapon_parachute":
		return 2 // Secondary

	case "tf_weapon_bat", "tf_weapon_shovel", "tf_weapon_fireaxe", "tf_weapon_club",
		"tf_weapon_bonesaw", "tf_weapon_fists", "tf_weapon_wrench", "tf_weapon_knife",
		"tf_weapon_sword", "tf_weapon_sledgehammer":
		return 3 // Melee

	case "tf_weapon_pda_spy", "tf_weapon_pda_engineer_build", "tf_weapon_pda_engineer_destroy",
		"tf_weapon_builder", "tf_weapon_spellbook":
		return 4 // PDA / Action

	default:
		return 5 // Other
	}
}

// getQualityPriority returns priority for Unique quality.
func getQualityPriority(quality uint32) int {
	if quality == schema.QualityUnique { // 6
		return 1 // Unique items come first in groups of identical items
	}

	return 2 // All other qualities (Strange, Unusual, etc.) come next
}

// isSlotOccupiedByLockedItem checks if the item located at the slot is locked.
func isSlotOccupiedByLockedItem(pos uint32, allItems []*tf2.Item, lockedMap map[uint64]bool) bool {
	for _, item := range allItems {
		if item.Position() == pos && lockedMap[item.ID] {
			return true
		}
	}

	return false
}
