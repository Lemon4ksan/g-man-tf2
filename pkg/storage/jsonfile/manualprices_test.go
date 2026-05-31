// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

func TestManualPricesStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manualprices-store-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "manual_prices.json")

	store, err := NewManualPricesStore(filePath, log.Discard)
	require.NoError(t, err)

	// Test Set and GetAll
	err = store.Set("5021;6", storage.ManualPriceEntry{
		BuyKeys:   1,
		BuyMetal:  10.0,
		SellKeys:  1,
		SellMetal: 15.0,
	})
	require.NoError(t, err)

	prices, err := store.GetAll()
	require.NoError(t, err)

	entry, exists := prices["5021;6"]
	require.True(t, exists)
	assert.Equal(t, 1, entry.BuyKeys)
	assert.Equal(t, 10.0, entry.BuyMetal)
	assert.Equal(t, 1, entry.SellKeys)
	assert.Equal(t, 15.0, entry.SellMetal)

	// Test Delete
	err = store.Delete("5021;6")
	require.NoError(t, err)

	prices, err = store.GetAll()
	require.NoError(t, err)

	_, exists = prices["5021;6"]
	assert.False(t, exists)
}
