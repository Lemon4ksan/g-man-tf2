// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricelistsaver provides a high-performance background writer that persisting
// pricedb updates to local pricelist JSON files with robust debouncing.
package pricelistsaver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

// BehaviorName is the unique name of the pricelist saver behavior.
const BehaviorName = "pricelist_saver"

// Config holds configuration parameters for the PricelistSaver.
type Config struct {
	// PricelistPath is the target file path where the pricelist is saved.
	PricelistPath string `json:"pricelist_path"`
	// SilenceWindow defines the debouncing delay used to aggregate rapid price updates.
	SilenceWindow time.Duration `json:"silence_window"`
	// MaxDelay defines the maximum allowed delay before a pending write is flushed to disk.
	MaxDelay time.Duration `json:"max_delay"`
}

// DefaultConfig returns a [Config] containing production-ready default paths and delays.
func DefaultConfig() Config {
	return Config{
		PricelistPath: "cache/tf2/pricelist.json",
		SilenceWindow: 500 * time.Millisecond,
		MaxDelay:      5 * time.Second,
	}
}

// PriceProvider defines the interface required to snapshot prices from pricedb.Manager.
type PriceProvider interface {
	// GetAllPrices returns a copy of all currently cached prices in a thread-safe way.
	GetAllPrices() []*pricedb.Price
}

// PricelistSaver persists real-time price updates atomically to disk using write debouncing.
// Use [Save] or [New] to register it with the orchestrator.
type PricelistSaver struct {
	config   Config
	logger   log.Logger
	bus      *bus.Bus
	priceMgr PriceProvider
	mu       sync.Mutex
}

// Save returns a [behavior.Option] that registers the [PricelistSaver] with the orchestrator.
func Save(priceMgr PriceProvider, cfg Config) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(priceMgr, o.Bus(), o.Logger(), cfg))
	}
}

// New creates a new [PricelistSaver] behavior.
func New(priceMgr PriceProvider, b *bus.Bus, logger log.Logger, cfg Config) *PricelistSaver {
	if cfg.PricelistPath == "" {
		cfg.PricelistPath = DefaultConfig().PricelistPath
	}

	if cfg.SilenceWindow == 0 {
		cfg.SilenceWindow = DefaultConfig().SilenceWindow
	}

	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = DefaultConfig().MaxDelay
	}

	return &PricelistSaver{
		config:   cfg,
		logger:   logger.With(log.Module(BehaviorName)),
		bus:      b,
		priceMgr: priceMgr,
	}
}

// Name returns the unique name of the [PricelistSaver] behavior.
func (s *PricelistSaver) Name() string {
	return BehaviorName
}

// Run starts the background event loop, debouncing updates and flushing them to disk.
// Returns an error if the context is cancelled.
func (s *PricelistSaver) Run(ctx context.Context) error {
	s.logger.Info("PricelistSaver started", log.String("path", s.config.PricelistPath))

	sub := s.bus.Subscribe(&pricedb.PricelistUpdatedEvent{})
	defer sub.Unsubscribe()

	var (
		timer         *time.Timer
		timerChan     <-chan time.Time
		firstChangeAt time.Time
	)

	flush := func() {
		if err := s.SavePricelist(); err != nil {
			s.logger.Error("Failed to save pricelist", log.Err(err))
		} else {
			s.logger.Debug("Pricelist successfully flushed to disk")
		}

		timerChan = nil
		firstChangeAt = time.Time{}
	}

	for {
		select {
		case <-ctx.Done():
			if timerChan != nil {
				flush()
			}

			if timer != nil {
				timer.Stop()
			}

			return ctx.Err()

		case <-sub.C():
			s.logger.Debug("Price change detected, queuing save...")

			now := time.Now()

			if firstChangeAt.IsZero() {
				firstChangeAt = now
			}

			delay := s.config.SilenceWindow
			if now.Add(delay).After(firstChangeAt.Add(s.config.MaxDelay)) {
				delay = max(firstChangeAt.Add(s.config.MaxDelay).Sub(now), 0)
			}

			if timer == nil {
				timer = time.NewTimer(delay)
				timerChan = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}

				timer.Reset(delay)
				timerChan = timer.C
			}

		case <-timerChan:
			flush()
		}
	}
}

// SavePricelist retrieves all prices and writes them atomically to disk.
// Returns an error if reading, marshalling, or writing fails.
func (s *PricelistSaver) SavePricelist() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prices := s.priceMgr.GetAllPrices()

	type EntryData struct {
		Defindex  int                `json:"defindex"`
		Quality   int                `json:"quality"`
		Autoprice bool               `json:"autoprice"`
		Min       int                `json:"min"`
		Max       int                `json:"max"`
		Buy       pricedb.Currencies `json:"buy"`
		Sell      pricedb.Currencies `json:"sell"`
	}

	dataMap := make(map[string]EntryData)

	existingBytes, err := os.ReadFile(s.config.PricelistPath)
	if err == nil {
		_ = json.Unmarshal(existingBytes, &dataMap)
	} else if !os.IsNotExist(err) {
		s.logger.Warn(
			"Failed to read existing pricelist file",
			log.Err(err),
			log.String("path", s.config.PricelistPath),
		)
	}

	for _, p := range prices {
		var defindex, quality int

		parts := strings.Split(p.SKU, ";")
		if len(parts) >= 2 {
			defindex, _ = strconv.Atoi(parts[0])
			quality, _ = strconv.Atoi(parts[1])
		} else if len(parts) == 1 {
			if val, err := strconv.Atoi(parts[0]); err == nil {
				defindex = val
			}

			quality = 6
		}

		autoprice := p.Source != "Manual" && p.Source != "Autokeys"

		if existing, exists := dataMap[p.SKU]; exists {
			existing.Buy = p.Buy
			existing.Sell = p.Sell
			existing.Autoprice = autoprice
			dataMap[p.SKU] = existing
		} else {
			dataMap[p.SKU] = EntryData{
				Defindex:  defindex,
				Quality:   quality,
				Autoprice: autoprice,
				Min:       0,
				Max:       1,
				Buy:       p.Buy,
				Sell:      p.Sell,
			}
		}
	}

	newBytes, err := json.MarshalIndent(dataMap, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.config.PricelistPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpPath := s.config.PricelistPath + ".tmp"
	if err := os.WriteFile(tmpPath, newBytes, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.config.PricelistPath)
}
