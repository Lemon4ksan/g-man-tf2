// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/test/module"
	"github.com/lemon4ksan/g-man/test/requester"
)

func setupSchema(t *testing.T, cfg Config) (*Manager, *requester.Mock) {
	t.Helper()

	defaultCfg := DefaultConfig()
	if cfg.ItemsGameMirrorURL == "" {
		cfg.ItemsGameMirrorURL = defaultCfg.ItemsGameMirrorURL
	}

	if cfg.PaintKitURL == "" {
		cfg.PaintKitURL = defaultCfg.PaintKitURL
	}

	mockAPI := requester.New()
	init := module.NewInitContext()
	init.SetService(mockAPI)

	sm := NewManager(cfg)
	if err := sm.Init(init); err != nil {
		t.Fatalf("failed to init schema manager: %v", err)
	}

	return sm, mockAPI
}

func TestManager_New_ConfigDefaults_ValidatesIntervals(t *testing.T) {
	t.Parallel()

	cfg := Config{UpdateInterval: 10 * time.Second}
	sm := NewManager(cfg)

	if sm.config.UpdateInterval != 24*time.Hour {
		t.Errorf("expected 24h interval, got %v", sm.config.UpdateInterval)
	}

	cfgValid := Config{UpdateInterval: 5 * time.Minute}

	smValid := NewManager(cfgValid)
	if smValid.config.UpdateInterval != 5*time.Minute {
		t.Errorf("expected 5m interval, got %v", smValid.config.UpdateInterval)
	}
}

func TestManager_PruneItemsGame_LiteMode_PrunesVDFKeys(t *testing.T) {
	t.Parallel()

	sm, _ := setupSchema(t, Config{LiteMode: true})

	raw := &Raw{
		ItemsGame: map[string]any{
			"prefabs":         map[string]any{"test": 1},
			"items":           map[string]any{"1": "test_item"},
			"equip_conflicts": map[string]any{"test": 2},
		},
	}

	sm.pruneItemsGame(raw)

	if _, exists := raw.ItemsGame["prefabs"]; exists {
		t.Error("expected 'prefabs' to be pruned")
	}

	if _, exists := raw.ItemsGame["equip_conflicts"]; exists {
		t.Error("expected 'equip_conflicts' to be pruned")
	}

	if _, exists := raw.ItemsGame["items"]; !exists {
		t.Error("expected 'items' to be kept")
	}
}

func TestManager_Refresh_StandardFlow_SuccessfullyPopulatesSchema(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{LiteMode: false})

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{
		"result": map[string]any{
			"qualities": map[string]any{"Normal": 0, "Genuine": 1},
		},
	})

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaItems", map[string]any{
		"result": map[string]any{
			"items": []any{
				map[string]any{"defindex": 5021, "name": "Mann Co. Supply Crate Key"},
			},
			"next": 0,
		},
	})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "proto_obj_defs") {
			vdf := "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t\t\"9_12_weapon 12\" \"Nutcracker\"\n\t}\n}\n"

			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(vdf)),
				StatusCode: 200,
			}, nil
		}

		if strings.Contains(path, "items_game") {
			vdf := "\"items_game\"\n{\n\t\"valid_key\" \"value\"\n}\n"

			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(vdf)),
				StatusCode: 200,
			}, nil
		}

		return nil, fmt.Errorf("unexpected REST path: %s", path)
	}
	sub := sm.Bus.Subscribe(&UpdatedEvent{})

	err := sm.Refresh(t.Context())
	if err != nil {
		t.Fatalf("unexpected error during Refresh: %v", err)
	}

	schema := sm.Get()
	if schema == nil {
		t.Fatal("expected schema to be populated, got nil")
	}

	if len(schema.Raw.Schema.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(schema.Raw.Schema.Items))
	}

	if schema.Raw.Schema.Items[0].Defindex != 5021 {
		t.Errorf("expected item defindex 5021, got %d", schema.Raw.Schema.Items[0].Defindex)
	}

	if len(schema.Raw.Schema.Qualities) == 0 {
		t.Error("expected raw schema qualities to be populated, but it is empty")
	} else if val, ok := schema.Raw.Schema.Qualities["Normal"]; !ok || val != 0 {
		t.Errorf("expected quality 'Normal' to be 0, got %v (exists: %t)", val, ok)
	}

	select {
	case <-sub.C():
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("SchemaUpdatedEvent was not published")
	}
}

func TestManager_Refresh_PriceDBFlow_SuccessfullyPopulatesSchema(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	priceDBResp := map[string]any{
		"version": "4.5.3",
		"time":    1778817514050.0,
		"raw": map[string]any{
			"schema": map[string]any{
				"items": []any{
					map[string]any{
						"defindex":  5021,
						"name":      "Mann Co. Supply Crate Key",
						"item_name": "Mann Co. Supply Crate Key",
					},
				},
				"paintkits": map[string]any{
					"0": "Red Rock Roscoe",
				},
				"items_game_url": "https://example.com/items_game.txt",
			},
		},
	}

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "pricedb.io/api/schema") {
			respBody, _ := json.Marshal(priceDBResp)

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
			}, nil
		}

		if strings.Contains(path, "items_game") {
			content := "\"items_game\"\n{\n\t\"valid_key\" \"value\"\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(content)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected REST path: %s", path)
	}

	err := sm.Refresh(t.Context())
	if err != nil {
		t.Fatalf("unexpected error during Refresh: %v", err)
	}

	schema := sm.Get()
	if schema == nil {
		t.Fatal("expected schema to be populated, got nil")
	}

	if schema.Version != "4.5.3" {
		t.Errorf("expected version 4.5.3, got %s", schema.Version)
	}

	if len(schema.Raw.Schema.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(schema.Raw.Schema.Items))
	}

	if schema.Raw.Schema.Items[0].Defindex != 5021 {
		t.Errorf("expected item defindex 5021, got %d", schema.Raw.Schema.Items[0].Defindex)
	}

	if schema.SkinByID(0) != "Red Rock Roscoe" {
		t.Errorf("expected paintkit 0 to be Red Rock Roscoe, got %s", schema.SkinByID(0))
	}
}

func TestManager_Refresh_APIErrors_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mockSetup func(m *requester.Mock)
	}{
		{
			name: "items_webapi_error",
			mockSetup: func(m *requester.Mock) {
				m.ResponseErrs["IEconItems_440/GetSchemaItems"] = errors.New("steam api timeout")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm, mockAPI := setupSchema(t, Config{})

			mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{"result": map[string]any{}})
			mockAPI.SetJSONResponse(
				"IEconItems_440",
				"GetSchemaItems",
				map[string]any{"result": map[string]any{"items": []any{}}},
			)

			tt.mockSetup(mockAPI)

			err := sm.Refresh(t.Context())
			if err == nil {
				t.Error("expected error during Refresh, got nil")
			}
		})
	}
}

func TestManager_Refresh_APIErrors_FallbackSucceeds(t *testing.T) {
	t.Parallel()

	t.Run("overview_webapi_error", func(t *testing.T) {
		t.Parallel()

		sm, mockAPI := setupSchema(t, Config{})

		mockAPI.SetJSONResponse(
			"IEconItems_440",
			"GetSchemaItems",
			map[string]any{"result": map[string]any{"items": []any{}}},
		)

		mockAPI.ResponseErrs["IEconItems_440/GetSchemaOverview"] = errors.New("steam api down")

		err := sm.Refresh(t.Context())
		if err != nil {
			t.Errorf("expected Refresh to succeed with fallback overview, got error: %v", err)
		}
	})

	t.Run("github_resource_down", func(t *testing.T) {
		t.Parallel()

		sm, mockAPI := setupSchema(t, Config{})

		mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{"result": map[string]any{}})
		mockAPI.SetJSONResponse(
			"IEconItems_440",
			"GetSchemaItems",
			map[string]any{"result": map[string]any{"items": []any{}}},
		)

		mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
			if strings.Contains(path, "items_game") {
				return nil, errors.New("github connection failed")
			}

			if strings.Contains(path, "proto_obj_defs") {
				return &http.Response{
					Body:       io.NopCloser(strings.NewReader("\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t}\n}\n")),
					StatusCode: 200,
				}, nil
			}

			return &http.Response{
				Body:       io.NopCloser(strings.NewReader("\"items_game\"\n{\n}\n")),
				StatusCode: 200,
			}, nil
		}

		err := sm.Refresh(t.Context())
		if err != nil {
			t.Errorf("expected Refresh to succeed when items_game is down, got error: %v", err)
		}
	})
}

func TestManager_HandleUpdateRequested_EventDispatched_RefreshesAsynchronously(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{
		"result": map[string]any{"qualities": map[string]any{}},
	})
	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaItems", map[string]any{
		"result": map[string]any{"items": []any{}, "next": 0},
	})
	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		var content string
		if strings.Contains(path, "proto_obj_defs") {
			content = "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t}\n}\n"
		} else {
			content = "\"items_game\"\n{\n\t\"valid_key\" \"value\"\n}\n"
		}

		return &http.Response{
			Body:       io.NopCloser(strings.NewReader(content)),
			StatusCode: 200,
		}, nil
	}

	sub := sm.Bus.Subscribe(&UpdatedEvent{})
	subFail := sm.Bus.Subscribe(&UpdateFailedEvent{})

	sm.handleUpdateRequested(&UpdateRequestedEvent{
		Version:      1234,
		ItemsGameURL: "http://example.com/items_game.txt",
	})

	select {
	case <-sub.C():
		// OK
	case ev := <-subFail.C():
		t.Fatalf("Schema update failed: %v", ev.(*UpdateFailedEvent).Error)
	case <-time.After(5 * time.Second):
		t.Error("Schema was not updated after request (timed out)")
	}
}

func TestManager_Refresh_ConcurrentCalls_DeDuplicatesAndBlocks(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{
		"result": map[string]any{"qualities": map[string]any{}},
	})
	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaItems", map[string]any{
		"result": map[string]any{"items": []any{}, "next": 0},
	})

	var (
		callCount int64
		mu        sync.Mutex
		aStarted  = make(chan struct{})
		once      sync.Once
	)

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		var content string
		switch {
		case strings.Contains(path, "proto_obj_defs"):
			content = "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t}\n}\n"
		case strings.Contains(path, "items_game"):
			content = "\"items_game\"\n{\n\t\"valid_key\" \"value\"\n}\n"
		case strings.Contains(path, "pricedb.io/api/schema"):
			mu.Lock()
			callCount++
			mu.Unlock()

			// Signal that Goroutine A has started and reached the slow fetch mock
			once.Do(func() {
				close(aStarted)
			})

			time.Sleep(50 * time.Millisecond)

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"version":"1.0","raw":{"schema":{"items":[]}}}`)),
			}, nil
		}

		return &http.Response{
			Body:       io.NopCloser(strings.NewReader(content)),
			StatusCode: 200,
		}, nil
	}

	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()

	go func() {
		defer wg.Done()

		_ = sm.Refresh(t.Context())
	}()

	go func() {
		defer wg.Done()

		// Wait for Goroutine A to fully initiate and enter the slow PriceDB mock
		select {
		case <-aStarted:
		case <-time.After(1 * time.Second): // Fail-safe
		}

		_ = sm.Refresh(t.Context())
	}()

	wg.Wait()

	duration := time.Since(start)

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	if finalCount != 1 {
		t.Errorf("expected PriceDB schema to only be fetched 1 time, got %d", finalCount)
	}

	if duration.Milliseconds() < 50 {
		t.Errorf("expected concurrent calls to block and wait, took only %d ms", duration.Milliseconds())
	}
}
