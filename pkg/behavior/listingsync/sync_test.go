// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package listingsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockBackpackProvider struct {
	stock map[string]int
	items map[string][]uint64
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return m.stock[sku]
}

func (m *mockBackpackProvider) GetItemsBySKU(targetSKU string) []uint64 {
	return m.items[targetSKU]
}

type mockListingProvider struct {
	mu       sync.Mutex
	listings []*bptf.ListingResponse
	upserts  []bptf.ListingResolvable
	deletes  []string
}

func (m *mockListingProvider) FindListingBySKU(sku, intent string) *bptf.ListingResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, l := range m.listings {
		if l.Intent == intent && (l.Details == sku || sku == currency.SKUKey) {
			return l
		}
	}

	return nil
}

func (m *mockListingProvider) Upsert(
	ctx context.Context,
	listing bptf.ListingResolvable,
) (*bptf.ListingResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upserts = append(m.upserts, listing)

	return &bptf.ListingResponse{ID: "mock_id"}, nil
}

func (m *mockListingProvider) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deletes = append(m.deletes, id)

	return nil
}

func (m *mockListingProvider) Client() *bptf.Client {
	return nil // Simulate no client for simple single-upserts fallback
}

type mockPriceProvider struct {
	prices map[string]*pricedb.Price
}

func (m *mockPriceProvider) GetPrice(sku string) (*pricedb.Price, bool) {
	p, ok := m.prices[sku]
	return p, ok
}

type mockConfigProvider struct {
	cfg trading.Config
}

func (m *mockConfigProvider) GetConfig() trading.Config {
	return m.cfg
}

type mockCritProvider struct {
	mu       sync.Mutex
	listings []crit.Listing
	created  []crit.Listing
	updated  []crit.Listing
	deleted  []string
}

func (m *mockCritProvider) FetchMyListings(ctx context.Context) ([]crit.Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listings, nil
}

func (m *mockCritProvider) CreateListing(
	ctx context.Context,
	assetID string,
	currencies pricedb.Currencies,
) (*crit.Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l := crit.Listing{
		ID:         123,
		AssetID:    assetID,
		PriceKeys:  currencies.Keys,
		PriceMetal: rest.Float64String(currencies.Metal),
		SKU:        "5021;6",
	}
	m.created = append(m.created, l)
	m.listings = append(m.listings, l)

	return &l, nil
}

func (m *mockCritProvider) UpdateListing(
	ctx context.Context,
	listingID string,
	currencies pricedb.Currencies,
) (*crit.Listing, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l := crit.Listing{
		ID:         123,
		AssetID:    "9876",
		PriceKeys:  currencies.Keys,
		PriceMetal: rest.Float64String(currencies.Metal),
		SKU:        "5021;6",
	}
	m.updated = append(m.updated, l)

	return &l, nil
}

func (m *mockCritProvider) DeleteListing(ctx context.Context, listingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleted = append(m.deleted, listingID)

	return nil
}

func TestListingsSynchronizer_Sync(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{
		stock: map[string]int{
			"5021;6": 5, // currently have 5 keys
		},
		items: map[string][]uint64{
			"5021;6": {10001, 10002, 10003, 10004, 10005},
		},
	}

	listingMgr := &mockListingProvider{}
	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"5021;6": {
				SKU:  "5021;6",
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 62.0},
			},
		},
	}

	cfgMgr := &mockConfigProvider{
		cfg: trading.Config{
			DefaultMaxStock: 10,
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:        "5021;6",
					MaxStock:   10,
					MinStock:   0,
					EnableBuy:  true,
					EnableSell: true,
				},
			},
		},
	}

	critClient := &mockCritProvider{}

	cfg := Config{
		BptfRateLimit: 10 * time.Millisecond,
		CritRateLimit: 10 * time.Millisecond,
		BatchDelay:    10 * time.Millisecond,
	}

	syncer := New(bp, listingMgr, priceMgr, cfgMgr, critClient, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = syncer.Run(ctx)
	}()

	// Give subscription registration some time to complete
	time.Sleep(50 * time.Millisecond)

	// Publish price updated event
	eventBus.Publish(&pricedb.PricelistUpdatedEvent{
		SKU:    "5021;6",
		Buy:    pricedb.Currencies{Metal: 60.0},
		Sell:   pricedb.Currencies{Metal: 62.0},
		Source: "Autokeys",
	})

	// Wait for queue processing and rate limiter sleep
	time.Sleep(200 * time.Millisecond)

	// Verify crit.tf update
	critClient.mu.Lock()
	assert.Len(t, critClient.created, 5) // Should list all 5 keys!
	critClient.mu.Unlock()

	// Verify backpack.tf listings updates
	listingMgr.mu.Lock()
	// Should have created buy and sell listings because stock is 5 (min 0, max 10)
	require.Len(t, listingMgr.upserts, 2)
	assert.Equal(t, "buy", listingMgr.upserts[0].Intent)
	assert.Equal(t, 60.0, listingMgr.upserts[0].Currencies["metal"])
	assert.Equal(t, "sell", listingMgr.upserts[1].Intent)
	assert.Equal(t, 62.0, listingMgr.upserts[1].Currencies["metal"])
	listingMgr.mu.Unlock()
}
