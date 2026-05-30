// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
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
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	tf2schema "github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// mockSchemaProvider returns a non-nil schema for testing
type testSchemaProvider struct{}

func (t *testSchemaProvider) Get() *tf2schema.Schema {
	return &tf2schema.Schema{}
}

func TestFIFOSubscriber_IntakeAndOuttake(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fifo-sub-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.New(log.DefaultConfig(log.LevelDebug))
	defer logger.Close()

	cbStore, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	var sub *tf2trading.FIFOSubscriber

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		cbStore.Start(ctx)
		close(done)
	}()

	defer func() {
		cancel()
		<-done

		if sub != nil {
			sub.Wait()
		}
	}()

	// Setup priceDB manager
	pdbClient := pricedb.NewClient(&http.Client{})
	priceMgr := pricedb.NewManager(pdbClient, logger)

	// Inject key price and item price to priceMgr cache
	priceMgr.Watch(currency.SKUKey)
	priceMgr.Watch("123;6")

	// Seed cache directly for test speed
	setUnexportedField(priceMgr, "cache", map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"123;6": {
			SKU:  "123;6",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 10.0}, // 90 scrap
			Sell: pricedb.Currencies{Keys: 0, Metal: 12.0}, // 108 scrap
		},
	})

	eventBus := bus.New()
	sub = tf2trading.NewFIFOSubscriber(cbStore, priceMgr, eventBus, logger)
	sub.Start(ctx)

	// Verify INTAKE (FIFO Push)
	// We give 1 key (50 ref / 450 scrap) + 2 ref (18 scrap) = 468 scrap total.
	// We receive 1 unit of SKU "123;6", base buy is 10 ref = 90 scrap.
	// totalDiff = 468 - 90 = 378 scrap distributed overpay.
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

	// Wait for event to propagate and save to write
	time.Sleep(100 * time.Millisecond)

	entries := cbStore.GetEntriesForTesting()
	require.Len(t, entries, 1)
	assert.Equal(t, "123;6", entries[0].SKU)
	assert.Equal(t, 378, entries[0].Diff)
	assert.Equal(t, 10.0, entries[0].BuyMetal)

	// Verify OUTTAKE (FIFO Pop)
	// We sell one unit of SKU "123;6"
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

	time.Sleep(100 * time.Millisecond)

	// Entry should be popped out of memory cache
	entriesAfter := cbStore.GetEntriesForTesting()
	assert.Len(t, entriesAfter, 0)

	// PPUState's LastSoldTime should be set
	state, ok := cbStore.GetPPUState("123;6")
	assert.True(t, ok)
	assert.False(t, state.LastSoldTime.IsZero())
}

func TestPPUMiddleware_Modulation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ppu-mw-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")
	logger := log.Discard

	cbStore, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		cbStore.Start(ctx)
		close(done)
	}()

	defer func() {
		cancel()
		<-done
	}()

	// Set cost basis for SKU "123;6": buy = 10 ref (90 scrap), diff = 0.
	// Net cost basis = 90 scrap.
	cbStore.Push("123;6", storage.CostBasisEntry{
		SKU:       "123;6",
		BuyKeys:   0,
		BuyMetal:  10.0,
		Diff:      0,
		Timestamp: time.Now(),
	})

	// Setup Config Manager
	cfgPath := filepath.Join(tmpDir, "trading_config.json")
	cfgManager, err := tf2trading.NewConfigManager(cfgPath)
	require.NoError(t, err)

	cfg := cfgManager.GetConfig()
	cfg.PPUHoldDuration = "24h"
	cfg.PPUGracePeriod = "1h"
	cfg.PPUMaxStockLimit = 1
	cfg.PPUMinProfitScrap = 9 // 1 ref profit
	// Protected sell price should be: net_cost_basis (90 scrap) + min_profit (9 scrap) = 99 scrap (11 ref)
	setUnexportedField(cfgManager, "cfg", cfg)

	// Setup mock backpack with stock = 1 (triggers PPU since stock <= maxLimit)
	bp := backpack.New()
	cache := &mockBackpackCache{items: []*tf2.Item{
		{ID: 1, SKU: "123;6"},
	}}
	setUnexportedField(bp, "cache", cache)
	setUnexportedField(bp, "manager", &testSchemaProvider{})

	// Setup trade context
	offer := &trading.TradeOffer{
		ItemsToGive: []*trading.Item{
			{SKU: "123;6"},
		},
	}
	tradeCtx := engine.NewTradeContext(context.Background(), offer)

	// Market prices:
	// Sell price is below protected sell threshold (market is 9.5 ref / 85 scrap < protected 11 ref / 99 scrap)
	// Buy price is above our cost basis (market is 11 ref / 99 scrap > cost 10 ref / 90 scrap) - should be capped!
	prices := map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
		"123;6": {
			SKU:  "123;6",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 11.0}, // 99 scrap
			Sell: pricedb.Currencies{Keys: 0, Metal: 9.5},  // 85 scrap
		},
	}
	tradeCtx.Set("prices", prices)

	mw := tf2trading.PPUMiddleware(bp, cbStore, cfgManager, logger)
	handler := mw(func(c *engine.TradeContext) error {
		return nil
	})

	err = handler(tradeCtx)
	assert.NoError(t, err)

	// Validate modulated prices
	modulatedRaw, ok := tradeCtx.Get("prices")
	require.True(t, ok)

	modulatedPrices := modulatedRaw.(map[string]*pricedb.Price)

	p := modulatedPrices["123;6"]
	// Sell price should be frozen at protected sell price (11.0 ref / 99 scrap)
	assert.Equal(t, 11.0, p.Sell.Metal)

	// Buy price should be capped at net cost basis (10.0 ref / 90 scrap)
	assert.Equal(t, 10.0, p.Buy.Metal)

	// State should show PPU is active
	state, ok := cbStore.GetPPUState("123;6")
	assert.True(t, ok)
	assert.True(t, state.IsPartialPriced)
}
