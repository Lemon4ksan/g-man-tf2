// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manualprices

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricing"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage/jsonfile"
)

type mockPriceProvider struct {
	mu     sync.Mutex
	prices map[string]currency.Currency
	source map[string]pricing.Source
}

func (m *mockPriceProvider) SetPrice(sku string, buy, sell currency.Currency, source pricing.Source) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prices[sku+":buy"] = buy
	m.prices[sku+":sell"] = sell
	m.source[sku] = source
}

func TestManager_LoadAndApply(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manualprices-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "manual_prices.json")

	t.Run("Applies static configuration prices when no dynamic prices exist", func(t *testing.T) {
		priceMgr := &mockPriceProvider{
			prices: make(map[string]currency.Currency),
			source: make(map[string]pricing.Source),
		}
		store, err := jsonfile.NewManualPricesStore(filePath, log.Discard)
		require.NoError(t, err)

		// Create static prices configuration callback
		getStaticCfg := func() map[string]PriceEntry {
			return map[string]PriceEntry{
				"5021;6": {
					Buy:  currency.Currency{Keys: 1, Metal: 0},
					Sell: currency.Currency{Keys: 1, Metal: 5},
				},
			}
		}

		mgr := New(store, priceMgr, getStaticCfg, log.Discard)
		err = mgr.LoadAndApply()
		require.NoError(t, err)

		priceMgr.mu.Lock()
		buy, existsBuy := priceMgr.prices["5021;6:buy"]
		sell, existsSell := priceMgr.prices["5021;6:sell"]
		src := priceMgr.source["5021;6"]
		priceMgr.mu.Unlock()

		require.True(t, existsBuy)
		require.True(t, existsSell)
		assert.Equal(t, 1.0, buy.Keys)
		assert.Equal(t, 0.0, buy.Metal)
		assert.Equal(t, 1.0, sell.Keys)
		assert.Equal(t, 5.0, sell.Metal)
		assert.Equal(t, pricing.SourceManual, src)
	})

	t.Run("Dynamic manual prices override static configuration prices", func(t *testing.T) {
		priceMgr := &mockPriceProvider{
			prices: make(map[string]currency.Currency),
			source: make(map[string]pricing.Source),
		}
		store, err := jsonfile.NewManualPricesStore(filePath, log.Discard)
		require.NoError(t, err)

		// Set dynamic manual price in store
		err = store.Set("5021;6", storage.ManualPriceEntry{
			BuyKeys:   2,
			BuyMetal:  10.0,
			SellKeys:  2,
			SellMetal: 20.0,
		})
		require.NoError(t, err)

		getStaticCfg := func() map[string]PriceEntry {
			return map[string]PriceEntry{
				"5021;6": {
					Buy:  currency.Currency{Keys: 1, Metal: 0},
					Sell: currency.Currency{Keys: 1, Metal: 5},
				},
			}
		}

		mgr := New(store, priceMgr, getStaticCfg, log.Discard)
		err = mgr.LoadAndApply()
		require.NoError(t, err)

		priceMgr.mu.Lock()
		buy, existsBuy := priceMgr.prices["5021;6:buy"]
		sell, existsSell := priceMgr.prices["5021;6:sell"]
		src := priceMgr.source["5021;6"]
		priceMgr.mu.Unlock()

		require.True(t, existsBuy)
		require.True(t, existsSell)
		assert.Equal(t, 2.0, buy.Keys)
		assert.Equal(t, 10.0, buy.Metal)
		assert.Equal(t, 2.0, sell.Keys)
		assert.Equal(t, 20.0, sell.Metal)
		assert.Equal(t, pricing.SourceManual, src)
	})

	t.Run("Dynamically setting price writes to store and updates in-memory immediately", func(t *testing.T) {
		_ = os.Remove(filePath)

		priceMgr := &mockPriceProvider{
			prices: make(map[string]currency.Currency),
			source: make(map[string]pricing.Source),
		}
		store, err := jsonfile.NewManualPricesStore(filePath, log.Discard)
		require.NoError(t, err)

		mgr := New(store, priceMgr, nil, log.Discard)

		err = mgr.SetPrice("5002;6", currency.Currency{Metal: 1.0}, currency.Currency{Metal: 1.05})
		require.NoError(t, err)

		// Assert immediate in-memory update
		priceMgr.mu.Lock()
		buy := priceMgr.prices["5002;6:buy"]
		sell := priceMgr.prices["5002;6:sell"]
		priceMgr.mu.Unlock()

		assert.Equal(t, 1.0, buy.Metal)
		assert.Equal(t, 1.05, sell.Metal)

		// Assert persistent JSON file update via store
		saved, err := store.GetAll()
		require.NoError(t, err)

		savedEntry, exists := saved["5002;6"]
		require.True(t, exists)
		assert.Equal(t, 1.0, savedEntry.BuyMetal)
		assert.Equal(t, 1.05, savedEntry.SellMetal)
	})
}
