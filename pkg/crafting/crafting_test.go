// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Shared mock declarations for the crafting package tests
type mockInventory struct {
	mock.Mock
}

func (m *mockInventory) FindCraftableItems(defIndex uint32, count int) []uint64 {
	args := m.Called(defIndex, count)
	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).([]uint64)
}

func (m *mockInventory) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	args := m.Called(class)
	if args.Get(0) == nil {
		return nil
	}

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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]uint64), args.Error(1)
}

type mockFetcher struct {
	mock.Mock
}

func (m *mockFetcher) GetAssetIDs(sku string) []uint64 {
	args := m.Called(sku)
	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).([]uint64)
}

func (m *mockFetcher) GetPureStock() currency.PureStock {
	args := m.Called()
	return args.Get(0).(currency.PureStock)
}

func (m *mockFetcher) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	args := m.Called(class)
	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).([]*tf2.Item)
}

func (m *mockFetcher) GetMetalCount(defIndex uint32) int {
	args := m.Called(defIndex)
	return args.Int(0)
}

func TestManager_CombineMetal(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()
		items := []uint64{1, 2, 3}

		inv.On("FindCraftableItems", DefIndexScrap, 3).Return(items)
		gc.On("Craft", ctx, items, RecipeCombineScrap).Return([]uint64{10}, nil)

		res, err := mgr.CombineMetal(ctx, DefIndexScrap)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{10}, res)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("not_enough_metal", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2})

		res, err := mgr.CombineMetal(t.Context(), DefIndexScrap)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "not enough metal")
	})

	t.Run("invalid_defindex", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindCraftableItems", uint32(9999), 3).Return([]uint64{1, 2, 3})

		res, err := mgr.CombineMetal(t.Context(), 9999)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "invalid metal defindex for combination")
	})
}

func TestManager_SmeltMetal(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()
		items := []uint64{10}

		inv.On("FindCraftableItems", DefIndexRefined, 1).Return(items)
		gc.On("Craft", ctx, items, RecipeSmeltRefined).Return([]uint64{1, 2, 3}, nil)

		res, err := mgr.SmeltMetal(ctx, DefIndexRefined)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{1, 2, 3}, res)
	})

	t.Run("no_metal_found", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{})

		res, err := mgr.SmeltMetal(t.Context(), DefIndexRefined)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "no metal found")
	})

	t.Run("invalid_defindex", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindCraftableItems", uint32(9999), 1).Return([]uint64{1})

		res, err := mgr.SmeltMetal(t.Context(), 9999)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "invalid metal defindex for smelting")
	})
}

func TestManager_SmeltWeapons(t *testing.T) {
	t.Parallel()

	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	ctx := t.Context()
	gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{10}, nil)

	res, err := mgr.SmeltWeapons(ctx, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, []uint64{10}, res)
}

func TestManager_CondenseMetal(t *testing.T) {
	t.Parallel()

	t.Run("success_scrap_and_reclaimed", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()
		inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2, 3}).Once()
		gc.On("Craft", ctx, []uint64{1, 2, 3}, RecipeCombineScrap).Return([]uint64{10}, nil).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()

		inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()
		inv.On("FindCraftableItems", DefIndexReclaimed, 3).Return([]uint64{10, 11, 12}).Once()
		gc.On("Craft", ctx, []uint64{10, 11, 12}, RecipeCombineReclaimed).Return([]uint64{100}, nil).Once()

		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()

		crafts, err := mgr.CondenseMetal(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, crafts)
	})

	t.Run("scrap_combination_fails", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()
		inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2, 3}).Once()
		gc.On("Craft", ctx, []uint64{1, 2, 3}, RecipeCombineScrap).
			Return([]uint64(nil), errors.New("combine failed")).
			Once()

		crafts, err := mgr.CondenseMetal(ctx)
		assert.Error(t, err)
		assert.Equal(t, 0, crafts)
		assert.Contains(t, err.Error(), "condense scrap failed")
	})

	t.Run("reclaimed_combination_fails", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()
		inv.On("FindCraftableItems", DefIndexReclaimed, 3).Return([]uint64{10, 11, 12}).Once()
		gc.On("Craft", ctx, []uint64{10, 11, 12}, RecipeCombineReclaimed).
			Return([]uint64(nil), errors.New("combine failed")).
			Once()

		crafts, err := mgr.CondenseMetal(ctx)
		assert.Error(t, err)
		assert.Equal(t, 0, crafts)
		assert.Contains(t, err.Error(), "condense reclaimed failed")
	})
}

func TestManager_MakeChange(t *testing.T) {
	t.Parallel()

	t.Run("success_cascaded_smelting", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()

		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexRefined).Return(1).Once()

		inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{100})
		gc.On("Craft", mock.Anything, []uint64{100}, RecipeSmeltRefined).Return([]uint64{10, 11, 12}, nil)

		inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(3).Once()

		inv.On("FindCraftableItems", DefIndexReclaimed, 1).Return([]uint64{10})
		gc.On("Craft", mock.Anything, []uint64{10}, RecipeSmeltReclaimed).Return([]uint64{1, 2, 3}, nil)

		inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()

		err := mgr.MakeChange(ctx, DefIndexScrap, 1)
		assert.NoError(t, err)
	})

	t.Run("unsupported_item_type", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("GetMetalCount", uint32(9999)).Return(0)

		err := mgr.MakeChange(t.Context(), 9999, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot smelt this item type")
	})

	t.Run("no_refined_metal_remaining", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

		err := mgr.MakeChange(t.Context(), DefIndexReclaimed, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no refined metal left to smelt")
	})
}

func TestManager_SmeltClassWeapons(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)

		ctx := t.Context()

		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		})
		gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{100}, nil)

		res, err := mgr.SmeltClassWeapons(ctx, "Scout")
		assert.NoError(t, err)
		assert.Equal(t, []uint64{100}, res)
	})

	t.Run("not_enough_weapons", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{{ID: 1}})

		res, err := mgr.SmeltClassWeapons(t.Context(), "Scout")
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "not enough weapons for class")
	})

	t.Run("non_tradable_weapons_refusal", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)

		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: false}, {ID: 2, IsTradable: true},
		})

		res, err := mgr.SmeltClassWeapons(t.Context(), "Scout")
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "refusing to smelt")
	})
}
