// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
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

func TestListingManager(t *testing.T) {
	t.Parallel()

	t.Run("sync_loop_multipage_and_errors", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		total := 150

		results := make([]ListingResponse, 0, total)
		for i := range 150 {
			results = append(results, ListingResponse{ID: strconv.Itoa(i + 1)})
		}

		mockResp := ListingsResponse{
			Results: results,
			Cursor:  Cursor{Total: total, Limit: 500, Skip: 0},
		}
		stub.SetJSONResponse("api/v2/classifieds/listings", 200, mockResp)

		client := New(aoni.NewClient(stub), "", "")
		mgr := NewListingManager(client, nil, log.Discard)
		assert.Equal(t, client, mgr.Client())

		err := mgr.Sync(t.Context())
		require.NoError(t, err)

		stub.ClearCalls()
	})

	t.Run("sync_multipage_loop", func(t *testing.T) {
		respPage1 := ListingsResponse{
			Results: []ListingResponse{{ID: "1"}, {ID: "2"}},
			Cursor:  Cursor{Total: 3, Limit: 2, Skip: 0},
		}
		respPage2 := ListingsResponse{
			Results: []ListingResponse{{ID: "3"}},
			Cursor:  Cursor{Total: 3, Limit: 2, Skip: 2},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			skipStr := r.URL.Query().Get("skip")
			if skipStr == "2" {
				_ = json.NewEncoder(w).Encode(respPage2)
			} else {
				_ = json.NewEncoder(w).Encode(respPage1)
			}
		}))
		defer server.Close()

		client := New(aoni.NewClient(nil), "", "")
		client.rest = client.rest.WithBaseURL(server.URL)

		mgr := NewListingManager(client, nil, log.Discard)

		err := mgr.Sync(t.Context())
		require.NoError(t, err)
	})

	t.Run("sync_error_bubbles_up", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/v2/classifieds/listings", 500, nil)

		client := New(aoni.NewClient(stub), "", "")
		mgr := NewListingManager(client, nil, log.Discard)

		err := mgr.Sync(t.Context())
		assert.Error(t, err)
	})

	t.Run("upsert_delete_find", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/classifieds/listings", 200, ListingResponse{ID: "list_123"})
		stub.SetJSONResponse("api/v2/classifieds/listings/list_123", 200, map[string]any{"success": true})

		client := New(aoni.NewClient(stub), "", "")
		mgr := NewListingManager(client, nil, log.Discard)

		res, err := mgr.Upsert(t.Context(), ListingResolvable{})
		require.NoError(t, err)
		assert.Equal(t, "list_123", res.ID)

		err = mgr.Delete(t.Context(), "list_123")
		require.NoError(t, err)
	})

	t.Run("upsert_delete_errors", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/v2/classifieds/listings", 500, nil)
		stub.SetJSONResponse("api/v2/classifieds/listings/list_123", 500, nil)

		client := New(aoni.NewClient(stub), "", "")
		mgr := NewListingManager(client, nil, log.Discard)

		_, err := mgr.Upsert(t.Context(), ListingResolvable{})
		assert.Error(t, err)

		err = mgr.Delete(t.Context(), "list_123")
		assert.Error(t, err)
	})

	t.Run("delete_all_empty", func(t *testing.T) {
		mgr := NewListingManager(nil, nil, log.Discard)
		err := mgr.DeleteAll(t.Context())
		assert.NoError(t, err)
	})

	t.Run("delete_all_batches", func(t *testing.T) {
		var callCount int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/classifieds/listings/batch" && r.Method == http.MethodDelete {
				callCount++

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			}
		}))
		defer server.Close()

		client := New(aoni.NewClient(nil), "", "")
		client.rest = client.rest.WithBaseURL(server.URL)

		mgr := NewListingManager(client, nil, log.Discard)
		for i := range 150 {
			mgr.AddMockListing(&ListingResponse{ID: strconv.Itoa(i + 1)})
		}

		err := mgr.DeleteAll(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("delete_all_failure", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/v2/classifieds/listings/batch", 500, nil)

		client := New(aoni.NewClient(stub), "", "")
		mgr := NewListingManager(client, nil, log.Discard)
		mgr.AddMockListing(&ListingResponse{ID: "1"})

		err := mgr.DeleteAll(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "batch delete failed")
	})

	t.Run("matches_sku_variants", func(t *testing.T) {
		mgrNoSchema := NewListingManager(nil, nil, log.Discard)

		l1 := &ListingResponse{
			Details: "SKU: 5021;6",
		}
		mgrNoSchema.AddMockListing(l1)
		found := mgrNoSchema.FindListingBySKU("5021;6", "")
		assert.NotNil(t, found)

		l2 := &ListingResponse{
			Details: "Other Details",
			Item:    ItemDocument{Name: "Key"},
		}
		mgrNoSchema.AddMockListing(l2)
		found2 := mgrNoSchema.FindListingBySKU("5021;6", "")
		assert.Nil(t, found2)
	})

	t.Run("item_to_sku_and_find_listing", func(t *testing.T) {
		sm := setupSchemaManager(t)
		client := New(nil, "", "")
		mgr := NewListingManager(client, sm, log.Discard)

		docMatch := ItemDocument{
			Name:      "Mann Co. Supply Crate Key",
			BaseName:  "Mann Co. Supply Crate Key",
			Tradable:  true,
			Craftable: true,
			Quality:   Entity{ID: 6, Name: "Unique"},
		}
		skuStr := mgr.ItemToSKU(docMatch)
		assert.Equal(t, "5021;6", skuStr)

		docFallback := ItemDocument{
			Name:      "Unusual Special Hat",
			BaseName:  "Special Hat",
			Tradable:  true,
			Craftable: true,
			Quality:   Entity{ID: 5, Name: "Unusual"},
		}
		skuFallback := mgr.ItemToSKU(docFallback)
		assert.Equal(t, "100;5", skuFallback)

		mockListing1 := &ListingResponse{
			ID:      "1",
			Details: "5021;6",
			Intent:  "buy",
			Item: ItemDocument{
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

		mockListing2 := &ListingResponse{
			ID:     "2",
			Intent: "buy",
			Item: ItemDocument{
				Name:      "Unusual Special Hat",
				BaseName:  "Special Hat",
				Tradable:  true,
				Craftable: true,
				Quality:   Entity{ID: 5, Name: "Unusual"},
			},
		}
		mgr.AddMockListing(mockListing2)

		found2 := mgr.FindListingBySKU("100;5", "buy")
		require.NotNil(t, found2)
		assert.Equal(t, "2", found2.ID)
	})
}
