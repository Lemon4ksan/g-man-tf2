// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricemanager

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
)

func TestPriceManager(t *testing.T) {
	t.Parallel()

	t.Run("update_basic_and_complex_skus", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		mockResp := bptf.V4PricesResponseExt{
			Success: bptf.SuccessBoolProp(1),
			Items: map[string]bptf.V4PricesBaseItemDocExt{
				"Mann Co. Supply Crate Key": {
					Defindex: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
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
					Defindex: []string{"100"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
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
					Defindex: []string{"500"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
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

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		cfg := Config{CachePath: filepath.Join(t.TempDir(), "prices.json")}
		manager := NewPriceManager(r, log.Discard, cfg)

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

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		manager := NewPriceManager(r, log.Discard, Config{})
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
		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		manager := NewPriceManager(r, log.Discard, Config{SyncInterval: 10 * time.Millisecond})

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := manager.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
