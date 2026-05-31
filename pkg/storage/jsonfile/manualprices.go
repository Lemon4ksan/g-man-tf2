// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// ManualPricesStore manages persistent manual prices using a local JSON file database.
type ManualPricesStore struct {
	mu       sync.RWMutex
	filePath string
	logger   log.Logger
}

// NewManualPricesStore creates and returns a new [ManualPricesStore] instance.
func NewManualPricesStore(path string, logger log.Logger) (*ManualPricesStore, error) {
	s := &ManualPricesStore{
		filePath: path,
		logger:   logger.With(log.String("module", "manual_prices_store")),
	}

	// Ensure the parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return s, nil
}

// GetAll returns all manual price entries currently tracked in the JSON file store.
func (s *ManualPricesStore) GetAll() (map[string]storage.ManualPriceEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prices := make(map[string]storage.ManualPriceEntry)

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return prices, nil
		}

		return nil, err
	}

	if err := json.Unmarshal(data, &prices); err != nil {
		return nil, err
	}

	return prices, nil
}

// Set adds or updates a manual price entry for the given SKU and writes to disk.
func (s *ManualPricesStore) Set(sku string, entry storage.ManualPriceEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prices := make(map[string]storage.ManualPriceEntry)

	data, err := os.ReadFile(s.filePath)
	if err == nil {
		_ = json.Unmarshal(data, &prices)
	}

	prices[sku] = entry

	newData, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}

// Delete removes a manual price entry for the given SKU from disk.
func (s *ManualPricesStore) Delete(sku string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prices := make(map[string]storage.ManualPriceEntry)

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	_ = json.Unmarshal(data, &prices)

	if _, exists := prices[sku]; !exists {
		return nil
	}

	delete(prices, sku)

	newData, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.filePath)
}

// GetModTime returns the last modification time of the manual prices JSON file on disk.
func (s *ManualPricesStore) GetModTime() (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, err := os.Stat(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}

		return time.Time{}, err
	}

	return info.ModTime(), nil
}
