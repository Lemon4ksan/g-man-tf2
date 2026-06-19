// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type mockInventory struct {
	mock.Mock
}

func (m *mockInventory) FindCraftableItems(defIndex uint32, count int) []uint64 {
	args := m.Called(defIndex, count)
	return args.Get(0).([]uint64)
}

func (m *mockInventory) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	args := m.Called(class)
	return args.Get(0).([]*tf2.Item)
}

func (m *mockInventory) GetMetalCount(defIndex uint32) int {
	args := m.Called(defIndex)
	return args.Int(0)
}

type mockGC struct {
	mock.Mock
}

func (m *mockGC) Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error) {
	args := m.Called(ctx, items, recipe)
	return args.Get(0).([]uint64), args.Error(1)
}

func TestManager_CombineMetal(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := context.Background()
		items := []uint64{1, 2, 3}

		inv.On("FindCraftableItems", DefIndexScrap, 3).Return(items)
		gc.On("Craft", ctx, items, RecipeCombineScrap).Return([]uint64{10}, nil)

		res, err := mgr.CombineMetal(ctx, DefIndexScrap)

		assert.NoError(t, err)
		assert.Equal(t, []uint64{10}, res)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("Not_Enough", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2})

		res, err := mgr.CombineMetal(context.Background(), DefIndexScrap)

		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "not enough metal")
	})
}

func TestManager_SmeltMetal(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := context.Background()
	items := []uint64{10}

	inv.On("FindCraftableItems", DefIndexRefined, 1).Return(items)
	gc.On("Craft", ctx, items, RecipeSmeltRefined).Return([]uint64{1, 2, 3}, nil)

	res, err := mgr.SmeltMetal(ctx, DefIndexRefined)

	assert.NoError(t, err)
	assert.Equal(t, []uint64{1, 2, 3}, res)
}

func TestManager_MakeChange(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := context.Background()

	// Goal: 1 scrap. We have 0 scrap, 0 rec, 1 ref.
	// 1. Check scrap (0 < 1)
	// 2. Check rec (0 == 0) -> Call MakeChange(Rec, 1)
	// 3. MakeChange(Rec, 1):
	//    - Check rec (0 < 1)
	//    - Check ref (1 > 0) -> Smelt Refined
	// 4. After smelting ref, we have rec.
	// 5. Back to scrap loop:
	//    - Check scrap (0 < 1)
	//    - Check rec (3 > 0) -> Smelt Reclaimed
	// 6. After smelting rec, we have 3 scrap.
	// 7. Loop finishes.

	inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()

	// MakeChange(Rec, 1) starts
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(1).Once()

	inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{100})
	gc.On("Craft", mock.Anything, []uint64{100}, RecipeSmeltRefined).Return([]uint64{10, 11, 12}, nil)

	// After smelting ref, MakeChange(Rec, 1) loop checks again
	inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()

	// Back to MakeChange(Scrap, 1) loop
	inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()

	inv.On("FindCraftableItems", DefIndexReclaimed, 1).Return([]uint64{10})
	gc.On("Craft", mock.Anything, []uint64{10}, RecipeSmeltReclaimed).Return([]uint64{1, 2, 3}, nil)

	// Final check for Scrap
	inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()

	err := mgr.MakeChange(ctx, DefIndexScrap, 1)

	assert.NoError(t, err)
}

func TestCoverage_Auto_Options_Name_Run(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	logger := log.Discard
	opt := WithLogger(logger)
	a := NewAutomator(mgr, inv, opt)

	assert.Equal(t, "pure_liquidator", a.Name())

	b := bus.New()
	orch := behavior.NewOrchestrator(b, logger)
	WithPureLiquidator(orch, mgr, inv)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	inv.On("GetMetalCount", DefIndexScrap).Return(0)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0)
	inv.On("GetMetalCount", DefIndexRefined).Return(0)
	inv.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

	err := a.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCoverage_Auto_Tick_RemainingCases(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	auto := NewAutomator(mgr, inv)

	ctx := t.Context()

	// Case 1: Reclaimed supply low, smelting Refined
	inv.On("GetMetalCount", DefIndexScrap).Return(3)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(1).Once()
	inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{100}).Once()
	gc.On("Craft", ctx, []uint64{100}, RecipeSmeltRefined).Return([]uint64{10, 11, 12}, nil).Once()

	err := auto.Tick(ctx)
	assert.NoError(t, err)

	// Case 2: Too much Reclaimed, combining into Refined
	inv.On("GetMetalCount", DefIndexScrap).Return(3)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(10).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(1).Once()
	inv.On("FindCraftableItems", DefIndexReclaimed, 3).Return([]uint64{10, 11, 12}).Once()
	gc.On("Craft", ctx, []uint64{10, 11, 12}, RecipeCombineReclaimed).Return([]uint64{100}, nil).Once()

	err = auto.Tick(ctx)
	assert.NoError(t, err)
}

func TestCoverage_Auto_CleanInventory_Error(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	auto := NewAutomator(mgr, inv)

	ctx := t.Context()

	inv.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: true},
		{ID: 2, IsTradable: true},
	}).Once()

	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64(nil), errors.New("craft fail")).Once()

	inv.On("GetMetalCount", DefIndexScrap).Return(0)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0)

	err := auto.CleanInventory(ctx)
	assert.NoError(t, err)
}

func TestCoverage_Manager_Errors(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := t.Context()

	// CombineMetal: invalid defindex
	inv.On("FindCraftableItems", uint32(9999), 3).Return([]uint64{1, 2, 3}).Once()

	_, err := mgr.CombineMetal(ctx, 9999)
	assert.ErrorContains(t, err, "invalid metal defindex")

	// SmeltMetal: no metal found
	inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{}).Once()
	_, err = mgr.SmeltMetal(ctx, DefIndexRefined)
	assert.ErrorContains(t, err, "no metal found")

	// SmeltMetal: invalid defindex
	inv.On("FindCraftableItems", uint32(9999), 1).Return([]uint64{1}).Once()

	_, err = mgr.SmeltMetal(ctx, 9999)
	assert.ErrorContains(t, err, "invalid metal defindex")

	// SmeltWeapons
	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{10}, nil).Once()
	res, err := mgr.SmeltWeapons(ctx, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, []uint64{10}, res)

	// CondenseMetal scrap fails
	inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()
	inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2, 3}).Once()
	gc.On("Craft", ctx, []uint64{1, 2, 3}, RecipeCombineScrap).
		Return([]uint64(nil), errors.New("combine scrap failed")).
		Once()
	_, err = mgr.CondenseMetal(ctx)
	assert.ErrorContains(t, err, "condense scrap failed")

	// CondenseMetal reclaimed fails
	inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()
	inv.On("FindCraftableItems", DefIndexReclaimed, 3).Return([]uint64{10, 11, 12}).Once()
	gc.On("Craft", ctx, []uint64{10, 11, 12}, RecipeCombineReclaimed).
		Return([]uint64(nil), errors.New("combine rec failed")).
		Once()
	_, err = mgr.CondenseMetal(ctx)
	assert.ErrorContains(t, err, "condense reclaimed failed")
}

func TestCoverage_MakeChange_Errors(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := t.Context()

	// MakeChange default case
	inv.On("GetMetalCount", uint32(9999)).Return(0).Once()

	err := mgr.MakeChange(ctx, 9999, 1)
	assert.ErrorContains(t, err, "cannot smelt this item type")

	// MakeChange Refined missing
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

	err = mgr.MakeChange(ctx, DefIndexReclaimed, 1)
	assert.ErrorContains(t, err, "no refined metal left to smelt")
}

func TestCoverage_SmeltClassWeapons(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := t.Context()

	// Not enough weapons
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{}).Once()

	_, err := mgr.SmeltClassWeapons(ctx, "Scout")
	assert.ErrorContains(t, err, "not enough weapons")

	// Not tradable weapons
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: false},
		{ID: 2, IsTradable: true},
	}).Once()

	_, err = mgr.SmeltClassWeapons(ctx, "Scout")
	assert.ErrorContains(t, err, "refusing to smelt")

	// Success
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: true},
		{ID: 2, IsTradable: true},
	}).Once()
	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{100}, nil).Once()
	res, err := mgr.SmeltClassWeapons(ctx, "Scout")
	assert.NoError(t, err)
	assert.Equal(t, []uint64{100}, res)
}

func TestCoverage_MetalManager_SelectMetal_Errors(t *testing.T) {
	fetcher := new(mockFetcher)
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	mm := NewMetalManager(fetcher, mgr, log.Discard)

	ctx := t.Context()

	// SelectMetal with <= 0 needed
	ids, err := mm.SelectMetal(ctx, 0)
	assert.NoError(t, err)
	assert.Nil(t, ids)

	// SelectMetal MakeChange fails
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()
	inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

	_, err = mm.SelectMetal(ctx, 1)
	assert.ErrorContains(t, err, "no refined metal left to smelt")

	// SelectMetal with not enough metal remaining
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()
	inv.On("GetMetalCount", DefIndexScrap).Return(1).Once()

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

	_, err = mm.SelectMetal(ctx, 1)
	assert.ErrorContains(t, err, "not enough metal: missing 1 scrap")
}

func TestCoverage_MetalManager_SelectKeysAndMetal(t *testing.T) {
	fetcher := new(mockFetcher)
	mm := NewMetalManager(fetcher, nil, log.Discard)

	// Keys missing
	fetcher.On("GetAssetIDs", currency.SKUKey).Return([]uint64{10}).Once()

	_, err := mm.SelectKeysAndMetal(2, 0)
	assert.ErrorContains(t, err, "not enough keys")

	// SelectChange fails
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

	_, err = mm.SelectKeysAndMetal(0, 1)
	assert.ErrorIs(t, err, ErrNotEnoughChange)

	// Success
	fetcher.On("GetAssetIDs", currency.SKUKey).Return([]uint64{10, 11}).Once()
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1}).Once()

	res, err := mm.SelectKeysAndMetal(2, 1)
	assert.NoError(t, err)
	assert.Equal(t, []uint64{10, 11, 1}, res)
}

func TestCoverage_MetalManager_TryToSmeltForChange_More(t *testing.T) {
	fetcher := new(mockFetcher)
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	mm := NewMetalManager(fetcher, mgr, log.Discard)

	ctx := t.Context()

	// totalValue < needed
	fetcher.On("GetPureStock").Return(currency.PureStock{Scrap: 1}).Once()

	err := mm.TryToSmeltForChange(ctx, 2)
	assert.ErrorContains(t, err, "insufficient total metal value")

	// Smelt fails
	fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1}).Once()
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()
	inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
	inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

	err = mm.TryToSmeltForChange(ctx, 1)
	assert.ErrorContains(t, err, "smelting failed")

	// Smelt succeeds but still needs change, weapon smelting resolves it
	fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1, Scrap: 0}).Once()
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

	inv.On("GetMetalCount", DefIndexScrap).Return(1).Once()

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

	fetcher.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: true},
		{ID: 2, IsTradable: true},
	}).Once()
	fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()
	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{50}, nil).Once()

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{50}).Once()

	err = mm.TryToSmeltForChange(ctx, 1)
	assert.NoError(t, err)
}

func TestCoverage_SmeltDuplicates_Errors(t *testing.T) {
	fetcher := new(mockFetcher)
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	mm := NewMetalManager(fetcher, mgr, log.Discard)

	ctx := t.Context()

	// No duplicate weapons found to smelt
	fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

	err := mm.SmeltDuplicates(ctx, 1)
	assert.ErrorContains(t, err, "no duplicate weapons found")

	// Weapon not tradable
	fetcher.ExpectedCalls = nil
	fetcher.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: false},
		{ID: 2, IsTradable: true},
	}).Once()
	fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

	err = mm.SmeltDuplicates(ctx, 1)
	assert.ErrorContains(t, err, "refusing to smelt")

	// SmeltWeapons returns error
	fetcher.ExpectedCalls = nil
	fetcher.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1, IsTradable: true},
		{ID: 2, IsTradable: true},
	}).Once()
	fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()
	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64(nil), errors.New("craft fail")).Once()
	err = mm.SmeltDuplicates(ctx, 1)
	assert.ErrorContains(t, err, "craft fail")
}
