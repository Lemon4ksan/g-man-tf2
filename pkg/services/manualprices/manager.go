// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package manualprices provides a service for managing manual buy/sell prices.
package manualprices

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricing"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// PriceEntry represents a manual buy/sell price set dynamically using core domain types.
type PriceEntry struct {
	Buy  currency.Currency `json:"buy"`
	Sell currency.Currency `json:"sell"`
}

// PriceProvider defines a domain-level interface for updating pricing structures.
// It is completely decoupled from any specific external price database implementation.
type PriceProvider interface {
	SetPrice(sku string, buy, sell currency.Currency, source pricing.Source)
}

// Manager orchestrates static manual prices from config and dynamic manual prices from storage,
// registering them cleanly into the active price provider.
type Manager struct {
	store        storage.ManualPriceStore
	priceMgr     PriceProvider
	getStaticCfg func() map[string]PriceEntry
	logger       log.Logger
	mu           sync.Mutex
	lastModified time.Time
}

// New creates and returns a new [Manager] instance.
func New(
	store storage.ManualPriceStore,
	priceMgr PriceProvider,
	getStaticCfg func() map[string]PriceEntry,
	logger log.Logger,
) *Manager {
	return &Manager{
		store:        store,
		priceMgr:     priceMgr,
		getStaticCfg: getStaticCfg,
		logger:       logger.With(log.String("module", "manual_prices")),
	}
}

// LoadAndApply merges initial static manual prices from configuration
// and dynamic manual prices from persistent storage, applying the priority hierarchy.
func (m *Manager) LoadAndApply() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resolvedPrices := make(map[string]PriceEntry)

	if m.getStaticCfg != nil {
		maps.Copy(resolvedPrices, m.getStaticCfg())
	}

	modTime, err := m.store.GetModTime()
	if err == nil {
		dynamicPrices, err := m.store.GetAll()
		if err == nil {
			for sku, entry := range dynamicPrices {
				// Dynamic price overrules static configuration price
				resolvedPrices[sku] = PriceEntry{
					Buy: currency.Currency{
						Keys:  float64(entry.BuyKeys),
						Metal: entry.BuyMetal,
					},
					Sell: currency.Currency{
						Keys:  float64(entry.SellKeys),
						Metal: entry.SellMetal,
					},
				}
			}
		}

		m.lastModified = modTime
	} else {
		return err
	}

	for sku, entry := range resolvedPrices {
		m.priceMgr.SetPrice(sku, entry.Buy, entry.Sell, pricing.SourceManual)
	}

	m.logger.Info("Successfully resolved and applied manual prices",
		log.Int("total_resolved", len(resolvedPrices)),
	)

	return nil
}

// SetPrice writes a new manual price for a SKU to the persistent store,
// which automatically takes highest priority, and triggers an immediate in-memory update.
func (m *Manager) SetPrice(sku string, buy, sell currency.Currency) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.store.Set(sku, storage.ManualPriceEntry{
		BuyKeys:   int(buy.Keys),
		BuyMetal:  buy.Metal,
		SellKeys:  int(sell.Keys),
		SellMetal: sell.Metal,
	})
	if err != nil {
		return err
	}

	m.priceMgr.SetPrice(sku, buy, sell, pricing.SourceManual)

	m.logger.Info("Dynamically set manual price",
		log.String("sku", sku),
		log.Float64("buy_keys", buy.Keys),
		log.Float64("buy_metal", buy.Metal),
		log.Float64("sell_keys", sell.Keys),
		log.Float64("sell_metal", sell.Metal),
	)

	return nil
}

// StartWatcher starts a background loop that monitors persistent storage
// for external modifications (e.g., from a third-party REST API modifying the JSON file)
// and automatically reloads them.
func (m *Manager) StartWatcher(ctx context.Context, checkInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				modTime, err := m.store.GetModTime()
				if err == nil {
					m.mu.Lock()
					isModified := !modTime.Equal(m.lastModified)
					m.mu.Unlock()

					if isModified {
						if err := m.LoadAndApply(); err != nil {
							m.logger.Error("Failed to auto-reload manual prices", log.Err(err))
						}
					}
				}
			}
		}
	}()
}
