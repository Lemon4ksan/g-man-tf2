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
