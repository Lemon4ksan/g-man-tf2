// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package trading implements Team Fortress 2 automated trading, valuation, and security logic.
package trading

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
)

type ItemConfig struct {
	SKU            string             `json:"sku"`
	Name           string             `json:"name,omitempty"`
	MaxStock       int                `json:"max_stock"`
	MinStock       int                `json:"min_stock"`
	EnableBuy      bool               `json:"enable_buy"`
	EnableSell     bool               `json:"enable_sell"`
	MinBuyPrice    currency.Currency  `json:"min_buy_price"`
	MaxBuyPrice    currency.Currency  `json:"max_buy_price"`
	MinSellPrice   currency.Currency  `json:"min_sell_price"`
	MaxSellPrice   currency.Currency  `json:"max_sell_price"`
	FixedBuyPrice  *currency.Currency `json:"fixed_buy_price,omitempty"`
	FixedSellPrice *currency.Currency `json:"fixed_sell_price,omitempty"`
}

type PriceSwingLimits struct {
	MaxBuyIncrease  float64 `json:"max_buy_increase"`
	MaxSellDecrease float64 `json:"max_sell_decrease"`
}

type Config struct {
	GlobalMaxStock              int                     `json:"global_max_stock"`
	DefaultMaxStock             int                     `json:"default_max_stock"`
	Items                       map[string]ItemConfig   `json:"items"`
	UseSeparateKeyRates         bool                    `json:"use_separate_key_rates"`
	EnableAutoCancelStaleOffers bool                    `json:"enable_auto_cancel_stale_offers"`
	CancelStaleOffersAfter      string                  `json:"cancel_stale_offers_after"`
	CritCommandDescriptions     map[string]string       `json:"crit_command_descriptions,omitempty"`
	FallbackSpellPremiums       map[string]float64      `json:"fallback_spell_premiums,omitempty"`
	BackpackSortingSections     []BackpackSectionConfig `json:"backpack_sorting_sections,omitempty"`
}

type BackpackSectionConfig struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

// ConfigManager handles thread-safe loading and hot-reloading of trading configurations.
type ConfigManager struct {
	mu           sync.RWMutex
	path         string
	cfg          Config
	lastModified time.Time
}

func NewConfigManager(path string) (*ConfigManager, error) {
	cm := &ConfigManager{path: path}
	if err := cm.Load(); err != nil {
		return nil, err
	}

	return cm, nil
}

func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	defaultCfg := Config{
		GlobalMaxStock:              3000,
		DefaultMaxStock:             5,
		Items:                       make(map[string]ItemConfig),
		EnableAutoCancelStaleOffers: false,
		CancelStaleOffersAfter:      "15m",
		BackpackSortingSections:     []BackpackSectionConfig{},
		FallbackSpellPremiums: map[string]float64{
			"Exorcism": 3.0, "Voices from Below": 5.0, "Pumpkin Bombs": 10.0,
		},
	}

	data, err := os.ReadFile(cm.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cm.cfg = defaultCfg

			if err := os.MkdirAll(filepath.Dir(cm.path), 0o755); err != nil {
				return err
			}

			data, err := json.MarshalIndent(cm.cfg, "", "  ")
			if err != nil {
				return err
			}

			return os.WriteFile(cm.path, data, 0o644)
		}

		return err
	}

	if err := json.Unmarshal(data, &defaultCfg); err != nil {
		return err
	}

	cm.cfg = defaultCfg

	if info, err := os.Stat(cm.path); err == nil {
		cm.lastModified = info.ModTime()
	}

	return nil
}

func (cm *ConfigManager) StartWatching(ctx context.Context, interval time.Duration, logger log.Logger) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(cm.path)
				if err != nil {
					continue
				}

				cm.mu.RLock()
				lastMod := cm.lastModified
				cm.mu.RUnlock()

				if info.ModTime().After(lastMod) {
					logger.Info("Config file modification detected, reloading...", log.String("path", cm.path))
					_ = cm.Load()
				}
			}
		}
	}()
}

func (cm *ConfigManager) GetConfig() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.cfg
}

func (cm *ConfigManager) GetItemConfig(sku string) (ItemConfig, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	item, ok := cm.cfg.Items[sku]

	return item, ok
}
