// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricemanager provides a backpack.tf price manager for the g-man-tf2 bot.
package pricemanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/lemon4ksan/g-man/pkg/behavior"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// BehaviorName is the unique orchestrator identifier for the backpack.tf price manager behavior.
const BehaviorName = "bptf_prices"

// ErrCachePathNotConfigured is returned by [PriceManager.Load] when attempting to load
// cached prices from disk without configuring a non-empty CachePath.
var ErrCachePathNotConfigured = errors.New("pricemanager: cache path not configured")

// WithPriceManager registers a newly constructed [PriceManager] with the provided
// behavior orchestrator, HTTP client, and configuration options.
func WithPriceManager(orch *behavior.Orchestrator, r *aoni.Client, cfg Config) {
	orch.Register(NewPriceManager(r, orch.Logger(), cfg))
}

// Config specifies runtime parameters for [PriceManager].
type Config struct {
	// CachePath specifies the local filesystem path where the serialized price cache
	// JSON envelope is persisted. If empty, disk caching is disabled.
	CachePath string

	// SyncInterval defines the duration between periodic full price fetches from backpack.tf.
	// Defaults to 2 hours if unspecified.
	SyncInterval time.Duration

	// TTL defines the maximum time-to-live for cached prices before they are considered
	// expired. If zero, TTL defaults to SyncInterval + 1 hour. If negative, TTL expiry
	// checks are disabled.
	TTL time.Duration
}

// DefaultConfig returns the standard production configuration for [PriceManager]
// with a 2-hour synchronization interval, 3-hour TTL, and default file cache path.
func DefaultConfig() Config {
	return Config{
		CachePath:    "cache/tf2/prices.json",
		SyncInterval: 2 * time.Hour,
		TTL:          3 * time.Hour,
	}
}

// CacheMetadata encapsulates point-in-time diagnostic and operational metadata
// for the price cache.
type CacheMetadata struct {
	// Timestamp records the wall-clock time when prices were last successfully fetched or loaded.
	Timestamp time.Time `json:"timestamp"`

	// TTL specifies the configured time-to-live validity window for prices.
	TTL time.Duration `json:"ttl"`

	// Version is a monotonically increasing counter incremented on every cache rebuild or invalidation.
	Version int64 `json:"version"`

	// ItemCount represents the total number of distinct canonical item SKUs currently indexed.
	ItemCount int `json:"item_count"`

	// IsExpired indicates whether the cache age has exceeded its configured TTL.
	IsExpired bool `json:"is_expired"`
}

// PriceManager coordinates full pricelist retrieval from the backpack.tf API, maintains an
// indexed in-memory lookup cache, enforces TTL expiration, manages programmatic cache invalidation,
// applies jittered exponential backoff retry on transient upstream errors, and transparently falls
// back to cached snapshots on disk or in memory when remote services are degraded.
//
// Thread Safety:
//   - Fully thread-safe for concurrent readers and writers.
//   - Read operations (such as [PriceManager.GetPrice], [PriceManager.GetPriceStale],
//     and [PriceManager.Metadata]) acquire a shared read lock (RLock).
//   - Write operations (such as [PriceManager.Update], [PriceManager.Invalidate],
//     and [PriceManager.InvalidateAll]) acquire an exclusive write lock (Lock).
type PriceManager struct {
	config Config
	r      *aoni.Client
	logger log.Logger

	mu        sync.RWMutex
	index     map[string]bptf.V4PricesEntry
	timestamp time.Time
	ttl       time.Duration
	version   int64
}

// NewPriceManager creates a new [PriceManager] instance configured with an HTTP client,
// logger, and cache settings. If cfg.TTL is 0, it defaults to cfg.SyncInterval + 1 hour (or 3 hours).
func NewPriceManager(r *aoni.Client, l log.Logger, cfg Config) *PriceManager {
	ttl := cfg.TTL
	if ttl == 0 {
		if cfg.SyncInterval > 0 {
			ttl = cfg.SyncInterval + time.Hour
		} else {
			ttl = 3 * time.Hour
		}
	} else if ttl < 0 {
		ttl = 0 // negative value explicitly disables TTL expiry checks
	}

	return &PriceManager{
		config: cfg,
		r:      r,
		logger: l.With(log.Module(BehaviorName)),
		index:  make(map[string]bptf.V4PricesEntry),
		ttl:    ttl,
	}
}

// Name returns the behavior identifier string "[BehaviorName]" for registration with the orchestrator.
func (m *PriceManager) Name() string { return BehaviorName }

// Run implements [behavior.Behavior]. It performs an initial cache warm-up from disk,
// executes an initial price update, and periodically polls backpack.tf at the configured
// SyncInterval until ctx is cancelled.
func (m *PriceManager) Run(ctx context.Context) error {
	m.logger.Info("BPTF Price Sync behavior started", log.Duration("interval", m.config.SyncInterval))

	// Initial warm-up: load from disk cache immediately if index is empty
	if m.isIndexEmpty() {
		if err := m.loadFromCache(); err != nil {
			m.logger.Debug("No existing disk cache found during startup warm-up", log.Err(err))
		} else {
			m.logger.Info("Warmed up price index from disk cache", log.Int("skus", len(m.index)))
		}
	}

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	if err := m.Update(ctx); err != nil {
		m.logger.Error("Initial price update failed", log.Err(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.Update(ctx); err != nil {
				m.logger.Error("Price update failed", log.Err(err))
			}
		}
	}
}

func (m *PriceManager) isIndexEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.index) == 0
}

// Update fetches the latest prices with exponential backoff retry and falls back
// transparently to disk or in-memory cache if backpack.tf is degraded or unreachable.
func (m *PriceManager) Update(ctx context.Context) error {
	m.logger.Debug("Fetching full pricelist from backpack.tf...")

	resp, err := m.fetchPricesWithRetry(ctx)
	if err != nil {
		m.logger.Warn("Remote BPTF pricelist update failed after retries", log.Err(err))

		// Transparent fallback: if index is empty, attempt disk cache load
		if m.isIndexEmpty() {
			if loadErr := m.loadFromCache(); loadErr == nil && !m.isIndexEmpty() {
				m.logger.Info("Transparently fell back to disk cache after remote BPTF degradation",
					log.Int("unique_skus", len(m.index)),
				)

				return nil
			}
		} else {
			// In-memory cache is already populated; retain existing prices
			m.logger.Info("Transparently retaining existing in-memory price index after remote BPTF degradation",
				log.Int("unique_skus", len(m.index)),
			)

			return nil
		}

		return fmt.Errorf("bptf update failed and no cache fallback available: %w", err)
	}

	newIndex := make(map[string]bptf.V4PricesEntry, len(resp.Items)*2)

	for _, itemData := range resp.Items {
		if len(itemData.Defindex) == 0 {
			continue
		}

		defindex, _ := strconv.Atoi(itemData.Defindex[0])
		m.indexItemPrices(defindex, itemData.Prices, newIndex)
	}

	m.mu.Lock()
	m.index = newIndex
	m.timestamp = time.Now()
	m.version++
	m.mu.Unlock()

	if err := m.saveToCache(); err != nil {
		m.logger.Warn("Failed to save prices to cache", log.Err(err))
	}

	m.logger.Info("Bptf index rebuilt", log.Int("unique_skus", len(newIndex)), log.Int64("version", m.version))

	return nil
}

func (m *PriceManager) fetchPricesWithRetry(ctx context.Context) (*bptf.V4PricesResponseExt, error) {
	if m.r == nil {
		return nil, errors.New("pricemanager: aoni client is nil")
	}

	const maxAttempts = 4

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := m.r.GetTo[bptf.V4PricesResponseExt](ctx, "IGetPrices/v4", mod.WithQuery("raw=1"))
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !isRetriableBptfError(err) || attempt == maxAttempts {
			break
		}

		delay := calculateBptfRetryDelay(attempt, err)
		m.logger.Warn("Transient error fetching BPTF prices, retrying with backoff",
			log.Int("attempt", attempt),
			log.Duration("delay", delay),
			log.Err(err),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}

func isRetriableBptfError(err error) bool {
	if err == nil {
		return false
	}

	if aoni.IsRateLimited(err) || aoni.IsTimeout(err) {
		return true
	}

	var apiErr *aoni.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			http.StatusTooManyRequests:
			return true
		}
	}

	return aoni.IsServerError(err)
}

func calculateBptfRetryDelay(attempt int, _ error) time.Duration {
	base := 500 * time.Millisecond * time.Duration(1<<(attempt-1))
	if base > 10*time.Second {
		base = 10 * time.Second
	}

	// Full Jitter
	jitter := time.Duration(rand.Int64N(int64(base / 2)))

	return base/2 + jitter
}

func (m *PriceManager) indexItemPrices(
	defindex int,
	prices map[string]map[string]map[string]map[string]bptf.V4PricesEntry,
	outIndex map[string]bptf.V4PricesEntry,
) {
	normDef := schema.NormalizeDefindex(defindex)

	for quality, tradableMap := range prices {
		qInt, _ := strconv.Atoi(quality)
		indexQualityPrices(normDef, qInt, tradableMap, outIndex)
	}
}

func indexQualityPrices(
	defindex, qInt int,
	tradableMap map[string]map[string]map[string]bptf.V4PricesEntry,
	outIndex map[string]bptf.V4PricesEntry,
) {
	for tradable, craftableMap := range tradableMap {
		isTradable := tradable == "Tradable"

		for craftable, priceIndexMap := range craftableMap {
			isCraftable := craftable == "Craftable"

			for pIndex, entry := range priceIndexMap {
				sItem := &sku.Item{
					Defindex:  defindex,
					Quality:   qInt,
					Tradable:  isTradable,
					Craftable: isCraftable,
				}

				if pInt, err := strconv.Atoi(pIndex); err == nil && pInt != 0 {
					if qInt == schema.QualityUnusual {
						sItem.Effect = pInt
					} else {
						sItem.Crateseries = pInt
					}
				}

				outIndex[sku.FromObject(sItem)] = entry
			}
		}
	}
}

// GetPrice retrieves the cached price entry for the specified canonical SKU string.
// It verifies that the cache has not expired past its configured TTL.
// Returns the price entry and true if found and unexpired, or an empty entry and false
// if the SKU is missing or the cache has expired.
func (m *PriceManager) GetPrice(sku string) (bptf.V4PricesEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.isExpiredLocked() {
		return bptf.V4PricesEntry{}, false
	}

	entry, ok := m.index[sku]

	return entry, ok
}

// GetPriceStale retrieves the cached price entry for the specified canonical SKU string
// without checking TTL expiration. This method is intended as a resilience fallback when
// external price APIs are temporarily degraded or unreachable.
func (m *PriceManager) GetPriceStale(sku string) (bptf.V4PricesEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.index[sku]

	return entry, ok
}

// Invalidate programmatically removes a single SKU from the in-memory cache and increments
// the cache version. It returns true if the SKU was present and evicted, or false if it was not found.
func (m *PriceManager) Invalidate(sku string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.index[sku]; !exists {
		return false
	}

	delete(m.index, sku)
	m.version++

	return true
}

// InvalidateAll clears the entire in-memory SKU index, resets the cache timestamp,
// and increments the cache version.
func (m *PriceManager) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.index = make(map[string]bptf.V4PricesEntry)
	m.timestamp = time.Time{}
	m.version++
}

// Metadata returns a point-in-time [CacheMetadata] snapshot describing the current
// cache timestamp, TTL, version, item count, and expiration state.
func (m *PriceManager) Metadata() CacheMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return CacheMetadata{
		Timestamp: m.timestamp,
		TTL:       m.ttl,
		Version:   m.version,
		ItemCount: len(m.index),
		IsExpired: m.isExpiredLocked(),
	}
}

// Version returns the current monotonically increasing version counter of the cache.
func (m *PriceManager) Version() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.version
}

// Timestamp returns the time when the cache was last populated via [PriceManager.Update] or [PriceManager.Load].
func (m *PriceManager) Timestamp() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.timestamp
}

// TTL returns the configured time-to-live duration for cache entries.
func (m *PriceManager) TTL() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.ttl
}

// IsExpired reports whether the cache has exceeded its configured TTL window.
func (m *PriceManager) IsExpired() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.isExpiredLocked()
}

func (m *PriceManager) isExpiredLocked() bool {
	if m.ttl <= 0 {
		return false
	}

	if m.timestamp.IsZero() {
		return true
	}

	return time.Since(m.timestamp) > m.ttl
}

// Load reads and parses the price cache from disk at the configured CachePath.
// It supports both versioned JSON envelopes containing metadata and legacy raw index maps.
// Returns [ErrCachePathNotConfigured] if CachePath is empty.
func (m *PriceManager) Load() error {
	return m.loadFromCache()
}

type cacheFileEnvelope struct {
	Version   int64                         `json:"version"`
	Timestamp time.Time                     `json:"timestamp"`
	TTL       time.Duration                 `json:"ttl"`
	Index     map[string]bptf.V4PricesEntry `json:"index"`
}

func (m *PriceManager) saveToCache() error {
	if m.config.CachePath == "" {
		return nil
	}

	m.mu.RLock()
	env := cacheFileEnvelope{
		Version:   m.version,
		Timestamp: m.timestamp,
		TTL:       m.ttl,
		Index:     m.index,
	}
	m.mu.RUnlock()

	data, err := json.Marshal(env)
	if err != nil {
		return err
	}

	dir := filepath.Dir(m.config.CachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(m.config.CachePath, data, 0o644)
}

func (m *PriceManager) loadFromCache() error {
	if m.config.CachePath == "" {
		return ErrCachePathNotConfigured
	}

	data, err := os.ReadFile(m.config.CachePath)
	if err != nil {
		return err
	}

	var env cacheFileEnvelope
	if err := json.Unmarshal(data, &env); err == nil && env.Index != nil {
		m.mu.Lock()
		m.index = env.Index
		m.timestamp = env.Timestamp

		if env.TTL > 0 {
			m.ttl = env.TTL
		}

		m.version = env.Version
		if m.version == 0 {
			m.version = 1
		}

		m.mu.Unlock()

		return nil
	}

	// Fallback for legacy raw map format
	var index map[string]bptf.V4PricesEntry
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	m.mu.Lock()
	m.index = index
	m.timestamp = time.Now()
	m.version = 1
	m.mu.Unlock()

	return nil
}
