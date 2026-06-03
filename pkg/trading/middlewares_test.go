// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"errors"
	"path/filepath"
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
	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/rep"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

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

type mockGC struct {
	mock.Mock
}

func (m *mockGC) FindCraftableItems(defIndex uint32, count int) []uint64 {
	args := m.Called(defIndex, count)
	return args.Get(0).([]uint64)
}

func (m *mockGC) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	args := m.Called(class)
	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).([]*tf2.Item)
}

func (m *mockGC) GetMetalCount(defIndex uint32) int {
	args := m.Called(defIndex)
	return args.Int(0)
}

func (m *mockGC) Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error) {
	args := m.Called(ctx, items, recipe)
	return args.Get(0).([]uint64), args.Error(1)
}

func (m *mockGC) mockManager() *crafting.Manager {
	return crafting.NewManager(m, m)
}

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

func setUnexportedField(target any, fieldName string, value any) {
	val := reflect.ValueOf(target).Elem()
	field := val.FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func TestSmartCounterMiddleware_CorrectValue_ExactMatch_AcceptsOffer(t *testing.T) {
	t.Parallel()

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
			{SKU: "5021;6", MarketHashName: "Mann Co. Supply Crate Key"},
		},
	}

	ctx := engine.NewTradeContext(t.Context(), offer)
	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
	}
	ctx.Set("prices", prices)

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionAccept, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)
}

func TestSmartCounterMiddleware_Overpaid_ChangeAvailable_BotWePayChange_CountersOffer(t *testing.T) {
	t.Parallel()

	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 5002, IsTradable: true},
			{ID: 11, DefIndex: 5000, IsTradable: true},
		},
	}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

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

	ctx := engine.NewTradeContext(t.Context(), offer)
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

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{10})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{11})

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	assert.Equal(t, trading.ActionCounter, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)

	counterParams := ctx.Verdict.Data.(*trading.CounterParams)
	assert.Len(t, counterParams.ItemsToGive, 3)
}

func TestSmartCounterMiddleware_Overpaid_NotEnoughChange_SmeltSucceeds_RetriesOnNextRun(t *testing.T) {
	t.Parallel()

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

	ctx := engine.NewTradeContext(t.Context(), offer)
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

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{100}).Once()
	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{}).Once()
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{1, 2, 3})
	fetcher.On("GetPureStock").Return(currency.PureStock{Refined: 1})

	mockCraft.On("GetMetalCount", uint32(5000)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5002)).Return(1).Once()
	mockCraft.On("FindCraftableItems", uint32(5002), 1).Return([]uint64{100})
	mockCraft.On("Craft", mock.Anything, []uint64{100}, int16(crafting.RecipeSmeltRefined)).
		Return([]uint64{10, 11, 12}, nil)
	mockCraft.On("GetMetalCount", uint32(5001)).Return(3).Once()
	mockCraft.On("GetMetalCount", uint32(5000)).Return(0).Once()
	mockCraft.On("GetMetalCount", uint32(5001)).Return(3).Once()
	mockCraft.On("FindCraftableItems", uint32(5001), 1).Return([]uint64{10})
	mockCraft.On("Craft", mock.Anything, []uint64{10}, int16(crafting.RecipeSmeltReclaimed)).
		Return([]uint64{1, 2, 3}, nil)
	mockCraft.On("GetMetalCount", uint32(5000)).Return(3).Once()

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
}

func TestSmartCounterMiddleware_Overpaid_NotEnoughChange_SmeltFails_DeclinesOffer(t *testing.T) {
	t.Parallel()

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

	ctx := engine.NewTradeContext(t.Context(), offer)
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

	fetcher.On("GetAssetIDs", currency.SKURefined).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUReclaimed).Return([]uint64{})
	fetcher.On("GetAssetIDs", currency.SKUScrap).Return([]uint64{})
	fetcher.On("GetPureStock").Return(currency.PureStock{})

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	assert.Equal(t, tf2reason.DeclineNoChange, ctx.Verdict.Reason)
}

func TestSmartCounterMiddleware_Underpaid_PartnerHasCurrency_PartnerUnderpaid_CountersOffer(t *testing.T) {
	t.Parallel()

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
			{SKU: "5001;6", MarketHashName: "Reclaimed Metal"},
			{SKU: "5001;6", MarketHashName: "Reclaimed Metal"},
			{SKU: "5000;6", MarketHashName: "Scrap Metal"},
		},
	}

	ctx := engine.NewTradeContext(t.Context(), offer)
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

	partnerItems := []*trading.Item{
		{AssetID: 888, SKU: "5000;6", MarketHashName: "Scrap Metal"},
		{AssetID: 889, SKU: "5000;6", MarketHashName: "Scrap Metal"},
	}
	invProvider.On("GetPartnerInventory", mock.Anything, offer.OtherSteamID).Return(partnerItems, nil)

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	assert.Equal(t, trading.ActionCounter, ctx.Verdict.Action)
	assert.Equal(t, reason.AcceptCorrectValue, ctx.Verdict.Reason)

	counterParams := ctx.Verdict.Data.(*trading.CounterParams)
	assert.Len(t, counterParams.ItemsToReceive, len(offer.ItemsToReceive)+2)
}

func TestSmartCounterMiddleware_Underpaid_PartnerMissingCurrency_PartnerUnderpaidNoChange_DeclinesOffer(t *testing.T) {
	t.Parallel()

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

	ctx := engine.NewTradeContext(t.Context(), offer)
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

	partnerItems := []*trading.Item{
		{AssetID: 999, SKU: "123;6", MarketHashName: "Some Weapon"},
	}
	invProvider.On("GetPartnerInventory", mock.Anything, offer.OtherSteamID).Return(partnerItems, nil)

	mw := SmartCounterMiddleware(nil, metalMgr, bp, invProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	assert.Equal(t, tf2reason.DeclineUnderpaid, ctx.Verdict.Reason)
}

type mockPriceProvider struct {
	mock.Mock
}

func (m *mockPriceProvider) GetPrice(sku string) (*pricedb.Price, bool) {
	args := m.Called(sku)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}

	return args.Get(0).(*pricedb.Price), args.Bool(1)
}

func (m *mockPriceProvider) Watch(sku string) {
	m.Called(sku)
}

func (m *mockPriceProvider) Fetch(ctx context.Context, skus []string) (map[string]*pricedb.Price, error) {
	args := m.Called(ctx, skus)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(map[string]*pricedb.Price), args.Error(1)
}

type mockDupeChecker struct {
	mock.Mock
}

func (m *mockDupeChecker) CheckHistory(ctx context.Context, assetID uint64) (backpack.HistoryStatus, error) {
	args := m.Called(ctx, assetID)
	return args.Get(0).(backpack.HistoryStatus), args.Error(1)
}

type mockReputationChecker struct {
	mock.Mock
}

func (m *mockReputationChecker) CheckBans(ctx context.Context, partnerID id.ID) (*rep.BanResult, error) {
	args := m.Called(ctx, partnerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*rep.BanResult), args.Error(1)
}

func TestMiddlewares_Pricer(t *testing.T) {
	t.Parallel()

	// 1. Success case: resolves all prices directly
	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		mgr := new(mockPriceProvider)
		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "5021;6"},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		mgr.On("GetPrice", "5021;6").Return(&pricedb.Price{SKU: "5021;6"}, true)

		mw := PricerMiddleware(mgr, func() *schema.Schema { return nil }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionSkip, ctx.Verdict.Action)
	})

	// 2. Fetch failure case: price manager fails to fetch uncached items
	t.Run("Fetch_Failure", func(t *testing.T) {
		t.Parallel()

		mgr := new(mockPriceProvider)
		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "5021;6"},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		mgr.On("GetPrice", "5021;6").Return(nil, false)
		mgr.On("Watch", "5021;6").Return()
		mgr.On("Fetch", mock.Anything, []string{"5021;6"}).Return(nil, errors.New("pricedb error"))

		mw := PricerMiddleware(mgr, func() *schema.Schema { return nil }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.Error(t, err)
		assert.Equal(t, tf2reason.ReviewPricerDown, ctx.Verdict.Reason)
	})

	// 3. Unpriced item case: Fetch succeeds but the item is still not priced in priceMap
	t.Run("Unpriced_Item", func(t *testing.T) {
		t.Parallel()

		mgr := new(mockPriceProvider)
		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "5021;6"},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		mgr.On("GetPrice", "5021;6").Return(nil, false)
		mgr.On("Watch", "5021;6").Return()
		mgr.On("Fetch", mock.Anything, []string{"5021;6"}).Return(map[string]*pricedb.Price{}, nil)

		mw := PricerMiddleware(mgr, func() *schema.Schema { return nil }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.Error(t, err)
		assert.Equal(t, tf2reason.ReviewUnpricedItem, ctx.Verdict.Reason)
	})

	// 4. Weapon currency case: item is a unique weapon, bypasses price check and gets accepted
	t.Run("Weapon_Currency", func(t *testing.T) {
		t.Parallel()

		mgr := new(mockPriceProvider)
		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "199;6"},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		mgr.On("GetPrice", "199;6").Return(nil, false)
		mgr.On("Watch", "199;6").Return()
		mgr.On("Fetch", mock.Anything, []string{"199;6"}).Return(map[string]*pricedb.Price{}, nil)

		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{Defindex: 199, CraftClass: "weapon"},
		}
		mockSchema := schema.New(raw)

		mw := PricerMiddleware(mgr, func() *schema.Schema { return mockSchema }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
	})
}

func TestMiddlewares_DupeCheck(t *testing.T) {
	t.Parallel()

	t.Run("No_Unusuals", func(t *testing.T) {
		t.Parallel()

		checker := new(mockDupeChecker)
		offer := &trading.TradeOffer{
			ItemsToReceive: []*trading.Item{
				{SKU: "5021;6", AssetID: 100},
				{SKU: ""},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)
		mw := DupeCheckMiddleware(checker, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
	})

	t.Run("Checker_Error", func(t *testing.T) {
		t.Parallel()

		checker := new(mockDupeChecker)
		offer := &trading.TradeOffer{
			ItemsToReceive: []*trading.Item{
				{SKU: "378;5;u13", AssetID: 100},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		checker.On("CheckHistory", mock.Anything, uint64(100)).
			Return(backpack.HistoryStatus{}, errors.New("backpack.tf error"))

		mw := DupeCheckMiddleware(checker, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionSkip, ctx.Verdict.Action)
	})

	t.Run("Item_Duped", func(t *testing.T) {
		t.Parallel()

		checker := new(mockDupeChecker)
		offer := &trading.TradeOffer{
			ItemsToReceive: []*trading.Item{
				{SKU: "378;5;u13", AssetID: 100},
			},
		}

		ctx := engine.NewTradeContext(t.Context(), offer)

		checker.On("CheckHistory", mock.Anything, uint64(100)).
			Return(backpack.HistoryStatus{Recorded: true, IsDuped: true}, nil)

		mw := DupeCheckMiddleware(checker, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, tf2reason.ReviewDupedItems, ctx.Verdict.Reason)
	})
}

func TestMiddlewares_BanCheck(t *testing.T) {
	t.Parallel()

	t.Run("Not_Banned", func(t *testing.T) {
		t.Parallel()

		bans := new(mockReputationChecker)
		offer := &trading.TradeOffer{
			OtherSteamID: 76561198000000000,
		}

		ctx := engine.NewTradeContext(t.Context(), offer)
		bans.On("CheckBans", mock.Anything, offer.OtherSteamID).Return(&rep.BanResult{IsBanned: false}, nil)

		mw := BanCheckMiddleware(bans, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
	})

	t.Run("Checker_Error", func(t *testing.T) {
		t.Parallel()

		bans := new(mockReputationChecker)
		offer := &trading.TradeOffer{
			OtherSteamID: 76561198000000000,
		}

		ctx := engine.NewTradeContext(t.Context(), offer)
		bans.On("CheckBans", mock.Anything, offer.OtherSteamID).Return(nil, errors.New("ban check failed"))

		mw := BanCheckMiddleware(bans, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
	})

	t.Run("SteamRep_Banned", func(t *testing.T) {
		t.Parallel()

		bans := new(mockReputationChecker)
		offer := &trading.TradeOffer{
			OtherSteamID: 76561198000000000,
		}

		ctx := engine.NewTradeContext(t.Context(), offer)
		bans.On("CheckBans", mock.Anything, offer.OtherSteamID).Return(&rep.BanResult{
			IsBanned: true,
			Details: map[string]string{
				"steamrep.com": "scammer",
			},
		}, nil)

		mw := BanCheckMiddleware(bans, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, reason.DeclineBanned, ctx.Verdict.Reason)
	})

	t.Run("BackpackTF_Banned", func(t *testing.T) {
		t.Parallel()

		bans := new(mockReputationChecker)
		offer := &trading.TradeOffer{
			OtherSteamID: 76561198000000000,
		}

		ctx := engine.NewTradeContext(t.Context(), offer)
		bans.On("CheckBans", mock.Anything, offer.OtherSteamID).Return(&rep.BanResult{
			IsBanned: true,
			Details: map[string]string{
				"backpack.tf": "banned",
			},
		}, nil)

		mw := BanCheckMiddleware(bans, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, tf2reason.DeclineBannedBptf, ctx.Verdict.Reason)
	})
}

func TestMiddlewares_IsUnusual(t *testing.T) {
	t.Parallel()

	assert.True(t, isUnusual("378;5;u13"))
	assert.False(t, isUnusual("5021;6"))
	assert.False(t, isUnusual("invalid"))
}

func TestMiddlewares_FindPartnerCurrency_Backtracking(t *testing.T) {
	t.Parallel()

	items1 := []*trading.Item{
		{MarketHashName: "Refined Metal"},
	}
	_, ok1 := FindPartnerCurrency(items1, 5, 0, nil)
	assert.False(t, ok1)

	items2 := []*trading.Item{
		{MarketHashName: "Refined Metal"},
		{MarketHashName: "Reclaimed Metal"},
		{MarketHashName: "Reclaimed Metal"},
	}
	res2, ok2 := FindPartnerCurrency(items2, 15, 0, nil)
	assert.True(t, ok2)
	assert.Len(t, res2, 3)
}

func TestSmartCounterMiddleware_SeparateKeyRates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "trading_config.json")
	cfgManager, err := NewConfigManager(cfgPath)
	assert.NoError(t, err)

	fetcher := new(mockAssetFetcher)
	metalMgr := crafting.NewMetalManager(fetcher, nil, log.Discard)
	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	invProvider := new(mockPartnerInvProvider)

	offer := &trading.TradeOffer{
		OtherSteamID: 76561198000000000,
		ItemsToGive: []*trading.Item{
			{SKU: "123;6", MarketHashName: "Some Key-priced Item"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "123;6", MarketHashName: "Some Key-priced Item"},
		},
	}

	prices := map[string]*pricedb.Price{
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 61.0},
		},
		"123;6": {
			Buy:  pricedb.Currencies{Keys: 1, Metal: 0.0},
			Sell: pricedb.Currencies{Keys: 1, Metal: 0.0},
		},
	}

	t.Run("UseSeparateKeyRates is false - both keys valued equally", func(t *testing.T) {
		cfg := cfgManager.GetConfig()
		cfg.UseSeparateKeyRates = false
		setUnexportedField(cfgManager, "cfg", cfg)

		ctx := engine.NewTradeContext(t.Context(), offer)
		ctx.Set("prices", prices)

		mw := SmartCounterMiddleware(cfgManager, metalMgr, bp, invProvider, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionAccept, ctx.Verdict.Action)
	})

	t.Run("UseSeparateKeyRates is true - gives are valued at sell, receives at buy", func(t *testing.T) {
		cfg := cfgManager.GetConfig()
		cfg.UseSeparateKeyRates = true
		setUnexportedField(cfgManager, "cfg", cfg)

		ctx := engine.NewTradeContext(t.Context(), offer)
		ctx.Set("prices", prices)

		invProvider.On("GetPartnerInventory", mock.Anything, offer.OtherSteamID).Return([]*trading.Item{}, nil).Once()

		mw := SmartCounterMiddleware(cfgManager, metalMgr, bp, invProvider, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
		assert.Equal(t, tf2reason.DeclineUnderpaid, ctx.Verdict.Reason)
	})
}

type mockSpellPredictor struct {
	mock.Mock
}

func (m *mockSpellPredictor) PredictSpellPrice(
	ctx context.Context,
	spells, item string,
) (*pricedb.SpellPredictionResponse, error) {
	args := m.Called(ctx, spells, item)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*pricedb.SpellPredictionResponse), args.Error(1)
}

func TestHalloweenSpellMiddleware(t *testing.T) {
	t.Parallel()

	sh := schema.New(&schema.Raw{})

	predictor := &mockSpellPredictor{}

	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{
				SKU: "205;11",
				Descriptions: []trading.Description{
					{
						Value: "Exorcism",
						Color: "7ea9d1",
					},
				},
			},
		},
		ItemsToReceive: []*trading.Item{
			{
				SKU: "205;6",
				Descriptions: []trading.Description{
					{
						Value: "Voices from Below",
						Color: "7ea9d1",
					},
				},
			},
		},
	}

	prices := map[string]*pricedb.Price{
		"205;11": {
			SKU:  "205;11",
			Name: "Strange Rocket Launcher",
			Buy:  pricedb.Currencies{Metal: 20.0},
			Sell: pricedb.Currencies{Metal: 22.0},
		},
		"205;6": {
			SKU:  "205;6",
			Name: "Rocket Launcher",
			Buy:  pricedb.Currencies{Metal: 1.0},
			Sell: pricedb.Currencies{Metal: 1.1},
		},
	}

	predGive := &pricedb.SpellPredictionResponse{
		ItemName: "Strange Rocket Launcher",
		Spells:   []string{"Exorcism"},
	}
	predGive.PremiumRanges = map[string]struct {
		Ref       float64 `json:"ref"`
		Formatted string  `json:"formatted"`
	}{
		"mid": {Ref: 5.0, Formatted: "5 ref"},
	}

	predRecv := &pricedb.SpellPredictionResponse{
		ItemName: "Rocket Launcher",
		Spells:   []string{"Voices from Below"},
	}
	predRecv.PremiumRanges = map[string]struct {
		Ref       float64 `json:"ref"`
		Formatted string  `json:"formatted"`
	}{
		"mid": {Ref: 8.0, Formatted: "8 ref"},
	}

	predictor.On("PredictSpellPrice", mock.Anything, "Exorcism", "Strange Rocket Launcher").Return(predGive, nil).Once()
	predictor.On("PredictSpellPrice", mock.Anything, "Voices from Below", "Rocket Launcher").
		Return(predRecv, nil).
		Once()

	ctx := engine.NewTradeContext(t.Context(), offer)
	ctx.Set("prices", prices)

	mockCfgProvider := func() Config {
		return Config{
			FallbackSpellPremiums: map[string]float64{
				"Exorcism":          3.0,
				"Voices from Below": 10.0,
			},
		}
	}

	mw := HalloweenSpellMiddleware(predictor, func() *schema.Schema { return sh }, mockCfgProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	ourVal, okOur := ctx.Get("our_spell_premium_scrap")
	assert.True(t, okOur)
	assert.Equal(t, currency.Scrap(45), ourVal.(currency.Scrap))

	theirVal, okTheir := ctx.Get("their_spell_premium_scrap")
	assert.True(t, okTheir)
	assert.Equal(t, currency.Scrap(72), theirVal.(currency.Scrap))

	diff, err := calculateValueDiff(ctx, false)
	assert.NoError(t, err)

	assert.Equal(t, currency.Scrap(-162), diff)

	predictor.AssertExpectations(t)
}

func TestHalloweenSpellMiddleware_Fallback(t *testing.T) {
	t.Parallel()

	sh := schema.New(&schema.Raw{})
	predictor := &mockSpellPredictor{}

	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{
				SKU: "205;11",
				Descriptions: []trading.Description{
					{
						Value: "Exorcism",
						Color: "7ea9d1",
					},
				},
			},
		},
	}

	prices := map[string]*pricedb.Price{
		"205;11": {
			SKU:  "205;11",
			Name: "Strange Rocket Launcher",
			Buy:  pricedb.Currencies{Metal: 20.0},
			Sell: pricedb.Currencies{Metal: 22.0},
		},
	}

	// PredictSpellPrice fails, triggering fallback
	predictor.On("PredictSpellPrice", mock.Anything, "Exorcism", "Strange Rocket Launcher").
		Return(nil, errors.New("pricedb down")).Once()

	ctx := engine.NewTradeContext(t.Context(), offer)
	ctx.Set("prices", prices)

	mockCfgProvider := func() Config {
		return Config{
			FallbackSpellPremiums: map[string]float64{
				"Exorcism": 4.0, // 4.0 ref fallback
			},
		}
	}

	mw := HalloweenSpellMiddleware(predictor, func() *schema.Schema { return sh }, mockCfgProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	ourVal, okOur := ctx.Get("our_spell_premium_scrap")
	assert.True(t, okOur)
	// Exorcism has 4.0 ref premium = 36 scrap
	assert.Equal(t, currency.Scrap(36), ourVal.(currency.Scrap))

	predictor.AssertExpectations(t)
}

func TestCalculateValueDiff_WeaponCurrency(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 199, CraftClass: "weapon"},
		{Defindex: 200, CraftClass: "weapon"},
	}
	sh := schema.New(raw)

	prices := map[string]*pricedb.Price{
		"5021;6": { // Priced item, e.g. Reclaimed Metal worth 3 scrap
			SKU:  "5021;6",
			Name: "Reclaimed Metal",
			Buy:  pricedb.Currencies{Metal: 0.33},
			Sell: pricedb.Currencies{Metal: 0.33},
		},
	}

	t.Run("Weapon_as_change_floor_rounding", func(t *testing.T) {
		t.Parallel()

		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "5021;6"}, // 3 scrap
				{SKU: "199;6"},  // 0.5 scrap (Unique weapon, unpriced)
			},
			ItemsToReceive: []*trading.Item{
				{SKU: "5021;6"}, // 3 scrap
				{SKU: "5021;6"}, // 3 scrap
				{SKU: "199;6"},  // 0.5 scrap (Unique weapon, unpriced)
				{SKU: "200;6"},  // 0.5 scrap (Unique weapon, unpriced)
			},
		}
		// Give: 3.5 scrap
		// Receive: 7.0 scrap
		// Diff: 7.0 - 3.5 = 3.5 scrap. Floored: 3 scrap.

		ctx := engine.NewTradeContext(t.Context(), offer)
		ctx.Set("schema", sh)
		ctx.Set("prices", prices)

		diff, err := calculateValueDiff(ctx, false)
		assert.NoError(t, err)
		assert.Equal(t, currency.Scrap(3), diff)

		isProfitable, ok := ctx.Get("is_profitable")
		assert.True(t, ok)
		assert.True(t, isProfitable.(bool))
	})

	t.Run("Underpay_by_half_scrap_is_profitable_zero", func(t *testing.T) {
		t.Parallel()

		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "5021;6"}, // 3 scrap
			},
			ItemsToReceive: []*trading.Item{
				{SKU: "5021;6"}, // 3 scrap
				{SKU: "199;6"},  // 0.5 scrap
			},
		}
		// Give: 3 scrap
		// Receive: 3.5 scrap
		// Diff: 3.5 - 3.0 = 0.5 scrap. Floored: 0 scrap.
		// Since we round down in our favor, we treat 0.5 scrap overpayment as 0 scrap net difference, which is acceptable/profitable.

		ctx := engine.NewTradeContext(t.Context(), offer)
		ctx.Set("schema", sh)
		ctx.Set("prices", prices)

		diff, err := calculateValueDiff(ctx, false)
		assert.NoError(t, err)
		assert.Equal(t, currency.Scrap(0), diff)

		isProfitable, ok := ctx.Get("is_profitable")
		assert.True(t, ok)
		assert.True(t, isProfitable.(bool))
	})
}

func TestFindPartnerCurrency_WithWeapons(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 199, CraftClass: "weapon"},
		{Defindex: 200, CraftClass: "weapon"},
		{Defindex: 201, CraftClass: "weapon"},
	}
	sh := schema.New(raw)

	t.Run("Pull_two_weapons_for_one_scrap", func(t *testing.T) {
		t.Parallel()

		items := []*trading.Item{
			{SKU: "199;6", MarketHashName: "Shotgun"},
			{SKU: "200;6", MarketHashName: "Pistol"},
			{SKU: "201;6", MarketHashName: "Fists"},
		}

		res, ok := FindPartnerCurrency(items, 1, 0, sh)
		assert.True(t, ok)
		assert.Len(t, res, 2)
		// Should pull two weapons
		assert.Contains(t, []string{"199;6", "200;6"}, res[0].SKU)
		assert.Contains(t, []string{"199;6", "200;6"}, res[1].SKU)
	})

	t.Run("Fail_not_enough_weapons", func(t *testing.T) {
		t.Parallel()

		items := []*trading.Item{
			{SKU: "199;6", MarketHashName: "Shotgun"},
			{SKU: "200;6", MarketHashName: "Pistol"},
			{SKU: "201;6", MarketHashName: "Fists"},
		}

		// Needs 2 scrap (= 4 weapons), but they only have 3 weapons
		_, ok := FindPartnerCurrency(items, 2, 0, sh)
		assert.False(t, ok)
	})
}

func TestPricerMiddleware_PaintedItemFallback(t *testing.T) {
	t.Parallel()

	mgr := new(mockPriceProvider)
	offer := &trading.TradeOffer{
		ItemsToReceive: []*trading.Item{
			{SKU: "211;6;p5027"}, // Painted item not explicitly in pricelist
		},
	}

	ctx := engine.NewTradeContext(t.Context(), offer)

	// PricerMiddleware will:
	// 1. Check price for "211;6;p5027" -> not found
	// 2. Fetch price for "211;6;p5027" -> empty
	// 3. Fall back to "211;6" (base SKU) -> found!
	mgr.On("GetPrice", "211;6;p5027").Return(nil, false)
	mgr.On("Watch", "211;6;p5027").Return()
	mgr.On("Fetch", mock.Anything, []string{"211;6;p5027"}).Return(map[string]*pricedb.Price{}, nil)

	// Base item is priced in pricedb
	mgr.On("GetPrice", "211;6").Return(&pricedb.Price{
		SKU:  "211;6",
		Name: "Cosmetic",
		Buy:  pricedb.Currencies{Metal: 1.11},
		Sell: pricedb.Currencies{Metal: 1.22},
	}, true)

	mw := PricerMiddleware(mgr, func() *schema.Schema { return nil }, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err := handler(ctx)
	assert.NoError(t, err)

	// Verify that context contains prices mapped with the base price!
	pricesRaw, ok := ctx.Get("prices")
	assert.True(t, ok)

	priceMap := pricesRaw.(map[string]*pricedb.Price)
	p, ok := priceMap["211;6;p5027"]
	assert.True(t, ok)
	assert.Equal(t, 1.11, p.Buy.Metal)
	assert.Equal(t, 1.22, p.Sell.Metal)
	assert.Contains(t, p.Name, "Painted")
}

func TestDonations_IsJunk_HasSpells(t *testing.T) {
	t.Parallel()

	assert.True(t, IsJunk(nil))
	assert.True(t, IsJunk(&trading.Item{SKU: ""}))

	itemWithSpells := &trading.Item{
		SKU: "5021;6",
		Attributes: []trading.Attribute{
			{Defindex: 1004, Value: "0"},
		},
	}
	assert.False(t, IsJunk(itemWithSpells))

	crateItem := &trading.Item{
		SKU: "5022;6;c1",
		Attributes: []trading.Attribute{
			{Defindex: schema.AttrCrateSeries, Value: "1"},
		},
	}
	assert.True(t, IsJunk(crateItem))

	standardItem := &trading.Item{
		SKU: "5021;6",
	}

	assert.False(t, IsJunk(standardItem))
	assert.False(t, HasSpells(nil))

	itemSpellAttr := &trading.Item{
		SKU: "5021;6",
		Attributes: []trading.Attribute{
			{Defindex: 1007, Value: "1"},
		},
	}
	assert.True(t, HasSpells(itemSpellAttr))

	itemSpellDesc := &trading.Item{
		SKU: "5021;6",
		Descriptions: []trading.Description{
			{Value: "Halloween: Squash Rockets"},
		},
	}

	assert.True(t, HasSpells(itemSpellDesc))
	assert.False(t, HasSpells(standardItem))
}
