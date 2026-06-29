// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager_LoadAndWatch_FileUpdated_HotReloadsConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "trading_config.json")

	initialCfg := Config{
		GlobalMaxStock:  3000,
		DefaultMaxStock: 5,
	}
	data, err := json.MarshalIndent(initialCfg, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0o644)
	require.NoError(t, err)

	backdateTime := time.Now().Add(-10 * time.Second)
	err = os.Chtimes(configPath, backdateTime, backdateTime)
	require.NoError(t, err)

	cm, err := NewConfigManager(configPath)
	require.NoError(t, err)
	assert.Equal(t, 3000, cm.GetConfig().GlobalMaxStock)
	assert.Equal(t, 5, cm.GetConfig().DefaultMaxStock)

	cm.StartWatching(t.Context(), 10*time.Millisecond, log.Discard)

	updatedConfig := Config{
		GlobalMaxStock:  5000,
		DefaultMaxStock: 10,
		Items: map[string]ItemConfig{
			"5021;6": {
				SKU:      "5021;6",
				MaxStock: 50,
			},
		},
	}

	data, err = json.MarshalIndent(updatedConfig, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(configPath, data, 0o644)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return cm.GetConfig().GlobalMaxStock == 5000
	}, 1*time.Second, 10*time.Millisecond)

	assert.Equal(t, 10, cm.GetConfig().DefaultMaxStock)

	itemCfg, ok := cm.GetItemConfig("5021;6")
	assert.True(t, ok)
	assert.Equal(t, 50, itemCfg.MaxStock)
}

func TestConfigManager_Load_Errors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid_config.json")

	err := os.WriteFile(configPath, []byte(`{invalid}`), 0o644)
	require.NoError(t, err)

	_, err = NewConfigManager(configPath)
	assert.Error(t, err)

	parentFile := filepath.Join(tmpDir, "parent_file")
	err = os.WriteFile(parentFile, []byte("junk"), 0o644)
	require.NoError(t, err)

	nestedPath := filepath.Join(parentFile, "nested", "config.json")
	_, err = NewConfigManager(nestedPath)
	assert.Error(t, err)

	dirPath := filepath.Join(tmpDir, "some_dir")
	err = os.MkdirAll(dirPath, 0o755)
	require.NoError(t, err)

	_, err = NewConfigManager(dirPath)
	assert.Error(t, err)
}
