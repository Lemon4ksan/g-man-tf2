// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonfile

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

func TestStatsStore_PushAndPersist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stats.json")
	logger := log.Discard

	store, err := NewStatsStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		store.Start(ctx)
		close(done)
	}()

	log1 := storage.TradeProfitLog{
		TradeID:         "123",
		Timestamp:       time.Now(),
		NetKeys:         1,
		NetMetalRef:     1.55,
		FIFOProfitScrap: 9,
		IsEstimate:      false,
	}

	err = store.Push(log1)
	assert.NoError(t, err)

	// Give the background persistence worker a tiny moment to process the write channel
	time.Sleep(20 * time.Millisecond)

	// Stop persistence to flush and wait until it's finished
	cancel()
	<-done

	// Load back using a new store instance
	loadedStore, err := NewStatsStore(filePath, logger)
	require.NoError(t, err)

	summary, err := loadedStore.GetProfitSummary(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.TotalTrades)
	assert.Equal(t, 1, summary.NetKeys)
	assert.Equal(t, 1.55, summary.NetMetalRef)
	assert.Equal(t, 9, summary.FIFOProfitScrap)
}

func TestStatsStore_GetProfitSummary_Cutoff(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stats.json")
	logger := log.Discard

	store, err := NewStatsStore(filePath, logger)
	require.NoError(t, err)

	now := time.Now()

	// 2 hours ago
	store.Push(storage.TradeProfitLog{
		TradeID:         "1",
		Timestamp:       now.Add(-2 * time.Hour),
		NetKeys:         2,
		NetMetalRef:     1.0,
		FIFOProfitScrap: 18,
	})

	// 10 hours ago
	store.Push(storage.TradeProfitLog{
		TradeID:         "2",
		Timestamp:       now.Add(-10 * time.Hour),
		NetKeys:         1,
		NetMetalRef:     0.5,
		FIFOProfitScrap: 9,
	})

	// 30 hours ago (outside 24h window)
	store.Push(storage.TradeProfitLog{
		TradeID:         "3",
		Timestamp:       now.Add(-30 * time.Hour),
		NetKeys:         5,
		NetMetalRef:     10.0,
		FIFOProfitScrap: 90,
	})

	summary24h, err := store.GetProfitSummary(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 2, summary24h.TotalTrades)
	assert.Equal(t, 3, summary24h.NetKeys)
	assert.Equal(t, 1.5, summary24h.NetMetalRef)
	assert.Equal(t, 27, summary24h.FIFOProfitScrap)

	summary5h, err := store.GetProfitSummary(5 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, summary5h.TotalTrades)
	assert.Equal(t, 2, summary5h.NetKeys)
}

func TestStatsStore_GetDailyProfit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stats.json")
	logger := log.Discard

	store, err := NewStatsStore(filePath, logger)
	require.NoError(t, err)

	now := time.Now()

	// Today
	store.Push(storage.TradeProfitLog{
		TradeID:         "1",
		Timestamp:       now,
		NetKeys:         1,
		NetMetalRef:     1.0,
		FIFOProfitScrap: 9,
	})

	// Yesterday
	store.Push(storage.TradeProfitLog{
		TradeID:         "2",
		Timestamp:       now.AddDate(0, 0, -1),
		NetKeys:         2,
		NetMetalRef:     2.0,
		FIFOProfitScrap: 18,
	})

	dailies, err := store.GetDailyProfit(3)
	require.NoError(t, err)
	require.Len(t, dailies, 3)

	// Index 0 is today
	assert.Equal(t, 1, dailies[0].TotalTrades)
	assert.Equal(t, 1, dailies[0].NetKeys)

	// Index 1 is yesterday
	assert.Equal(t, 1, dailies[1].TotalTrades)
	assert.Equal(t, 2, dailies[1].NetKeys)

	// Index 2 is day before yesterday
	assert.Equal(t, 0, dailies[2].TotalTrades)
}

func TestStatsStore_Prune(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stats.json")
	logger := log.Discard

	store, err := NewStatsStore(filePath, logger)
	require.NoError(t, err)

	now := time.Now()

	store.Push(storage.TradeProfitLog{
		TradeID:   "1",
		Timestamp: now.Add(-5 * time.Hour),
		NetKeys:   1,
	})

	store.Push(storage.TradeProfitLog{
		TradeID:   "2",
		Timestamp: now.Add(-30 * time.Hour),
		NetKeys:   2,
	})

	err = store.Prune(24 * time.Hour)
	assert.NoError(t, err)

	summary, err := store.GetProfitSummary(48 * time.Hour)
	require.NoError(t, err)
	// Log "2" should be pruned because it's 30h old
	assert.Equal(t, 1, summary.TotalTrades)
	assert.Equal(t, 1, summary.NetKeys)
}
