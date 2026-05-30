// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricelistsaver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
)

type mockPriceProvider struct {
	mu     sync.Mutex
	prices []*pricedb.Price
}

func (m *mockPriceProvider) GetAllPrices() []*pricedb.Price {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prices
}

func TestPricelistSaver_Debouncing(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), "pricelist_debounce.json")

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	provider := &mockPriceProvider{
		prices: []*pricedb.Price{
			{
				SKU:    "5021;6",
				Source: "Autokeys",
				Buy:    pricedb.Currencies{Metal: 50.0},
				Sell:   pricedb.Currencies{Metal: 51.0},
			},
		},
	}

	cfg := Config{
		PricelistPath: tempPath,
		SilenceWindow: 150 * time.Millisecond,
		MaxDelay:      1 * time.Second,
	}

	saver := New(provider, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = saver.Run(ctx)
	})

	// Send multiple events rapidly
	for range 5 {
		eventBus.Publish(&pricedb.PricelistUpdatedEvent{})
		time.Sleep(20 * time.Millisecond) // total 100ms
	}

	// Verify file does not exist yet (debouncer is waiting for 150ms silence)
	_, err := os.Stat(tempPath)
	assert.True(t, os.IsNotExist(err))

	// Wait for silence window to expire
	time.Sleep(200 * time.Millisecond)

	// File should exist now
	assert.FileExists(t, tempPath)

	// Read and verify content
	bytes, err := os.ReadFile(tempPath)
	require.NoError(t, err)

	type EntryData struct {
		Defindex  int                `json:"defindex"`
		Quality   int                `json:"quality"`
		Autoprice bool               `json:"autoprice"`
		Min       int                `json:"min"`
		Max       int                `json:"max"`
		Buy       pricedb.Currencies `json:"buy"`
		Sell      pricedb.Currencies `json:"sell"`
	}

	var dataMap map[string]EntryData

	err = json.Unmarshal(bytes, &dataMap)
	require.NoError(t, err)

	entry, exists := dataMap["5021;6"]
	require.True(t, exists)
	assert.Equal(t, 5021, entry.Defindex)
	assert.Equal(t, 6, entry.Quality)
	assert.False(t, entry.Autoprice)
	assert.Equal(t, 50.0, entry.Buy.Metal)
	assert.Equal(t, 51.0, entry.Sell.Metal)

	cancel()
	wg.Wait()
}

func TestPricelistSaver_MaxDelay(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), "pricelist_max_delay.json")

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	provider := &mockPriceProvider{
		prices: []*pricedb.Price{
			{
				SKU:    "5002;6",
				Source: "Manual",
				Buy:    pricedb.Currencies{Metal: 1.0},
				Sell:   pricedb.Currencies{Metal: 1.0},
			},
		},
	}

	// Setting silence window long, but max delay short
	cfg := Config{
		PricelistPath: tempPath,
		SilenceWindow: 300 * time.Millisecond,
		MaxDelay:      200 * time.Millisecond,
	}

	saver := New(provider, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = saver.Run(ctx)
	})

	// Give the subscriber time to initialize
	time.Sleep(50 * time.Millisecond)

	// Send first event to start max delay timer
	eventBus.Publish(&pricedb.PricelistUpdatedEvent{})

	// Send continuous updates every 50ms (shorter than 300ms silence window, so it would starve standard debouncer, but MaxDelay is 200ms)
	for range 6 {
		time.Sleep(50 * time.Millisecond)
		eventBus.Publish(&pricedb.PricelistUpdatedEvent{})
	}

	assert.Eventually(t, func() bool {
		_, err := os.Stat(tempPath)
		return err == nil
	}, 1*time.Second, 10*time.Millisecond, "Pricelist file should have been flushed due to MaxDelay")

	cancel()
	wg.Wait()
}

func TestPricelistSaver_AtypicalSKUs(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), "pricelist_atypical.json")

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()

	provider := &mockPriceProvider{
		prices: []*pricedb.Price{
			{
				SKU:    "10151297782", // Pure Asset ID
				Source: "Manual",
				Buy:    pricedb.Currencies{Metal: 10.0},
				Sell:   pricedb.Currencies{Metal: 12.0},
			},
			{
				SKU:    "5002;6", // Standard SKU
				Source: "Autokeys",
				Buy:    pricedb.Currencies{Metal: 1.0},
				Sell:   pricedb.Currencies{Metal: 1.1},
			},
		},
	}

	cfg := Config{
		PricelistPath: tempPath,
		SilenceWindow: 50 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
	}

	saver := New(provider, eventBus, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Go(func() {
		_ = saver.Run(ctx)
	})

	// Give the subscriber time to initialize
	time.Sleep(50 * time.Millisecond)

	// Trigger flush
	eventBus.Publish(&pricedb.PricelistUpdatedEvent{})
	time.Sleep(150 * time.Millisecond)

	assert.FileExists(t, tempPath)

	bytes, err := os.ReadFile(tempPath)
	require.NoError(t, err)

	type EntryData struct {
		Defindex  int                `json:"defindex"`
		Quality   int                `json:"quality"`
		Autoprice bool               `json:"autoprice"`
		Buy       pricedb.Currencies `json:"buy"`
		Sell      pricedb.Currencies `json:"sell"`
	}

	var dataMap map[string]EntryData

	err = json.Unmarshal(bytes, &dataMap)
	require.NoError(t, err)

	// Verify standard SKU exists
	entryStandard, existsStandard := dataMap["5002;6"]
	require.True(t, existsStandard)
	assert.Equal(t, 5002, entryStandard.Defindex)
	assert.Equal(t, 6, entryStandard.Quality)

	// Verify pure Asset ID exists and was not dropped!
	entryAtypical, existsAtypical := dataMap["10151297782"]
	require.True(t, existsAtypical)
	assert.Equal(t, 10151297782, entryAtypical.Defindex)
	assert.Equal(t, 6, entryAtypical.Quality) // Default Unique quality

	cancel()
	wg.Wait()
}
