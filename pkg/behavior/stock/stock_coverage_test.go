// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

func TestCoverage_Control_Name(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, Config{})
	assert.Equal(t, BehaviorName, strategist.Name())

	orch := behavior.NewOrchestrator(logger, eventBus)
	opt := Control(bp, priceMgr, cfgMgr, cost, craft, Config{})
	opt(orch)
}

func TestCoverage_Run_Events(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{}
	cost := &mockCostBasisProvider{}
	craft := &mockCraftingProvider{}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, Config{
		ConfigCheckInterval: 5 * time.Millisecond,
		AuditInterval:       5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = strategist.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)

	eventBus.Publish(&tf2.ConnectedEvent{})
	eventBus.Publish(&tf2.DisconnectedEvent{})
	eventBus.Publish(&tf2.BackpackLoadedEvent{})

	time.Sleep(20 * time.Millisecond)
}

func TestCoverage_discountStagnantItems_Clamping(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Sell: pricedb.Currencies{Metal: 60.0},
			},
			"5020;6": {
				SKU:    "5020;6",
				Buy:    pricedb.Currencies{Metal: 60.0},
				Sell:   pricedb.Currencies{Metal: 60.0},
				Source: "Manual",
			},
			"5002;6": {
				SKU:    "5002;6",
				Buy:    pricedb.Currencies{Metal: 60.0},
				Sell:   pricedb.Currencies{Metal: 60.0},
				Source: "Manual",
			},
		},
	}

	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			PPUMinProfitScrap: 18, // 2.0 ref profit
			Items: map[string]trading.ItemConfig{
				"5020;6": {SKU: "5020;6"},
				"5002;6": {SKU: "5002;6"},
			},
		},
	}

	purchaseTime := time.Now().Add(-15 * 24 * time.Hour)
	cost := &mockCostBasisProvider{
		entries: map[string]storage.CostBasisEntry{
			"5020;6": {
				SKU:       "5020;6",
				BuyKeys:   1,   // 60 ref
				BuyMetal:  5.0, // cost = 65 ref
				Timestamp: purchaseTime,
			},
			"5002;6": {
				SKU:       "5002;6",
				BuyMetal:  10.0, // cost = 10 ref
				Timestamp: purchaseTime,
			},
		},
	}
	craft := &mockCraftingProvider{}

	cfg := Config{
		StagnantThreshold:  14 * 24 * time.Hour,
		DiscountPercent:    0.20, // 60 * 0.8 = 48 ref
		MaxAllowedDiscount: 0.10, // 60 * 0.9 = 54 ref
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craft, eventBus, logger, cfg)

	strategist.runAudit(t.Context())

	priceMgr.mu.Lock()
	// item 5020;6: cost is 65, minProfit is 2. minAllowed is 67. Max allowed discount is 54. 67 > 54 -> clamps to 67.
	assert.Equal(t, 67.0, priceMgr.sets["5020;6"].Metal)
	// item 5002;6: cost is 10, minProfit is 2. minAllowed is 12. Max allowed discount is 54. 12 < 54 -> clamps to 54.
	assert.Equal(t, 54.0, priceMgr.sets["5002;6"].Metal)
	priceMgr.mu.Unlock()
}

func TestCoverage_coordinateCrafting_Details(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpackProvider{
		pureStock: currency.PureStock{
			Scrap:     1,
			Reclaimed: 1,
			Refined:   10,
		},
	}
	priceMgr := &mockPriceProvider{}
	cfgMgr := &mockConfigProvider{}
	cost := &mockCostBasisProvider{}

	craftErr := &mockCraftingProvider{
		smeltErr: errors.New("smelt error"),
	}

	cfg := Config{
		MinScrapMetal:     5,
		MinReclaimedMetal: 5,
	}

	strategist := New(bp, priceMgr, cfgMgr, cost, craftErr, eventBus, logger, cfg)

	// GC disconnected: should skip crafting
	strategist.coordinateCrafting(t.Context())
	assert.Empty(t, craftErr.smeltedClass)

	// Connect GC
	strategist.mu.Lock()
	strategist.gcConnected = true
	strategist.mu.Unlock()

	// Should trigger smelt class weapons, then MakeChange for 5000 and 5001.
	strategist.coordinateCrafting(t.Context())
	assert.NotEmpty(t, craftErr.smeltedClass)
	assert.NotEmpty(t, craftErr.splitCalls)

	// Excess case
	bpExcess := &mockBackpackProvider{
		pureStock: currency.PureStock{
			Scrap:     20,
			Reclaimed: 10,
		},
	}
	craftExcess := &mockCraftingProvider{}
	strategistExcess := New(bpExcess, priceMgr, cfgMgr, cost, craftExcess, eventBus, logger, cfg)
	strategistExcess.mu.Lock()
	strategistExcess.gcConnected = true
	strategistExcess.mu.Unlock()

	strategistExcess.coordinateCrafting(t.Context())
	assert.Equal(t, 1, craftExcess.condensed)
}
