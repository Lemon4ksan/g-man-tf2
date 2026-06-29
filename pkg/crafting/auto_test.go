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

	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func TestAutomator_Tick(t *testing.T) {
	t.Parallel()

	t.Run("smelt_reclaimed_when_scrap_low", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(0)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(1)
		inv.On("GetMetalCount", DefIndexRefined).Return(1)

		inv.On("FindCraftableItems", DefIndexReclaimed, 1).Return([]uint64{10})
		gc.On("Craft", ctx, []uint64{10}, RecipeSmeltReclaimed).Return([]uint64{1, 2, 3}, nil)

		err := auto.Tick(ctx)
		assert.NoError(t, err)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("smelt_refined_when_reclaimed_low", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(3)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0)
		inv.On("GetMetalCount", DefIndexRefined).Return(1)

		inv.On("FindCraftableItems", DefIndexRefined, 1).Return([]uint64{100})
		gc.On("Craft", ctx, []uint64{100}, RecipeSmeltRefined).Return([]uint64{10, 11, 12}, nil)

		err := auto.Tick(ctx)
		assert.NoError(t, err)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("combine_scrap_when_scrap_high", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(10)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(5)
		inv.On("GetMetalCount", DefIndexRefined).Return(1)

		inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2, 3})
		gc.On("Craft", ctx, []uint64{1, 2, 3}, RecipeCombineScrap).Return([]uint64{10}, nil)

		err := auto.Tick(ctx)
		assert.NoError(t, err)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("combine_reclaimed_when_reclaimed_high", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(3)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(10)
		inv.On("GetMetalCount", DefIndexRefined).Return(1)

		inv.On("FindCraftableItems", DefIndexReclaimed, 3).Return([]uint64{10, 11, 12})
		gc.On("Craft", ctx, []uint64{10, 11, 12}, RecipeCombineReclaimed).Return([]uint64{100}, nil)

		err := auto.Tick(ctx)
		assert.NoError(t, err)
		inv.AssertExpectations(t)
		gc.AssertExpectations(t)
	})

	t.Run("no_action_when_balanced", func(t *testing.T) {
		inv := new(mockInventory)
		mgr := NewManager(inv, nil)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("GetMetalCount", DefIndexScrap).Return(5)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(5)
		inv.On("GetMetalCount", DefIndexRefined).Return(5)

		err := auto.Tick(ctx)
		assert.NoError(t, err)
		inv.AssertExpectations(t)
	})
}

func TestAutomator_CleanInventory(t *testing.T) {
	t.Parallel()

	t.Run("smelt_weapons_success", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		}).Once()
		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		}).Once()
		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{}).Once()

		gc.On("Craft", mock.Anything, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{100}, nil).Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0)

		err := auto.CleanInventory(ctx)
		assert.NoError(t, err)
	})

	t.Run("smelt_weapons_fail", func(t *testing.T) {
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		auto := NewAutomator(mgr, inv)

		ctx := t.Context()

		inv.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()

		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		}).Once()
		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
			{ID: 1, IsTradable: true}, {ID: 2, IsTradable: true},
		}).Once()
		inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{}).Once()

		gc.On("Craft", mock.Anything, []uint64{1, 2}, RecipeSmeltWeapons).
			Return([]uint64(nil), errors.New("craft failed")).
			Once()

		inv.On("GetMetalCount", DefIndexScrap).Return(0)
		inv.On("GetMetalCount", DefIndexReclaimed).Return(0)

		err := auto.CleanInventory(ctx)
		assert.NoError(t, err)
	})
}

func TestAutomator_RunAndRegistration(t *testing.T) {
	t.Parallel()

	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)

	logger := log.Discard
	a := NewAutomator(mgr, inv, WithLogger(logger))

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
