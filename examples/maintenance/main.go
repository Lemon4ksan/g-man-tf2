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
	username, password := os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS")
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

// Define strict presentation sections for logical grouping.
const (
	SectionPureCurrency = 1
	SectionWeapons      = 2
	SectionCosmetics    = 3
	SectionTaunts       = 4
	SectionToolsActions = 5
	SectionCratesCases  = 6
)

// SortInventoryByComplexRules performs hierarchical sorting of the backpack:
// 1. Division by Tradable (first pages) and Untradable (last pages) blocks.
// 2. Division into strict logical sections (Currency -> Weapons -> Cosmetics -> Taunts -> Tools -> Crates).
// 3. Weapons and cosmetics are strictly sorted by character class.
// 4. Weapons are sorted by slots (Primary -> Secondary -> Melee -> PDA).
// 5. Each major section dynamically starts on a completely fresh backpack page.
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

	// Sort unlockedItems in memory using a chain of presentation rules
	slices.SortFunc(unlockedItems, func(a, b *tf2.Item) int {
		// Rule 1: High-level Trade Ban segregation (Tradables (1) first, Untradables (2) last)
		aTrade := 1
		if !a.IsTradable {
			aTrade = 2
		}
		bTrade := 1
		if !b.IsTradable {
			bTrade = 2
		}
		if aTrade != bTrade {
			return aTrade - bTrade
		}

		// Rule 2: Logical Section Priority (Currency -> Weapons -> Cosmetics -> Taunts -> Tools -> Crates)
		aSec := getSectionPriority(a, s)
		bSec := getSectionPriority(b, s)
		if aSec != bSec {
			return aSec - bSec
		}

		// Rule 3: Pure currency checks (within the Currency section)
		if aSec == SectionPureCurrency {
			aPure := getPurePriority(a.DefIndex, s)
			bPure := getPurePriority(b.DefIndex, s)
			if aPure != bPure {
				return aPure - bPure
			}
			if a.DefIndex != b.DefIndex {
				return int(a.DefIndex) - int(b.DefIndex)
			}
			return int(a.ID - b.ID)
		}

		// Rule 4: Group by character class (Scout -> ... -> Spy -> Multiclass -> All-Class)
		aClassPri := getClassPriority(a, s)
		bClassPri := getClassPriority(b, s)
		if aClassPri != bClassPri {
			return aClassPri - bClassPri
		}

		// Rule 5: Group by weapon slot (Primary -> Secondary -> Melee -> PDA)
		if aSec == SectionWeapons {
			aSlotPri := getSlotPriority(a, s)
			bSlotPri := getSlotPriority(b, s)
			if aSlotPri != bSlotPri {
				return aSlotPri - bSlotPri
			}
		}

		// Rule 6: Group identical items (by base DefIndex)
		if a.DefIndex != b.DefIndex {
			return int(a.DefIndex) - int(b.DefIndex)
		}

		// Rule 7: Unique quality first, followed by specialized qualities (Strange, Unusual, etc.)
		aQualPri := getQualityPriority(a.Quality)
		bQualPri := getQualityPriority(b.Quality)
		if aQualPri != bQualPri {
			return aQualPri - bQualPri
		}

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

	// Generate moves, dynamically shifting sections to fresh pages for premium presentation
	var moves []tf2.ItemPos

	currentSlot := 1
	currentPage := 1
	lastSection := -1
	lastTradeBlock := -1

	for _, item := range unlockedItems {
		section := getSectionPriority(item, s)
		tradeBlock := 1
		if !item.IsTradable {
			tradeBlock = 2
		}

		// If we enter a new section or shift from tradables to untradables,
		// align the start onto a completely fresh page.
		isSectionChanged := lastSection != -1 && section != lastSection
		isTradeBlockChanged := lastTradeBlock != -1 && tradeBlock != lastTradeBlock

		if isSectionChanged || isTradeBlockChanged {
			if currentSlot > 1 {
				currentSlot = 1
				currentPage++
			}
		}
		lastSection = section
		lastTradeBlock = tradeBlock

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

// getSectionPriority determines the presentation section of an item.
func getSectionPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return SectionToolsActions
	}

	// 1. Pure Currency Check
	norm := s.NormalizeDefindex(int(item.DefIndex))
	if norm == schema.DefKey || norm == schema.DefRefined || norm == schema.DefReclaimed || norm == schema.DefScrap {
		return SectionPureCurrency
	}

	// 2. Crates & Cases Check
	if sch.ItemClass == "supply_crate" {
		return SectionCratesCases
	}

	// 3. Taunts Check
	if sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:") {
		return SectionTaunts
	}

	// 4. Weapons Check
	if sch.CraftClass == "weapon" {
		return SectionWeapons
	}

	// 5. Cosmetics Check (hats or wearables)
	if sch.CraftClass == "hat" || sch.ItemClass == "tf_wearable" {
		return SectionCosmetics
	}

	// 6. Tools, Actions, etc.
	return SectionToolsActions
}

// getPurePriority determines the priority of pure currency (Keys -> Refined -> Reclaimed -> Scrap).
func getPurePriority(defIndex uint32, s *schema.Schema) int {
	norm := s.NormalizeDefindex(int(defIndex))
	switch norm {
	case schema.DefKey:
		return 1
	case schema.DefRefined:
		return 2
	case schema.DefReclaimed:
		return 3
	case schema.DefScrap:
		return 4
	default:
		return 5
	}
}

// getClassPriority determines the class grouping priority (Scout -> ... -> Spy -> Multiclass -> All-Class).
func getClassPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil || len(sch.UsedByClasses) == 0 {
		return 12 // Misc/All-Class
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

// getSlotPriority resolves weapon slots with 100% accuracy using prefix-matching.
func getSlotPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return 5
	}

	if sch.CraftClass != "weapon" {
		return 5
	}

	cls := sch.ItemClass
	def := item.DefIndex

	// 1. Primary Weapons
	// Check known primary defindexes for classes where Shotgun/Parachute might overlap
	if def == 9 || def == 141 || def == 527 || def == 588 || def == 997 || def == 1153 {
		return 1 // Primary Shotguns for Engineer
	}

	if strings.Contains(cls, "scattergun") ||
		strings.Contains(cls, "rocketlauncher") ||
		strings.Contains(cls, "flamethrower") ||
		strings.Contains(cls, "grenadelauncher") ||
		strings.Contains(cls, "minigun") ||
		strings.Contains(cls, "syringegun") ||
		strings.Contains(cls, "sniperrifle") ||
		strings.Contains(cls, "revolver") ||
		strings.Contains(cls, "crossbow") ||
		strings.Contains(cls, "compound_bow") ||
		strings.Contains(cls, "particle_cannon") ||
		strings.Contains(cls, "soda_popper") ||
		strings.Contains(cls, "handgun_scout_primary") ||
		def == 1178 { // Dragon's Fury
		return 1
	}

	// 2. Secondary Weapons
	if strings.Contains(cls, "pistol") ||
		strings.Contains(cls, "pipebomblauncher") ||
		strings.Contains(cls, "smg") ||
		strings.Contains(cls, "medigun") ||
		strings.Contains(cls, "buff_item") ||
		strings.Contains(cls, "parachute") ||
		strings.Contains(cls, "lunchbox") ||
		strings.Contains(cls, "jar") ||
		strings.Contains(cls, "laser_pointer") || // Wrangler
		strings.Contains(cls, "shotgun") || // pyro/soldier/heavy shotguns
		strings.Contains(cls, "handgun_scout_secondary") ||
		strings.Contains(cls, "raygun") ||
		def == 131 || def == 406 || def == 1101 { // Shields (Targe, Screen, Tide Turner)
		return 2
	}

	// 3. Melee Weapons
	if strings.Contains(cls, "bat") ||
		strings.Contains(cls, "shovel") ||
		strings.Contains(cls, "fireaxe") ||
		strings.Contains(cls, "club") ||
		strings.Contains(cls, "bonesaw") ||
		strings.Contains(cls, "fists") ||
		strings.Contains(cls, "wrench") ||
		strings.Contains(cls, "knife") ||
		strings.Contains(cls, "sword") ||
		strings.Contains(cls, "sledgehammer") ||
		strings.Contains(cls, "mechanical_arm") || // Gunslinger
		strings.Contains(cls, "stick") {
		return 3
	}

	// 4. PDA / Action / Builder
	if strings.Contains(cls, "pda") ||
		strings.Contains(cls, "builder") ||
		strings.Contains(cls, "spellbook") {
		return 4
	}

	return 5
}

// getQualityPriority returns priority for Unique quality.
func getQualityPriority(quality uint32) int {
	if quality == schema.QualityUnique {
		return 1
	}

	return 2
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
