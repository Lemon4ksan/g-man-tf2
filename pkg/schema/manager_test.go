// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRoundTripper struct {
	OnRoundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.OnRoundTrip(req)
}

func setupSchema(t *testing.T, cfg Config) (*Manager, *mock.ServiceMock) {
	t.Helper()

	defaultCfg := DefaultConfig()
	cfg.ItemsGameMirrorURL = generic.Coalesce(cfg.ItemsGameMirrorURL, defaultCfg.ItemsGameMirrorURL)
	cfg.PaintKitURL = generic.Coalesce(cfg.PaintKitURL, defaultCfg.PaintKitURL)

	mockAPI := mock.NewServiceMock()

	rt := &mockRoundTripper{
		OnRoundTrip: func(req *http.Request) (*http.Response, error) {
			return mockAPI.Request(req.Context(), req.Method, req.URL.String(), func(r *http.Request) {
				r.Header = req.Header
				r.Body = req.Body
			})
		},
	}

	hc := &http.Client{Transport: rt}
	aoniClient := aoni.NewClient(hc)

	init := mock.NewInitContext()
	init.SetService(mockAPI)
	init.SetRest(aoniClient)

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

	mockAPI.ResponseErrs["api/schema"] = errors.New("pricedb not available in standard flow")

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
	defer sub.Unsubscribe()

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
	case <-time.After(2 * time.Second):
		t.Error("SchemaUpdatedEvent was not published")
	}
}

func TestManager_Refresh_PriceDBFlow_SuccessfullyPopulatesSchema(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	priceDBResp := map[string]any{
		"version": "1",
		"time":    float64(time.Now().UnixNano() / int64(time.Millisecond)),
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

	if schema.Version != "1" {
		t.Errorf("expected version 1, got %s", schema.Version)
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
		mockSetup func(m *mock.ServiceMock)
	}{
		{
			name: "items_webapi_error",
			mockSetup: func(m *mock.ServiceMock) {
				m.ResponseErrs["IEconItems_440/GetSchemaItems"] = errors.New("generic steam error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sm, mockAPI := setupSchema(t, Config{})

			mockAPI.ResponseErrs["api/schema"] = errors.New("pricedb not available in error flow")

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

	mockAPI.ResponseErrs["api/schema"] = errors.New("pricedb not available in update flow")

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
	defer sub.Unsubscribe()

	sm.handleUpdateRequested(&UpdateRequestedEvent{
		Version:      1234,
		ItemsGameURL: "http://example.com/items_game.txt",
	})

	select {
	case <-sub.C():
		// OK
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
		bCanEnter = make(chan struct{})
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

			once.Do(func() {
				close(aStarted)
			})

			<-bCanEnter

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

	go func() {
		defer wg.Done()

		_ = sm.Refresh(t.Context())
	}()

	go func() {
		defer wg.Done()

		<-aStarted
		close(bCanEnter)

		_ = sm.Refresh(t.Context())
	}()

	wg.Wait()

	mu.Lock()
	finalCount := callCount
	mu.Unlock()

	assert.Equal(t, int64(1), finalCount)
}

func TestManager_ParseTfEnglish(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		vdf := "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t\t\"#Scattergun\" \"Scattergun\"\n\t\t\"#Pistol\" \"Pistol\"\n\t\t\"InvalidLine\"\n\t}\n}\n"

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(vdf)),
		}, nil
	}

	res := sm.parseTfEnglish(t.Context())
	require.NotNil(t, res)
	assert.Equal(t, "Scattergun", res["#Scattergun"])
	assert.Equal(t, "Pistol", res["#Pistol"])
}

func TestManager_ParseItemsGameItems(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "tf_english") {
			vdf := "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t\t\"#TF_Scattergun\" \"Scattergun\"\n\t\t\"Crate_Style1\" \"Mann Co. Crate\"\n\t}\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(vdf)),
			}, nil
		}

		if strings.Contains(path, "items_game") {
			vdf := "\"items_game\"\n{\n\t\"items\"\n\t{\n\t\t\"5021\"\n\t\t{\n\t\t\t\"name\" \"TF_WEAPON_SCATTERGUN\"\n\t\t\t\"localizedname\" \"#TF_Scattergun\"\n\t\t\t\"item_class\" \"tf_weapon_scattergun\"\n\t\t\t\"item_slot\" \"primary\"\n\t\t\t\"proper_name\" \"1\"\n\t\t\t\"craft_class\" \"weapon\"\n\t\t}\n\t\t\"5022\"\n\t\t{\n\t\t\t\"name\" \"Crate\"\n\t\t\t\"localizedname\" \"Crate_Style1\"\n\t\t}\n\t\t\"invalid_line\"\n\t}\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(vdf)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res := sm.parseItemsGameItems(t.Context(), "http://mock/items_game")
	require.Len(t, res, 2)

	item1 := res[0].(map[string]any)
	assert.Equal(t, float64(5021), item1["defindex"])
	assert.Equal(t, "Scattergun", item1["item_name"])
	assert.Equal(t, "tf_weapon_scattergun", item1["item_class"])
	assert.Equal(t, "primary", item1["item_slot"])
	assert.True(t, item1["proper_name"].(bool))
	assert.Equal(t, "weapon", item1["craft_class"])

	item2 := res[1].(map[string]any)
	assert.Equal(t, float64(5022), item2["defindex"])
	assert.Equal(t, "Mann Co. Crate", item2["item_name"])
}

func TestManager_GetPaintKits(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "paint_kits") || strings.Contains(path, "resource") {
			vdf := "\"lang\"\n{\n\t\"Tokens\"\n\t{\n\t\t\"9_12_weapon 12\" \"Woodsy Widowmaker\"\n\t\t\"invalid_token\" \"value\"\n\t}\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(vdf)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected REST path: %s", path)
	}

	res, err := sm.getPaintKits(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Woodsy Widowmaker", res["12"])
}

func TestManager_HandleUpdateRequested_Skipping(t *testing.T) {
	t.Parallel()

	sm, _ := setupSchema(t, Config{})
	raw := minimalRawSchema()
	sm.schema = New(raw)
	sm.schema.Version = "http://example.com/items_game.txt"
	sm.lastGCVersion = 1234

	sm.handleUpdateRequested(&UpdateRequestedEvent{
		Version:      1234,
		ItemsGameURL: "http://example.com/items_game.txt",
	})

	sm.handleUpdateRequested(&UpdateRequestedEvent{
		Version:      5678,
		ItemsGameURL: "http://example.com/items_game.txt",
	})
	assert.Equal(t, uint32(5678), sm.lastGCVersion)
}

func TestManager_RefreshLoop_Error(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{
		UpdateInterval: 1 * time.Millisecond,
	})

	mockAPI.ResponseErrs["IEconItems_440/GetSchemaOverview"] = errors.New("refresh failed")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go sm.refreshLoop(ctx)

	select {
	case <-time.After(10 * time.Millisecond):
	case <-ctx.Done():
	}
}

func TestSchema_StartAuthed_EventDispatched_Refreshes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "schema.json")

	raw := minimalRawSchema()
	sObj := New(raw)
	sObj.Version = "1"
	sObj.Time = time.Now()
	data, _ := json.Marshal(sObj)
	_ = os.WriteFile(cacheFile, data, 0o644)

	sm, mockAPI := setupSchema(t, Config{
		CachePath:      cacheFile,
		UpdateInterval: 10 * time.Minute,
	})

	priceDBResp := map[string]any{
		"version": "1.0",
		"time":    float64(time.Now().UnixNano() / int64(time.Millisecond)),
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
				"items_game_url": "http://example.com/items_game.txt",
			},
		},
	}

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{
		"result": map[string]any{
			"qualities": map[string]any{"Normal": 0, "Genuine": 1},
		},
	})

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

	authCtx := mock.NewAuthContext(7656119)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := sm.StartAuthed(ctx, authCtx)
	assert.NoError(t, err)

	sub := sm.Bus.Subscribe(&UpdatedEvent{})
	defer sub.Unsubscribe()

	sm.handleUpdateRequested(&UpdateRequestedEvent{
		Version:      5678,
		ItemsGameURL: "http://example.com/items_game.txt",
	})

	select {
	case <-sub.C():
		// Success
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for schema update event")
	}
}

func TestCoverage_ManagerCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "schema.json")

	sm := NewManager(Config{CachePath: cacheFile})

	err := sm.loadFromCache()
	assert.Error(t, err)

	err = sm.saveToCache()
	assert.NoError(t, err)

	raw := minimalRawSchema()
	sm.schema = New(raw)

	err = sm.saveToCache()
	assert.NoError(t, err)

	sm2 := NewManager(Config{CachePath: cacheFile})
	err = sm2.loadFromCache()
	assert.NoError(t, err)
	assert.NotNil(t, sm2.Get())
	assert.Equal(t, len(sm.schema.Raw.Schema.Items), len(sm2.schema.Raw.Schema.Items))

	err = os.WriteFile(cacheFile, []byte(`{"version":"1"}`), 0o644)
	assert.NoError(t, err)

	sm3 := NewManager(Config{CachePath: cacheFile})
	err = sm3.loadFromCache()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete")

	err = os.WriteFile(cacheFile, []byte(`invalid json`), 0o644)
	assert.NoError(t, err)

	sm4 := NewManager(Config{CachePath: cacheFile})
	err = sm4.loadFromCache()
	assert.Error(t, err)

	sm5 := NewManager(Config{CachePath: ""})
	assert.ErrorContains(t, sm5.loadFromCache(), "not configured")
	assert.Nil(t, sm5.saveToCache())
}

func TestCoverage_MirrorFetch(t *testing.T) {
	t.Parallel()

	sm, _ := setupSchema(t, Config{
		SchemaMirrorURL: "",
	})

	_, err := sm.fetchFromMirror(t.Context())
	assert.ErrorContains(t, err, "not configured")

	sm2, mockAPI2 := setupSchema(t, Config{
		SchemaMirrorURL: "http://mirror/overview",
	})

	mockAPI2.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "overview") {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`{"result":{"qualities":{}}}`)),
				StatusCode: 200,
			}, nil
		}

		if strings.Contains(path, "items") {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				StatusCode: 200,
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res, err := sm2.fetchFromMirror(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestManager_StartAuthed_FailsToLoadCache(t *testing.T) {
	t.Parallel()
	sm, mockAPI := setupSchema(t, Config{
		CachePath: "//",
	})

	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaOverview", map[string]any{
		"result": map[string]any{"qualities": map[string]any{}},
	})
	mockAPI.SetJSONResponse("IEconItems_440", "GetSchemaItems", map[string]any{
		"result": map[string]any{"items": []any{}},
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

	authCtx := mock.NewAuthContext(7656119)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := sm.StartAuthed(ctx, authCtx)
	assert.NoError(t, err)
}

func TestCoverage_ManagerErrors(t *testing.T) {
	t.Parallel()

	sm := NewManager(Config{})

	assert.True(t, sm.isForbiddenError(errors.New("403 Forbidden")))
	assert.False(t, sm.isForbiddenError(errors.New("generic error")))

	apiErr := &service.SteamAPIError{
		StatusCode: 403,
		Message:    "Forbidden",
	}
	assert.True(t, sm.isForbiddenError(apiErr))

	restErr := &aoni.APIError{
		StatusCode: 403,
	}
	assert.True(t, sm.isForbiddenError(restErr))
}

func TestCoverage_GetItemsGame_Deep(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "test_items_game") {
			vdf := "\"items_game\"\n{\n\t\"items\"\n\t{\n\t\t\"5022\"\n\t\t{\n\t\t\t\"static_attrs\"\n\t\t\t{\n\t\t\t\t\"set supply crate series\" \"1\"\n\t\t\t}\n\t\t}\n\t}\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(vdf)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res, err := sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	items := res["items"].(map[string]any)
	assert.Contains(t, items, "5022")

	sm.config.ItemsGameMirrorURL = "http://mock/test_items_game"
	res2, err := sm.getItemsGame(t.Context(), "")
	assert.NoError(t, err)
	assert.NotNil(t, res2)

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("error")),
		}, nil
	}
	_, err = sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.Error(t, err)

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		return nil, errors.New("network error")
	}
	_, err = sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.Error(t, err)
}

func TestCoverage_FetchItemsFromMirror(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{
		ItemsMirrorURL: "http://mirror/items",
	})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "items") {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				StatusCode: 200,
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res, err := sm.fetchItemsFromMirror(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, res)
}
