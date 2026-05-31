// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"encoding/json"
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
	// ListingCommentTemplate represents the message template appended to generated marketplace listings.
	ListingCommentTemplate string `json:"listing_comment_template,omitempty"`
	// ExcludedSteamIDs contains the list of player IDs that the bot will refuse to trade with.
	ExcludedSteamIDs []string `json:"excluded_steam_ids,omitempty"`
	// TrustedSteamIDs contains the list of administrator or authorized player IDs.
	TrustedSteamIDs []string `json:"trusted_steam_ids,omitempty"`
	// ExcludedListingDescriptions contains keywords used to identify and filter out special items (e.g. spells).
	ExcludedListingDescriptions []string `json:"excluded_listing_descriptions,omitempty"`
	// PriceSwingLimits defines bounds on automatic price modifications.
	PriceSwingLimits PriceSwingLimits `json:"price_swing_limits"`
	// Items contains mapping from item SKUs to their respective trading configurations.
	Items map[string]ItemConfig `json:"items"`

	// PPUHoldDuration defines how long a cost basis entry remains valid for price protection (e.g. "24h").
	PPUHoldDuration string `json:"ppu_hold_duration"`
	// PPUGracePeriod defines how long price protection remains active after an item is sold out (e.g. "1h").
	PPUGracePeriod string `json:"ppu_grace_period"`
	// PPUMaxStockLimit represents the maximum stock level at which price protection remains active.
	PPUMaxStockLimit int `json:"ppu_max_stock_limit"`
	// PPUMinProfitScrap represents the minimum profit threshold added to the cost basis during PPU calculations.
	PPUMinProfitScrap int `json:"ppu_min_profit_scrap"`
	// CritCommandDescriptions overrides default command description strings in the chat interface.
	CritCommandDescriptions map[string]string `json:"crit_command_descriptions,omitempty"`
}

// GetPPUHoldDuration parses the [Config.PPUHoldDuration] string and returns a [time.Duration].
// It defaults to 24 hours if the string is empty or invalid.
func (c *Config) GetPPUHoldDuration() time.Duration {
	if c.PPUHoldDuration == "" {
		return 24 * time.Hour
	}

	d, err := time.ParseDuration(c.PPUHoldDuration)
	if err != nil {
		return 24 * time.Hour
	}

	return d
}

// GetPPUGracePeriod parses the [Config.PPUGracePeriod] string and returns a [time.Duration].
// It defaults to 1 hour if the string is empty or invalid.
func (c *Config) GetPPUGracePeriod() time.Duration {
	if c.PPUGracePeriod == "" {
		return 1 * time.Hour
	}

	d, err := time.ParseDuration(c.PPUGracePeriod)
	if err != nil {
		return 1 * time.Hour
	}

	return d
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

	if _, err := os.Stat(cm.path); os.IsNotExist(err) {
		cm.cfg = Config{
			GlobalMaxStock:  3000,
			DefaultMaxStock: 5,
			ExcludedListingDescriptions: []string{
				"spell", "spells", "spelled", "exorcism", "pumpkin bombs", "chromatic",
				"die job", "spectral spectrum", "putrescent pigmentation", "sinister staining",
			},
			PriceSwingLimits: PriceSwingLimits{
				MaxBuyIncrease: 0.10,
			},
			Items:             make(map[string]ItemConfig),
			PPUHoldDuration:   "24h",
			PPUGracePeriod:    "1h",
			PPUMaxStockLimit:  1,
			PPUMinProfitScrap: 1,
		}

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

	data, err := os.ReadFile(cm.path)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	if cfg.Items == nil {
		cfg.Items = make(map[string]ItemConfig)
	}

	if cfg.PPUHoldDuration == "" {
		cfg.PPUHoldDuration = "24h"
	}

	if cfg.PPUGracePeriod == "" {
		cfg.PPUGracePeriod = "1h"
	}

	if cfg.PPUMaxStockLimit <= 0 {
		cfg.PPUMaxStockLimit = 1
	}

	if cfg.PPUMinProfitScrap <= 0 {
		cfg.PPUMinProfitScrap = 1
	}

	cm.cfg = cfg

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
