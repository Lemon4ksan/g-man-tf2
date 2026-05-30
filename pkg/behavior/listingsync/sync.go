// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package listingsync provides automated reactive classified listings synchronization with backpack.tf and crit.tf based on inventory and price updates.
package listingsync

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"golang.org/x/time/rate"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// BehaviorName is the unique name of this behavior.
const BehaviorName = "listings_synchronizer"

// AuditRequestedEvent is sent to trigger an immediate, forced reconciliation of the specified SKUs.
type AuditRequestedEvent struct {
	bus.BaseEvent
	SKUs []string
}

// Config defines the configuration for ListingsSynchronizer.
type Config struct {
	BptfRateLimit time.Duration `json:"bptf_rate_limit"` // e.g. 2s
	CritRateLimit time.Duration `json:"crit_rate_limit"` // e.g. 1s
	BatchDelay    time.Duration `json:"batch_delay"`     // e.g. 500ms
}

// DefaultConfig returns default limits.
func DefaultConfig() Config {
	return Config{
		BptfRateLimit: 2 * time.Second,
		CritRateLimit: 1 * time.Second,
		BatchDelay:    500 * time.Millisecond,
	}
}

// BackpackProvider defines the subset of Backpack methods needed by ListingsSynchronizer.
type BackpackProvider interface {
	GetStock(sku string) int
	GetItemsBySKU(targetSKU string) []uint64
}

// ListingProvider defines the subset of ListingManager methods needed by ListingsSynchronizer.
type ListingProvider interface {
	FindListingBySKU(sku, intent string) *bptf.ListingResponse
	Upsert(ctx context.Context, listing bptf.ListingResolvable) (*bptf.ListingResponse, error)
	Delete(ctx context.Context, id string) error
	Client() *bptf.Client
}

// PriceProvider defines pricedb methods.
type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
}

// ConfigProvider defines trading config manager.
type ConfigProvider interface {
	GetConfig() trading.Config
}

// CritProvider defines crit.tf client.
type CritProvider interface {
	FetchMyListings(ctx context.Context) ([]crit.Listing, error)
	CreateListing(ctx context.Context, assetID string, currencies pricedb.Currencies) (*crit.Listing, error)
	UpdateListing(ctx context.Context, listingID string, currencies pricedb.Currencies) (*crit.Listing, error)
	DeleteListing(ctx context.Context, listingID string) error
}

// ListingsSynchronizer coordinates in-game stock, pricedb changes, and external marketplace listings.
type ListingsSynchronizer struct {
	config Config
	logger log.Logger
	bus    *bus.Bus

	bp       BackpackProvider
	listings ListingProvider
	priceMgr PriceProvider
	cfgMgr   ConfigProvider
	crit     CritProvider

	mu          sync.Mutex
	bptfQueue   chan string
	bptfPending map[string]bool
	critQueue   chan string
	critPending map[string]bool
}

// Sync returns a behavior.Option to install ListingsSynchronizer.
func Sync(
	bp BackpackProvider,
	listings ListingProvider,
	priceMgr PriceProvider,
	cfgMgr ConfigProvider,
	crit CritProvider,
	cfg Config,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(bp, listings, priceMgr, cfgMgr, crit, o.Bus(), o.Logger(), cfg))
	}
}

// New constructs a new ListingsSynchronizer.
func New(
	bp BackpackProvider,
	listings ListingProvider,
	priceMgr PriceProvider,
	cfgMgr ConfigProvider,
	crit CritProvider,
	b *bus.Bus,
	logger log.Logger,
	cfg Config,
) *ListingsSynchronizer {
	if cfg.BptfRateLimit == 0 {
		cfg.BptfRateLimit = DefaultConfig().BptfRateLimit
	}

	if cfg.CritRateLimit == 0 {
		cfg.CritRateLimit = DefaultConfig().CritRateLimit
	}

	if cfg.BatchDelay == 0 {
		cfg.BatchDelay = DefaultConfig().BatchDelay
	}

	return &ListingsSynchronizer{
		config:      cfg,
		logger:      logger.With(log.Module(BehaviorName)),
		bus:         b,
		bp:          bp,
		listings:    listings,
		priceMgr:    priceMgr,
		cfgMgr:      cfgMgr,
		crit:        crit,
		bptfQueue:   make(chan string, 500),
		bptfPending: make(map[string]bool),
		critQueue:   make(chan string, 500),
		critPending: make(map[string]bool),
	}
}

// Name returns the unique behavior name.
func (s *ListingsSynchronizer) Name() string {
	return BehaviorName
}

// Run starts the worker threads and event subscriptions.
func (s *ListingsSynchronizer) Run(ctx context.Context) error {
	s.logger.Info("ListingsSynchronizer started")

	sub := s.bus.Subscribe(
		&pricedb.PricelistUpdatedEvent{},
		&tf2.BackpackLoadedEvent{},
		&tf2.ItemAcquiredEvent{},
		&tf2.ItemRemovedEvent{},
		&tf2.ItemUpdatedEvent{},
		&AuditRequestedEvent{},
	)
	defer sub.Unsubscribe()

	// Launch backpack.tf worker
	go s.bptfWorker(ctx)

	// Launch crit.tf worker
	if s.crit != nil {
		go s.critWorker(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case ev := <-sub.C():
			switch e := ev.(type) {
			case *pricedb.PricelistUpdatedEvent:
				s.enqueue(e.SKU)

			case *tf2.BackpackLoadedEvent:
				cfg := s.cfgMgr.GetConfig()
				for skuStr := range cfg.Items {
					s.enqueue(skuStr)
				}

				s.enqueue(currency.SKUKey)

			case *tf2.ItemAcquiredEvent:
				if e.Item != nil {
					skuStr := fmt.Sprintf("%d;%d", e.Item.DefIndex, e.Item.Quality)
					s.enqueue(skuStr)
				}

			case *tf2.ItemRemovedEvent:
				cfg := s.cfgMgr.GetConfig()
				for skuStr := range cfg.Items {
					s.enqueue(skuStr)
				}

				s.enqueue(currency.SKUKey)

			case *tf2.ItemUpdatedEvent:
				if e.Item != nil {
					skuStr := fmt.Sprintf("%d;%d", e.Item.DefIndex, e.Item.Quality)
					s.enqueue(skuStr)
				}

			case *AuditRequestedEvent:
				s.logger.Info("Forced audit requested via event", log.Int("skus_count", len(e.SKUs)))

				for _, skuStr := range e.SKUs {
					s.enqueue(skuStr)
				}
			}
		}
	}
}

func (s *ListingsSynchronizer) enqueue(sku string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.bptfPending[sku] {
		s.bptfPending[sku] = true
		s.bptfQueue <- sku
	}

	if s.crit != nil && !s.critPending[sku] {
		s.critPending[sku] = true
		s.critQueue <- sku
	}
}

// bptfWorker gathers items and synchronizes listings with backpack.tf
func (s *ListingsSynchronizer) bptfWorker(ctx context.Context) {
	limiter := rate.NewLimiter(rate.Every(s.config.BptfRateLimit), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case sku := <-s.bptfQueue:
			// Clear pending flag
			s.mu.Lock()
			delete(s.bptfPending, sku)
			s.mu.Unlock()

			// Gather sequential tasks in window to enable batching
			skus := []string{sku}

			time.Sleep(s.config.BatchDelay)

		drainLoop:
			for len(skus) < 100 {
				select {
				case nextSKU := <-s.bptfQueue:
					s.mu.Lock()
					delete(s.bptfPending, nextSKU)
					s.mu.Unlock()

					skus = append(skus, nextSKU)

				default:
					break drainLoop
				}
			}

			// Synchronize the batch
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			s.logger.Debug("Syncing batch to backpack.tf", log.Strings("skus", skus))
			s.syncBptfBatch(ctx, skus)
		}
	}
}

// critWorker gathers items and synchronizes listings with crit.tf
func (s *ListingsSynchronizer) critWorker(ctx context.Context) {
	limiter := rate.NewLimiter(rate.Every(s.config.CritRateLimit), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case sku := <-s.critQueue:
			s.mu.Lock()
			delete(s.critPending, sku)
			s.mu.Unlock()

			// Gather sequential tasks
			skus := []string{sku}

			time.Sleep(s.config.BatchDelay)

		drainLoop:
			for len(skus) < 100 {
				select {
				case nextSKU := <-s.critQueue:
					s.mu.Lock()
					delete(s.critPending, nextSKU)
					s.mu.Unlock()

					skus = append(skus, nextSKU)

				default:
					break drainLoop
				}
			}

			if err := limiter.Wait(ctx); err != nil {
				return
			}

			s.logger.Debug("Syncing batch to crit.tf", log.Strings("skus", skus))
			s.syncCritBatch(ctx, skus)
		}
	}
}

func (s *ListingsSynchronizer) syncCritBatch(ctx context.Context, skus []string) {
	cfg := s.cfgMgr.GetConfig()

	// Fetch current active listings from crit.tf
	activeListings, err := s.crit.FetchMyListings(ctx)
	if err != nil {
		s.logger.Error("Failed to fetch active listings from crit.tf", log.Err(err))
		return
	}

	// Index active listings by SKU and AssetID
	listingsBySKU := make(map[string][]crit.Listing)

	listingsByAssetID := make(map[string]crit.Listing)
	for _, l := range activeListings {
		listingsBySKU[l.SKU] = append(listingsBySKU[l.SKU], l)
		listingsByAssetID[l.AssetID] = l
	}

	for _, skuStr := range skus {
		price, ok := s.priceMgr.GetPrice(skuStr)
		if !ok || price.Sell.Metal <= 0 {
			continue
		}

		physicalIDs := s.bp.GetItemsBySKU(skuStr)

		minStock := 0

		enableSell := true
		if itemCfg, exists := cfg.Items[skuStr]; exists {
			minStock = itemCfg.MinStock
			enableSell = itemCfg.EnableSell
		}

		var targetIDs []uint64
		if enableSell && len(physicalIDs) > minStock {
			targetIDs = physicalIDs[minStock:]
		}

		targetAssetIDs := make(map[string]bool)
		for _, id := range targetIDs {
			idStr := strconv.FormatUint(id, 10)
			targetAssetIDs[idStr] = true

			// If no listing exists on crit.tf for this physical asset ID, create it!
			if _, exists := listingsByAssetID[idStr]; !exists {
				s.logger.Debug("Creating listing on crit.tf", log.String("sku", skuStr), log.String("asset_id", idStr))

				if _, err := s.crit.CreateListing(ctx, idStr, price.Sell); err != nil {
					s.logger.Error("Failed to create crit.tf listing", log.String("asset_id", idStr), log.Err(err))
				}
			} else {
				// Listing exists, check if price needs update
				existing := listingsByAssetID[idStr]
				if existing.PriceKeys != price.Sell.Keys || float64(existing.PriceMetal) != price.Sell.Metal {
					s.logger.Debug(
						"Updating listing price on crit.tf",
						log.String("sku", skuStr),
						log.String("asset_id", idStr),
					)

					listingID := fmt.Sprintf("%d", existing.ID)
					if _, err := s.crit.UpdateListing(ctx, listingID, price.Sell); err != nil {
						s.logger.Error(
							"Failed to update crit.tf listing",
							log.String("listing_id", listingID),
							log.Err(err),
						)
					}
				}
			}
		}

		// Delete any active listings on crit.tf for this SKU that are no longer targeted
		for _, l := range listingsBySKU[skuStr] {
			if !targetAssetIDs[l.AssetID] {
				listingID := fmt.Sprintf("%d", l.ID)
				s.logger.Debug(
					"Deleting stale listing from crit.tf",
					log.String("sku", skuStr),
					log.String("listing_id", listingID),
				)

				if err := s.crit.DeleteListing(ctx, listingID); err != nil {
					s.logger.Error(
						"Failed to delete crit.tf listing",
						log.String("listing_id", listingID),
						log.Err(err),
					)
				}
			}
		}
	}
}

func (s *ListingsSynchronizer) syncBptfBatch(ctx context.Context, skus []string) {
	cfg := s.cfgMgr.GetConfig()

	var (
		resolvables []bptf.ListingResolvable
		deleteIDs   []string
	)

	for _, skuStr := range skus {
		price, ok := s.priceMgr.GetPrice(skuStr)
		if !ok || price.Buy.Metal <= 0 || price.Sell.Metal <= 0 {
			continue
		}

		stock := s.bp.GetStock(skuStr)

		// Determine limits
		maxStock := cfg.DefaultMaxStock
		minStock := 0
		enableBuy := true
		enableSell := true

		if itemCfg, exists := cfg.Items[skuStr]; exists {
			maxStock = itemCfg.MaxStock
			minStock = itemCfg.MinStock
			enableBuy = itemCfg.EnableBuy
			enableSell = itemCfg.EnableSell
		}

		// Ensure Buy listing if stock allows
		existingBuy := s.listings.FindListingBySKU(skuStr, "buy")
		if enableBuy && stock < maxStock {
			resolvables = append(resolvables, bptf.ListingResolvable{
				Item:   skuStr,
				Intent: "buy",
				Currencies: map[string]float64{
					"metal": price.Buy.Metal,
				},
				Details: fmt.Sprintf("⚡ G-man | Buying %s | Stock: %d/%d", skuStr, stock, maxStock),
			})
		} else if existingBuy != nil {
			deleteIDs = append(deleteIDs, existingBuy.ID)
		}

		// Ensure Sell listing if stock allows
		existingSell := s.listings.FindListingBySKU(skuStr, "sell")
		if enableSell && stock > minStock {
			resolvables = append(resolvables, bptf.ListingResolvable{
				Item:   skuStr,
				Intent: "sell",
				Currencies: map[string]float64{
					"metal": price.Sell.Metal,
				},
				Details: fmt.Sprintf("⚡ G-man | Selling %s | Stock: %d/%d", skuStr, stock, maxStock),
			})
		} else if existingSell != nil {
			deleteIDs = append(deleteIDs, existingSell.ID)
		}
	}

	// Delete obsolete listings
	if len(deleteIDs) > 0 {
		for _, id := range deleteIDs {
			if err := s.listings.Delete(ctx, id); err != nil {
				s.logger.Error("Failed to delete bptf listing", log.String("id", id), log.Err(err))
			}
		}
	}

	// Create/Update active listings
	if len(resolvables) > 0 {
		// Use batch API if multiple listings exist
		if len(resolvables) > 1 && s.listings.Client() != nil {
			if _, err := s.listings.Client().BatchCreateListings(ctx, resolvables); err != nil {
				s.logger.Error("Failed batch create listings", log.Err(err))
			}
		} else {
			for _, r := range resolvables {
				if _, err := s.listings.Upsert(ctx, r); err != nil {
					s.logger.Error(
						"Failed single listing upsert",
						log.String("sku", fmt.Sprintf("%v", r.Item)),
						log.Err(err),
					)
				}
			}
		}
	}
}
