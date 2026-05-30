// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package listingsync

import (
	"context"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

func TestCoverage_DefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	assert.NotZero(t, cfg.BptfRateLimit)
	assert.NotZero(t, cfg.CritRateLimit)
}

func TestCoverage_SyncAndName(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard

	syncer := New(nil, nil, nil, nil, nil, b, logger, Config{})
	assert.Equal(t, BehaviorName, syncer.Name())

	orch := behavior.NewOrchestrator(logger, b)
	opt := Sync(nil, nil, nil, nil, nil, Config{})
	opt(orch)
}

func TestCoverage_NewZeroDefaults(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	syncer := New(nil, nil, nil, nil, nil, eventBus, logger, Config{})
	assert.Equal(t, DefaultConfig().BptfRateLimit, syncer.config.BptfRateLimit)
	assert.Equal(t, DefaultConfig().CritRateLimit, syncer.config.CritRateLimit)
	assert.Equal(t, DefaultConfig().BatchDelay, syncer.config.BatchDelay)
}

func TestCoverage_Run_LifeCycle(t *testing.T) {
	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	listingMgr := &mockListingProvider{}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {},
			},
		},
	}

	syncer := New(bp, listingMgr, priceMgr, cfgMgr, nil, eventBus, logger, Config{
		BptfRateLimit: 5 * time.Millisecond,
		CritRateLimit: 5 * time.Millisecond,
		BatchDelay:    5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = syncer.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)

	eventBus.Publish(&tf2.BackpackLoadedEvent{})
	eventBus.Publish(&tf2.ItemAcquiredEvent{Item: &tf2.Item{DefIndex: 5021, Quality: 6}})
	eventBus.Publish(&tf2.ItemRemovedEvent{ItemID: 10001})
	eventBus.Publish(&tf2.ItemUpdatedEvent{Item: &tf2.Item{DefIndex: 5021, Quality: 6}})
	eventBus.Publish(&AuditRequestedEvent{SKUs: []string{"5021;6"}})

	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestCoverage_SyncCritBatch_Scenarios(t *testing.T) {
	t.Parallel()

	logger := log.Discard

	bp := &mockBackpackProvider{
		items: map[string][]uint64{
			"5021;6": {10001},
		},
	}

	listingMgr := &mockListingProvider{}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 0.0},
			},
			"5002;6": {
				SKU:  "5002;6",
				Buy:  pricedb.Currencies{Metal: 1.0},
				Sell: pricedb.Currencies{Metal: 1.1},
			},
		},
	}

	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:        "5021;6",
					EnableSell: false,
				},
				"5002;6": {
					SKU:        "5002;6",
					EnableSell: true,
					MinStock:   0,
				},
			},
		},
	}

	critClient := &mockCritProvider{
		listings: []crit.Listing{
			{
				ID:         123,
				AssetID:    "10001",
				PriceKeys:  0,
				PriceMetal: 1.0,
				SKU:        "5002;6",
			},
			{
				ID:         456,
				AssetID:    "10002",
				PriceKeys:  0,
				PriceMetal: 1.1,
				SKU:        "5002;6",
			},
		},
	}

	syncer := New(bp, listingMgr, priceMgr, cfgMgr, critClient, bus.New(), logger, Config{})

	syncer.syncCritBatch(t.Context(), []string{"unknown"})

	syncer.syncCritBatch(t.Context(), []string{"5021;6"})

	bp.items = map[string][]uint64{
		"5002;6": {10001},
	}

	syncer.syncCritBatch(t.Context(), []string{"5002;6"})

	assert.NotEmpty(t, critClient.updated)
	assert.NotEmpty(t, critClient.deleted)
}

func TestCoverage_SyncBptfBatch_Scenarios(t *testing.T) {
	t.Parallel()

	logger := log.Discard

	bp := &mockBackpackProvider{
		stock: map[string]int{
			"5021;6": 15,
			"5002;6": 0,
		},
	}

	listingMgr := &mockListingProvider{
		listings: []*bptf.ListingResponse{
			{
				ID:     "buy_id",
				Intent: "buy",
			},
			{
				ID:     "sell_id",
				Intent: "sell",
			},
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 62.0},
			},
			"5002;6": {
				SKU:  "5002;6",
				Buy:  pricedb.Currencies{Metal: 1.0},
				Sell: pricedb.Currencies{Metal: 1.1},
			},
		},
	}

	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			DefaultMaxStock: 10,
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:      "5021;6",
					MaxStock: 10,
				},
				"5002;6": {
					SKU:      "5002;6",
					MinStock: 0,
				},
			},
		},
	}

	syncer := New(bp, listingMgr, priceMgr, cfgMgr, nil, bus.New(), logger, Config{})

	syncer.syncBptfBatch(t.Context(), []string{"unknown"})

	syncer.syncBptfBatch(t.Context(), []string{"5021;6", "5002;6"})

	assert.NotEmpty(t, listingMgr.deletes)
}
