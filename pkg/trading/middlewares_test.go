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
	"github.com/stretchr/testify/require"

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
		{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
	}
	_, ok1 := FindPartnerCurrency(items1, 5, 0, nil)
	assert.False(t, ok1)

	items2 := []*trading.Item{
		{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
		{SKU: currency.SKUReclaimed, MarketHashName: "Reclaimed Metal"},
		{SKU: currency.SKUReclaimed, MarketHashName: "Reclaimed Metal"},
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

	predictor.On("PredictSpellPrice", mock.Anything, "Exorcism", "Strange Rocket Launcher").
		Return(nil, errors.New("pricedb down")).Once()

	ctx := engine.NewTradeContext(t.Context(), offer)
	ctx.Set("prices", prices)

	mockCfgProvider := func() Config {
		return Config{
			FallbackSpellPremiums: map[string]float64{
				"Exorcism": 4.0,
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
		"5021;6": {
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
	}

	t.Run("GiveWeaponReceiveWeapon", func(t *testing.T) {
		t.Parallel()

		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{SKU: "199;6"},
			},
			ItemsToReceive: []*trading.Item{
				{SKU: "200;6"},
			},
		}
		ctx := engine.NewTradeContext(t.Context(), offer)
		ctx.Set("prices", prices)
		ctx.Set("schema", sh)

		diff, err := calculateValueDiff(ctx, false)
		require.NoError(t, err)
		assert.Equal(t, currency.Scrap(0), diff)
	})
}

func TestMiddlewares_IsJunk_And_HasSpells(t *testing.T) {
	t.Parallel()

	assert.True(t, IsJunk(nil))
	assert.True(t, IsJunk(&trading.Item{SKU: ""}))

	assert.False(t, HasSpells(nil))
}

func TestStockLimitMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("empty_receive_skipped", func(t *testing.T) {
		t.Parallel()

		bp := backpack.New()
		offer := &trading.TradeOffer{
			ItemsToGive:    []*trading.Item{{SKU: "5021;6"}},
			ItemsToReceive: nil,
		}
		ctx := engine.NewTradeContext(t.Context(), offer)
		mw := StockLimitMiddleware(bp, StockConfig{}, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})
		err := handler(ctx)
		assert.NoError(t, err)
	})
}

func TestStockLimitMiddleware_NoReceiveItems(t *testing.T) {
	t.Parallel()

	mw := StockLimitMiddleware(nil, StockConfig{}, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	offer := &trading.TradeOffer{ItemsToReceive: nil}
	ctx := engine.NewTradeContext(t.Context(), offer)
	err := handler(ctx)
	assert.NoError(t, err)
}

type mockBackpack struct {
	totalCount int
	stock      map[string]int
}

func (m *mockBackpack) GetTotalCount() int                  { return m.totalCount }
func (m *mockBackpack) GetStock(sku string) int             { return m.stock[sku] }
func (m *mockBackpack) GetItem(id uint64) (*tf2.Item, bool) { return nil, false }

func TestStockLimitMiddleware_VariousLimits(t *testing.T) {
	t.Parallel()

	bp := &mockBackpack{
		totalCount: 50,
		stock:      map[string]int{"5021;6": 5},
	}

	cfg := StockConfig{
		MaxTotal:   100,
		MaxPerSKU:  map[string]int{"5021;6": 4},
		DefaultMax: 10,
	}

	mw := StockLimitMiddleware(bp, cfg, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	offer := &trading.TradeOffer{
		ItemsToReceive: []*trading.Item{
			{SKU: "5021;6"},
		},
	}
	ctx := engine.NewTradeContext(t.Context(), offer)
	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	assert.Equal(t, reason.DeclineOverstocked, ctx.Verdict.Reason)

	cfgUnlimited := StockConfig{
		MaxTotal:   100,
		MaxPerSKU:  map[string]int{"5021;6": 0},
		DefaultMax: 10,
	}
	mwUnlimited := StockLimitMiddleware(bp, cfgUnlimited, log.Discard)
	handlerUnlimited := mwUnlimited(func(c *engine.TradeContext) error {
		return nil
	})
	ctxUnlimited := engine.NewTradeContext(t.Context(), offer)
	err = handlerUnlimited(ctxUnlimited)
	assert.NoError(t, err)
	assert.Equal(t, trading.ActionSkip, ctxUnlimited.Verdict.Action)
}

type mockEscrowCheckerWithError struct {
	err error
}

func (m *mockEscrowCheckerWithError) CheckEscrow(ctx context.Context, offer *trading.TradeOffer) (bool, error) {
	return false, m.err
}

func TestEscrowMiddleware_Error(t *testing.T) {
	t.Parallel()

	mw := EscrowMiddleware(&mockEscrowCheckerWithError{err: errors.New("steam down")}, log.Discard)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	offer := &trading.TradeOffer{}
	ctx := engine.NewTradeContext(t.Context(), offer)
	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, reason.ReviewEscrowCheckFailed, ctx.Verdict.Reason)
}

func TestHalloweenSpellMiddleware_MissingSchema_And_InvalidSKU(t *testing.T) {
	t.Parallel()

	predictor := &mockSpellPredictor{}
	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{
				SKU: "invalid-sku",
				Descriptions: []trading.Description{
					{Value: "Exorcism", Color: "7ea9d1"},
				},
			},
		},
	}

	ctx := engine.NewTradeContext(t.Context(), offer)
	ctx.Set("prices", map[string]*pricedb.Price{})

	t.Run("nil_schema", func(t *testing.T) {
		mw := HalloweenSpellMiddleware(predictor, func() *schema.Schema { return nil }, nil, log.Discard)
		handler := mw(func(c *engine.TradeContext) error { return nil })
		err := handler(ctx)
		assert.NoError(t, err)
	})

	t.Run("unknown_spell_name", func(t *testing.T) {
		sh := schema.New(&schema.Raw{})
		offerUnknown := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{
					SKU: "205;11",
					Descriptions: []trading.Description{
						{Value: "Unknown Spell Name", Color: "7ea9d1"},
					},
				},
			},
		}
		ctxUnknown := engine.NewTradeContext(t.Context(), offerUnknown)
		prices := map[string]*pricedb.Price{
			"205;11": {SKU: "205;11", Buy: pricedb.Currencies{Metal: 1.0}},
		}
		ctxUnknown.Set("prices", prices)

		mw := HalloweenSpellMiddleware(predictor, func() *schema.Schema { return sh }, nil, log.Discard)
		handler := mw(func(c *engine.TradeContext) error { return nil })
		err := handler(ctxUnknown)
		assert.NoError(t, err)
	})
}

type mockMetalChangeManagerError struct {
	err error
}

func (m *mockMetalChangeManagerError) SelectChange(amount currency.Scrap) ([]uint64, error) {
	return nil, m.err
}

func (m *mockMetalChangeManagerError) TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error {
	return m.err
}

func TestSmartCounterMiddleware_API_Error_Paths(t *testing.T) {
	t.Parallel()

	bp := backpack.New()
	cache := &mockBackpackCache{}
	setUnexportedField(bp, "cache", cache)

	t.Run("calculateValueDiff_error", func(t *testing.T) {
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{})
		mw := SmartCounterMiddleware(nil, nil, bp, nil, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})
		err := handler(ctx)
		assert.Error(t, err)
	})

	t.Run("SelectChange_generic_error", func(t *testing.T) {
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			ItemsToReceive: []*trading.Item{{SKU: "5000;6"}},
		})
		prices := map[string]*pricedb.Price{
			"5000;6": {SKU: "5000;6", Buy: pricedb.Currencies{Metal: 1.0}},
		}
		ctx.Set("prices", prices)

		mgrErr := &mockMetalChangeManagerError{err: errors.New("select change failed")}
		mw := SmartCounterMiddleware(nil, mgrErr, bp, nil, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})
		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, tf2reason.DeclineNoChange, ctx.Verdict.Reason)
	})

	t.Run("partnerInv_fetch_failed", func(t *testing.T) {
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			ItemsToGive: []*trading.Item{{SKU: "5000;6"}},
		})
		prices := map[string]*pricedb.Price{
			"5000;6": {SKU: "5000;6", Sell: pricedb.Currencies{Metal: 1.0}},
		}
		ctx.Set("prices", prices)

		invErr := new(mockPartnerInvProvider)
		invErr.On("GetPartnerInventory", mock.Anything, ctx.Offer.OtherSteamID).Return(nil, errors.New("fetch failed"))

		mw := SmartCounterMiddleware(nil, nil, bp, invErr, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})
		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, reason.ReviewPartnerInventoryFetchFailed, ctx.Verdict.Reason)
	})
}

func TestFindPartnerCurrency_WithKeys(t *testing.T) {
	t.Parallel()

	items := []*trading.Item{
		{SKU: currency.SKUKey, MarketHashName: "Key"},
		{SKU: currency.SKUKey, MarketHashName: "Key"},
	}

	res, ok := FindPartnerCurrency(items, 100, 50, nil)
	assert.True(t, ok)
	assert.Len(t, res, 2)
	assert.Equal(t, currency.SKUKey, res[0].SKU)
}

func TestGetPricingSKU_Error(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "invalid_sku", GetPricingSKU("invalid_sku"))
}

func TestCalculateValueDiff_Error_Paths(t *testing.T) {
	t.Parallel()

	t.Run("missing_prices", func(t *testing.T) {
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{})
		_, err := calculateValueDiff(ctx, false)
		assert.ErrorContains(t, err, "prices not found")
	})

	t.Run("unpriced_item_on_receive", func(t *testing.T) {
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			ItemsToReceive: []*trading.Item{{SKU: "unpriced-item"}},
		})
		prices := map[string]*pricedb.Price{}
		ctx.Set("prices", prices)

		_, err := calculateValueDiff(ctx, false)
		assert.ErrorContains(t, err, "unpriced item")
	})
}

func TestIsUniqueWeapon_AllCases(t *testing.T) {
	t.Parallel()

	assert.False(t, isUniqueWeapon("5021;6", nil))
	assert.False(t, isUniqueWeapon("invalid", &schema.Schema{}))

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 1, CraftClass: "weapon", ItemClass: "tf_weapon_scattergun"},
		{Defindex: 10, CraftClass: "hat"},
	}
	s := schema.New(raw)

	assert.True(t, isUniqueWeapon("1;6", s))
	assert.False(t, isUniqueWeapon("1;11", s))
	assert.False(t, isUniqueWeapon("10;6", s))
	assert.False(t, isUniqueWeapon("1;6;festive", s))
}

func TestMiddlewares_FindPartnerCurrency_Detailed(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 199, CraftClass: "weapon"},
	}
	s := schema.New(raw)

	items := []*trading.Item{
		{SKU: currency.SKUKey},
		{SKU: currency.SKURefined},
		{SKU: currency.SKUReclaimed},
		{SKU: currency.SKUScrap},
	}

	res, ok := FindPartnerCurrency(items, 63, 50, s)
	assert.True(t, ok)
	assert.Len(t, res, 4)
}

func TestHalloweenSpellMiddleware_PredictionEmpty(t *testing.T) {
	t.Parallel()

	sh := schema.New(&schema.Raw{})
	predictor := &mockSpellPredictor{}

	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{
				SKU: "205;11",
				Descriptions: []trading.Description{
					{Value: "Exorcism", Color: "7ea9d1"},
				},
			},
		},
	}

	prices := map[string]*pricedb.Price{
		"205;11": {
			SKU:  "205;11",
			Name: "Strange Rocket Launcher",
			Buy:  pricedb.Currencies{Metal: 20.0},
		},
	}

	pred := &pricedb.SpellPredictionResponse{
		ItemName: "Strange Rocket Launcher",
	}
	predictor.On("PredictSpellPrice", mock.Anything, "Exorcism", "Strange Rocket Launcher").Return(pred, nil)

	ctx := engine.NewTradeContext(t.Context(), offer)
	ctx.Set("prices", prices)

	mockCfgProvider := func() Config {
		return Config{
			FallbackSpellPremiums: map[string]float64{
				"Exorcism": 4.0,
			},
		}
	}

	mw := HalloweenSpellMiddleware(predictor, func() *schema.Schema { return sh }, mockCfgProvider, log.Discard)
	handler := mw(func(c *engine.TradeContext) error { return nil })

	err := handler(ctx)
	assert.NoError(t, err)

	ourVal, ok := ctx.Get("our_spell_premium_scrap")
	assert.True(t, ok)
	assert.Equal(t, currency.Scrap(36), ourVal.(currency.Scrap))
}

func funcMockSchema() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 5021, ItemQuality: 6, Name: "Mann Co. Supply Crate Key", ItemName: "Mann Co. Supply Crate Key"},
		{Defindex: 5002, ItemQuality: 6, Name: "Refined Metal", ItemName: "Refined Metal"},
		{Defindex: 5001, ItemQuality: 6, Name: "Reclaimed Metal", ItemName: "Reclaimed Metal"},
		{Defindex: 5000, ItemQuality: 6, Name: "Scrap Metal", ItemName: "Scrap Metal"},
		{Defindex: 199, ItemQuality: 6, Name: "Unique Weapon", ItemName: "Unique Weapon", CraftClass: "weapon"},
		{Defindex: 205, ItemQuality: 6, Name: "Rocket Launcher", ItemName: "Rocket Launcher", CraftClass: "weapon"},
		{
			Defindex: 5027,
			Name:     "Aged Moustache Grey Paint Can",
			ItemName: "Aged Moustache Grey",
			Attributes: []schema.ItemAttribute{
				{Name: "set_item_tint_value", Value: 8290046},
			},
		},
	}
	raw.Schema.AttributeControlledAttachedParticles = []*schema.ParticleEffect{
		{ID: 17, Name: "Sunbeams"},
	}
	raw.Schema.KillEaterScoreTypes = []*schema.KillEaterScoreType{
		{Type: 39, TypeName: "Robots Destroyed"},
	}
	raw.Schema.Qualities = map[string]int{
		"Unusual": 5,
		"Strange": 11,
	}
	raw.Schema.QualityNames = map[string]string{
		"Unusual": "Unusual",
		"Strange": "Strange",
	}

	return schema.New(raw)
}
