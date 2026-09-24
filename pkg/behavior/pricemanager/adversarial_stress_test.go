// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricemanager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
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

// -----------------------------------------------------------------------------
// 1. Concurrency & Data Race Safety Harness
// -----------------------------------------------------------------------------

// TestAdversarial_ConcurrentReadWriteInvalidateReload tests the PriceManager under
// aggressive concurrent access where multiple goroutines call GetPrice, GetPriceStale,
// Metadata, Invalidate, InvalidateAll, Load, AND Update concurrently.
func TestAdversarial_ConcurrentReadWriteInvalidateReload(t *testing.T) {
	t.Parallel()

	// Prepare mock response with 100 items
	items := make(map[string]bptf.V4PricesBaseItemDocExt, 100)
	for i := 1; i <= 100; i++ {
		name := fmt.Sprintf("Item %d", i)
		items[name] = bptf.V4PricesBaseItemDocExt{
			Defindex: []string{strconv.Itoa(i)},
			Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
				"6": {"Tradable": {"Craftable": {"0": {Value: float64(i)}}}},
			},
		}
	}

	stub := mock.NewHTTPStub()
	stub.SetJSONResponse("api/IGetPrices/v4", 200, bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items:   items,
	})

	r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
	cacheFile := filepath.Join(t.TempDir(), "adv_race_prices.json")
	mgr := NewPriceManager(r, log.Discard, Config{
		CachePath:    cacheFile,
		SyncInterval: 10 * time.Millisecond,
		TTL:          200 * time.Millisecond,
	})

	require.NoError(t, mgr.Update(t.Context()))

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// 5 concurrent writers calling Update()
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = mgr.Update(ctx)

					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}

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
					sku := fmt.Sprintf("%d;6", (id%100)+1)
					_, _ = mgr.GetPrice(sku)
					_, _ = mgr.GetPriceStale(sku)
				}
			}
		}(i)
	}

	// 5 concurrent readers calling Metadata & Version
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
					_ = mgr.IsExpired()
				}
			}
		}()
	}

	// 5 concurrent invalidators calling Invalidate()
	for i := 0; i < 5; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := fmt.Sprintf("%d;6", (id%100)+1)
					mgr.Invalidate(sku)

					time.Sleep(2 * time.Millisecond)
				}
			}
		}(i)
	}

	// 2 concurrent reloaders calling Load() and InvalidateAll()
	for i := 0; i < 2; i++ {
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

					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
}

// TestAdversarial_100ConcurrentGoroutines_InvalidateSaveGetPrice executes 100 concurrent
// goroutines calling Invalidate(sku), InvalidateAll(), GetPrice(sku), GetPriceStale(sku),
// and Save() simultaneously to verify that the concurrent map iteration/write crash and
// m.version data race are completely eliminated under aggressive load.
func TestAdversarial_100ConcurrentGoroutines_InvalidateSaveGetPrice(t *testing.T) {
	t.Parallel()

	// Prepare mock response with 200 items
	items := make(map[string]bptf.V4PricesBaseItemDocExt, 200)
	for i := 1; i <= 200; i++ {
		name := fmt.Sprintf("Item %d", i)
		items[name] = bptf.V4PricesBaseItemDocExt{
			Defindex: []string{strconv.Itoa(i)},
			Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
				"6": {"Tradable": {"Craftable": {"0": {Value: float64(i)}}}},
			},
		}
	}

	stub := mock.NewHTTPStub()
	stub.SetJSONResponse("api/IGetPrices/v4", 200, bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items:   items,
	})

	r := aoni.NewClient(stub, option.WithBaseURL("https://backpack.tf/api"))
	cacheFile := filepath.Join(t.TempDir(), "concurrent_100_prices.json")
	mgr := NewPriceManager(r, log.Discard, Config{
		CachePath:    cacheFile,
		SyncInterval: 100 * time.Millisecond,
		TTL:          500 * time.Millisecond,
	})

	require.NoError(t, mgr.Update(t.Context()))

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	// 100 concurrent goroutines:
	// 25 GetPrice readers
	// 25 GetPriceStale readers
	// 20 Invalidate mutators
	// 10 InvalidateAll & Update mutators
	// 20 Save mutators

	for i := 0; i < 25; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := fmt.Sprintf("%d;6", (id%200)+1)
					_, _ = mgr.GetPrice(sku)
				}
			}
		}(i)
	}

	for i := 0; i < 25; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := fmt.Sprintf("%d;6", (id%200)+1)
					_, _ = mgr.GetPriceStale(sku)
				}
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					sku := fmt.Sprintf("%d;6", (id%200)+1)
					mgr.Invalidate(sku)

					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					mgr.InvalidateAll()
					_ = mgr.Update(ctx)

					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = mgr.Save()

					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()

	meta := mgr.Metadata()
	assert.GreaterOrEqual(t, meta.Version, int64(1))
}

// -----------------------------------------------------------------------------
// 2. Cache Persistence: Forward, Backward, and Malformed Envelope Testing
// -----------------------------------------------------------------------------

func TestAdversarial_CachePersistence_EdgeCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("forward_compatibility_extra_fields_and_higher_version", func(t *testing.T) {
		t.Parallel()

		futureFile := filepath.Join(tmpDir, "future_envelope.json")
		futureData := []byte(`{
			"version": 42,
			"timestamp": "2026-09-23T12:00:00Z",
			"ttl": 7200000000000,
			"schema_revision": "v2.5-alpha",
			"metadata_checksum": "sha256:abc123def456",
			"compression": "none",
			"index": {
				"5021;6": {"value": 78.5, "currency": "metal"},
				"100;5;u19": {"value": 150.0, "currency": "keys"}
			}
		}`)
		require.NoError(t, os.WriteFile(futureFile, futureData, 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{
			CachePath: futureFile,
			TTL:       time.Hour,
		})
		require.NoError(t, mgr.Load())

		assert.Equal(t, int64(42), mgr.Version())
		assert.Equal(t, 2, mgr.Metadata().ItemCount)

		p1, ok := mgr.GetPriceStale("5021;6")
		assert.True(t, ok)
		assert.Equal(t, 78.5, p1.Value)

		p2, ok := mgr.GetPriceStale("100;5;u19")
		assert.True(t, ok)
		assert.Equal(t, 150.0, p2.Value)
	})

	t.Run("malformed_json_syntax_returns_error", func(t *testing.T) {
		t.Parallel()

		corruptFile := filepath.Join(tmpDir, "corrupted_syntax.json")
		require.NoError(t, os.WriteFile(corruptFile, []byte(`{"version": 1, "index": { broken json`), 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: corruptFile})
		err := mgr.Load()
		assert.Error(t, err, "Malformed JSON syntax must return a non-nil error")
	})

	t.Run("malformed_envelope_type_mismatch_returns_error", func(t *testing.T) {
		t.Parallel()

		typeMismatchFile := filepath.Join(tmpDir, "type_mismatch.json")
		// "version" is a string instead of an int64, and "index" is an array
		badPayload := []byte(`{"version": "forty-two", "index": [1, 2, 3]}`)
		require.NoError(t, os.WriteFile(typeMismatchFile, badPayload, 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: typeMismatchFile})
		err := mgr.Load()
		assert.Error(t, err, "Type-mismatched envelope must return error")
	})

	t.Run("malformed_envelope_nil_index_with_version_and_timestamp", func(t *testing.T) {
		t.Parallel()

		nilIndexFile := filepath.Join(tmpDir, "nil_index.json")
		// Envelope metadata present but index is null
		nilPayload := []byte(`{"version": 3, "timestamp": "2026-09-23T12:00:00Z", "index": null}`)
		require.NoError(t, os.WriteFile(nilIndexFile, nilPayload, 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: nilIndexFile})
		err := mgr.Load()
		assert.Error(t, err, "Envelope with null index and metadata cannot fallback cleanly to raw map")
	})

	t.Run("empty_file_zero_bytes_returns_error", func(t *testing.T) {
		t.Parallel()

		emptyFile := filepath.Join(tmpDir, "empty_zero_bytes.json")
		require.NoError(t, os.WriteFile(emptyFile, []byte{}, 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: emptyFile})
		err := mgr.Load()
		assert.Error(t, err, "Zero-byte file must return EOF / unmarshal error")
	})

	t.Run("nonexistent_file_returns_error", func(t *testing.T) {
		t.Parallel()

		nonexistentFile := filepath.Join(tmpDir, "does_not_exist.json")
		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: nonexistentFile})
		err := mgr.Load()
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("empty_cache_path_returns_typed_error", func(t *testing.T) {
		t.Parallel()

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: ""})
		err := mgr.Load()
		assert.ErrorIs(t, err, ErrCachePathNotConfigured)
	})

	t.Run("directory_as_cache_path_returns_error", func(t *testing.T) {
		t.Parallel()

		subDir := filepath.Join(tmpDir, "is_a_dir")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: subDir})
		err := mgr.Load()
		assert.Error(t, err, "Reading a directory path as cache file must fail")
	})

	t.Run("legacy_empty_map_loads_empty_index", func(t *testing.T) {
		t.Parallel()

		legacyEmptyFile := filepath.Join(tmpDir, "legacy_empty.json")
		require.NoError(t, os.WriteFile(legacyEmptyFile, []byte(`{}`), 0o644))

		mgr := NewPriceManager(nil, log.Discard, Config{CachePath: legacyEmptyFile})
		err := mgr.Load()
		assert.NoError(t, err)
		assert.Equal(t, 0, mgr.Metadata().ItemCount)
		assert.Equal(t, int64(1), mgr.Version())
	})
}

// -----------------------------------------------------------------------------
// 3. Upstream Chaos & Retry Backoff Testing
// -----------------------------------------------------------------------------

type chaosDoer struct {
	mu           sync.Mutex
	attempts     int
	statuses     []int
	headers      map[int]map[string]string
	netErrors    map[int]error
	successBody  []byte
	requestTimes []time.Time
}

func (c *chaosDoer) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := c.attempts
	c.attempts++
	c.requestTimes = append(c.requestTimes, time.Now())

	if netErr, hasErr := c.netErrors[idx]; hasErr && netErr != nil {
		return nil, netErr
	}

	var status int
	if idx < len(c.statuses) {
		status = c.statuses[idx]
	} else {
		status = c.statuses[len(c.statuses)-1]
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	if hdrs, ok := c.headers[idx]; ok {
		for k, v := range hdrs {
			header.Set(k, v)
		}
	}

	var body io.ReadCloser
	if status == http.StatusOK {
		body = io.NopCloser(bytes.NewReader(c.successBody))
	} else {
		body = io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"status":%d,"message":"upstream error"}`, status))))
	}

	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       body,
		Request:    req,
	}, nil
}

func (c *chaosDoer) Attempts() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attempts
}

func (c *chaosDoer) RequestTimes() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make([]time.Time, len(c.requestTimes))
	copy(copied, c.requestTimes)

	return copied
}

func TestAdversarial_UpstreamChaos_RetryAndFallback(t *testing.T) {
	t.Parallel()

	validResp := bptf.V4PricesResponseExt{
		Success: bptf.SuccessBoolProp(1),
		Items: map[string]bptf.V4PricesBaseItemDocExt{
			"Mann Co. Supply Crate Key": {
				Defindex: []string{"5021"},
				Prices: map[string]map[string]map[string]map[string]bptf.V4PricesEntry{
					"6": {"Tradable": {"Craftable": {"0": {Value: 75.0}}}},
				},
			},
		},
	}
	validBytes, err := json.Marshal(validResp)
	require.NoError(t, err)

	t.Run("dropped_connection_unexpected_eof_then_success", func(t *testing.T) {
		t.Parallel()

		// Attempt 0: connection dropped (io.ErrUnexpectedEOF)
		// Attempt 1: 200 OK
		doer := &chaosDoer{
			statuses:    []int{http.StatusOK, http.StatusOK},
			netErrors:   map[int]error{0: io.ErrUnexpectedEOF},
			successBody: validBytes,
		}

		client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
		mgr := NewPriceManager(client, log.Discard, Config{})

		err := mgr.Update(t.Context())
		// If isRetriableBptfError handles dropped responses, Update succeeds on attempt 2.
		// If it fails to recognize io.ErrUnexpectedEOF as retriable, Update aborts on attempt 1.
		if err == nil {
			assert.Equal(t, 2, doer.Attempts(), "Must retry after dropped response and succeed on attempt 2")

			p, ok := mgr.GetPriceStale("5021;6")
			assert.True(t, ok)
			assert.Equal(t, 75.0, p.Value)
		} else {
			t.Logf(
				"OBSERVED DEFECT: dropped connection (io.ErrUnexpectedEOF) was NOT retried: %v (attempts: %d)",
				err,
				doer.Attempts(),
			)
			assert.Fail(t, "PriceManager failed to retry on transient dropped connection (io.ErrUnexpectedEOF)")
		}
	})

	t.Run("dropped_connection_network_reset_then_success", func(t *testing.T) {
		t.Parallel()

		netErr := &net.OpError{
			Op:  "read",
			Net: "tcp",
			Err: errors.New("connection reset by peer"),
		}

		doer := &chaosDoer{
			statuses:    []int{http.StatusOK, http.StatusOK},
			netErrors:   map[int]error{0: netErr},
			successBody: validBytes,
		}

		client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
		mgr := NewPriceManager(client, log.Discard, Config{})

		err := mgr.Update(t.Context())
		if err == nil {
			assert.Equal(t, 2, doer.Attempts(), "Must retry on connection reset")
		} else {
			t.Logf(
				"OBSERVED DEFECT: connection reset was NOT retried: %v (attempts: %d)",
				err,
				doer.Attempts(),
			)
			assert.Fail(t, "PriceManager failed to retry on transient connection reset")
		}
	})

	t.Run("http_429_retry_after_delay_duration_respected", func(t *testing.T) {
		t.Parallel()

		// Attempt 0: 429 with Retry-After: 1 (1 second)
		// Attempt 1: 200 OK
		doer := &chaosDoer{
			statuses: []int{http.StatusTooManyRequests, http.StatusOK},
			headers: map[int]map[string]string{
				0: {"Retry-After": "1"},
			},
			successBody: validBytes,
		}

		client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
		mgr := NewPriceManager(client, log.Discard, Config{})

		start := time.Now()
		err := mgr.Update(t.Context())
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, 2, doer.Attempts())

		// If calculateBptfRetryDelay respects Retry-After: 1, elapsed should be >= 1s (or close to 1s).
		// If calculateBptfRetryDelay discards Retry-After and uses base=500ms jitter, elapsed will be < 600ms.
		t.Logf("Elapsed time for 429 Retry-After: 1 was %v", elapsed)
		assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "Must enforce Retry-After: 1s delay")
	})

	t.Run("chaos_mixed_sequence_502_503_504_then_success", func(t *testing.T) {
		t.Parallel()

		doer := &chaosDoer{
			statuses: []int{
				http.StatusBadGateway,         // attempt 1
				http.StatusServiceUnavailable, // attempt 2
				http.StatusGatewayTimeout,     // attempt 3
				http.StatusOK,                 // attempt 4 (succeeds)
			},
			successBody: validBytes,
		}

		client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
		mgr := NewPriceManager(client, log.Discard, Config{})

		err := mgr.Update(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 4, doer.Attempts())

		p, ok := mgr.GetPriceStale("5021;6")
		assert.True(t, ok)
		assert.Equal(t, 75.0, p.Value)
	})

	t.Run("context_cancellation_aborts_retry_loop", func(t *testing.T) {
		t.Parallel()

		doer := &chaosDoer{
			statuses: []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable},
		}

		client := aoni.NewClient(doer, option.WithBaseURL("https://backpack.tf/api"))
		mgr := NewPriceManager(client, log.Discard, Config{})

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		err := mgr.Update(ctx)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("nil_client_fails_cleanly", func(t *testing.T) {
		t.Parallel()

		mgr := NewPriceManager(nil, log.Discard, Config{})
		err := mgr.Update(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "aoni client is nil")
	})
}

// -----------------------------------------------------------------------------
// 4. TTL & Stale Fallback Boundary Conditions
// -----------------------------------------------------------------------------

func TestAdversarial_TTL_Expiry_BoundaryConditions(t *testing.T) {
	t.Parallel()

	t.Run("rapid_ttl_expiry_race_transition", func(t *testing.T) {
		t.Parallel()

		mgr := NewPriceManager(nil, log.Discard, Config{
			TTL: 10 * time.Millisecond,
		})

		// Prepopulate
		mgr.mu.Lock()
		mgr.index["5021;6"] = bptf.V4PricesEntry{Value: 75.0}
		mgr.timestamp = time.Now()
		mgr.version = 1
		mgr.mu.Unlock()

		// Concurrently poll GetPrice and GetPriceStale across the 10ms expiry transition
		var (
			wg                     sync.WaitGroup
			getPriceSuccesses      atomic.Int64
			getPriceStaleSuccesses atomic.Int64
		)

		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Millisecond)
		defer cancel()

		for i := 0; i < 8; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				for {
					select {
					case <-ctx.Done():
						return
					default:
						if _, ok := mgr.GetPrice("5021;6"); ok {
							getPriceSuccesses.Add(1)
						}

						if _, ok := mgr.GetPriceStale("5021;6"); ok {
							getPriceStaleSuccesses.Add(1)
						}

						time.Sleep(500 * time.Microsecond)
					}
				}
			}()
		}

		wg.Wait()

		// After 60ms, TTL (10ms) has long expired
		assert.True(t, mgr.IsExpired())

		// Final check: GetPrice must return false, GetPriceStale must return true
		_, okActive := mgr.GetPrice("5021;6")
		assert.False(t, okActive, "GetPrice must return false after TTL expiry")

		pStale, okStale := mgr.GetPriceStale("5021;6")
		assert.True(t, okStale, "GetPriceStale must return true even after TTL expiry")
		assert.Equal(t, 75.0, pStale.Value)

		assert.Greater(t, getPriceStaleSuccesses.Load(), int64(0))
	})

	t.Run("zero_timestamp_reports_expired", func(t *testing.T) {
		t.Parallel()

		mgr := NewPriceManager(nil, log.Discard, Config{TTL: time.Hour})
		assert.True(t, mgr.IsExpired(), "Uninitialized cache with zero timestamp must report expired")
		assert.True(t, mgr.Metadata().IsExpired)
	})

	t.Run("invalidate_nonexistent_returns_false_and_preserves_version", func(t *testing.T) {
		t.Parallel()

		mgr := NewPriceManager(nil, log.Discard, Config{TTL: time.Hour})
		mgr.mu.Lock()
		mgr.index["5021;6"] = bptf.V4PricesEntry{Value: 75.0}
		mgr.version = 5
		mgr.mu.Unlock()

		evicted := mgr.Invalidate("nonexistent_sku")
		assert.False(t, evicted)
		assert.Equal(t, int64(5), mgr.Version(), "Version must not increment when Invalidate is a no-op")
	})
}
