// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/listingsync"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockBackpackProvider struct {
	stock     map[string]int
	items     map[string][]uint64
	pureStock currency.PureStock
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return m.stock[sku]
}

func (m *mockBackpackProvider) GetItemsBySKU(targetSKU string) []uint64 {
	return m.items[targetSKU]
}

func (m *mockBackpackProvider) GetPureStock() currency.PureStock {
	return m.pureStock
}

type mockPriceProvider struct {
	mu      sync.Mutex
	prices  map[string]*pricedb.Price
	sets    map[string]pricedb.Currencies
	watched []string
}

func (m *mockPriceProvider) GetPrice(sku string) (*pricedb.Price, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.prices[sku]

	return p, ok
}

func (m *mockPriceProvider) SetPrice(sku string, buy, sell pricedb.Currencies, source pricedb.PricelistChangedSource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sets == nil {
		m.sets = make(map[string]pricedb.Currencies)
	}

	m.sets[sku] = sell
	if p, ok := m.prices[sku]; ok {
		p.Buy = buy
		p.Sell = sell
	} else {
		m.prices[sku] = &pricedb.Price{
			SKU:  sku,
			Buy:  buy,
			Sell: sell,
		}
	}
}

func (m *mockPriceProvider) Watch(sku string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.watched {
		if w == sku {
			return
		}
	}

	m.watched = append(m.watched, sku)
}

func (m *mockPriceProvider) Unwatch(sku string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newWatched []string
	for _, w := range m.watched {
		if w != sku {
			newWatched = append(newWatched, w)
		}
	}

	m.watched = newWatched
}

func (m *mockPriceProvider) GetWatchedSKUs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watched
}

type mockConfigProvider struct {
	mu  sync.Mutex
	cfg trading.Config
}

func (m *mockConfigProvider) GetConfig() trading.Config {
	m.mu.Lock()
	defer m.mu.Unlock()

	copiedItems := make(map[string]trading.ItemConfig)
	for k, v := range m.cfg.Items {
		copiedItems[k] = v
	}

	return trading.Config{
		GlobalMaxStock:    m.cfg.GlobalMaxStock,
		DefaultMaxStock:   m.cfg.DefaultMaxStock,
		PPUMinProfitScrap: m.cfg.PPUMinProfitScrap,
		Items:             copiedItems,
	}
}

func (m *mockConfigProvider) setItem(sku string, item trading.ItemConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg.Items == nil {
		m.cfg.Items = make(map[string]trading.ItemConfig)
	}

	m.cfg.Items[sku] = item
}

type mockCostBasisProvider struct {
	entries map[string]storage.CostBasisEntry
}

func (m *mockCostBasisProvider) GetOldestEntry(sku string) (storage.CostBasisEntry, bool) {
	entry, ok := m.entries[sku]
	return entry, ok
}

type makeChangeCall struct {
	defIndex uint32
	count    int
}

type mockCraftingProvider struct {
	mu           sync.Mutex
	condensed    int
	splitCalls   []makeChangeCall
	smeltedClass []string
	smeltErr     error
}

func (m *mockCraftingProvider) CondenseMetal(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.condensed++

	return 1, nil
}

func (m *mockCraftingProvider) MakeChange(ctx context.Context, targetDefIndex uint32, targetCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.splitCalls = append(m.splitCalls, makeChangeCall{defIndex: targetDefIndex, count: targetCount})

	return nil
}

func (m *mockCraftingProvider) SmeltClassWeapons(ctx context.Context, class string) ([]uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.smeltedClass = append(m.smeltedClass, class)
	if m.smeltErr != nil {
		return nil, m.smeltErr
	}

	return []uint64{1}, nil
}

func TestStockStrategist_WatchlistSync(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		watched: []string{currency.SKUKey, "5002;6"}, // Key and Refined are currently watched
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"}, // config has only keys SKU
			},
		},
	}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	cfg := Config{}
	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	strategist.syncWatchlist()

	// Keys should stay, Refined should be unwatched, Key item SKU ("5021;6") should be watched
	watched := priceMgr.GetWatchedSKUs()
	assert.Contains(t, watched, currency.SKUKey)
	assert.Contains(t, watched, "5021;6")
	assert.NotContains(t, watched, "5002;6")
}

func TestStockStrategist_StagnantFIFODiscount(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 60.0},
			},
		},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			PPUMinProfitScrap: 1, // 1 Scrap profit boundary
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}

	// 15 days ago purchase record
	purchaseTime := time.Now().Add(-15 * 24 * time.Hour)
	cost := &mockCostBasisProvider{
		entries: map[string]storage.CostBasisEntry{
			"5021;6": {
				SKU:       "5021;6",
				BuyMetal:  45.0, // Bought at 45 refined
				Timestamp: purchaseTime,
			},
		},
	}
	craft := &mockCraftingProvider{}

	cfg := Config{
		StagnantThreshold: 14 * 24 * time.Hour,
		DiscountPercent:   0.05, // 5% discount
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	strategist.runAudit(context.Background())

	// 60 ref * 0.95 = 57 ref (new discounted sell price)
	// 57 ref covers cost (45 ref) + profit, so it should be applied!
	priceMgr.mu.Lock()
	assert.Equal(t, 57.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestStockStrategist_PPUFloorClamp(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 60.0},
			},
		},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			PPUMinProfitScrap: 9, // 9 Scrap profit floor (= 1.0 refined)
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}

	// 15 days ago purchase record
	purchaseTime := time.Now().Add(-15 * 24 * time.Hour)
	cost := &mockCostBasisProvider{
		entries: map[string]storage.CostBasisEntry{
			"5021;6": {
				SKU:       "5021;6",
				BuyMetal:  58.0, // Bought at 58 refined (extremely high cost)
				Timestamp: purchaseTime,
			},
		},
	}
	craft := &mockCraftingProvider{}

	cfg := Config{
		StagnantThreshold: 14 * 24 * time.Hour,
		DiscountPercent:   0.05, // 5% discount would be 57 ref (below cost 58 ref!)
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	strategist.runAudit(context.Background())

	// PPU protection: cost (58) + minProfit (1) = 59 refined.
	// So 5% discount (57 ref) should be clamped to 59 refined!
	priceMgr.mu.Lock()
	assert.Equal(t, 59.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestStockStrategist_CraftingTrigger(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{
		pureStock: currency.PureStock{
			Scrap:     2, // 2 scrap < 9 minimum scrap limits -> triggers weapon smelting!
			Reclaimed: 1, // 1 reclaimed < 3 minimum reclaimed limits -> triggers Refined splitting!
			Refined:   10,
		},
	}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{
		smeltErr: assert.AnError, // Force weapon smelting to fail to test metal splitting fallbacks!
	}

	cfg := Config{
		MinScrapMetal:     9,
		MinReclaimedMetal: 3,
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)
	strategist.gcConnected = true

	strategist.coordinateCrafting(context.Background())

	// Crafting provider should have coordinates smelting across classes and splitting metals
	craft.mu.Lock()
	assert.NotEmpty(t, craft.smeltedClass) // Smelt weapons triggered
	require.Len(t, craft.splitCalls, 2)
	assert.Equal(t, uint32(5000), craft.splitCalls[0].defIndex)
	assert.Equal(t, 9, craft.splitCalls[0].count)
	assert.Equal(t, uint32(5001), craft.splitCalls[1].defIndex)
	assert.Equal(t, 3, craft.splitCalls[1].count)
	craft.mu.Unlock()
}

func TestStockStrategist_LiveConfigUpdates(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		watched: []string{currency.SKUKey},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"}, // keys in config
			},
		},
	}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	cfg := Config{}
	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = strategist.Run(ctx)
	}()

	// Give startup time to complete
	time.Sleep(50 * time.Millisecond)

	// Subscribe to ListingsAuditRequestedEvent
	sub := eventBus.Subscribe(&listingsync.AuditRequestedEvent{})
	defer sub.Unsubscribe()

	// Simulate config change (add new SKU "5002;6")
	cfgMgr.setItem("5002;6", trading.ItemConfig{SKU: "5002;6"})

	// Wait for configuration monitoring ticker (polls every 5s, let's wait a bit)
	time.Sleep(6 * time.Second)

	// Pricedb Watch should now include the new SKU
	priceMgr.mu.Lock()
	assert.Contains(t, priceMgr.watched, "5002;6")
	priceMgr.mu.Unlock()

	// AuditRequestedEvent should have been emitted
	select {
	case ev := <-sub.C():
		auditEv, ok := ev.(*listingsync.AuditRequestedEvent)
		require.True(t, ok)
		assert.Contains(t, auditEv.SKUs, "5002;6")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for AuditRequestedEvent")
	}
}

func TestStockStrategist_StagnantRestoration(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:    "5021;6",
				Buy:    pricedb.Currencies{Metal: 60.0},
				Sell:   pricedb.Currencies{Metal: 60.0},
				Source: "Manual",
			},
		},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			PPUMinProfitScrap: 1,
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}

	// Stagnant first: 15 days ago purchase record
	purchaseTime := time.Now().Add(-15 * 24 * time.Hour)
	cost := &mockCostBasisProvider{
		entries: map[string]storage.CostBasisEntry{
			"5021;6": {
				SKU:       "5021;6",
				BuyMetal:  45.0,
				Timestamp: purchaseTime,
			},
		},
	}
	craft := &mockCraftingProvider{}

	cfg := Config{
		StagnantThreshold: 14 * 24 * time.Hour,
		DiscountPercent:   0.05,
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	// Run audit - stagnant, should apply 5% discount (60 * 0.95 = 57 ref)
	strategist.runAudit(context.Background())

	priceMgr.mu.Lock()
	assert.Equal(t, 57.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()

	// Update costbasis record to be FRESH (0 days ago)
	cost.entries["5021;6"] = storage.CostBasisEntry{
		SKU:       "5021;6",
		BuyMetal:  45.0,
		Timestamp: time.Now(),
	}

	// Update pricedb to reflect the discounted price to simulate what really happens in pricedb
	priceMgr.mu.Lock()
	priceMgr.prices["5021;6"].Sell.Metal = 57.0
	priceMgr.prices["5021;6"].Source = "StockControl"
	priceMgr.mu.Unlock()

	// Run audit again - no longer stagnant, should restore original base price (60.0)
	strategist.runAudit(context.Background())

	priceMgr.mu.Lock()
	assert.Equal(t, 60.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}
