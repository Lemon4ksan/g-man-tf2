// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func TestMetalManager_SelectMetal(t *testing.T) {
	t.Parallel()

	t.Run("needed_scrap_zero_or_negative", func(t *testing.T) {
		mm := NewMetalManager(nil, nil, log.Discard)
		ids, err := mm.SelectMetal(t.Context(), 0)
		assert.NoError(t, err)
		assert.Nil(t, ids)
	})

	t.Run("cascaded_smelting_triggered_and_succeeds", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		ctx := t.Context()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

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

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{11, 12}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2, 3}).Once()

		ids, err := mm.SelectMetal(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{1}, ids)
	})

	t.Run("make_change_fails_returns_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

		ids, err := mm.SelectMetal(t.Context(), 1)
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "no refined metal left to smelt")
	})

	t.Run("insufficient_balance_after_make_change_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		ctx := t.Context()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(1).Once()
		inv.On("FindCraftableItems", DefIndexReclaimed, 1).Return([]uint64{10}).Once()
		gc.On("Craft", ctx, []uint64{10}, RecipeSmeltReclaimed).Return([]uint64{1, 2, 3}, nil).Once()
		inv.On("GetMetalCount", DefIndexScrap).Return(3).Once()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		ids, err := mm.SelectMetal(ctx, 10)
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "not enough metal: missing 10 scrap")
	})
}

func TestMetalManager_SelectChange(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100, 101})
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2, 3, 4, 5})

		ids, err := mm.SelectChange(11)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{100, 1, 2}, ids)
	})

	t.Run("insufficient_change_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1})

		ids, err := mm.SelectChange(5)
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.ErrorIs(t, err, ErrNotEnoughChange)
	})
}

func TestMetalManager_SelectKeysAndMetal(t *testing.T) {
	t.Parallel()

	t.Run("success_keys_and_change", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKUKey).Return([]uint64{10, 11, 12}).Once()
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2}).Once()

		ids, err := mm.SelectKeysAndMetal(2, 2)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{10, 11, 1, 2}, ids)
	})

	t.Run("not_enough_keys_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKUKey).Return([]uint64{10}).Once()

		_, err := mm.SelectKeysAndMetal(2, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enough keys")
	})

	t.Run("select_change_fails_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetAssetIDs", currency.SKUKey).Return([]uint64{10}).Once()
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		_, err := mm.SelectKeysAndMetal(1, 1)
		assert.ErrorIs(t, err, ErrNotEnoughChange)
	})
}

func TestMetalManager_TryToSmeltForChange(t *testing.T) {
	t.Parallel()

	t.Run("insufficient_total_metal_value_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("GetPureStock").Return(currency.PureStock{Scrap: 1}).Once()

		err := mm.TryToSmeltForChange(t.Context(), 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient total metal value")
	})

	t.Run("make_change_fails_returns_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		ctx := t.Context()

		fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1}).Once()
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0).Once()
		inv.On("GetMetalCount", DefIndexRefined).Return(0).Once()

		err := mm.TryToSmeltForChange(ctx, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "smelting failed")
	})

	t.Run("smelt_duplicates_fallback_resolves_change", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		ctx := t.Context()

		fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1, Scrap: 0}).Once()
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(1).Once()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		fetcher.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		}).Once()
		fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()
		gc.On("Craft", ctx, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{50}, nil).Once()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{50}).Once()

		err := mm.TryToSmeltForChange(ctx, 1)
		assert.NoError(t, err)
	})

	t.Run("smelt_duplicates_unresolved_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		ctx := t.Context()

		fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1}).Once()
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(1).Once()

		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

		err := mm.TryToSmeltForChange(ctx, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "smelting didn't resolve the change problem")
	})
}

func TestMetalManager_SmeltDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("no_duplicates_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

		err := mm.SmeltDuplicates(t.Context(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no duplicate weapons found to smelt")
	})

	t.Run("weapon_not_tradable_error", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		fetcher.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: false}, {ID: 2, IsTradable: true},
		}).Once()
		fetcher.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

		err := mm.SmeltDuplicates(t.Context(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to smelt")
	})
}
