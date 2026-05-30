// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package autokeys

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type mockBackpackProvider struct {
	stock     currency.PureStock
	stockMap  map[string]int
	items     []*tf2.Item
	schemaObj *schema.Schema
}

func (m *mockBackpackProvider) GetPureStock() currency.PureStock {
	return m.stock
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return m.stockMap[sku]
}

func (m *mockBackpackProvider) Cache() backpack.ItemCache {
	return &mockItemCache{items: m.items}
}

func (m *mockBackpackProvider) Schema() backpack.SchemaProvider {
	return &mockSchemaProvider{s: m.schemaObj}
}

type mockItemCache struct {
	items []*tf2.Item
}

func (m *mockItemCache) GetItems() []*tf2.Item {
	return m.items
}

func (m *mockItemCache) GetItem(id uint64) (*tf2.Item, bool) {
	return nil, false
}

func (m *mockItemCache) GetMaxSlots() int {
	return 1000
}

type mockSchemaProvider struct {
	s *schema.Schema
}

func (m *mockSchemaProvider) Get() *schema.Schema {
	return m.s
}

type mockPriceProvider struct {
	mu       sync.Mutex
	prices   map[string]*pricedb.Price
	sets     map[string]pricedb.Currencies
	setCalls int
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

	m.setCalls++

	if m.sets == nil {
		m.sets = make(map[string]pricedb.Currencies)
	}

	m.sets[sku+"_buy"] = buy
	m.sets[sku+"_sell"] = sell

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

type mockAlertProvider struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockAlertProvider) MessageAdmins(ctx context.Context, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, message)

	return nil
}

func TestAutokeys_Scan_BuyingMode_UpdatesKeyPrices(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:               10,
		MaxKeys:               50,
		MinRefs:               50.0,
		MaxRefs:               100.0,
		EnableBanking:         false,
		EnableScrapAdjustment: false,
		CheckInterval:         1 * time.Second,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    15,
			Refined: 150,
		},
		stockMap: map[string]int{
			currency.SKUKey: 15,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	alert := &mockAlertProvider{}
	ak := New(bp, priceMgr, logger, nil, cfg, alert)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	defer priceMgr.mu.Unlock()

	assert.Equal(t, 1, priceMgr.setCalls)
	assert.Equal(t, 60.0, priceMgr.sets[currency.SKUKey+"_buy"].Metal)
	assert.Equal(t, 62.0, priceMgr.sets[currency.SKUKey+"_sell"].Metal)

	assert.Equal(t, "buying", ak.GetStatus())
	assert.True(t, ak.IsEnabled())
	assert.True(t, ak.IsActive())
}

func TestAutokeys_Scan_SellingMode_RemainsIdleOnLowDeficit(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:       10,
		MaxKeys:       50,
		MinRefs:       50.0,
		MaxRefs:       100.0,
		EnableBanking: false,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    25,
			Refined: 30,
		},
		stockMap: map[string]int{
			currency.SKUKey: 25,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	alert := &mockAlertProvider{}
	ak := New(bp, priceMgr, logger, nil, cfg, alert)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	defer priceMgr.mu.Unlock()

	assert.Equal(t, 1, priceMgr.setCalls)
	assert.Equal(t, 60.0, priceMgr.sets[currency.SKUKey+"_buy"].Metal)
	assert.Equal(t, 62.0, priceMgr.sets[currency.SKUKey+"_sell"].Metal)

	assert.Equal(t, "idle", ak.GetStatus())
}

func TestAutokeys_Scan_BankingMode_EnablesKeysBanking(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:       10,
		MaxKeys:       50,
		MinRefs:       50.0,
		MaxRefs:       100.0,
		EnableBanking: true,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    20,
			Refined: 75,
		},
		stockMap: map[string]int{
			currency.SKUKey: 20,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	ak := New(bp, priceMgr, logger, nil, cfg, nil)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	defer priceMgr.mu.Unlock()

	assert.Equal(t, 1, priceMgr.setCalls)
	assert.Equal(t, 60.0, priceMgr.sets[currency.SKUKey+"_buy"].Metal)
	assert.Equal(t, 62.0, priceMgr.sets[currency.SKUKey+"_sell"].Metal)

	assert.Equal(t, "banking", ak.GetStatus())
	assert.True(t, ak.IsActive())
}

func TestAutokeys_Scan_LowReserveAlert_TriggersDebouncedAlert(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:       10,
		MaxKeys:       50,
		MinRefs:       50.0,
		MaxRefs:       100.0,
		EnableBanking: false,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    5,
			Refined: 10,
		},
		stockMap: map[string]int{
			currency.SKUKey: 5,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	alert := &mockAlertProvider{}
	ak := New(bp, priceMgr, logger, nil, cfg, alert)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	alert.mu.Lock()
	assert.Len(t, alert.messages, 1)
	assert.Contains(t, alert.messages[0], "[CRITICAL]")
	alert.mu.Unlock()

	err = ak.scan(t.Context())
	require.NoError(t, err)

	alert.mu.Lock()
	assert.Len(t, alert.messages, 1)
	alert.mu.Unlock()

	bp.stock.Keys = 20
	bp.stock.Refined = 70
	err = ak.scan(t.Context())
	require.NoError(t, err)

	alert.mu.Lock()
	assert.Len(t, alert.messages, 2)
	assert.Contains(t, alert.messages[1], "recovered")
	alert.mu.Unlock()
}

func TestAutokeys_Scan_ScrapAdjustment_AppliesScrapOffsets(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:               10,
		MaxKeys:               50,
		MinRefs:               50.0,
		MaxRefs:               100.0,
		EnableBanking:         false,
		EnableScrapAdjustment: true,
		ScrapAdjustmentValue:  2,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    15,
			Refined: 120,
		},
		stockMap: map[string]int{
			currency.SKUKey: 15,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	ak := New(bp, priceMgr, logger, nil, cfg, nil)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	defer priceMgr.mu.Unlock()

	assert.InDelta(t, 60.22, priceMgr.sets[currency.SKUKey+"_buy"].Metal, 0.01)
	assert.InDelta(t, 62.22, priceMgr.sets[currency.SKUKey+"_sell"].Metal, 0.01)
}

func TestAutokeys_Scan_UnchangedState_SkipsRedundantUpdates(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	cfg := Config{
		MinKeys:       10,
		MaxKeys:       50,
		MinRefs:       50.0,
		MaxRefs:       100.0,
		EnableBanking: true,
	}

	bp := &mockBackpackProvider{
		stock: currency.PureStock{
			Keys:    20,
			Refined: 75,
		},
		stockMap: map[string]int{
			currency.SKUKey: 20,
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Keys: 0, Metal: 60.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 62.0},
			},
		},
	}

	ak := New(bp, priceMgr, logger, nil, cfg, nil)

	err := ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	initialSets := priceMgr.setCalls
	priceMgr.mu.Unlock()
	assert.Equal(t, 1, initialSets)

	err = ak.scan(t.Context())
	require.NoError(t, err)

	priceMgr.mu.Lock()
	assert.Equal(t, initialSets, priceMgr.setCalls)
	priceMgr.mu.Unlock()
}
