// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricemanager provides a backpack.tf price manager for the g-man-tf2 bot.
package pricemanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

const BehaviorName = "bptf_prices"

var ErrCachePathNotConfigured = errors.New("pricemanager: cache path not configured")

func WithPriceManager(orch *behavior.Orchestrator, client *bptf.Client, cfg Config) {
	orch.Register(NewPriceManager(client, orch.Logger(), cfg))
}

type Config struct {
	CachePath    string
	SyncInterval time.Duration
}

func DefaultConfig() Config {
	return Config{
		CachePath:    "cache/tf2/prices.json",
		SyncInterval: 2 * time.Hour,
	}
}

// PriceManager manages price fetching, disk caching, and indexing from backpack.tf.
//
// Thread Safety:
//   - Fully thread-safe. Reads are guarded by RWMutex, mutations acquire exclusive Lock.
type PriceManager struct {
	config Config
	bptf   *bptf.Client
	logger log.Logger

	mu    sync.RWMutex
	index map[string]bptf.PriceEntry
}

func NewPriceManager(c *bptf.Client, l log.Logger, cfg Config) *PriceManager {
	return &PriceManager{
		config: cfg,
		bptf:   c,
		logger: l.With(log.Module(BehaviorName)),
		index:  make(map[string]bptf.PriceEntry),
	}
}

func (m *PriceManager) Name() string { return BehaviorName }

func (m *PriceManager) Run(ctx context.Context) error {
	m.logger.Info("BPTF Price Sync behavior started", log.Duration("interval", m.config.SyncInterval))

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	if m.isIndexEmpty() {
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

func (m *PriceManager) isIndexEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.index) == 0
}

func (m *PriceManager) Update(ctx context.Context) error {
	m.logger.Debug("Fetching full pricelist from backpack.tf...")

	resp, err := m.bptf.GetPricesV4(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("bptf update failed: %w", err)
	}

	newIndex := make(map[string]bptf.PriceEntry, len(resp.Items)*2)

	for _, itemData := range resp.Items {
		if len(itemData.Defindexes) == 0 {
			continue
		}

		defindex, _ := strconv.Atoi(itemData.Defindexes[0])
		m.indexItemPrices(defindex, itemData.Prices, newIndex)
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

func (m *PriceManager) indexItemPrices(
	defindex int,
	prices map[string]map[string]map[string]map[string]bptf.PriceEntry,
	outIndex map[string]bptf.PriceEntry,
) {
	normDef := schema.NormalizeDefindex(defindex)

	for quality, tradableMap := range prices {
		qInt, _ := strconv.Atoi(quality)
		indexQualityPrices(normDef, qInt, tradableMap, outIndex)
	}
}

func indexQualityPrices(
	defindex, qInt int,
	tradableMap map[string]map[string]map[string]bptf.PriceEntry,
	outIndex map[string]bptf.PriceEntry,
) {
	for tradable, craftableMap := range tradableMap {
		isTradable := tradable == "Tradable"

		for craftable, priceIndexMap := range craftableMap {
			isCraftable := craftable == "Craftable"

			for pIndex, entry := range priceIndexMap {
				sItem := &sku.Item{
					Defindex:  defindex,
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

				outIndex[sku.FromObject(sItem)] = entry
			}
		}
	}
}

func (m *PriceManager) GetPrice(sku string) (bptf.PriceEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.index[sku]

	return entry, ok
}

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
		return ErrCachePathNotConfigured
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
