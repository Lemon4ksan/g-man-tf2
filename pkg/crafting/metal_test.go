// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type mockFetcher struct {
	mock.Mock
}

func (m *mockFetcher) GetAssetIDs(sku string) []uint64 {
	args := m.Called(sku)
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

func TestMetalManager_SelectChange(t *testing.T) {
	fetcher := new(mockFetcher)
	mgr := NewManager(nil, nil)
	mm := NewMetalManager(fetcher, mgr, log.Discard)

	// Goal: 11 scrap (1 ref, 2 scrap)
	// We have: 2 ref, 0 rec, 5 scrap
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100, 101})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2, 3, 4, 5})

	ids, err := mm.SelectChange(11)

	assert.NoError(t, err)
	assert.Equal(t, []uint64{100, 1, 2}, ids)
}

func TestMetalManager_TryToSmeltForChange(t *testing.T) {
	t.Run("No_Smelt_Needed", func(t *testing.T) {
		fetcher := new(mockFetcher)
		mm := NewMetalManager(fetcher, nil, log.Discard)

		// Need 1 scrap. Have 1 scrap.
		fetcher.On("GetPureStock").Return(currency.PureStock{Scrap: 1})
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1})

		err := mm.TryToSmeltForChange(context.Background(), 1)

		assert.NoError(t, err)
	})

	t.Run("Triggers_Smelt", func(t *testing.T) {
		fetcher := new(mockFetcher)
		inv := new(mockInventory)
		gc := new(mockGC)
		mgr := NewManager(inv, gc)
		mm := NewMetalManager(fetcher, mgr, log.Discard)

		// Need 1 scrap. Have 1 ref, 0 rec, 0 scrap.
		fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1})

		// Initial greedy select fails to find 1 scrap
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()

		// MakeChange(Scrap, 1) flow
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

		// Final check after smelt
		fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{}).Once()
		fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{11, 12}).Once()
		fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2, 3}).Once()

		err := mm.TryToSmeltForChange(context.Background(), 1)

		assert.NoError(t, err)
	})
}
