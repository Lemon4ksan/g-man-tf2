// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	tf2schema "github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type fifoTestSchemaProvider struct{}

func (t *fifoTestSchemaProvider) Get() *tf2schema.Schema {
	return &tf2schema.Schema{}
}

func TestFIFOSubscriber_IntakeAndOuttake_AcceptedOffer_ProcessesFIFO(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.Discard

	cbStore, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	go cbStore.Start(ctx)

	pdbClient := pricedb.NewClient(&http.Client{})
	priceMgr := pricedb.NewManager(pdbClient, logger)

	priceMgr.Watch(currency.SKUKey)
	priceMgr.Watch("123;6")

	setUnexportedField(priceMgr, "cache", map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"123;6": {
			SKU:  "123;6",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 10.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 12.0},
		},
	})

	eventBus := bus.New()
	sub := NewFIFOSubscriber(cbStore, nil, priceMgr, eventBus, logger)
	sub.Start(ctx)
	t.Cleanup(func() {
		sub.Wait()
	})

	// Process INTAKE
	offer := &trading.TradeOffer{
		ID:    9999,
		State: trading.OfferStateAccepted,
		ItemsToGive: []*trading.Item{
			{SKU: currency.SKUKey, MarketHashName: "Mann Co. Supply Crate Key"},
			{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
			{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: "123;6", MarketHashName: "Some Weapon"},
		},
	}

	eventBus.Publish(&web.OfferChangedEvent{
		Offer:    offer,
		OldState: trading.OfferStateActive,
	})

	// Non-blocking state poll instead of time.Sleep
	var entries []storage.CostBasisEntry
	assert.Eventually(t, func() bool {
		entries = cbStore.GetEntriesForTesting()
		return len(entries) == 1
	}, 1*time.Second, 10*time.Millisecond)

	assert.Equal(t, "123;6", entries[0].SKU)
	assert.Equal(t, 378, entries[0].Diff)
	assert.Equal(t, 10.0, entries[0].BuyMetal)

	// Process OUTTAKE
	offerOut := &trading.TradeOffer{
		ID:    10000,
		State: trading.OfferStateAccepted,
		ItemsToGive: []*trading.Item{
			{SKU: "123;6", MarketHashName: "Some Weapon"},
		},
		ItemsToReceive: []*trading.Item{
			{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
		},
	}

	eventBus.Publish(&web.OfferChangedEvent{
		Offer:    offerOut,
		OldState: trading.OfferStateActive,
	})

	assert.Eventually(t, func() bool {
		entries = cbStore.GetEntriesForTesting()
		return len(entries) == 0
	}, 1*time.Second, 10*time.Millisecond)

	state, ok := cbStore.GetPPUState("123;6")
	assert.True(t, ok)
	assert.False(t, state.LastSoldTime.IsZero())
	cancel()
	time.Sleep(25 * time.Millisecond)
}

func TestPPUMiddleware_Modulation_MarketPriceBelowProtected_ModulatesPrices(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "costBasis.json")
	logger := log.Discard

	cbStore, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	go cbStore.Start(ctx)

	cbStore.Push("123;6", storage.CostBasisEntry{
		SKU:       "123;6",
		BuyKeys:   0,
		BuyMetal:  10.0,
		Diff:      0,
		Timestamp: time.Now(),
	})

	cfgPath := filepath.Join(tmpDir, "trading_config.json")
	cfgManager, err := NewConfigManager(cfgPath)
	require.NoError(t, err)

	cfg := cfgManager.GetConfig()
	cfg.PPUHoldDuration = "24h"
	cfg.PPUGracePeriod = "1h"
	cfg.PPUMaxStockLimit = 1
	cfg.PPUMinProfitScrap = 9
	setUnexportedField(cfgManager, "cfg", cfg)

	bp := backpack.New()
	cache := &mockBackpackCache{items: []*tf2.Item{
		{ID: 1, SKU: "123;6"},
	}}
	setUnexportedField(bp, "cache", cache)
	setUnexportedField(bp, "manager", &fifoTestSchemaProvider{})

	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{SKU: "123;6"},
		},
	}
	tradeCtx := engine.NewTradeContext(t.Context(), offer)

	prices := map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"123;6": {
			SKU:  "123;6",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 11.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 9.5},
		},
	}
	tradeCtx.Set("prices", prices)

	mw := PPUMiddleware(bp, cbStore, cfgManager, logger)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err = handler(tradeCtx)
	assert.NoError(t, err)

	modulatedRaw, ok := tradeCtx.Get("prices")
	require.True(t, ok)

	modulatedPrices := modulatedRaw.(map[string]*pricedb.Price)

	p := modulatedPrices["123;6"]
	assert.Equal(t, 11.0, p.Sell.Metal)
	assert.Equal(t, 10.0, p.Buy.Metal)

	state, ok := cbStore.GetPPUState("123;6")
	assert.True(t, ok)
	assert.True(t, state.IsPartialPriced)
	cancel()
	time.Sleep(25 * time.Millisecond)
}

type fakeProfitStore struct {
	mu   sync.RWMutex
	logs []storage.TradeProfitLog
}

func (f *fakeProfitStore) Push(entry storage.TradeProfitLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logs = append(f.logs, entry)

	return nil
}

func (f *fakeProfitStore) GetProfitSummary(since time.Duration) (storage.ProfitSummary, error) {
	return storage.ProfitSummary{}, nil
}

func (f *fakeProfitStore) GetDailyProfit(days int) ([]storage.ProfitSummary, error) {
	return nil, nil
}

func (f *fakeProfitStore) Prune(keepDuration time.Duration) error {
	return nil
}

func (f *fakeProfitStore) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.logs)
}

func (f *fakeProfitStore) Get(idx int) storage.TradeProfitLog {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.logs[idx]
}

func TestFIFOSubscriber_StatsIntegration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "costBasis.json")
	logger := log.Discard

	cbStore, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	priceMgr := pricedb.NewManager(nil, logger)
	priceMgr.Watch("123;6")

	setUnexportedField(priceMgr, "cache", map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"123;6": {
			SKU:  "123;6",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 10.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 12.0},
		},
	})

	statsStore := &fakeProfitStore{logs: make([]storage.TradeProfitLog, 0)}

	eventBus := bus.New()
	sub := NewFIFOSubscriber(cbStore, statsStore, priceMgr, eventBus, logger)
	sub.Start(ctx)
	t.Cleanup(func() {
		sub.Wait()
	})

	cbStore.Push("123;6", storage.CostBasisEntry{
		SKU:       "123;6",
		BuyKeys:   0,
		BuyMetal:  10.0,
		Diff:      0,
		Timestamp: time.Now(),
	})

	offer := &trading.TradeOffer{
		ID:    7777,
		State: trading.OfferStateAccepted,
		ItemsToGive: []*trading.Item{
			{SKU: currency.SKUKey, MarketHashName: "Mann Co. Supply Crate Key"},
			{SKU: "123;6", MarketHashName: "Some Weapon"},
		},
		ItemsToReceive: []*trading.Item{},
	}

	for range 62 {
		offer.ItemsToReceive = append(
			offer.ItemsToReceive,
			&trading.Item{SKU: currency.SKURefined, MarketHashName: "Refined Metal"},
		)
	}

	eventBus.Publish(&web.OfferChangedEvent{
		Offer:    offer,
		OldState: trading.OfferStateActive,
	})

	// Wait for event subscriber to record logs in statsStore
	assert.Eventually(t, func() bool {
		return statsStore.Len() == 1
	}, 1*time.Second, 10*time.Millisecond)

	logEntry := statsStore.Get(0)
	assert.Equal(t, "7777", logEntry.TradeID)
	// We gave 1 key, received 0 keys -> net keys is -1
	assert.Equal(t, -1, logEntry.NetKeys)
	// We received 62 ref, gave 0 ref -> net metal ref is 62.0
	assert.Equal(t, 62.0, logEntry.NetMetalRef)
	// Weapon: sold for 12 ref (= 108 scrap), bought for 10 ref (= 90 scrap) -> FIFO profit is 18 scrap
	assert.Equal(t, 18, logEntry.FIFOProfitScrap)
}
