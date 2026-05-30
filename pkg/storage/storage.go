// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package storage provides interfaces and implementations for persisting tf2 logic state.
package storage

import "time"

// CostBasisEntry represents a single unit purchase cost basis log.
type CostBasisEntry struct {
	SKU        string    `json:"sku"`
	BuyKeys    float64   `json:"buy_keys"`
	BuyMetal   float64   `json:"buy_metal"` // In refined
	Diff       int       `json:"diff"`      // Distributed overpay/underpay in Scrap
	TradeID    string    `json:"trade_id"`
	Timestamp  time.Time `json:"timestamp"`
	IsEstimate bool      `json:"is_estimate"`
}

// PPUState tracks the price protection unit details for a SKU.
type PPUState struct {
	SKU               string    `json:"sku"`
	LastInStockTime   time.Time `json:"last_in_stock_time"`
	LastSoldTime      time.Time `json:"last_sold_time"`
	IsPartialPriced   bool      `json:"is_partial_priced"`
	ProtectionStarted time.Time `json:"protection_started"`
}

// CostBasisStore is an interface for managing cost basis data for PPU state tracking.
type CostBasisStore interface {
	// GetAllPPUStates returns all PPU states currently tracked in the store.
	GetAllPPUStates() map[string]PPUState

	// GetOldestEntry returns the oldest cost basis entry for the given SKU, if one exists.
	GetOldestEntry(sku string) (CostBasisEntry, bool)

	// GetPPUState returns the PPU state for the given SKU, if one exists.
	GetPPUState(sku string) (PPUState, bool)

	// Pop removes and returns the oldest cost basis entry for the given SKU, if one exists.
	Pop(sku string) (CostBasisEntry, bool)

	// Prune removes cost basis entries that are older than the given hold duration.
	Prune(holdDuration time.Duration)

	// Push adds a new cost basis entry for the given SKU.
	Push(sku string, entry CostBasisEntry)

	// SetPPUState sets the PPU state for the given SKU.
	SetPPUState(sku string, state PPUState)
}
