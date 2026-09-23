// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricemanager

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/lemon4ksan/g-man/pkg/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
)

func TestPriceManager(t *testing.T) {
	t.Parallel()

	t.Run("update_basic_and_complex_skus", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		mockResp := bptf.V4PricesResponseExt{
			Success: bptf.SuccessBoolProp(1),
			Items: map[string]bptf.V4PricesBaseItemDocExt{
				"Mann Co. Supply Crate Key": {
					Defindex: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"6": {
							"Tradable": {
								"Craftable": {
									"0": {Value: 75},
								},
							},
						},
					},
				},
				"Unusual Hat": {
					Defindex: []string{"100"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"5": {
							"Tradable": {
								"Craftable": {
									"19": {Value: 120},
								},
							},
						},
					},
				},
				"Crate": {
					Defindex: []string{"500"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"6": {
							"Tradable": {
								"Craftable": {
									"82": {Value: 15},
								},
							},
						},
					},
				},
			},
		}
		stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		cfg := Config{CachePath: filepath.Join(t.TempDir(), "prices.json")}
		manager := NewPriceManager(r, log.Discard, cfg)

		err := manager.Update(t.Context())
		require.NoError(t, err)

		p1, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), p1.Value)

		p2, ok := manager.GetPrice("100;5;u19")
		assert.True(t, ok)
		assert.Equal(t, float64(120), p2.Value)

		p3, ok := manager.GetPrice("500;6;c82")
		assert.True(t, ok)
		assert.Equal(t, float64(15), p3.Value)

		assert.Equal(t, "bptf_prices", manager.Name())

		err = manager.Load()
		require.NoError(t, err)

		pLoad, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), pLoad.Value)
	})

	t.Run("update_error_fails_cleanly", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/IGetPrices/v4", 500, nil)

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		manager := NewPriceManager(r, log.Discard, Config{})
		err := manager.Update(t.Context())
		assert.Error(t, err)
	})

	t.Run("load_and_save_errors", func(t *testing.T) {
		t.Parallel()

		managerEmpty := NewPriceManager(nil, log.Discard, Config{})
		err := managerEmpty.Load()
		assert.Error(t, err)
	})

	t.Run("price_manager_run_lifecycle", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		manager := NewPriceManager(r, log.Discard, Config{SyncInterval: 10 * time.Millisecond})

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := manager.Run(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestPriceManager_TTLAndExpiry(t *testing.T) {
	t.Parallel()

	t.Run("ttl_expiry_and_stale_lookup", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		mockResp := bptf.V4PricesResponseExt{
			Success: bptf.SuccessBoolProp(1),
			Items: map[string]bptf.V4PricesBaseItemDocExt{
				"Mann Co. Supply Crate Key": {
					Defindex: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
					},
				},
			},
		}
		stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		cfg := Config{
			CachePath:    filepath.Join(t.TempDir(), "prices.json"),
			SyncInterval: 500 * time.Millisecond,
			TTL:          250 * time.Millisecond,
		}
		manager := NewPriceManager(r, log.Discard, cfg)

		// Before update: empty & expired
		assert.True(t, manager.IsExpired())
		assert.Equal(t, int64(0), manager.Version())

		_, ok := manager.GetPrice("5021;6")
		assert.False(t, ok)

		// Update
		err := manager.Update(t.Context())
		require.NoError(t, err)

		// Immediately after update: valid and not expired
		assert.False(t, manager.IsExpired())
		assert.Equal(t, int64(1), manager.Version())
		assert.False(t, manager.Timestamp().IsZero())
		assert.Equal(t, 250*time.Millisecond, manager.TTL())

		meta := manager.Metadata()
		assert.Equal(t, int64(1), meta.Version)
		assert.Equal(t, 1, meta.ItemCount)
		assert.False(t, meta.IsExpired)

		p, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), p.Value)

		// Wait for TTL expiration
		time.Sleep(300 * time.Millisecond)

		assert.True(t, manager.IsExpired())

		metaExpired := manager.Metadata()
		assert.True(t, metaExpired.IsExpired)

		// GetPrice returns false on expired cache
		_, ok = manager.GetPrice("5021;6")
		assert.False(t, ok)

		// GetPriceStale still returns the cached price
		pStale, ok := manager.GetPriceStale("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), pStale.Value)
	})

	t.Run("negative_ttl_disables_expiry", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		mockResp := bptf.V4PricesResponseExt{
			Success: bptf.SuccessBoolProp(1),
			Items: map[string]bptf.V4PricesBaseItemDocExt{
				"Mann Co. Supply Crate Key": {
					Defindex: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
					},
				},
			},
		}
		stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		cfg := Config{
			CachePath:    filepath.Join(t.TempDir(), "prices.json"),
			SyncInterval: 10 * time.Millisecond,
			TTL:          -1, // disabled
		}
		manager := NewPriceManager(r, log.Discard, cfg)
		err := manager.Update(t.Context())
		require.NoError(t, err)

		time.Sleep(20 * time.Millisecond)
		assert.False(t, manager.IsExpired())

		_, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
	})
}

func TestPriceManager_InvalidationAPI(t *testing.T) {
	t.Parallel()

	stub := mock.NewHTTPStub()
	mockResp := bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items: map[string]bptf.V4PricesBaseItemDocExt{
			"Mann Co. Supply Crate Key": {
				Defindex: []string{"5021"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
				},
			},
			"Refined Metal": {
				Defindex: []string{"5002"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 1}}}},
				},
			},
		},
	}
	stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

	r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
	cfg := Config{
		CachePath:    filepath.Join(t.TempDir(), "prices.json"),
		SyncInterval: time.Hour,
		TTL:          2 * time.Hour,
	}
	manager := NewPriceManager(r, log.Discard, cfg)

	err := manager.Update(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(1), manager.Version())

	// Both items present
	_, ok1 := manager.GetPrice("5021;6")
	assert.True(t, ok1)

	_, ok2 := manager.GetPrice("5002;6")
	assert.True(t, ok2)

	// Invalidate single item
	deleted := manager.Invalidate("5021;6")
	assert.True(t, deleted)
	assert.Equal(t, int64(2), manager.Version())

	// Deleted item gone, remaining item still present
	_, ok1 = manager.GetPrice("5021;6")
	assert.False(t, ok1)

	_, ok2 = manager.GetPrice("5002;6")
	assert.True(t, ok2)

	// Invalidating non-existent item is a no-op
	deletedAgain := manager.Invalidate("5021;6")
	assert.False(t, deletedAgain)
	assert.Equal(t, int64(2), manager.Version()) // Version unchanged

	// InvalidateAll
	manager.InvalidateAll()
	assert.Equal(t, int64(3), manager.Version())
	assert.True(t, manager.Timestamp().IsZero())
	assert.True(t, manager.IsExpired())

	_, ok2 = manager.GetPrice("5002;6")
	assert.False(t, ok2)
	assert.Equal(t, 0, manager.Metadata().ItemCount)
}

func TestPriceManager_CacheEnvelopeBackwardCompatibility(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("save_and_load_envelope", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		mockResp := bptf.V4PricesResponseExt{
			Success: bptf.SuccessBoolProp(1),
			Items: map[string]bptf.V4PricesBaseItemDocExt{
				"Mann Co. Supply Crate Key": {
					Defindex: []string{"5021"},
					Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
						"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
					},
				},
			},
		}
		stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

		cacheFile := filepath.Join(tmpDir, "envelope.json")
		r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
		mgr1 := NewPriceManager(r, log.Discard, Config{
			CachePath:    cacheFile,
			SyncInterval: time.Hour,
			TTL:          2 * time.Hour,
		})

		require.NoError(t, mgr1.Update(t.Context()))
		origMeta := mgr1.Metadata()

		mgr2 := NewPriceManager(nil, log.Discard, Config{
			CachePath: cacheFile,
			TTL:       2 * time.Hour,
		})
		require.NoError(t, mgr2.Load())

		meta2 := mgr2.Metadata()
		assert.Equal(t, origMeta.Version, meta2.Version)
		assert.Equal(t, origMeta.ItemCount, meta2.ItemCount)
		assert.Equal(t, origMeta.TTL, meta2.TTL)
		assert.False(t, meta2.IsExpired)

		p, ok := mgr2.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, float64(75), p.Value)
	})

	t.Run("load_legacy_raw_map", func(t *testing.T) {
		t.Parallel()

		legacyFile := filepath.Join(tmpDir, "legacy.json")
		legacyData := []byte(`{"5021;6": {"value": 80.5, "currency": "metal"}}`)
		require.NoError(t, os.WriteFile(legacyFile, legacyData, 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{
			CachePath: legacyFile,
			TTL:       time.Hour,
		})
		require.NoError(t, mgr.Load())

		assert.Equal(t, int64(1), mgr.Version())
		assert.Equal(t, 1, mgr.Metadata().ItemCount)

		p, ok := mgr.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, 80.5, p.Value)
	})
}

func TestPriceManager_ConcurrentRaceSafety(t *testing.T) {
	t.Parallel()

	stub := mock.NewHTTPStub()
	mockResp := bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items: map[string]bptf.V4PricesBaseItemDocExt{
			"Mann Co. Supply Crate Key": {
				Defindex: []string{"5021"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
				},
			},
			"Refined Metal": {
				Defindex: []string{"5002"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 1}}}},
				},
			},
		},
	}
	stub.SetJSONResponse("api/IGetPrices/v4", 200, mockResp)

	r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
	cacheFile := filepath.Join(t.TempDir(), "race_prices.json")
	mgr := NewPriceManager(r, log.Discard, Config{
		CachePath:    cacheFile,
		SyncInterval: 10 * time.Millisecond,
		TTL:          100 * time.Millisecond,
	})

	require.NoError(t, mgr.Update(t.Context()))

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	// 10 concurrent readers calling GetPrice & GetPriceStale
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := "5021;6"
					if id%2 == 0 {
						sku = "5002;6"
					}

					_, _ = mgr.GetPrice(sku)
					_, _ = mgr.GetPriceStale(sku)
				}
			}
		}(i)
	}

	// 5 concurrent readers querying Metadata, Version, Timestamp, IsExpired
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = mgr.Metadata()
					_ = mgr.Version()
					_ = mgr.Timestamp()
					_ = mgr.TTL()
					_ = mgr.IsExpired()
				}
			}
		}()
	}

	// 3 concurrent mutators calling Invalidate
	for i := 0; i < 3; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := "5021;6"
					if id%2 == 0 {
						sku = "5002;6"
					}

					mgr.Invalidate(sku)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// 1 periodic updater calling InvalidateAll and Load
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				mgr.InvalidateAll()
				_ = mgr.Load()

				time.Sleep(15 * time.Millisecond)
			}
		}
	}()

	wg.Wait()
}

type sequenceDoer struct {
	mu          sync.Mutex
	attempts    int
	statuses    []int
	headers     []string
	successBody []byte
}

func (s *sequenceDoer) Do(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.attempts
	s.attempts++

	if idx >= len(s.statuses) {
		idx = len(s.statuses) - 1
	}

	status := s.statuses[idx]
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	if idx < len(s.headers) && s.headers[idx] != "" {
		header.Set("Retry-After", s.headers[idx])
	}

	var body io.ReadCloser
	if status == http.StatusOK {
		body = io.NopCloser(bytes.NewReader(s.successBody))
	} else {
		body = io.NopCloser(strings.NewReader(`{"message":"upstream error"}`))
	}

	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       body,
		Request:    req,
	}, nil
}

func TestPriceManager_ResilienceAndFallback(t *testing.T) {
	t.Parallel()

	validResponse := bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items: map[string]bptf.V4PricesBaseItemDocExt{
			"Mann Co. Supply Crate Key": {
				Defindex: []string{"5021"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 75}}}},
				},
			},
		},
	}
	validBytes, err := json.Marshal(validResponse)
	require.NoError(t, err)

	tests := []struct {
		name              string
		statusSequence    []int
		retryAfterHeaders []string
		hasCacheFile      bool
		prepopulateMemory bool
		expectSuccess     bool
		expectedAttempts  int
	}{
		{
			name:             "happy_path_first_attempt",
			statusSequence:   []int{http.StatusOK},
			expectSuccess:    true,
			expectedAttempts: 1,
		},
		{
			name:             "retry_502_bad_gateway_then_success",
			statusSequence:   []int{http.StatusBadGateway, http.StatusOK},
			expectSuccess:    true,
			expectedAttempts: 2,
		},
		{
			name:             "retry_503_service_unavailable_then_success",
			statusSequence:   []int{http.StatusServiceUnavailable, http.StatusOK},
			expectSuccess:    true,
			expectedAttempts: 2,
		},
		{
			name:             "retry_504_gateway_timeout_then_success",
			statusSequence:   []int{http.StatusGatewayTimeout, http.StatusOK},
			expectSuccess:    true,
			expectedAttempts: 2,
		},
		{
			name:              "retry_429_too_many_requests_then_success",
			statusSequence:    []int{http.StatusTooManyRequests, http.StatusOK},
			retryAfterHeaders: []string{"1", ""},
			expectSuccess:     true,
			expectedAttempts:  2,
		},
		{
			name:             "permanent_503_fallback_to_disk_cache",
			statusSequence:   []int{503, 503, 503, 503},
			hasCacheFile:     true,
			expectSuccess:    true,
			expectedAttempts: 4,
		},
		{
			name:              "permanent_503_fallback_retains_memory_index",
			statusSequence:    []int{503, 503, 503, 503},
			prepopulateMemory: true,
			expectSuccess:     true,
			expectedAttempts:  4,
		},
		{
			name:             "permanent_503_no_cache_fails_cleanly",
			statusSequence:   []int{503, 503, 503, 503},
			expectSuccess:    false,
			expectedAttempts: 4,
		},
		{
			name:             "non_retriable_401_fails_immediately",
			statusSequence:   []int{http.StatusUnauthorized},
			expectSuccess:    false,
			expectedAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doer := &sequenceDoer{
				statuses:    tt.statusSequence,
				headers:     tt.retryAfterHeaders,
				successBody: validBytes,
			}

			cacheFile := filepath.Join(t.TempDir(), "prices.json")
			if tt.hasCacheFile {
				env := cacheFileEnvelope{
					Version:   1,
					Timestamp: time.Now(),
					TTL:       time.Hour,
					Index: map[string]bptf.V4PricesEntry{
						"5021;6": {Value: 75},
					},
				}
				data, mErr := json.Marshal(env)
				require.NoError(t, mErr)
				require.NoError(t, os.WriteFile(cacheFile, data, 0o644))
			}

			client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
			mgr := NewPriceManager(client, log.Discard, Config{
				CachePath:    cacheFile,
				SyncInterval: time.Hour,
				TTL:          2 * time.Hour,
			})

			if tt.prepopulateMemory {
				mgr.mu.Lock()
				mgr.index["5021;6"] = bptf.V4PricesEntry{Value: 75}
				mgr.timestamp = time.Now()
				mgr.version = 1
				mgr.mu.Unlock()
			}

			err := mgr.Update(t.Context())
			if tt.expectSuccess {
				assert.NoError(t, err)

				_, ok := mgr.GetPriceStale("5021;6")
				assert.True(t, ok)
			} else {
				assert.Error(t, err)
			}

			doer.mu.Lock()
			actualAttempts := doer.attempts
			doer.mu.Unlock()

			assert.Equal(t, tt.expectedAttempts, actualAttempts)
		})
	}
}
