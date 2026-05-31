// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

type statsDiskLayout struct {
	Logs []storage.TradeProfitLog `json:"logs"`
}

// StatsStore manages the trade profit analytics logs in-memory and persists them asynchronously.
type StatsStore struct {
	mu        sync.RWMutex
	logs      []storage.TradeProfitLog
	filePath  string
	writeChan chan struct{}
	logger    log.Logger
}

// NewStatsStore creates and loads a StatsStore from the specified file path.
func NewStatsStore(path string, logger log.Logger) (*StatsStore, error) {
	s := &StatsStore{
		filePath:  path,
		writeChan: make(chan struct{}, 1),
		logs:      make([]storage.TradeProfitLog, 0),
		logger:    logger.With(log.String("module", "stats_store")),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// Start runs the background debounced writer to flush logs asynchronously.
func (s *StatsStore) Start(ctx context.Context) {
	s.logger.Info("Stats persistence worker started", log.String("path", s.filePath))

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	var pending bool

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping stats persistence worker, flushing pending changes...")
			s.mu.Lock()
			if pending {
				if err := s.saveLocked(); err != nil {
					s.logger.Error("Failed to flush stats changes on shutdown", log.Err(err))
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
					s.logger.Error("Failed to auto-save stats changes", log.Err(err))
				} else {
					pending = false
				}
			}

			s.mu.Unlock()
		}
	}
}

// Push appends a trade profit log entry and schedules a save.
func (s *StatsStore) Push(entry storage.TradeProfitLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = append(s.logs, entry)

	select {
	case s.writeChan <- struct{}{}:
	default:
	}

	return nil
}

// GetProfitSummary calculates aggregated profit stats since a given relative duration (e.g. 24h).
func (s *StatsStore) GetProfitSummary(since time.Duration) (storage.ProfitSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-since)

	var (
		keys, fifo, trades int
		metal              float64
	)

	for _, logEntry := range s.logs {
		if logEntry.Timestamp.After(cutoff) || logEntry.Timestamp.Equal(cutoff) {
			keys += logEntry.NetKeys
			metal += logEntry.NetMetalRef
			fifo += logEntry.FIFOProfitScrap
			trades++
		}
	}

	return storage.ProfitSummary{
		TotalTrades:     trades,
		NetKeys:         keys,
		NetMetalRef:     metal,
		FIFOProfitScrap: fifo,
	}, nil
}

// GetDailyProfit returns aggregated daily profit summaries for the last n days.
func (s *StatsStore) GetDailyProfit(days int) ([]storage.ProfitSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	summaries := make([]storage.ProfitSummary, days)

	for i := range days {
		dayStart := todayMidnight.AddDate(0, 0, -i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var (
			keys, fifo, trades int
			metal              float64
		)

		for _, logEntry := range s.logs {
			if (logEntry.Timestamp.After(dayStart) || logEntry.Timestamp.Equal(dayStart)) &&
				logEntry.Timestamp.Before(dayEnd) {
				keys += logEntry.NetKeys
				metal += logEntry.NetMetalRef
				fifo += logEntry.FIFOProfitScrap
				trades++
			}
		}

		summaries[i] = storage.ProfitSummary{
			TotalTrades:     trades,
			NetKeys:         keys,
			NetMetalRef:     metal,
			FIFOProfitScrap: fifo,
		}
	}

	return summaries, nil
}

// Prune removes trade profit logs that are older than the given keep duration.
func (s *StatsStore) Prune(keepDuration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-keepDuration)

	var kept []storage.TradeProfitLog

	for _, logEntry := range s.logs {
		if logEntry.Timestamp.After(cutoff) || logEntry.Timestamp.Equal(cutoff) {
			kept = append(kept, logEntry)
		}
	}

	if len(kept) < len(s.logs) {
		s.logs = kept
		select {
		case s.writeChan <- struct{}{}:
		default:
		}
	}

	return nil
}

func (s *StatsStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var layout statsDiskLayout
	if err := json.Unmarshal(data, &layout); err != nil {
		return err
	}

	if layout.Logs != nil {
		s.logs = layout.Logs
	}

	return nil
}

func (s *StatsStore) saveLocked() error {
	layout := statsDiskLayout{
		Logs: s.logs,
	}

	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}
