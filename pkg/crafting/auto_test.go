// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func TestAutomator_Tick_SmeltRec(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	auto := NewAutomator(mgr, inv)

	ctx := context.Background()

	// Scrap is 0 (min is 3), Rec is 1. Should smelt Rec.
	inv.On("GetMetalCount", DefIndexScrap).Return(0)
	inv.On("GetMetalCount", DefIndexRefined).Return(1)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(1)

	inv.On("FindCraftableItems", DefIndexReclaimed, 1).Return([]uint64{10})
	gc.On("Craft", ctx, []uint64{10}, RecipeSmeltReclaimed).Return([]uint64{1, 2, 3}, nil)

	err := auto.Tick(ctx)

	assert.NoError(t, err)
	inv.AssertExpectations(t)
	gc.AssertExpectations(t)
}

func TestAutomator_Tick_CombineScrap(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	auto := NewAutomator(mgr, inv)

	ctx := context.Background()

	// Scrap is 10 (max is 9). Rec is 5 (not low). Should combine Scrap.
	inv.On("GetMetalCount", DefIndexScrap).Return(10)
	inv.On("GetMetalCount", DefIndexRefined).Return(1)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(5)

	inv.On("FindCraftableItems", DefIndexScrap, 3).Return([]uint64{1, 2, 3})
	gc.On("Craft", ctx, []uint64{1, 2, 3}, RecipeCombineScrap).Return([]uint64{10}, nil)

	err := auto.Tick(ctx)

	assert.NoError(t, err)
	inv.AssertExpectations(t)
	gc.AssertExpectations(t)
}

func TestAutomator_CleanInventory(t *testing.T) {
	inv := new(mockInventory)
	gc := new(mockGC)
	mgr := NewManager(inv, gc)
	auto := NewAutomator(mgr, inv)

	ctx := context.Background()

	// Scout weapons: 2 found. Should smelt.
	// Then loop again: 0 found. Break.
	// (And so on for other classes)
	// Finally calls CondenseMetal.

	inv.On("FindWeaponsByClassForSmelting", mock.Anything).Return([]*tf2.Item{}).Maybe()
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{
		{ID: 1}, {ID: 2},
	}).Once()
	inv.On("FindWeaponsByClassForSmelting", "Scout").Return([]*tf2.Item{}).Once()

	gc.On("Craft", mock.Anything, []uint64{1, 2}, RecipeSmeltWeapons).Return([]uint64{100}, nil)

	// CondenseMetal part
	inv.On("GetMetalCount", DefIndexScrap).Return(0)
	inv.On("GetMetalCount", DefIndexReclaimed).Return(0)

	err := auto.CleanInventory(ctx)

	assert.NoError(t, err)
}
