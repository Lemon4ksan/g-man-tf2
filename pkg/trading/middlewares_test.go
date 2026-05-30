// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// mockPartnerInvProvider is a mock for PartnerInventoryProvider interface.
type mockPartnerInvProvider struct {
	mock.Mock
}

func (m *mockPartnerInvProvider) GetPartnerInventory(ctx context.Context, partnerID id.ID) ([]*trading.Item, error) {
	args := m.Called(ctx, partnerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]*trading.Item), args.Error(1)
}

// mockAssetFetcher is a mock for crafting.AssetFetcher interface.
type mockAssetFetcher struct {
	mock.Mock
}

func (m *mockAssetFetcher) GetAssetIDs(sku string) []uint64 {
	args := m.Called(sku)
	return args.Get(0).([]uint64)
}

func (m *mockAssetFetcher) GetPureStock() currency.PureStock {
	args := m.Called()
	return args.Get(0).(currency.PureStock)
}

func (m *mockAssetFetcher) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	args := m.Called(class)
	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).([]*tf2.Item)
}

func (m *mockAssetFetcher) GetMetalCount(defIndex uint32) int {
	args := m.Called(defIndex)
	return args.Int(0)
}

// mockBackpackCache is a mock for backpack.ItemCache interface.
type mockBackpackCache struct {
	items []*tf2.Item
}

func (m *mockBackpackCache) GetItems() []*tf2.Item { return m.items }
func (m *mockBackpackCache) GetItem(id uint64) (*tf2.Item, bool) {
	for _, it := range m.items {
		if it.ID == id {
			return it, true
		}
	}

	return nil, false
}
func (m *mockBackpackCache) GetMaxSlots() int { return 3000 }

// helper to inject private fields into backpack.Backpack
func setUnexportedField(target any, fieldName string, value any) {
	val := reflect.ValueOf(target).Elem()
	field := val.FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func TestSmartCounterMiddleware_CorrectValue(t *testing.T) {
	// Prepare mocks
	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	// Context and prices setup
	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
	}
	ctx.Set("prices", prices)

	// Run middleware
	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionAccept, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)
}

func TestSmartCounterMiddleware_Overpaid_ChangeAvailable(t *testing.T) {
	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 5002, IsTradable: true}, // Refined (9 scrap)
			{ID: 11, DefIndex: 5000, IsTradable: true}, // Scrap (1 scrap)
		},
	}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	// Items setup:
	// We give 1 key (worth 50 ref / 450 scrap)
	// We receive 1 key + 10 scrap (value 460 scrap)
	// Difference is 10 scrap (partner overpaid -> we owe them 10 scrap change)
	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"5000;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
	}
	ctx.Set("prices", prices)

	// Fetcher has: 1 Ref (ID 10), 0 Rec, 1 Scrap (ID 11) -> Sum is 10 scrap
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{10})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{11})

	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Should counter offer with our change items added
	assert.Equal(t, trading.ActionCounter, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)

	counterParams := ctx.Verdict.Data.(*trading.CounterParams)
	assert.Len(t, counterParams.ItemsToGive, 3) // Original key + 1 Ref + 1 Scrap
	assert.Equal(t, "I've added the necessary change for you!", counterParams.Message)
}

func TestSmartCounterMiddleware_Overpaid_NotEnoughChange_SmeltSucceeds(t *testing.T) {
	fetcher := new(mockAssetFetcher)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	// Setup Crafting Manager Mock
	// TryToSmeltForChange returns nil (meaning smelting succeeded and will resolve on retry)
	mockCraft := new(mockGC) // Let's use a dummy crafting manager
	metalMgr := crafting.NewMetalManager(fetcher, mockCraft.mockManager(), log.Discard)

	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"5000;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
	}
	ctx.Set("prices", prices)

	// Fetcher has no loose scrap initially, but has 1 Ref in pure stock.
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})

	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})

	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).
		Return([]uint64{1, 2, 3})
		// Smelting succeeded, now we have loose scrap!

	fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1})

	// MakeChange call mock
	mockCraft.On("GetMetalCount", uint32(5000)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5002)).Return(1).Once()
	mockCraft.On("FindCraftableItems", uint32(5002), 1).Return([]uint64{100})

	mockCraft.On("Craft", mock.Anything, []uint64{100}, int16(RecipeSmeltRefined)).Return([]uint64{10, 11, 12}, nil)

	mockCraft.On("GetMetalCount", uint32(5001)).Return(3).Once()
	mockCraft.On("GetMetalCount", uint32(5000)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(3).Once()
	mockCraft.On("FindCraftableItems", uint32(5001), 1).Return([]uint64{10})

	mockCraft.On("Craft", mock.Anything, []uint64{10}, int16(RecipeSmeltReclaimed)).Return([]uint64{1, 2, 3}, nil)

	mockCraft.On("GetMetalCount", uint32(5000)).Return(3).Once()

	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Since smelting succeeded, verdict should remain undecided (waiting for next run / retry)
	assert.Equal(t, trading.ActionSkip, ctx.Verdict.Action)
}

func TestSmartCounterMiddleware_Overpaid_NotEnoughChange_SmeltFails(t *testing.T) {
	fetcher := new(mockAssetFetcher)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	mockCraft := new(mockGC)
	metalMgr := crafting.NewMetalManager(fetcher, mockCraft.mockManager(), log.Discard)

	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"5000;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
	}
	ctx.Set("prices", prices)

	// Fetcher has absolutely nothing
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{})

	fetcher.On("GetPureStock").Return(currency.PureStock{})

	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Should decline because we have no metal and cannot craft any
	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	assert.Equal(t, tf2reason.DeclineNoChange, ctx.Verdict.Reason)
}

func TestSmartCounterMiddleware_Underpaid_PartnerHasCurrency(t *testing.T) {
	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	// Items setup:
	// We give 1 key (value 450 scrap)
	// We receive 1 key minus 2 scrap (value 448 scrap)
	// Difference is -2 scrap (partner underpaid -> we extract 2 scrap from their inventory)
	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5002;6", MarketHashName: "Refined Metal"},
			{SKU: "5001;6", MarketHashName: "Reclaimed Metal"}, // 3 scrap
			{SKU: "5001;6", MarketHashName: "Reclaimed Metal"}, // 3 scrap
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},     // 1 scrap
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"5002;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 1.00},
			Sell: pricedb.Currencies{Keys: 0, Metal: 1.00},
		},
		"5001;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.33},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.33},
		},
		"5000;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
	}
	ctx.Set("prices", prices)

	// Partner inventory mock (has loose scrap we can extract)
	partnerItems := []*trading.Item{
		{AssetID: 888, SKU: "5000;6", MarketHashName: "Scrap Metal"},
		{AssetID: 889, SKU: "5000;6", MarketHashName: "Scrap Metal"},
	}
	invProvider.On("GetPartnerInventory", mock.Anything, offer.OtherSteamID).Return(partnerItems, nil)

	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Should counter offer adding the 2 missing scrap items from their inventory
	assert.Equal(t, trading.ActionCounter, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)

	counterParams := ctx.Verdict.Data.(*trading.CounterParams)
	assert.Len(t, counterParams.ItemsToReceive, len(offer.ItemsToReceive)+2)
	assert.Equal(t, "You were missing some change, I've added it for you!", counterParams.Message)
}

func TestSmartCounterMiddleware_Underpaid_PartnerMissingCurrency(t *testing.T) {
	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
		},
	}

	ctx := engine.NewTradeContext(context.Background(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"5000;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
	}
	ctx.Set("prices", prices)

	// Partner inventory mock (no metal or currency at all)
	partnerItems := []*trading.Item{
		{AssetID: 999, SKU: "123;6", MarketHashName: "Some Weapon"},
	}
	invProvider.On("GetPartnerInventory", mock.Anything, offer.OtherSteamID).Return(partnerItems, nil)

	mw := SmartCounterMiddleware(metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Should decline because partner does not have the missing change
	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	assert.Equal(t, tf2reason.DeclineUnderpaid, ctx.Verdict.Reason)
}

// Helpers for mock GC and crafting recipes
const (
	RecipeSmeltRefined   = 23
	RecipeSmeltReclaimed = 22
	DefIndexScrap        = 5000
	DefIndexReclaimed    = 5001
	DefIndexRefined      = 5002
)

type mockGC struct {
	mock.Mock
}

func (m *mockGC) Craft(ctx context.Context, ids []uint64, recipe int16) ([]uint64, error) {
	args := m.Called(ctx, ids, recipe)
	return args.Get(0).([]uint64), args.Error(1)
}

func (m *mockGC) mockManager() *crafting.Manager {
	mgr := crafting.NewManager(m, m)
	return mgr
}

func (m *mockGC) GetMetalCount(defIndex uint32) int {
	args := m.Called(defIndex)
	return args.Int(0)
}

func (m *mockGC) FindCraftableItems(defIndex uint32, count int) []uint64 {
	args := m.Called(defIndex, count)
	return args.Get(0).([]uint64)
}

func (m *mockGC) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	return nil
}
