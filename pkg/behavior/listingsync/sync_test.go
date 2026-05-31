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

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockBackpackProvider struct {
	stock       map[string]int
	items       map[string][]uint64
	pure        currency.PureStock
	itemDetails map[uint64]*tf2.Item
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return m.stock[sku]
}

func (m *mockBackpackProvider) GetItemsBySKU(targetSKU string) []uint64 {
	return m.items[targetSKU]
}

func (m *mockBackpackProvider) GetPureStock() currency.PureStock {
	return m.pure
}

func (m *mockBackpackProvider) GetItem(id uint64) (*tf2.Item, bool) {
	if m.itemDetails == nil {
		return nil, false
	}

	it, ok := m.itemDetails[id]

	return it, ok
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
	return nil
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

func (m *mockCritProvider) GetStorefrontURL(ctx context.Context) string {
	return "https://crit.tf/group/mock-store"
}

func TestListingsSynchronizer_Run_PriceUpdateEvent_UpdatesMarketplaces(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{
		stock: map[string]int{
			"5021;6": 5,
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

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go func() {
		_ = syncer.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	eventBus.Publish(&pricedb.PricelistUpdatedEvent{
		SKU:    "5021;6",
		Buy:    pricedb.Currencies{Metal: 60.0},
		Sell:   pricedb.Currencies{Metal: 62.0},
		Source: "Autokeys",
	})

	assert.Eventually(t, func() bool {
		eventBus.Publish(&pricedb.PricelistUpdatedEvent{
			SKU:    "5021;6",
			Buy:    pricedb.Currencies{Metal: 60.0},
			Sell:   pricedb.Currencies{Metal: 62.0},
			Source: "Autokeys",
		})

		critClient.mu.Lock()
		critLen := len(critClient.created)
		critClient.mu.Unlock()

		listingMgr.mu.Lock()
		bptfLen := len(listingMgr.upserts)
		listingMgr.mu.Unlock()

		return critLen >= 5 && bptfLen >= 2
	}, 3*time.Second, 100*time.Millisecond, "Expected listings to be created on crit.tf and backpack.tf")
}

func TestListingsSynchronizer_GenerateListingComment(t *testing.T) {
	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	bp := &mockBackpackProvider{}
	listingMgr := &mockListingProvider{}
	priceMgr := &mockPriceProvider{}
	critClient := &mockCritProvider{}

	cfg := Config{}

	t.Run("Standard fallback when template is empty", func(t *testing.T) {
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				ListingCommentTemplate: "",
			},
		}

		syncer := New(bp, listingMgr, priceMgr, cfgMgr, critClient, eventBus, logger, cfg)

		comment := syncer.generateListingComment(
			context.Background(),
			"5021;6",
			"buy",
			pricedb.Currencies{Keys: 1, Metal: 12.33},
			5,
			10,
		)
		assert.Equal(t, "⚡ G-man | Buying 5021;6 | Stock: 5/10", comment)

		commentSell := syncer.generateListingComment(
			context.Background(),
			"5021;6",
			"sell",
			pricedb.Currencies{Keys: 1, Metal: 12.33},
			5,
			10,
		)
		assert.Equal(t, "⚡ G-man | Selling 5021;6 | Stock: 5/10", commentSell)
	})

	t.Run("Template placeholder replacement", func(t *testing.T) {
		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				ListingCommentTemplate: "Price: %price% | Name: %name% | ECP: %ecp_item% | Stock: %current_stock%/%max_stock% | Crit Store: %crittf_store% | Crit Item: %crittf_item%",
				Items: map[string]trading.ItemConfig{
					"5021;6": {
						SKU:  "5021;6",
						Name: "Mann Co. Supply Crate Key",
					},
				},
			},
		}

		syncer := New(bp, listingMgr, priceMgr, cfgMgr, critClient, eventBus, logger, cfg)

		// 1. Buy intent (client perspective: sell)
		commentBuy := syncer.generateListingComment(
			context.Background(),
			"5021;6",
			"buy",
			pricedb.Currencies{Keys: 2, Metal: 3.55},
			2,
			5,
		)

		assert.Contains(t, commentBuy, "Price: 2 keys, 3.55 ref")
		assert.Contains(t, commentBuy, "Name: Mann Co. Supply Crate Key")
		// ECP: bot "buy" intent -> client "sell" ECP -> "sell_Mann_Co_Supply_Crate_Key"
		assert.Contains(t, commentBuy, "ECP: sell_Mann_Co_Supply_Crate_Key")
		assert.Contains(t, commentBuy, "Stock: 2/5")
		assert.Contains(t, commentBuy, "Crit Store: https://crit.tf/group/mock-store")
		assert.Contains(t, commentBuy, "Crit Item: https://crit.tf/group/mock-store/item/5021;6")

		// 2. Sell intent (client perspective: buy)
		commentSell := syncer.generateListingComment(
			context.Background(),
			"5021;6",
			"sell",
			pricedb.Currencies{Keys: 0, Metal: 15.0},
			4,
			5,
		)

		assert.Contains(t, commentSell, "Price: 15 ref")
		assert.Contains(t, commentSell, "Name: Mann Co. Supply Crate Key")
		// ECP: bot "sell" intent -> client "buy" ECP -> "buy_Mann_Co_Supply_Crate_Key"
		assert.Contains(t, commentSell, "ECP: buy_Mann_Co_Supply_Crate_Key")
		assert.Contains(t, commentSell, "Stock: 4/5")
		assert.Contains(t, commentSell, "Crit Store: https://crit.tf/group/mock-store")
		assert.Contains(t, commentSell, "Crit Item: https://crit.tf/group/mock-store/item/5021;6")
	})
}

func TestListingsSynchronizer_FilterCantAfford(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))

	priceMgr := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Buy:  pricedb.Currencies{Metal: 60.0},
				Sell: pricedb.Currencies{Metal: 62.0},
			},
			"5002;6": {
				SKU:  "5002;6",
				Buy:  pricedb.Currencies{Metal: 1.0},
				Sell: pricedb.Currencies{Metal: 1.0},
			},
			"some_item_sku": {
				SKU:  "some_item_sku",
				Buy:  pricedb.Currencies{Metal: 20.0},
				Sell: pricedb.Currencies{Metal: 22.0},
			},
		},
	}

	cfg := Config{}

	t.Run(
		"when FilterCantAfford is enabled and bot lacks balance, does not publish and deletes existing buy listing",
		func(t *testing.T) {
			bp := &mockBackpackProvider{
				stock: map[string]int{
					"some_item_sku": 0,
				},
				pure: currency.PureStock{
					Keys:    0,
					Refined: 5,
				},
			}

			listingMgr := &mockListingProvider{
				listings: []*bptf.ListingResponse{
					{
						ID:      "existing_buy_id",
						Intent:  "buy",
						Details: "some_item_sku",
					},
				},
			}

			cfgMgr := &mockConfigProvider{
				cfg: trading.Config{
					DefaultMaxStock:  5,
					FilterCantAfford: true,
				},
			}

			syncer := New(bp, listingMgr, priceMgr, cfgMgr, nil, bus.New(), logger, cfg)
			syncer.syncBptfBatch(t.Context(), []string{"some_item_sku"})

			assert.Empty(t, listingMgr.upserts)
			assert.Contains(t, listingMgr.deletes, "existing_buy_id")
		},
	)

	t.Run("when FilterCantAfford is enabled and bot has enough balance, publishes buy listing", func(t *testing.T) {
		bp := &mockBackpackProvider{
			stock: map[string]int{
				"some_item_sku": 0,
			},
			pure: currency.PureStock{
				Keys:    1,
				Refined: 0,
			},
		}

		listingMgr := &mockListingProvider{}

		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				DefaultMaxStock:  5,
				FilterCantAfford: true,
			},
		}

		syncer := New(bp, listingMgr, priceMgr, cfgMgr, nil, bus.New(), logger, cfg)
		syncer.syncBptfBatch(t.Context(), []string{"some_item_sku"})

		assert.Len(t, listingMgr.upserts, 1)
		assert.Equal(t, "some_item_sku", listingMgr.upserts[0].Item)
		assert.Equal(t, "buy", listingMgr.upserts[0].Intent)
		assert.Empty(t, listingMgr.deletes)
	})

	t.Run("when FilterCantAfford is disabled and bot lacks balance, publishes buy listing anyway", func(t *testing.T) {
		bp := &mockBackpackProvider{
			stock: map[string]int{
				"some_item_sku": 0,
			},
			pure: currency.PureStock{
				Keys:    0,
				Refined: 5,
			},
		}

		listingMgr := &mockListingProvider{}

		cfgMgr := &mockConfigProvider{
			cfg: trading.Config{
				DefaultMaxStock:  5,
				FilterCantAfford: false,
			},
		}

		syncer := New(bp, listingMgr, priceMgr, cfgMgr, nil, bus.New(), logger, cfg)
		syncer.syncBptfBatch(t.Context(), []string{"some_item_sku"})

		assert.Len(t, listingMgr.upserts, 1)
		assert.Equal(t, "some_item_sku", listingMgr.upserts[0].Item)
		assert.Equal(t, "buy", listingMgr.upserts[0].Intent)
		assert.Empty(t, listingMgr.deletes)
	})
}
