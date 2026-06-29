// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricemanager

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
)

func setupSchemaManager(t *testing.T) *schema.Manager {
	t.Helper()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:    5021,
			ItemName:    "Mann Co. Supply Crate Key",
			Name:        "Mann Co. Supply Crate Key",
			ItemQuality: 6,
		},
		{
			Defindex:    100,
			ItemName:    "Special Hat",
			Name:        "Special Hat",
			ItemQuality: 6,
		},
	}
	raw.Schema.Qualities = map[string]int{"Unique": 6, "Unusual": 5}
	raw.Schema.QualityNames = map[string]string{"Unique": "Unique", "Unusual": "Unusual"}

	schemaJSON, err := json.Marshal(map[string]any{
		"version": "1.0",
		"time":    time.Now(),
		"raw": map[string]any{
			"schema": raw.Schema,
		},
	})
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "schema.json")
	err = os.WriteFile(cachePath, schemaJSON, 0o644)
	require.NoError(t, err)

	sm := schema.NewManager(schema.Config{
		CachePath: cachePath,
	})
	ictx := mock.NewInitContext()
	ictx.SetRest(aoni.NewClient(nil))
	err = sm.Init(ictx)
	require.NoError(t, err)

	err = sm.StartAuthed(t.Context(), nil)
	require.NoError(t, err)

	return sm
}

func TestPriceManager(t *testing.T) {
	t.Parallel()

	t.Run("update_basic_and_complex_skus", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		mockResp := bptf.PricesResponseV4{
			Success: 1,
			Items: map[string]bptf.BaseItemDoc{
				"Mann Co. Supply Crate Key": {
					Defindexes: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.PriceEntry{
						"6": {
							"Tradable": {
								"Craftable": {
									"0": {Value: 75},
								},
							},
						},
					},
				},
				"Unusual Hat": {
					Defindexes: []string{"100"},
					Prices: map[string]map[string]map[string]map[string]bptf.PriceEntry{
						"5": {
							"Tradable": {
								"Craftable": {
									"19": {Value: 120},
								},
							},
						},
					},
				},
				"Crate": {
					Defindexes: []string{"500"},
					Prices: map[string]map[string]map[string]map[string]bptf.PriceEntry{
						"6": {
							"Tradable": {
								"Craftable": {
									"82": {Value: 15},
								},
							},
						},
					},
				},
			},
		}
		stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

		client := bptf.New(aoni.NewClient(stub), "", "")
		cfg := Config{CachePath: filepath.Join(t.TempDir(), "prices.json")}
		manager := NewPriceManager(client, log.Discard, cfg)

		err := manager.Update(t.Context())
		require.NoError(t, err)

		p1, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), p1.Value)

		p2, ok := manager.GetPrice("100;5;u19")
		assert.True(t, ok)
		assert.Equal(t, float64(120), p2.Value)

		p3, ok := manager.GetPrice("500;6;c82")
		assert.True(t, ok)
		assert.Equal(t, float64(15), p3.Value)

		assert.Equal(t, "bptf_prices", manager.Name())

		err = manager.Load()
		require.NoError(t, err)

		pLoad, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), pLoad.Value)
	})

	t.Run("update_error_fails_cleanly", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/IGetPrices/v4", 500, nil)

		client := bptf.New(aoni.NewClient(stub), "", "")
		manager := NewPriceManager(client, log.Discard, Config{})
		err := manager.Update(t.Context())
		assert.Error(t, err)
	})

	t.Run("load_and_save_errors", func(t *testing.T) {
		managerEmpty := NewPriceManager(nil, log.Discard, Config{})
		err := managerEmpty.Load()
		assert.Error(t, err)
	})

	t.Run("price_manager_run_lifecycle", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		client := bptf.New(aoni.NewClient(stub), "", "")
		manager := NewPriceManager(client, log.Discard, Config{SyncInterval: 10 * time.Millisecond})

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := manager.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestListingManager(t *testing.T) {
	t.Parallel()

	t.Run("sync_loop_multipage_and_errors", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		total := 150

		results := make([]bptf.ListingResponse, 0, total)
		for i := range 150 {
			results = append(results, bptf.ListingResponse{ID: strconv.Itoa(i + 1)})
		}

		mockResp := bptf.ListingsResponse{
			Results: results,
			Cursor:  bptf.Cursor{Total: total, Limit: 500, Skip: 0},
		}
		stub.SetJSONResponse("api/v2/classifieds/listings", 200, mockResp)

		client := bptf.New(aoni.NewClient(stub), "", "")
		mgr := bptf.NewListingManager(client, nil, log.Discard)
		assert.Equal(t, client, mgr.Client())

		err := mgr.Sync(t.Context())
		require.NoError(t, err)

		stub.ClearCalls()
	})

	t.Run("sync_error_bubbles_up", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/v2/classifieds/listings", 500, nil)

		client := bptf.New(aoni.NewClient(stub), "", "")
		mgr := bptf.NewListingManager(client, nil, log.Discard)

		err := mgr.Sync(t.Context())
		assert.Error(t, err)
	})

	t.Run("upsert_delete_find", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/classifieds/listings", 200, bptf.ListingResponse{ID: "list_123"})
		stub.SetJSONResponse("api/v2/classifieds/listings/list_123", 200, map[string]any{"success": true})

		client := bptf.New(aoni.NewClient(stub), "", "")
		mgr := bptf.NewListingManager(client, nil, log.Discard)

		res, err := mgr.Upsert(t.Context(), bptf.ListingResolvable{})
		require.NoError(t, err)
		assert.Equal(t, "list_123", res.ID)

		err = mgr.Delete(t.Context(), "list_123")
		require.NoError(t, err)
	})

	t.Run("item_to_sku_and_find_listing", func(t *testing.T) {
		sm := setupSchemaManager(t)
		client := bptf.New(nil, "", "")
		mgr := bptf.NewListingManager(client, sm, log.Discard)

		docMatch := bptf.ItemDocument{
			Name:      "Mann Co. Supply Crate Key",
			BaseName:  "Mann Co. Supply Crate Key",
			Tradable:  true,
			Craftable: true,
			Quality:   bptf.Entity{ID: 6, Name: "Unique"},
		}
		skuStr := mgr.ItemToSKU(docMatch)
		assert.Equal(t, "5021;6", skuStr)

		docFallback := bptf.ItemDocument{
			Name:      "Unusual Special Hat",
			BaseName:  "Special Hat",
			Tradable:  true,
			Craftable: true,
			Quality:   bptf.Entity{ID: 5, Name: "Unusual"},
		}
		skuFallback := mgr.ItemToSKU(docFallback)
		assert.Equal(t, "100;5", skuFallback)

		mockListing1 := &bptf.ListingResponse{
			ID:      "1",
			Details: "5021;6",
			Intent:  "buy",
			Item: bptf.ItemDocument{
				Name:      "Unrelated",
				BaseName:  "Unrelated",
				Tradable:  true,
				Craftable: true,
			},
		}
		mgr.AddMockListing(mockListing1)

		found := mgr.FindListingBySKU("5021;6", "buy")
		require.NotNil(t, found)
		assert.Equal(t, "1", found.ID)

		mockListing2 := &bptf.ListingResponse{
			ID:     "2",
			Intent: "buy",
			Item: bptf.ItemDocument{
				Name:      "Unusual Special Hat",
				BaseName:  "Special Hat",
				Tradable:  true,
				Craftable: true,
				Quality:   bptf.Entity{ID: 5, Name: "Unusual"},
			},
		}
		mgr.AddMockListing(mockListing2)

		found2 := mgr.FindListingBySKU("100;5", "buy")
		require.NotNil(t, found2)
		assert.Equal(t, "2", found2.ID)
	})
}

func TestMiddlewares(t *testing.T) {
	t.Parallel()

	t.Run("safety_middleware", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		resp := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				id.New(123): {
					Bans: &bptf.UserBans{
						BPTF: "banned",
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, resp)

		client := bptf.New(aoni.NewClient(stub), "key", "token")
		cache := generic.NewCache[string, any]()
		mw := bptf.SafetyMiddleware(client, cache, log.Discard)

		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			OtherSteamID: id.New(123),
		})

		handlerCalled := false
		next := func(tc *engine.TradeContext) error {
			handlerCalled = true
			return nil
		}

		err := mw(next)(ctx)
		require.NoError(t, err)

		assert.False(t, handlerCalled)
		assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	})

	t.Run("value_tier_middleware", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		resp := bptf.InventoryValues{
			Value: 600.0,
		}
		stub.SetJSONResponse("api/inventory/123/values", 200, resp)

		client := bptf.New(aoni.NewClient(stub), "key", "token")
		mw := bptf.ValueTierMiddleware(client)

		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			OtherSteamID: id.New(123),
		})

		handlerCalled := false
		next := func(tc *engine.TradeContext) error {
			handlerCalled = true
			return nil
		}

		err := mw(next)(ctx)
		require.NoError(t, err)

		assert.True(t, handlerCalled)

		val, ok := ctx.Get("partner_inv_value")
		assert.True(t, ok)
		assert.Equal(t, 600.0, val)

		isWhale, ok := ctx.Get("is_whale")
		assert.True(t, ok)
		assert.True(t, isWhale.(bool))
	})
}
