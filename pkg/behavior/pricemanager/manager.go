// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricemanager provides a backpack.tf price manager for the g-man-tf2 bot.
package pricemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// BehaviorName is the name of the price manager behavior.
const BehaviorName = "bptf_prices"

// WithPriceManager registers the price manager behavior with the orchestrator.
func WithPriceManager(orch *behavior.Orchestrator, client *bptf.Client, cfg Config) {
	orch.Register(NewPriceManager(client, orch.Logger(), cfg))
}

// Config holds the configuration for the price manager.
type Config struct {
	// CachePath is the path to the local price cache file.
	CachePath string
	// SyncInterval is the interval between price updates.
	SyncInterval time.Duration
}

// DefaultConfig returns a Config with production-ready defaults.
func DefaultConfig() Config {
	return Config{
		CachePath:    "cache/tf2/prices.json",
		SyncInterval: 2 * time.Hour,
	}
}

// PriceManager manages backpack.tf prices.
type PriceManager struct {
	config Config
	client *bptf.Client
	logger log.Logger

	mu    sync.RWMutex
	index map[string]bptf.PriceEntry
}

// NewPriceManager creates a new price manager.
func NewPriceManager(c *bptf.Client, l log.Logger, cfg Config) *PriceManager {
	return &PriceManager{
		config: cfg,
		client: c,
		logger: l.With(log.Module(BehaviorName)),
		index:  make(map[string]bptf.PriceEntry),
	}
}

// Name returns the unique name of the behavior.
func (m *PriceManager) Name() string {
	return BehaviorName
}

// Run starts the automated price synchronization loop.
func (m *PriceManager) Run(ctx context.Context) error {
	m.logger.Info("BPTF Price Sync behavior started", log.Duration("interval", m.config.SyncInterval))

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	// Initial update if index is empty
	m.mu.RLock()
	empty := len(m.index) == 0
	m.mu.RUnlock()

	if empty {
		if err := m.Update(ctx); err != nil {
			m.logger.Error("Initial price update failed", log.Err(err))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.Update(ctx); err != nil {
				m.logger.Error("Price update failed", log.Err(err))
			}
		}
	}
}

// Update downloads the price list and rebuilds the index.
func (m *PriceManager) Update(ctx context.Context) error {
	m.logger.Debug("Fetching full pricelist from backpack.tf...")

	res, err := m.client.GetPricesV4(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("bptf update failed: %w", err)
	}

	newIndex := make(map[string]bptf.PriceEntry)

	// Quality -> Tradability -> Craftability -> PriceIndex -> PriceEntry.
	for _, itemData := range res.Items {
		// An item can have multiple defindexes (Valve shenanigans)
		// We'll take the first one, as they usually overlap for different styles.
		if len(itemData.Defindexes) == 0 {
			continue
		}

		defindex, _ := strconv.Atoi(itemData.Defindexes[0])

		for quality, tradableMap := range itemData.Prices {
			qInt, _ := strconv.Atoi(quality)

			for tradable, craftableMap := range tradableMap {
				isTradable := tradable == "Tradable"

				for craftable, priceIndexMap := range craftableMap {
					isCraftable := craftable == "Craftable"

					for pIndex, entry := range priceIndexMap {
						sItem := &sku.Item{
							Defindex:  schema.NormalizeDefindex(defindex),
							Quality:   qInt,
							Tradable:  isTradable,
							Craftable: isCraftable,
						}

						if pInt, err := strconv.Atoi(pIndex); err == nil && pInt != 0 {
							if qInt == schema.QualityUnusual {
								sItem.Effect = pInt
							} else {
								sItem.Crateseries = pInt
							}
						}

						skuStr := sku.FromObject(sItem)
						newIndex[skuStr] = entry
					}
				}
			}
		}
	}

	m.mu.Lock()
	m.index = newIndex
	m.mu.Unlock()

	if err := m.saveToCache(); err != nil {
		m.logger.Warn("Failed to save prices to cache", log.Err(err))
	}

	m.logger.Info("Bptf index rebuilt", log.Int("unique_skus", len(newIndex)))

	return nil
}

// GetPrice returns the price for the SKU from memory.
func (m *PriceManager) GetPrice(sku string) (bptf.PriceEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.index[sku]

	return entry, ok
}

// Load loads the price list from cache. Returns error if cache is missing or invalid.
func (m *PriceManager) Load() error {
	return m.loadFromCache()
}

func (m *PriceManager) saveToCache() error {
	if m.config.CachePath == "" {
		return nil
	}

	m.mu.RLock()
	index := m.index
	m.mu.RUnlock()

	data, err := json.Marshal(index)
	if err != nil {
		return err
	}

	dir := filepath.Dir(m.config.CachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(m.config.CachePath, data, 0o644)
}

func (m *PriceManager) loadFromCache() error {
	if m.config.CachePath == "" {
		return errors.New("cache path not configured")
	}

	data, err := os.ReadFile(m.config.CachePath)
	if err != nil {
		return err
	}

	var index map[string]bptf.PriceEntry
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	m.mu.Lock()
	m.index = index
	m.mu.Unlock()

	return nil
}
