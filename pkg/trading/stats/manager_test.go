// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

type mockProfitStore struct {
	mock.Mock
}

func (m *mockProfitStore) Push(entry storage.TradeProfitLog) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *mockProfitStore) GetProfitSummary(since time.Duration) (storage.ProfitSummary, error) {
	args := m.Called(since)
	return args.Get(0).(storage.ProfitSummary), args.Error(1)
}

func (m *mockProfitStore) GetDailyProfit(days int) ([]storage.ProfitSummary, error) {
	args := m.Called(days)
	return args.Get(0).([]storage.ProfitSummary), args.Error(1)
}

func (m *mockProfitStore) Prune(keepDuration time.Duration) error {
	args := m.Called(keepDuration)
	return args.Error(0)
}

func TestStatsManager_GetProfitSummary(t *testing.T) {
	t.Parallel()

	store := new(mockProfitStore)
	mgr := New(store)

	expected := storage.ProfitSummary{
		TotalTrades:     5,
		NetKeys:         3,
		NetMetalRef:     10.5,
		FIFOProfitScrap: 18,
	}

	store.On("GetProfitSummary", 24*time.Hour).Return(expected, nil)

	res, err := mgr.GetProfitSummary(24 * time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
	store.AssertExpectations(t)
}

func TestStatsManager_GetDailyProfit(t *testing.T) {
	t.Parallel()

	store := new(mockProfitStore)
	mgr := New(store)

	expected := []storage.ProfitSummary{
		{TotalTrades: 2, NetKeys: 1},
		{TotalTrades: 3, NetKeys: 2},
	}

	store.On("GetDailyProfit", 2).Return(expected, nil)

	res, err := mgr.GetDailyProfit(2)
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
	store.AssertExpectations(t)
}

func TestStatsManager_GetEstimatedProfitRef(t *testing.T) {
	t.Parallel()

	store := new(mockProfitStore)
	mgr := New(store)

	// Summary: 2 net keys, 5.0 net metal ref, 18 realized FIFO profit scrap (= 2.0 ref)
	summary := storage.ProfitSummary{
		TotalTrades:     10,
		NetKeys:         2,
		NetMetalRef:     5.0,
		FIFOProfitScrap: 18,
	}

	store.On("GetProfitSummary", 24*time.Hour).Return(summary, nil)

	// Key price: 60.0 ref.
	// Keys profit in ref: 2 keys * 60.0 = 120.0 ref.
	// FIFO profit in ref: 18 scrap = 2.0 ref.
	// Net metal ref: 5.0 ref.
	// Total: 120.0 + 5.0 + 2.0 = 127.0 ref.
	profit, err := mgr.GetEstimatedProfitRef(24*time.Hour, 60.0)
	assert.NoError(t, err)
	assert.Equal(t, 127.0, profit)
	store.AssertExpectations(t)
}
