// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonfile_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage/jsonfile"
)

func TestCostBasisStore_FIFO(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "costbasis-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.New(log.DefaultConfig(log.LevelDebug))
	defer logger.Close()

	store, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		store.Start(ctx)
		close(done)
	}()

	defer func() {
		cancel()
		<-done
	}()

	// Push 3 entries for "sku1" and 1 for "sku2"
	now := time.Now()
	entry1 := storage.CostBasisEntry{SKU: "sku1", BuyKeys: 1, BuyMetal: 10, Diff: 5, TradeID: "t1", Timestamp: now}
	entry2 := storage.CostBasisEntry{
		SKU:       "sku1",
		BuyKeys:   1,
		BuyMetal:  12,
		Diff:      -3,
		TradeID:   "t2",
		Timestamp: now.Add(time.Second),
	}
	entry3 := storage.CostBasisEntry{SKU: "sku2", BuyKeys: 0, BuyMetal: 4.5, Diff: 0, TradeID: "t3", Timestamp: now}
	entry4 := storage.CostBasisEntry{
		SKU:       "sku1",
		BuyKeys:   2,
		BuyMetal:  0,
		Diff:      9,
		TradeID:   "t4",
		Timestamp: now.Add(2 * time.Second),
	}

	store.Push("sku1", entry1)
	store.Push("sku1", entry2)
	store.Push("sku2", entry3)
	store.Push("sku1", entry4)

	// Pop "sku1" - should be entry1 (FIFO: first in, first out)
	popped, ok := store.Pop("sku1")
	assert.True(t, ok)
	assert.Equal(t, "t1", popped.TradeID)
	assert.Equal(t, 5, popped.Diff)

	// Pop "sku1" again - should be entry2
	popped, ok = store.Pop("sku1")
	assert.True(t, ok)
	assert.Equal(t, "t2", popped.TradeID)

	// Pop "sku2" - should be entry3
	popped, ok = store.Pop("sku2")
	assert.True(t, ok)
	assert.Equal(t, "t3", popped.TradeID)

	// Pop "sku2" again - should be not found
	_, ok = store.Pop("sku2")
	assert.False(t, ok)
}

func TestCostBasisStore_PPUState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "costbasis-ppu-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.New(log.DefaultConfig(log.LevelDebug))
	defer logger.Close()

	store, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	now := time.Now()
	state1 := storage.PPUState{
		SKU:               "sku1",
		LastInStockTime:   now,
		IsPartialPriced:   true,
		ProtectionStarted: now.Add(-time.Hour),
	}

	store.SetPPUState("sku1", state1)

	got, ok := store.GetPPUState("sku1")
	assert.True(t, ok)
	assert.Equal(t, "sku1", got.SKU)
	assert.True(t, got.IsPartialPriced)
	assert.Equal(t, now.Unix(), got.LastInStockTime.Unix())

	all := store.GetAllPPUStates()
	assert.Len(t, all, 1)
	assert.Equal(t, "sku1", all["sku1"].SKU)
}

func TestCostBasisStore_Prune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "costbasis-prune-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.New(log.DefaultConfig(log.LevelDebug))
	defer logger.Close()

	store, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	now := time.Now()

	// Push an old entry and a new entry
	oldEntry := storage.CostBasisEntry{SKU: "sku1", TradeID: "old", Timestamp: now.Add(-10 * time.Hour)}
	newEntry := storage.CostBasisEntry{SKU: "sku2", TradeID: "new", Timestamp: now.Add(-5 * time.Minute)}

	store.Push("sku1", oldEntry)
	store.Push("sku2", newEntry)

	// Set PPU state active for both
	store.SetPPUState("sku1", storage.PPUState{
		SKU:             "sku1",
		IsPartialPriced: true,
		LastSoldTime:    now.Add(-5 * time.Minute),
	})
	store.SetPPUState("sku2", storage.PPUState{SKU: "sku2", IsPartialPriced: true})

	// PPU state for sku3: inactive, sold 40 days ago -> should be garbage collected
	store.SetPPUState("sku3", storage.PPUState{
		SKU:             "sku3",
		IsPartialPriced: false,
		LastSoldTime:    now.Add(-40 * 24 * time.Hour),
	})

	// PPU state for sku4: inactive, sold 5 minutes ago -> should NOT be garbage collected
	store.SetPPUState("sku4", storage.PPUState{
		SKU:             "sku4",
		IsPartialPriced: false,
		LastSoldTime:    now.Add(-5 * time.Minute),
	})

	// Prune with a 1-hour threshold
	store.Prune(1 * time.Hour)

	// sku1 entry was older than 1 hour -> should be pruned
	// Because sku1 has no remaining entries -> PPU state for sku1 should be deactivated!
	state1, ok := store.GetPPUState("sku1")
	assert.True(t, ok)
	assert.False(t, state1.IsPartialPriced)

	// sku2 entry was 5 minutes old -> should NOT be pruned
	// PPU state for sku2 should remain active
	state2, ok := store.GetPPUState("sku2")
	assert.True(t, ok)
	assert.True(t, state2.IsPartialPriced)

	// sku3 has no entries, is inactive, and was sold 40 days ago -> should be deleted
	_, ok = store.GetPPUState("sku3")
	assert.False(t, ok, "sku3 should be garbage collected")

	// sku4 has no entries, is inactive, but was sold 5 mins ago -> should remain
	_, ok = store.GetPPUState("sku4")
	assert.True(t, ok, "sku4 should NOT be garbage collected")

	// Validate remaining entries
	entries := store.GetEntriesForTesting()
	assert.Len(t, entries, 1)
	assert.Equal(t, "new", entries[0].TradeID)
}

func TestCostBasisStore_DebouncedSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "costbasis-debounce-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "costBasis.json")

	logger := log.New(log.DefaultConfig(log.LevelDebug))
	defer logger.Close()

	store, err := jsonfile.NewCostBasisStore(filePath, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		store.Start(ctx)
		close(done)
	}()

	defer func() {
		cancel()
		<-done
	}()

	// Push changes
	store.Push("sku1", storage.CostBasisEntry{SKU: "sku1", TradeID: "t1"})

	// Right away, the file shouldn't be saved yet because of the debounce timer
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "File should not be persisted immediately")

	// Wait 2 seconds (debounce is 1.5 seconds)
	time.Sleep(2 * time.Second)

	// File should exist now
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Read file to verify content
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var content struct {
		Entries []storage.CostBasisEntry `json:"entries"`
	}

	err = json.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Len(t, content.Entries, 1)
	assert.Equal(t, "sku1", content.Entries[0].SKU)
}
