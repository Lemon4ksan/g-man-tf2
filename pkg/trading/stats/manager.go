// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stats provides high-level APIs to calculate and analyze trade profitability.
package stats

import (
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// Manager integrates with TradeProfitStore to offer programmatic analytical interfaces.
type Manager struct {
	store storage.TradeProfitStore
}

// New instantiates a new StatsManager with the provided store.
func New(store storage.TradeProfitStore) *Manager {
	return &Manager{
		store: store,
	}
}

// GetProfitSummary aggregates total trades and raw key/metal changes since a given cutoff.
func (m *Manager) GetProfitSummary(since time.Duration) (storage.ProfitSummary, error) {
	return m.store.GetProfitSummary(since)
}

// GetDailyProfit returns aggregated summaries grouped by day for the last n days.
func (m *Manager) GetDailyProfit(days int) ([]storage.ProfitSummary, error) {
	return m.store.GetDailyProfit(days)
}

// GetEstimatedProfitRef converts total net changes (keys, metal, and FIFO profit)
// into a single refined metal sum based on the current key price.
func (m *Manager) GetEstimatedProfitRef(since time.Duration, keyPriceRef float64) (float64, error) {
	summary, err := m.store.GetProfitSummary(since)
	if err != nil {
		return 0, err
	}

	keysInRef := float64(summary.NetKeys) * keyPriceRef
	fifoInRef := float64(summary.FIFOProfitScrap) / 9.0

	return keysInRef + summary.NetMetalRef + fifoInRef, nil
}
