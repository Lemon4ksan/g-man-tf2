// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package storage provides interfaces and implementations for persisting tf2 logic state.
package storage

import "time"

// CostBasisEntry represents a single unit purchase cost basis log.
type CostBasisEntry struct {
	// SKU is the unique identifier for the SKU.
	SKU string `json:"sku"`
	// BuyKeys is the number of keys bought.
	BuyKeys float64 `json:"buy_keys"`
	// BuyMetal is the amount of metal bought, in refined units.
	BuyMetal float64 `json:"buy_metal"`
	// Diff is the distributed overpay/underpay in Scrap.
	Diff int `json:"diff"`
	// TradeID is the unique identifier for the trade.
	TradeID string `json:"trade_id"`
	// Timestamp is the time the cost basis entry was created.
	Timestamp time.Time `json:"timestamp"`
	// IsEstimate indicates whether the cost basis entry is an estimate.
	IsEstimate bool `json:"is_estimate"`
}

// PPUState tracks the price protection unit details for a SKU.
type PPUState struct {
	// SKU is the unique identifier for the SKU.
	SKU string `json:"sku"`
	// LastInStockTime is the time the SKU was last in stock.
	LastInStockTime time.Time `json:"last_in_stock_time"`
	// LastSoldTime is the time the SKU was last sold.
	LastSoldTime time.Time `json:"last_sold_time"`
	// IsPartialPriced indicates whether the SKU is partially priced.
	IsPartialPriced bool `json:"is_partial_priced"`
	// ProtectionStarted is the time the price protection started for the SKU.
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

// TradeProfitLog represents the financial ledger record of a completed trade.
type TradeProfitLog struct {
	// TradeID is the unique identifier for the trade.
	TradeID string `json:"trade_id"`
	// Timestamp is the time the trade was completed.
	Timestamp time.Time `json:"timestamp"`
	// NetKeys is the net change in key count (received - given).
	NetKeys int `json:"net_keys"`
	// NetMetalRef is the net change in refined metal count.
	NetMetalRef float64 `json:"net_metal_ref"`
	// FIFOProfitScrap is the realized FIFO profit/loss of regular items in Scrap.
	FIFOProfitScrap int `json:"fifo_profit_scrap"`
	// IsEstimate indicates whether FIFO estimation fallback was used.
	IsEstimate bool `json:"is_estimate"`
}

// ProfitSummary holds aggregated keys and metal profit.
type ProfitSummary struct {
	// TotalTrades is the total number of trades summarized.
	TotalTrades int `json:"total_trades"`
	// NetKeys is the net change in key count (received - given).
	NetKeys int `json:"net_keys"`
	// NetMetalRef is the net change in refined metal count.
	NetMetalRef float64 `json:"net_metal_ref"`
	// FIFOProfitScrap is the realized FIFO profit/loss of regular items in Scrap.
	FIFOProfitScrap int `json:"fifo_profit_scrap"`
}

// TradeProfitStore interface manages persisting and querying trade profit logs.
type TradeProfitStore interface {
	// Push appends a new trade profit log entry.
	Push(entry TradeProfitLog) error
	// GetProfitSummary calculates aggregated profit stats over a given duration.
	GetProfitSummary(since time.Duration) (ProfitSummary, error)
	// GetDailyProfit returns aggregated daily profit summaries for the last n days.
	GetDailyProfit(days int) ([]ProfitSummary, error)
	// Prune removes trade profit logs that are older than the given keep duration.
	Prune(keepDuration time.Duration) error
}
