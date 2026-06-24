// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
)

// ItemConfig defines the trading limits, stock rules, and price thresholds for a specific item SKU.
type ItemConfig struct {
	// SKU represents the canonical stock keeping unit identifier for the item.
	SKU string `json:"sku"`
	// Name represents the human-readable display name of the item.
	Name string `json:"name,omitempty"`
	// MaxStock represents the maximum number of copies of this item the bot is allowed to hold.
	MaxStock int `json:"max_stock"`
	// MinStock represents the minimum number of copies of this item the bot must keep before selling is enabled.
	MinStock int `json:"min_stock"`
	// EnableBuy indicates whether buying is enabled for this item.
	EnableBuy bool `json:"enable_buy"`
	// EnableSell indicates whether selling is enabled for this item.
	EnableSell bool `json:"enable_sell"`
	// MinBuyPrice represents the lowest buy price allowed for automatic trades.
	MinBuyPrice currency.Currency `json:"min_buy_price"`
	// MaxBuyPrice represents the highest buy price allowed for automatic trades.
	MaxBuyPrice currency.Currency `json:"max_buy_price"`
	// MinSellPrice represents the lowest sell price allowed for automatic trades.
	MinSellPrice currency.Currency `json:"min_sell_price"`
	// MaxSellPrice represents the highest sell price allowed for automatic trades.
	MaxSellPrice currency.Currency `json:"max_sell_price"`
	// FixedBuyPrice represents the fixed manual buy price that overrides automatic pricing.
	FixedBuyPrice *currency.Currency `json:"fixed_buy_price,omitempty"`
	// FixedSellPrice represents the fixed manual sell price that overrides automatic pricing.
	FixedSellPrice *currency.Currency `json:"fixed_sell_price,omitempty"`
}

// PriceSwingLimits defines the maximum percentage changes allowed in a single price update.
type PriceSwingLimits struct {
	// MaxBuyIncrease represents the maximum percentage increase allowed for buying.
	MaxBuyIncrease float64 `json:"max_buy_increase"`
	// MaxSellDecrease represents the maximum percentage decrease allowed for selling.
	MaxSellDecrease float64 `json:"max_sell_decrease"`
}

// Config holds the top-level trading strategy and inventory configuration rules.
type Config struct {
	// GlobalMaxStock represents the absolute maximum capacity of the bot's inventory across all items.
	GlobalMaxStock int `json:"global_max_stock"`
	// DefaultMaxStock represents the fallback limit applied to items without an explicit SKU configuration.
	DefaultMaxStock int `json:"default_max_stock"`
	// Items contains mapping from item SKUs to their respective trading configurations.
	Items map[string]ItemConfig `json:"items"`
	// UseSeparateKeyRates forces the valuation of keys to use the sell price when giving keys, and the buy price when receiving keys.
	UseSeparateKeyRates bool `json:"use_separate_key_rates"`
	// EnableAutoCancelStaleOffers, if true, automatically cancels sent trade offers that have been pending/active for too long.
	EnableAutoCancelStaleOffers bool `json:"enable_auto_cancel_stale_offers"`
	// CancelStaleOffersAfter defines how long an active sent trade offer remains pending before being automatically cancelled (e.g. "15m", "30m").
	CancelStaleOffersAfter string `json:"cancel_stale_offers_after"`
	// CritCommandDescriptions overrides default command description strings in the chat interface.
	CritCommandDescriptions map[string]string `json:"crit_command_descriptions,omitempty"`
	// FallbackSpellPremiums maps spell names to their fallback premiums in refined metal (ref).
	FallbackSpellPremiums map[string]float64 `json:"fallback_spell_premiums,omitempty"`
	// BackpackSortingSections defines custom layout sections for inventory sorting.
	BackpackSortingSections []BackpackSectionConfig `json:"backpack_sorting_sections,omitempty"`
}

// BackpackSectionConfig defines the custom layout properties for a single backpack category section.
type BackpackSectionConfig struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

// ConfigManager coordinates thread-safe loading, saving, and hot-reload polling of the [Config].
type ConfigManager struct {
	mu           sync.RWMutex
	path         string
	cfg          Config
	lastModified time.Time
}

// NewConfigManager loads a [ConfigManager] from the specified JSON file.
// It automatically initializes a default [Config] template file on disk if the path is missing.
// Returns an error if the directory cannot be created or the file is unreadable.
func NewConfigManager(path string) (*ConfigManager, error) {
	cm := &ConfigManager{path: path}
	if err := cm.Load(); err != nil {
		return nil, err
	}

	return cm, nil
}

// Load reads, parses, and validates the JSON configuration file from disk.
// Returns an error if file access is restricted or the JSON is syntax-invalid.
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
			"Exorcism":                  3.0,
			"Voices from Below":         5.0,
			"Pumpkin Bombs":             10.0,
			"Gourd Grenades":            10.0,
			"Squash Rockets":            10.0,
			"Sentry Quad-Pumpkins":      10.0,
			"Halloween Fire":            15.0,
			"Spectral Flame":            15.0,
			"Die Job":                   10.0,
			"Chromatic Corruption":      10.0,
			"Putrescent Pigmentation":   10.0,
			"Spectral Spectrum":         10.0,
			"Sinister Staining":         10.0,
			"Team Spirit Footprints":    40.0,
			"Headless Horseshoes":       40.0,
			"Gangreen Footprints":       40.0,
			"Corpse Gray Footprints":    40.0,
			"Violent Violet Footprints": 40.0,
			"Rotten Orange Footprints":  40.0,
			"Bruised Purple Footprints": 40.0,
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

			if err := os.WriteFile(cm.path, data, 0o644); err != nil {
				return err
			}

			if info, err := os.Stat(cm.path); err == nil {
				cm.lastModified = info.ModTime()
			}

			return nil
		}

		return err
	}

	if err := json.Unmarshal(data, &defaultCfg); err != nil {
		return err
	}

	if defaultCfg.Items == nil {
		defaultCfg.Items = make(map[string]ItemConfig)
	}

	if defaultCfg.BackpackSortingSections == nil {
		defaultCfg.BackpackSortingSections = []BackpackSectionConfig{}
	}

	cm.cfg = defaultCfg

	if info, err := os.Stat(cm.path); err == nil {
		cm.lastModified = info.ModTime()
	}

	return nil
}

// StartWatching launches a background polling worker to detect file changes and trigger [ConfigManager.Load].
// The hot-reload loop terminates automatically when the provided [context.Context] is cancelled.
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

					if err := cm.Load(); err != nil {
						logger.Error("Failed to auto-reload config file", log.Err(err))
					} else {
						logger.Info("Config file reloaded successfully")
					}
				}
			}
		}
	}()
}

// GetConfig returns the full thread-safe copy of the trading configuration.
func (cm *ConfigManager) GetConfig() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cfg
}

// GetItemConfig returns configuration for a specific SKU.
func (cm *ConfigManager) GetItemConfig(sku string) (ItemConfig, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	item, ok := cm.cfg.Items[sku]

	return item, ok
}
