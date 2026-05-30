// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jsonfile provides a JSON-based implementation of the CostBasisStore interface.
package jsonfile

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// diskLayout is the internal JSON layout persisted to costBasis.json
type diskLayout struct {
	Entries   []storage.CostBasisEntry    `json:"entries"`
	PPUStates map[string]storage.PPUState `json:"ppu_states"`
}

// CostBasisStore manages the cost basis database in-memory and persists it asynchronously.
type CostBasisStore struct {
	mu        sync.RWMutex
	entries   []storage.CostBasisEntry
	ppuStates map[string]storage.PPUState
	filePath  string
	writeChan chan struct{}
	logger    log.Logger
}

// NewCostBasisStore creates and loads a CostBasisStore.
func NewCostBasisStore(path string, logger log.Logger) (*CostBasisStore, error) {
	s := &CostBasisStore{
		filePath:  path,
		writeChan: make(chan struct{}, 1),
		ppuStates: make(map[string]storage.PPUState),
		logger:    logger.With(log.String("module", "costbasis_store")),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// Start runs the background debounced writer. It will write any pending changes
// to disk at most once every 1.5 seconds when triggered, and performs a final write on exit.
func (s *CostBasisStore) Start(ctx context.Context) {
	s.logger.Info("Cost basis persistence worker started", log.String("path", s.filePath))

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	var pending bool

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping persistence worker, flushing pending changes...")
			s.mu.Lock()
			if pending {
				if err := s.saveLocked(); err != nil {
					s.logger.Error("Failed to flush changes on shutdown", log.Err(err))
				}
			}

			s.mu.Unlock()

			return

		case <-s.writeChan:
			s.mu.Lock()
			pending = true
			s.mu.Unlock()
		case <-ticker.C:
			s.mu.Lock()
			if pending {
				if err := s.saveLocked(); err != nil {
					s.logger.Error("Failed to auto-save changes", log.Err(err))
				} else {
					pending = false
				}
			}

			s.mu.Unlock()
		}
	}
}

// Push appends an entry to the FIFO database and schedules a save.
func (s *CostBasisStore) Push(sku string, entry storage.CostBasisEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = append(s.entries, entry)
	s.triggerSaveLocked()
}

// Pop finds and removes the oldest entry (FIFO) for the given SKU.
func (s *CostBasisStore) Pop(sku string) (storage.CostBasisEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.entries {
		if entry.SKU == sku {
			// Found matching entry, remove it from slice
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.triggerSaveLocked()
			return entry, true
		}
	}

	return storage.CostBasisEntry{}, false
}

// GetOldestEntry retrieves the oldest CostBasisEntry for a SKU without removing it.
func (s *CostBasisStore) GetOldestEntry(sku string) (storage.CostBasisEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.entries {
		if entry.SKU == sku {
			return entry, true
		}
	}

	return storage.CostBasisEntry{}, false
}

// GetPPUState retrieves the PPUState for a given SKU.
func (s *CostBasisStore) GetPPUState(sku string) (storage.PPUState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.ppuStates[sku]

	return state, ok
}

// SetPPUState stores the PPUState for a given SKU and schedules a save.
func (s *CostBasisStore) SetPPUState(sku string, state storage.PPUState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ppuStates[sku] = state
	s.triggerSaveLocked()
}

// GetAllPPUStates returns a copy of all PPU states.
func (s *CostBasisStore) GetAllPPUStates() map[string]storage.PPUState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copyMap := make(map[string]storage.PPUState, len(s.ppuStates))
	maps.Copy(copyMap, s.ppuStates)

	return copyMap
}

// Prune removes entries older than the holdDuration.
// If after pruning, a SKU has no more entries, its PPU protection is automatically turned off.
// Additionally, garbage collects inactive SKU PPUStates from the map to keep storage size optimized.
func (s *CostBasisStore) Prune(holdDuration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	newEntries := make([]storage.CostBasisEntry, 0, len(s.entries))
	activeSKUs := make(map[string]bool)

	for _, entry := range s.entries {
		if now.Sub(entry.Timestamp) <= holdDuration {
			newEntries = append(newEntries, entry)
			activeSKUs[entry.SKU] = true
		}
	}

	entriesChanged := len(newEntries) != len(s.entries)
	s.entries = newEntries

	ppuStatesChanged := false
	// Deactivate protection for SKUs that no longer have cost basis entries
	// and garbage collect inactive SKU PPUStates sold more than 30 days ago (or never sold)
	for sku, state := range s.ppuStates {
		if !activeSKUs[sku] {
			if state.IsPartialPriced {
				state.IsPartialPriced = false
				state.ProtectionStarted = time.Time{}
				state.LastInStockTime = time.Time{}
				s.ppuStates[sku] = state
				ppuStatesChanged = true
			}

			// Garbage collect: IsPartialPriced is false, stock is 0 (no active entries),
			// and either never sold or sold more than 30 days ago.
			if !state.IsPartialPriced &&
				(state.LastSoldTime.IsZero() || now.Sub(state.LastSoldTime) > 30*24*time.Hour) {
				delete(s.ppuStates, sku)

				ppuStatesChanged = true
			}
		}
	}

	if entriesChanged || ppuStatesChanged {
		s.triggerSaveLocked()
	}
}

// GetEntriesForTesting returns entries slice (only for tests).
func (s *CostBasisStore) GetEntriesForTesting() []storage.CostBasisEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copySlice := make([]storage.CostBasisEntry, len(s.entries))
	copy(copySlice, s.entries)

	return copySlice
}

// triggerSaveLocked alerts the persistence worker that there are changes.
func (s *CostBasisStore) triggerSaveLocked() {
	select {
	case s.writeChan <- struct{}{}:
	default:
	}
}

// load reads the costBasis.json file.
func (s *CostBasisStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if len(data) == 0 {
		return nil
	}

	var layout diskLayout
	if err := json.Unmarshal(data, &layout); err != nil {
		return err
	}

	s.entries = layout.Entries
	if layout.PPUStates != nil {
		s.ppuStates = layout.PPUStates
	}

	return nil
}

// saveLocked persists the entries and PPU states atomically.
func (s *CostBasisStore) saveLocked() error {
	layout := diskLayout{
		Entries:   s.entries,
		PPUStates: s.ppuStates,
	}

	bytes, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}
