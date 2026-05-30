// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package autokeys

import (
	"context"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func TestCoverage_RegisterAndName(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard
	cfg := Config{}

	ak := New(nil, nil, logger, b, cfg, nil)
	assert.Equal(t, BehaviorName, ak.Name())

	orch := behavior.NewOrchestrator(logger, b)
	opt := Register(nil, nil, cfg, nil)
	opt(orch)
}

func TestCoverage_Run(t *testing.T) {
	b := bus.New()
	logger := log.Discard

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 62.0},
			},
		},
	}

	bp := &mockBackpackProvider{}
	cfg := Config{
		CheckInterval: 10 * time.Millisecond,
	}

	ak := New(bp, priceMgr, logger, b, cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = ak.Run(ctx)
	}()

	time.Sleep(25 * time.Millisecond)

	b.Publish(&tf2.BackpackLoadedEvent{})

	time.Sleep(15 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func mockSchemaForAutokeysCoverage() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:   13,
			CraftClass: "weapon",
		},
	}

	return schema.New(raw)
}

func TestCoverage_Scan_WeaponsAsCurrency(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard

	s := mockSchemaForAutokeysCoverage()
	bp := &mockBackpackProvider{
		schemaObj: s,
		items: []*tf2.Item{
			{DefIndex: 13, IsCraftable: true, IsTradable: true},
			{DefIndex: 13, IsCraftable: true, IsTradable: true},
		},
	}

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 62.0},
			},
		},
	}

	cfg := Config{
		WeaponsAsCurrency: true,
	}

	ak := New(bp, priceMgr, logger, b, cfg, nil)
	err := ak.scan(t.Context())
	assert.NoError(t, err)
}

func TestCoverage_Scan_PriceUnavailable(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard

	bp := &mockBackpackProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{}, // empty
	}

	cfg := Config{}

	ak := New(bp, priceMgr, logger, b, cfg, nil)
	err := ak.scan(t.Context())
	assert.NoError(t, err)
}
