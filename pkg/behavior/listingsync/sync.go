// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package listingsync provides automated reactive classified listings synchronization with backpack.tf and crit.tf based on inventory and price updates.
package listingsync

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"golang.org/x/time/rate"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/ecp"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// BehaviorName is the unique name of the listings synchronizer behavior.
const BehaviorName = "listings_synchronizer"

// AuditRequestedEvent is sent to trigger an immediate, forced reconciliation of the specified SKUs.
type AuditRequestedEvent struct {
	bus.BaseEvent
	// SKUs represents the list of item SKUs that require immediate reconciliation.
	SKUs []string
}

// Config defines the rate limits and window delays for marketplace updates.
type Config struct {
	// BptfRateLimit defines the minimum time delay between backpack.tf requests.
	BptfRateLimit time.Duration `json:"bptf_rate_limit"`
	// CritRateLimit defines the minimum time delay between crit.tf requests.
	CritRateLimit time.Duration `json:"crit_rate_limit"`
	// BatchDelay defines the collection window time used to accumulate sequential updates.
	BatchDelay time.Duration `json:"batch_delay"`
}

// DefaultConfig returns a [Config] containing production-ready default delays.
func DefaultConfig() Config {
	return Config{
		BptfRateLimit: 2 * time.Second,
		CritRateLimit: 1 * time.Second,
		BatchDelay:    500 * time.Millisecond,
	}
}

// BackpackProvider defines the subset of inventory methods required to track physical holdings.
type BackpackProvider interface {
	// GetStock returns the current stock count of the specified SKU.
	GetStock(sku string) int
	// GetItemsBySKU returns all local item IDs matching the specified SKU.
	GetItemsBySKU(targetSKU string) []uint64
}

// ListingProvider defines the subset of backpack.tf listing manager methods.
type ListingProvider interface {
	// FindListingBySKU searches for an active listing matching the SKU and intent.
	FindListingBySKU(sku, intent string) *bptf.ListingResponse
	// Upsert creates or updates a listing on backpack.tf.
	Upsert(ctx context.Context, listing bptf.ListingResolvable) (*bptf.ListingResponse, error)
	// Delete removes an active listing from backpack.tf.
	Delete(ctx context.Context, id string) error
	// Client returns the underlying backpack.tf client instance.
	Client() *bptf.Client
}

// PriceProvider defines the subset of pricedb methods needed for valuation.
type PriceProvider interface {
	// GetPrice retrieves the cached price of a given SKU.
	GetPrice(sku string) (*pricedb.Price, bool)
}

// ConfigProvider defines the interface to fetch the current trading configurations.
type ConfigProvider interface {
	// GetConfig returns the active trade settings.
	GetConfig() trading.Config
}

// CritProvider defines the interface to synchronize active storefront listings on crit.tf.
type CritProvider interface {
	// FetchMyListings retrieves all active listings for the authenticated user.
	FetchMyListings(ctx context.Context) ([]crit.Listing, error)
	// CreateListing creates a new sell listing on crit.tf.
	CreateListing(ctx context.Context, assetID string, currencies pricedb.Currencies) (*crit.Listing, error)
	// UpdateListing updates an existing listing by its database ID.
	UpdateListing(ctx context.Context, listingID string, currencies pricedb.Currencies) (*crit.Listing, error)
	// DeleteListing deletes an active listing by its database ID.
	DeleteListing(ctx context.Context, listingID string) error
	// GetStorefrontURL thread-safely generates the showcase / storefront link.
	GetStorefrontURL(ctx context.Context) string
}

// ListingsSynchronizer coordinates in-game stock, pricedb changes, and external marketplace listings.
// Use [Sync] or [New] to register it with the orchestrator.
type ListingsSynchronizer struct {
	config Config
	logger log.Logger
	bus    *bus.Bus

	bp       BackpackProvider
	listings ListingProvider
	priceMgr PriceProvider
	cfgMgr   ConfigProvider
	crit     CritProvider

	ecpEngine *ecp.EasyCopyPaste

	mu          sync.Mutex
	bptfQueue   chan string
	bptfPending map[string]bool
	critQueue   chan string
	critPending map[string]bool
}

// Sync returns a [behavior.Option] that registers the [ListingsSynchronizer] with the orchestrator.
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

// New constructs a new [ListingsSynchronizer] instance.
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
		ecpEngine:   ecp.New(),
		bptfQueue:   make(chan string, 500),
		bptfPending: make(map[string]bool),
		critQueue:   make(chan string, 500),
		critPending: make(map[string]bool),
	}
}

// Name returns the unique name of the [ListingsSynchronizer] behavior.
func (s *ListingsSynchronizer) Name() string {
	return BehaviorName
}

// Run starts the listener event loops and schedules background marketplace updates.
// Returns an error if the context is cancelled.
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

	go s.bptfWorker(ctx)

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

func (s *ListingsSynchronizer) bptfWorker(ctx context.Context) {
	limiter := rate.NewLimiter(rate.Every(s.config.BptfRateLimit), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case sku := <-s.bptfQueue:
			s.mu.Lock()
			delete(s.bptfPending, sku)
			s.mu.Unlock()

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

			if err := limiter.Wait(ctx); err != nil {
				return
			}

			s.logger.Debug("Syncing batch to backpack.tf", log.Strings("skus", skus))
			s.syncBptfBatch(ctx, skus)
		}
	}
}

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

	activeListings, err := s.crit.FetchMyListings(ctx)
	if err != nil {
		s.logger.Error("Failed to fetch active listings from crit.tf", log.Err(err))
		return
	}

	listingsBySKU := make(map[string][]crit.Listing)

	listingsByAssetID := make(map[string]crit.Listing)
	for _, l := range activeListings {
		listingsBySKU[l.SKU] = append(listingsBySKU[l.SKU], l)
		listingsByAssetID[l.AssetID] = l
	}

	for _, skuStr := range skus {
		price, ok := s.priceMgr.GetPrice(skuStr)
		if !ok {
			s.logger.Debug("syncCritBatch: price not found", log.String("sku", skuStr))
			continue
		}

		if price.Sell.Metal <= 0 {
			s.logger.Debug(
				"syncCritBatch: price <= 0",
				log.String("sku", skuStr),
				log.Float64("price", price.Sell.Metal),
			)

			continue
		}

		physicalIDs := s.bp.GetItemsBySKU(skuStr)

		minStock := 0

		enableSell := true
		if itemCfg, exists := cfg.Items[skuStr]; exists {
			minStock = itemCfg.MinStock
			enableSell = itemCfg.EnableSell
		}

		s.logger.Debug(
			"syncCritBatch: stock check",
			log.String("sku", skuStr),
			log.Int("items_count", len(physicalIDs)),
			log.Int("min_stock", minStock),
			log.Bool("enable_sell", enableSell),
		)

		var targetIDs []uint64
		if enableSell && len(physicalIDs) > minStock {
			targetIDs = physicalIDs[minStock:]
		}

		s.logger.Debug("syncCritBatch: targetIDs sliced", log.Int("target_count", len(targetIDs)))

		targetAssetIDs := make(map[string]bool)
		for _, id := range targetIDs {
			idStr := strconv.FormatUint(id, 10)
			targetAssetIDs[idStr] = true

			_, exists := listingsByAssetID[idStr]
			s.logger.Debug("syncCritBatch: loop asset", log.String("asset_id", idStr), log.Bool("exists", exists))

			if !exists {
				s.logger.Debug("Creating listing on crit.tf", log.String("sku", skuStr), log.String("asset_id", idStr))

				if _, err := s.crit.CreateListing(ctx, idStr, price.Sell); err != nil {
					s.logger.Error("Failed to create crit.tf listing", log.String("asset_id", idStr), log.Err(err))
				}
			} else {
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

		existingBuy := s.listings.FindListingBySKU(skuStr, "buy")
		if enableBuy && stock < maxStock {
			resolvables = append(resolvables, bptf.ListingResolvable{
				Item:   skuStr,
				Intent: "buy",
				Currencies: map[string]float64{
					"metal": price.Buy.Metal,
				},
				Details: s.generateListingComment(ctx, skuStr, "buy", price.Buy, stock, maxStock),
			})
		} else if existingBuy != nil {
			deleteIDs = append(deleteIDs, existingBuy.ID)
		}

		existingSell := s.listings.FindListingBySKU(skuStr, "sell")
		if enableSell && stock > minStock {
			resolvables = append(resolvables, bptf.ListingResolvable{
				Item:   skuStr,
				Intent: "sell",
				Currencies: map[string]float64{
					"metal": price.Sell.Metal,
				},
				Details: s.generateListingComment(ctx, skuStr, "sell", price.Sell, stock, maxStock),
			})
		} else if existingSell != nil {
			deleteIDs = append(deleteIDs, existingSell.ID)
		}
	}

	if len(deleteIDs) > 0 {
		for _, id := range deleteIDs {
			if err := s.listings.Delete(ctx, id); err != nil {
				s.logger.Error("Failed to delete bptf listing", log.String("id", id), log.Err(err))
			}
		}
	}

	if len(resolvables) > 0 {
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

func formatPrice(currencies pricedb.Currencies) string {
	keys := currencies.Keys
	metal := currencies.Metal

	switch {
	case keys > 0 && metal > 0:
		keyStr := "key"
		if keys > 1 {
			keyStr = "keys"
		}

		return fmt.Sprintf("%d %s, %g ref", keys, keyStr, metal)
	case keys > 0:
		keyStr := "key"
		if keys > 1 {
			keyStr = "keys"
		}

		return fmt.Sprintf("%d %s", keys, keyStr)
	default:
		return fmt.Sprintf("%g ref", metal)
	}
}

func (s *ListingsSynchronizer) generateListingComment(
	ctx context.Context,
	skuStr string,
	intent string,
	price pricedb.Currencies,
	stock int,
	maxStock int,
) string {
	cfg := s.cfgMgr.GetConfig()
	template := cfg.ListingCommentTemplate

	// Standard fallback if template is empty
	if template == "" {
		if intent == "buy" {
			return fmt.Sprintf("⚡ G-man | Buying %s | Stock: %d/%d", skuStr, stock, maxStock)
		}

		return fmt.Sprintf("⚡ G-man | Selling %s | Stock: %d/%d", skuStr, stock, maxStock)
	}

	// 1. %price%
	formattedPrice := formatPrice(price)

	// 2. %name%
	itemName := skuStr
	if itemCfg, exists := cfg.Items[skuStr]; exists && itemCfg.Name != "" {
		itemName = itemCfg.Name
	}

	// 3. %ecp_item%
	var ecpStr string
	if s.ecpEngine != nil {
		var err error

		ecpStr, err = s.ecpEngine.ToEcpString(itemName, intent)
		if err != nil {
			s.logger.Warn("Failed to generate ECP string", log.String("sku", skuStr), log.Err(err))

			ecpStr = ""
		}
	}

	// 4. %current_stock% & %max_stock%
	currentStockStr := strconv.Itoa(stock)
	maxStockStr := strconv.Itoa(maxStock)

	// 5. %crittf_store% & %crittf_item%
	critStoreURL := ""

	critItemURL := ""
	if s.crit != nil {
		critStoreURL = s.crit.GetStorefrontURL(ctx)
		if critStoreURL != "" {
			critItemURL = critStoreURL + "/item/" + skuStr
		}
	}

	// Replacements
	comment := template
	comment = strings.ReplaceAll(comment, "%price%", formattedPrice)
	comment = strings.ReplaceAll(comment, "%name%", itemName)
	comment = strings.ReplaceAll(comment, "%ecp_item%", ecpStr)
	comment = strings.ReplaceAll(comment, "%current_stock%", currentStockStr)
	comment = strings.ReplaceAll(comment, "%max_stock%", maxStockStr)
	comment = strings.ReplaceAll(comment, "%crittf_store%", critStoreURL)
	comment = strings.ReplaceAll(comment, "%crittf_item%", critItemURL)

	return comment
}
