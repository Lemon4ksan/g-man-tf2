// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license[s] that can be found in the LICENSE file.

package trading

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager_StartWatching_StatError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "temp_watching.json")

	initialCfg := Config{GlobalMaxStock: 100}
	data, _ := json.Marshal(initialCfg)
	_ = os.WriteFile(configPath, data, 0o644)

	cm, err := NewConfigManager(configPath)
	require.NoError(t, err)

	_ = os.Remove(configPath)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cm.StartWatching(ctx, 10*time.Millisecond, log.Discard)

	time.Sleep(50 * time.Millisecond)
}

func TestEnrichItem_AllAttributes(t *testing.T) {
	t.Parallel()

	s := funcMockSchema()

	item := &trading.Item{
		MarketHashName: "Strange Unusual Australium Festivized Rocket Launcher",
		Tradable:       true,
		Descriptions: []trading.Description{
			{Value: "Exterior: factory new"},
			{Value: "★ Unusual Effect: Sunbeams"},
			{Value: "Killstreak Active: Professional"},
			{Value: "Paint Color: Aged Moustache Grey"},
			{Value: "Crate Series #85"},
			{Value: "Robots Destroyed: 0", Color: "756b5e"},
			{Value: "Halloween: Voices from Below", Color: "7ea9d1"},
		},
	}

	EnrichItem(item, s)
	assert.NotEmpty(t, item.SKU)
	assert.Contains(t, item.SKU, "205")
	assert.Contains(t, item.SKU, ";5")
	assert.Contains(t, item.SKU, ";u17")
	assert.Contains(t, item.SKU, ";australium")
	assert.Contains(t, item.SKU, ";w1")
	assert.Contains(t, item.SKU, ";strange")
	assert.Contains(t, item.SKU, ";kt-3")
	assert.Contains(t, item.SKU, ";festive")
	assert.Contains(t, item.SKU, ";c85")
	assert.Contains(t, item.SKU, ";s-1006-1")
	assert.Contains(t, item.SKU, ";sp39")
	assert.Contains(t, item.SKU, ";p8290046")
}
