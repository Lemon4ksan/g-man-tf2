// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

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
)

func TestConfigManager_LoadAndWatch(t *testing.T) {
	// Setup a temporary directory for the config file
	tmpDir, err := os.MkdirTemp("", "g-man-config-test")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "trading_config.json")

	// Check default skeleton file creation
	cm, err := NewConfigManager(configPath)
	require.NoError(t, err)
	assert.Equal(t, 3000, cm.GetConfig().GlobalMaxStock)
	assert.Equal(t, 5, cm.GetConfig().DefaultMaxStock)

	// Start watching in the background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cm.StartWatching(ctx, 10*time.Millisecond, log.Discard)

	// Update the config on disk
	updatedConfig := Config{
		GlobalMaxStock:  5000,
		DefaultMaxStock: 10,
		PriceSwingLimits: PriceSwingLimits{
			MaxBuyIncrease:  0.15,
			MaxSellDecrease: 0.15,
		},
		Items: map[string]ItemConfig{
			"5021;6": {
				SKU:      "5021;6",
				MaxStock: 50,
			},
		},
	}

	data, err := json.MarshalIndent(updatedConfig, "", "  ")
	require.NoError(t, err)

	// Sleep slightly to guarantee mod time difference if the OS filesystem timestamp precision is low
	time.Sleep(100 * time.Millisecond)

	err = os.WriteFile(configPath, data, 0o644)
	require.NoError(t, err)

	// Wait for the reloader to pick up the change
	assert.Eventually(t, func() bool {
		return cm.GetConfig().GlobalMaxStock == 5000
	}, 1*time.Second, 10*time.Millisecond)

	// Verify the updated values
	assert.Equal(t, 10, cm.GetConfig().DefaultMaxStock)
	assert.Equal(t, 0.15, cm.GetConfig().PriceSwingLimits.MaxBuyIncrease)

	itemCfg, ok := cm.GetItemConfig("5021;6")
	assert.True(t, ok)
	assert.Equal(t, 50, itemCfg.MaxStock)
}
