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

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/listingsync"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockItemCache struct {
	items []*tf2.Item
}

func (m *mockItemCache) GetItems() []*tf2.Item {
	return m.items
}

func (m *mockItemCache) GetItem(id uint64) (*tf2.Item, bool) {
	for _, it := range m.items {
		if it.ID == id {
			return it, true
		}
	}

	return nil, false
}

func (m *mockItemCache) GetMaxSlots() int {
	return 3000
}

type mockSchemaProvider struct {
	sch *schema.Schema
}

func (m *mockSchemaProvider) Get() *schema.Schema {
	return m.sch
}

type mockBackpackProvider struct {
	stock         map[string]int
	items         map[string][]uint64
	pureStock     currency.PureStock
	cache         backpack.ItemCache
	schemaProv    backpack.SchemaProvider
	deleted       []uint64
	layoutApplied bool
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

func (m *mockBackpackProvider) Cache() backpack.ItemCache {
	if m.cache == nil {
		return &mockItemCache{}
	}

	return m.cache
}

func (m *mockBackpackProvider) Schema() backpack.SchemaProvider {
	if m.schemaProv == nil {
		return &mockSchemaProvider{}
	}

	return m.schemaProv
}

func (m *mockBackpackProvider) DeleteItem(ctx context.Context, itemID uint64) error {
	m.deleted = append(m.deleted, itemID)
	return nil
}

func (m *mockBackpackProvider) ApplyLayout(ctx context.Context, layout backpack.Layout) error {
	m.layoutApplied = true
	return nil
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
		p.Source = string(source)
	} else {
		m.prices[sku] = &pricedb.Price{
			SKU:    sku,
			Buy:    buy,
			Sell:   sell,
			Source: string(source),
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
		GlobalMaxStock:               m.cfg.GlobalMaxStock,
		DefaultMaxStock:              m.cfg.DefaultMaxStock,
		PPUMinProfitScrap:            m.cfg.PPUMinProfitScrap,
		AutoResetToAutopriceOnceSold: m.cfg.AutoResetToAutopriceOnceSold,
		EnableSmartTrashCleanup:      m.cfg.EnableSmartTrashCleanup,
		EnableAutoSorting:            m.cfg.EnableAutoSorting,
		Items:                        copiedItems,
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

func TestStockStrategist_SyncWatchlist_ValidConfig_UpdatesPriceDB(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		watched: []string{currency.SKUKey, "5002;6"},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	cfg := Config{}
	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	strategist.syncWatchlist()

	watched := priceMgr.GetWatchedSKUs()
	assert.Contains(t, watched, currency.SKUKey)
	assert.Contains(t, watched, "5021;6")
	assert.NotContains(t, watched, "5002;6")
}

func TestStockStrategist_Audit_StagnantFIFO_AppliesDiscount(t *testing.T) {
	t.Parallel()

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
			PPUMinProfitScrap: 1,
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}

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

	strategist.runAudit(t.Context())

	priceMgr.mu.Lock()
	assert.Equal(t, 57.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestStockStrategist_Audit_PPUProtection_ClampsDiscount(t *testing.T) {
	t.Parallel()

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
			PPUMinProfitScrap: 9,
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}

	purchaseTime := time.Now().Add(-15 * 24 * time.Hour)
	cost := &mockCostBasisProvider{
		entries: map[string]storage.CostBasisEntry{
			"5021;6": {
				SKU:       "5021;6",
				BuyMetal:  58.0,
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

	strategist.runAudit(t.Context())

	priceMgr.mu.Lock()
	assert.Equal(t, 59.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestStockStrategist_Audit_LowMetalReserves_TriggersCrafting(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{
		pureStock: currency.PureStock{
			Scrap:     2,
			Reclaimed: 1,
			Refined:   10,
		},
	}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{
		smeltErr: assert.AnError,
	}

	cfg := Config{
		MinScrapMetal:     9,
		MinReclaimedMetal: 3,
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)
	strategist.gcConnected = true

	strategist.coordinateCrafting(t.Context())

	craft.mu.Lock()
	assert.NotEmpty(t, craft.smeltedClass)
	require.Len(t, craft.splitCalls, 2)
	assert.Equal(t, uint32(5000), craft.splitCalls[0].defIndex)
	assert.Equal(t, 9, craft.splitCalls[0].count)
	assert.Equal(t, uint32(5001), craft.splitCalls[1].defIndex)
	assert.Equal(t, 3, craft.splitCalls[1].count)
	craft.mu.Unlock()
}

func TestStockStrategist_Run_ConfigChanges_SyncsWatchlistAndEmitsAudit(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		watched: []string{currency.SKUKey},
	}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {SKU: "5021;6"},
			},
		},
	}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	cfg := Config{
		ConfigCheckInterval: 10 * time.Millisecond,
	}
	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go func() {
		_ = strategist.Run(ctx)
	}()

	sub := eventBus.Subscribe(&listingsync.AuditRequestedEvent{})
	t.Cleanup(sub.Unsubscribe)

	time.Sleep(50 * time.Millisecond)

	cfgMgr.setItem("5002;6", trading.ItemConfig{SKU: "5002;6"})

	select {
	case ev := <-sub.C():
		auditEv, ok := ev.(*listingsync.AuditRequestedEvent)
		require.True(t, ok)
		assert.Contains(t, auditEv.SKUs, "5002;6")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for AuditRequestedEvent")
	}

	priceMgr.mu.Lock()
	assert.Contains(t, priceMgr.watched, "5002;6")
	priceMgr.mu.Unlock()
}

func TestStockStrategist_Audit_FreshFIFO_RestoresBasePrice(t *testing.T) {
	t.Parallel()

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

	strategist.runAudit(t.Context())

	priceMgr.mu.Lock()
	assert.Equal(t, 57.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()

	cost.entries["5021;6"] = storage.CostBasisEntry{
		SKU:       "5021;6",
		BuyMetal:  45.0,
		Timestamp: time.Now(),
	}

	priceMgr.mu.Lock()
	priceMgr.prices["5021;6"].Sell.Metal = 57.0
	priceMgr.prices["5021;6"].Source = "StockControl"
	priceMgr.mu.Unlock()

	strategist.runAudit(t.Context())

	priceMgr.mu.Lock()
	assert.Equal(t, 60.0, priceMgr.sets["5021;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestStockStrategist_AutoResetToAutoprice(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	t.Run("Resets manually priced item to autoprice when stock reaches 0", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{
			stock: map[string]int{
				"5021;6": 0,
			},
		}

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
				AutoResetToAutopriceOnceSold: true,
				Items: map[string]trading.ItemConfig{
					"5021;6": {SKU: "5021;6"},
				},
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		strategist.checkAndResetAutoPrices()

		priceMgr.mu.Lock()
		p := priceMgr.prices["5021;6"]
		assert.Equal(t, "PriceDB", p.Source)
		priceMgr.mu.Unlock()
	})

	t.Run("Does not reset manual price when stock is greater than 0", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{
			stock: map[string]int{
				"5021;6": 2,
			},
		}

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
				AutoResetToAutopriceOnceSold: true,
				Items: map[string]trading.ItemConfig{
					"5021;6": {SKU: "5021;6"},
				},
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		strategist.checkAndResetAutoPrices()

		priceMgr.mu.Lock()
		p := priceMgr.prices["5021;6"]
		assert.Equal(t, "Manual", p.Source)
		priceMgr.mu.Unlock()
	})

	t.Run("Does not reset manual price when feature is disabled", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{
			stock: map[string]int{
				"5021;6": 0,
			},
		}

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
				AutoResetToAutopriceOnceSold: false,
				Items: map[string]trading.ItemConfig{
					"5021;6": {SKU: "5021;6"},
				},
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		strategist.checkAndResetAutoPrices()

		priceMgr.mu.Lock()
		p := priceMgr.prices["5021;6"]
		assert.Equal(t, "Manual", p.Source)
		priceMgr.mu.Unlock()
	})

	t.Run("Smart Trash Cleanup deletes untradable junk items when enabled", func(t *testing.T) {
		t.Parallel()

		// Construct mock schema
		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{
				Defindex:  5021,
				ItemName:  "Mann Co. Supply Crate",
				ItemClass: "supply_crate",
			},
			{
				Defindex:  280,
				ItemName:  "Noise Maker - Witch",
				ItemClass: "action",
			},
			{
				Defindex:  5826,
				ItemName:  "Soul Gargoyle",
				ItemClass: "action",
			},
			{
				Defindex:  1,
				ItemName:  "Unusual Hat",
				ItemClass: "tf_wearable",
			},
		}
		sCustom := schema.New(raw)

		items := []*tf2.Item{
			// Untradable crate -> should be deleted
			{ID: 100, DefIndex: 5021, IsTradable: false},
			// Tradable crate -> should NOT be deleted
			{ID: 101, DefIndex: 5021, IsTradable: true},
			// Untradable Noise Maker -> should be deleted
			{ID: 102, DefIndex: 280, IsTradable: false},
			// Untradable Soul Gargoyle -> should be deleted
			{ID: 103, DefIndex: 5826, IsTradable: false},
			// Untradable Hat -> should NOT be deleted
			{ID: 104, DefIndex: 1, IsTradable: false},
		}

		bp := &mockBackpackProvider{
			cache:      &mockItemCache{items: items},
			schemaProv: &mockSchemaProvider{sch: sCustom},
		}

		priceMgr := &mockPriceProvider{}
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				EnableSmartTrashCleanup: true,
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		ctx := context.Background()
		strategist.smartTrashCleanup(ctx)

		// Assertions: only IDs 100, 102, 103 should be deleted
		assert.ElementsMatch(t, []uint64{100, 102, 103}, bp.deleted)
	})

	t.Run("Smart Trash Cleanup does nothing when disabled", func(t *testing.T) {
		t.Parallel()

		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{
				Defindex:  5021,
				ItemName:  "Mann Co. Supply Crate",
				ItemClass: "supply_crate",
			},
		}
		sCustom := schema.New(raw)

		items := []*tf2.Item{
			{ID: 100, DefIndex: 5021, IsTradable: false},
		}

		bp := &mockBackpackProvider{
			cache:      &mockItemCache{items: items},
			schemaProv: &mockSchemaProvider{sch: sCustom},
		}

		priceMgr := &mockPriceProvider{}
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				EnableSmartTrashCleanup: false,
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		ctx := context.Background()
		strategist.smartTrashCleanup(ctx)

		// Assertions: no items should be deleted since it is disabled
		assert.Empty(t, bp.deleted)
	})

	t.Run("autoSortBackpack applies layout when enabled", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{}
		priceMgr := &mockPriceProvider{}
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				EnableAutoSorting: true,
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		ctx := context.Background()
		strategist.autoSortBackpack(ctx)

		assert.True(t, bp.layoutApplied)
	})

	t.Run("autoSortBackpack does nothing when disabled", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{}
		priceMgr := &mockPriceProvider{}
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				EnableAutoSorting: false,
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		ctx := context.Background()
		strategist.autoSortBackpack(ctx)

		assert.False(t, bp.layoutApplied)
	})

	t.Run("autoSortBackpack applies custom layout sections when configured", func(t *testing.T) {
		t.Parallel()

		bp := &mockBackpackProvider{}
		priceMgr := &mockPriceProvider{}
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				EnableAutoSorting: true,
				BackpackSortingSections: []trading.BackpackSectionConfig{
					{Name: "My Currency", Category: "currency", StartPage: 1, EndPage: 2},
					{Name: "My Weapons", Category: "weapons", StartPage: 3, EndPage: 5},
				},
			},
		}

		cost := &mockCostBasisProvider{}
		craft := &mockCraftingProvider{}
		cfg := Config{}

		strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

		ctx := context.Background()
		strategist.autoSortBackpack(ctx)

		assert.True(t, bp.layoutApplied)
	})
}
