// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/foundation/async/log"
	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListingManager(t *testing.T) {
	t.Parallel()

	t.Run("sync_multipage_loop", func(t *testing.T) {
		respPage1 := ListingScrollable{
			Results: []Listing{{ID: "1"}, {ID: "2"}},
			Cursor:  &Cursor{Total: 3, Limit: 2, Skip: 0},
		}
		respPage2 := ListingScrollable{
			Results: []Listing{{ID: "3"}},
			Cursor:  &Cursor{Total: 3, Limit: 2, Skip: 2},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if r.URL.Query().Get("skip") == "2" {
				_ = json.NewEncoder(w).Encode(respPage2)
			} else {
				_ = json.NewEncoder(w).Encode(respPage1)
			}
		}))
		defer server.Close()

		client := NewAPI(aoni.NewClient(nil), option.WithBaseURL(server.URL))

		mgr := NewListingManager(client, nil, log.Discard)

		err := mgr.Sync(t.Context())
		require.NoError(t, err)
	})

	t.Run("delete_all_batches", func(t *testing.T) {
		var callCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/classifieds/listings/batch" && r.Method == http.MethodDelete {
				callCount.Add(1)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"success":true}`))
			}
		}))
		defer server.Close()

		client := NewAPI(aoni.NewClient(nil), option.WithBaseURL(server.URL))

		mgr := NewListingManager(client, nil, log.Discard)
		for i := range 150 {
			mgr.AddMockListing(&Listing{ID: strconv.Itoa(i + 1)})
		}

		err := mgr.DeleteAll(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int32(2), callCount.Load())
	})
}
