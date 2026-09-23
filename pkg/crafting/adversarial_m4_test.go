// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// statefulSimulatedInventory simulates live TF2 inventory and GC craft transitions.
type statefulSimulatedInventory struct {
	mu           sync.Mutex
	refinedIDs   []uint64
	reclaimedIDs []uint64
	scrapIDs     []uint64
	weapons      map[string][]*tf2.Item
	nextID       uint64
	craftLog     []craftRecord
}

type craftRecord struct {
	recipe int16
	inputs []uint64
}

func newStatefulSimulatedInventory(ref, rec, scrap int) *statefulSimulatedInventory {
	s := &statefulSimulatedInventory{
		weapons: make(map[string][]*tf2.Item),
		nextID:  1000,
	}

	for i := 0; i < ref; i++ {
		s.nextID++
		s.refinedIDs = append(s.refinedIDs, s.nextID)
	}

	for i := 0; i < rec; i++ {
		s.nextID++
		s.reclaimedIDs = append(s.reclaimedIDs, s.nextID)
	}

	for i := 0; i < scrap; i++ {
		s.nextID++
		s.scrapIDs = append(s.scrapIDs, s.nextID)
	}

	return s
}

func (s *statefulSimulatedInventory) FindCraftableItems(defIndex uint32, count int) []uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var list []uint64
	switch defIndex {
	case DefIndexRefined:
		list = s.refinedIDs
	case DefIndexReclaimed:
		list = s.reclaimedIDs
	case DefIndexScrap:
		list = s.scrapIDs
	}

	if len(list) < count {
		return append([]uint64(nil), list...)
	}

	return append([]uint64(nil), list[:count]...)
}

func (s *statefulSimulatedInventory) GetMetalCount(defIndex uint32) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch defIndex {
	case DefIndexRefined:
		return len(s.refinedIDs)
	case DefIndexReclaimed:
		return len(s.reclaimedIDs)
	case DefIndexScrap:
		return len(s.scrapIDs)
	default:
		return 0
	}
}

func (s *statefulSimulatedInventory) GetAssetIDs(sku string) []uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch sku {
	case currency.SKURefined:
		return append([]uint64(nil), s.refinedIDs...)
	case currency.SKUReclaimed:
		return append([]uint64(nil), s.reclaimedIDs...)
	case currency.SKUScrap:
		return append([]uint64(nil), s.scrapIDs...)
	default:
		return nil
	}
}

func (s *statefulSimulatedInventory) GetPureStock() currency.PureStock {
	s.mu.Lock()
	defer s.mu.Unlock()

	return currency.PureStock{
		Refined:   len(s.refinedIDs),
		Reclaimed: len(s.reclaimedIDs),
		Scrap:     len(s.scrapIDs),
	}
}

func (s *statefulSimulatedInventory) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	var craftable []*tf2.Item
	for _, w := range s.weapons[class] {
		if w.IsCraftable {
			craftable = append(craftable, w)
		}
	}

	return craftable
}

func (s *statefulSimulatedInventory) addWeapon(class string, id uint64, tradable, craftable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.weapons[class] = append(s.weapons[class], &tf2.Item{
		ID:          id,
		IsTradable:  tradable,
		IsCraftable: craftable,
		Quality:     uint32(schema.QualityUnique),
	})
}

func removeIDs(src, toRemove []uint64) []uint64 {
	m := make(map[uint64]bool)
	for _, id := range toRemove {
		m[id] = true
	}

	var out []uint64
	for _, id := range src {
		if !m[id] {
			out = append(out, id)
		}
	}

	return out
}

func (s *statefulSimulatedInventory) Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.craftLog = append(s.craftLog, craftRecord{recipe: recipe, inputs: items})

	switch recipe {
	case RecipeSmeltRefined:
		if len(s.refinedIDs) == 0 {
			return nil, errors.New("no refined metal to smelt")
		}

		s.refinedIDs = removeIDs(s.refinedIDs, items)

		var outputs []uint64
		for i := 0; i < 3; i++ {
			s.nextID++
			s.reclaimedIDs = append(s.reclaimedIDs, s.nextID)
			outputs = append(outputs, s.nextID)
		}

		return outputs, nil

	case RecipeSmeltReclaimed:
		if len(s.reclaimedIDs) == 0 {
			return nil, errors.New("no reclaimed metal to smelt")
		}

		s.reclaimedIDs = removeIDs(s.reclaimedIDs, items)

		var outputs []uint64
		for i := 0; i < 3; i++ {
			s.nextID++
			s.scrapIDs = append(s.scrapIDs, s.nextID)
			outputs = append(outputs, s.nextID)
		}

		return outputs, nil

	case RecipeCombineScrap:
		if len(items) < 3 {
			return nil, errors.New("need 3 scrap")
		}

		s.scrapIDs = removeIDs(s.scrapIDs, items)
		s.nextID++
		s.reclaimedIDs = append(s.reclaimedIDs, s.nextID)

		return []uint64{s.nextID}, nil

	case RecipeCombineReclaimed:
		if len(items) < 3 {
			return nil, errors.New("need 3 reclaimed")
		}

		s.reclaimedIDs = removeIDs(s.reclaimedIDs, items)
		s.nextID++
		s.refinedIDs = append(s.refinedIDs, s.nextID)

		return []uint64{s.nextID}, nil

	case RecipeSmeltWeapons:
		if len(items) < 2 {
			return nil, errors.New("need 2 weapons")
		}

		// Remove weapons from all classes
		for class, list := range s.weapons {
			var remaining []*tf2.Item
			for _, w := range list {
				if w.ID != items[0] && w.ID != items[1] {
					remaining = append(remaining, w)
				}
			}

			s.weapons[class] = remaining
		}

		s.nextID++
		s.scrapIDs = append(s.scrapIDs, s.nextID)

		return []uint64{s.nextID}, nil

	default:
		return nil, fmt.Errorf("unsupported recipe: %d", recipe)
	}
}

// -----------------------------------------------------------------------------------------
// 1. Bidirectional Metal Change Algorithm Tests
// -----------------------------------------------------------------------------------------

func TestBidirectionalMetalChange_Adversarial(t *testing.T) {
	t.Parallel()

	t.Run("exact_change_from_mixed_inventory", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		testCases := []struct {
			name        string
			needed      currency.Scrap
			expectCount int
		}{
			{"1_scrap", 1, 1},
			{"2_scrap", 2, 2},
			{"4_scrap_1rec_1scrap", 4, 2},
			{"9_scrap_1ref", 9, 1},
			{"10_scrap_1ref_1scrap", 10, 2},
			{"12_scrap_1ref_1rec", 12, 2},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				subSim := newStatefulSimulatedInventory(2, 3, 5)
				subMgr := NewManager(subSim, subSim)
				subMM := NewMetalManager(subSim, subMgr, log.Discard)

				selected, err := subMM.SelectMetal(ctx, tc.needed)
				require.NoError(t, err)
				assert.Len(t, selected, tc.expectCount)
				// No crafts needed because inventory had exact change
				assert.Empty(t, subSim.craftLog)
			})
		}
	})

	t.Run("odd_scrap_requests_only_refined_available_preserves_reclaimed", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		cases := []struct {
			name                 string
			needed               currency.Scrap
			initialRef           int
			expectedCrafts       int
			expectedRecInStock   int
			expectedScrapInStock int
			expectedSelectedLen  int
		}{
			{
				name:                 "1_scrap_needed_smelts_1ref_and_1rec_preserves_2rec",
				needed:               1,
				initialRef:           1,
				expectedCrafts:       2, // 1 Ref -> 3 Rec, 1 Rec -> 3 Scrap
				expectedRecInStock:   2, // 2 Rec preserved in stock!
				expectedScrapInStock: 3, // 3 Scrap in stock from the 1 smelted Rec
				expectedSelectedLen:  1, // 1 scrap item selected
			},
			{
				name:                 "2_scrap_needed_smelts_1ref_and_1rec_preserves_2rec",
				needed:               2,
				initialRef:           1,
				expectedCrafts:       2, // 1 Ref -> 3 Rec, 1 Rec -> 3 Scrap
				expectedRecInStock:   2, // 2 Rec preserved in stock!
				expectedScrapInStock: 3, // 3 Scrap in stock
				expectedSelectedLen:  2, // 2 scrap items selected
			},
			{
				name:                 "4_scrap_needed_smelts_1ref_and_1rec_preserves_1rec",
				needed:               4,
				initialRef:           1,
				expectedCrafts:       2, // 1 Ref -> 3 Rec, 1 Rec -> 3 Scrap
				expectedRecInStock:   2, // 2 Rec in stock (1 used in selected IDs, 1 preserved)
				expectedScrapInStock: 3, // 3 Scrap in stock (1 used in selected IDs, 2 preserved)
				expectedSelectedLen:  2, // 1 Rec + 1 Scrap = 4 scrap value
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				sim := newStatefulSimulatedInventory(tc.initialRef, 0, 0)
				mgr := NewManager(sim, sim)
				mm := NewMetalManager(sim, mgr, log.Discard)

				selected, err := mm.SelectMetal(ctx, tc.needed)
				require.NoError(t, err)
				assert.Len(t, selected, tc.expectedSelectedLen)

				// Verify crafts executed: exactly 1 SmeltRefined and 1 SmeltReclaimed
				require.Len(t, sim.craftLog, tc.expectedCrafts)
				assert.Equal(t, RecipeSmeltRefined, sim.craftLog[0].recipe)
				assert.Equal(t, RecipeSmeltReclaimed, sim.craftLog[1].recipe)

				// Verify remaining inventory: intermediate Reclaimed was NOT over-smelted!
				assert.Equal(t, tc.expectedRecInStock, sim.GetMetalCount(DefIndexReclaimed), "reclaimed preserved")
				assert.Equal(t, tc.expectedScrapInStock, sim.GetMetalCount(DefIndexScrap), "scrap in stock")
			})
		}
	})

	t.Run("requests_exceeding_pure_metal_supply", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		sim := newStatefulSimulatedInventory(0, 1, 1) // total 4 scrap (1 rec + 1 scrap)
		mgr := NewManager(sim, sim)
		mm := NewMetalManager(sim, mgr, log.Discard)

		// Request 5 scrap: exceeds total 4 scrap
		_, err := mm.SelectMetal(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not enough metal: missing 1 scrap")

		// With zero metal entirely
		emptySim := newStatefulSimulatedInventory(0, 0, 0)
		emptyMgr := NewManager(emptySim, emptySim)
		emptyMM := NewMetalManager(emptySim, emptyMgr, log.Discard)

		_, err = emptyMM.SelectMetal(ctx, 1)
		require.Error(t, err)
	})
}

// -----------------------------------------------------------------------------------------
// 2. Duplicate Weapon Smelting Tests
// -----------------------------------------------------------------------------------------

func TestDuplicateWeaponSmelting_Adversarial(t *testing.T) {
	t.Parallel()

	t.Run("zero_pure_metal_smelts_duplicate_weapons_of_same_class", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		sim := newStatefulSimulatedInventory(0, 0, 0) // Zero pure metal!
		sim.addWeapon("Scout", 101, true, true)
		sim.addWeapon("Scout", 102, true, true)

		mgr := NewManager(sim, sim)
		mm := NewMetalManager(sim, mgr, log.Discard)

		err := mm.TryToSmeltForChange(ctx, 1)
		require.NoError(t, err)

		// Verify RecipeSmeltWeapons was called with items 101 and 102
		require.Len(t, sim.craftLog, 1)
		assert.Equal(t, RecipeSmeltWeapons, sim.craftLog[0].recipe)
		assert.ElementsMatch(t, []uint64{101, 102}, sim.craftLog[0].inputs)

		// Verify 1 scrap is now in inventory
		assert.Equal(t, 1, sim.GetMetalCount(DefIndexScrap))
	})

	t.Run("untradable_weapons_are_strictly_rejected", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		sim := newStatefulSimulatedInventory(0, 0, 0)
		sim.addWeapon("Scout", 101, false, true) // UNTRADABLE!
		sim.addWeapon("Scout", 102, true, true)

		mgr := NewManager(sim, sim)
		mm := NewMetalManager(sim, mgr, log.Discard)

		// Direct call to SmeltDuplicates must return explicit tradability refusal
		err := mm.SmeltDuplicates(ctx, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to smelt: weapons must be tradable")
		assert.Empty(t, sim.craftLog, "no GC craft call should be made for untradable weapons")

		// TryToSmeltForChange must safely fail and not smelt any untradable items
		err = mm.TryToSmeltForChange(ctx, 1)
		require.Error(t, err)
		assert.Empty(t, sim.craftLog, "TryToSmeltForChange must not smelt untradable weapons")
	})

	t.Run("uncraftable_weapons_are_excluded", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		sim := newStatefulSimulatedInventory(0, 0, 0)
		sim.addWeapon("Scout", 101, true, false) // UNCRAFTABLE!
		sim.addWeapon("Scout", 102, true, true)

		mgr := NewManager(sim, sim)
		mm := NewMetalManager(sim, mgr, log.Discard)

		err := mm.SmeltDuplicates(ctx, 1)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoDuplicateWeapons)
		assert.Empty(t, sim.craftLog, "uncraftable weapon must not be smelted")
	})

	t.Run("zero_pure_metal_smelts_multiple_classes_to_satisfy_higher_change", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		sim := newStatefulSimulatedInventory(0, 0, 0)
		sim.addWeapon("Scout", 101, true, true)
		sim.addWeapon("Scout", 102, true, true)
		sim.addWeapon("Soldier", 201, true, true)
		sim.addWeapon("Soldier", 202, true, true)

		mgr := NewManager(sim, sim)
		mm := NewMetalManager(sim, mgr, log.Discard)

		err := mm.SmeltDuplicates(ctx, 2)
		require.NoError(t, err)

		// 2 pairs smelted -> 2 craft logs
		require.Len(t, sim.craftLog, 2)
		assert.Equal(t, RecipeSmeltWeapons, sim.craftLog[0].recipe)
		assert.Equal(t, RecipeSmeltWeapons, sim.craftLog[1].recipe)
		assert.Equal(t, 2, sim.GetMetalCount(DefIndexScrap))
	})
}

// -----------------------------------------------------------------------------------------
// 3. Pure Supply Balancer Tests
// -----------------------------------------------------------------------------------------

func TestPureSupplyBalancer_Adversarial(t *testing.T) {
	t.Parallel()

	t.Run("batch_balancing_in_single_tick_without_oscillation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Initial state:
		// Scrap: 15 (> maxScrap 9 -> excess 6 scrap -> 2 combineScrap needed)
		// Rec: 1 (< minRec 3 -> deficit 2 rec -> 1 smeltRef needed)
		// Ref: 10
		sim := newStatefulSimulatedInventory(10, 1, 15)
		mgr := NewManager(sim, sim)
		auto := NewAutomator(mgr, sim)

		// Execute single Tick
		err := auto.Tick(ctx)
		require.NoError(t, err)

		// Verify crafts performed in Tick 1:
		// 2 combineScrap (6 scrap -> 2 rec)
		// 1 smeltRef (1 ref -> 3 rec)
		// Total crafts = 3
		require.Len(t, sim.craftLog, 3)
		assert.Equal(t, RecipeCombineScrap, sim.craftLog[0].recipe)
		assert.Equal(t, RecipeCombineScrap, sim.craftLog[1].recipe)
		assert.Equal(t, RecipeSmeltRefined, sim.craftLog[2].recipe)

		// State after Tick 1:
		// Scrap: 15 - 6 = 9 (within [3, 9])
		// Rec: 1 + 2 (from combine) + 3 (from smelt) = 6 (within [3, 9])
		// Ref: 10 - 1 = 9
		assert.Equal(t, 9, sim.GetMetalCount(DefIndexScrap), "scrap balanced")
		assert.Equal(t, 6, sim.GetMetalCount(DefIndexReclaimed), "reclaimed balanced")
		assert.Equal(t, 9, sim.GetMetalCount(DefIndexRefined), "refined updated")

		// Adversarial verification: execute a SECOND Tick immediately!
		// It MUST NOT generate any further crafts (no oscillation / ping-pong)
		err = auto.Tick(ctx)
		require.NoError(t, err)
		assert.Len(t, sim.craftLog, 3, "no new crafts on subsequent tick (stable state reached)")
	})

	t.Run("depleted_pure_stock_does_not_craft", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Inventory: 0 ref, 2 rec, 2 scrap (depleted condition: ref<=0 && rec<=3 && scrap<=3)
		sim := newStatefulSimulatedInventory(0, 2, 2)
		mgr := NewManager(sim, sim)
		auto := NewAutomator(mgr, sim)

		err := auto.Tick(ctx)
		require.NoError(t, err)
		assert.Empty(t, sim.craftLog, "depleted pure metal stock must not trigger crafts")
	})

	t.Run("excess_reclaimed_and_deficit_scrap_balanced_simultaneously", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		// Inventory: 12 rec (> 9 -> 1 combineRec), 0 scrap (< 3 -> 1 smeltRec), 5 ref
		sim := newStatefulSimulatedInventory(5, 12, 0)
		mgr := NewManager(sim, sim)
		auto := NewAutomator(mgr, sim)

		err := auto.Tick(ctx)
		require.NoError(t, err)

		// Should combine excess Rec into Ref AND smelt Rec into Scrap
		require.Len(t, sim.craftLog, 2)
		assert.Equal(t, RecipeCombineReclaimed, sim.craftLog[0].recipe)
		assert.Equal(t, RecipeSmeltReclaimed, sim.craftLog[1].recipe)

		// State after:
		// Rec: 12 - 3 (combined) - 1 (smelted) = 8 (within [3, 9])
		// Scrap: 3 (within [3, 9])
		// Ref: 5 + 1 = 6
		assert.Equal(t, 8, sim.GetMetalCount(DefIndexReclaimed))
		assert.Equal(t, 3, sim.GetMetalCount(DefIndexScrap))
		assert.Equal(t, 6, sim.GetMetalCount(DefIndexRefined))

		// Subsequent tick must generate 0 crafts
		err = auto.Tick(ctx)
		require.NoError(t, err)
		assert.Len(t, sim.craftLog, 2, "no oscillation on next tick")
	})
}
