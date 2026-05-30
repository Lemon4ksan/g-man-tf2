// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stock provides automated watchlist seeding, dynamic
// FIFO stagnant stock discounting, and crafting coordination.
package stock

import (
	"context"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/listingsync"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// BehaviorName is the unique name of the stock control behavior.
const BehaviorName = "stock_control"

// PricelistChangedSourceStock is the change source for Stock Control pricing updates.
const PricelistChangedSourceStock pricedb.PricelistChangedSource = "StockControl"

// Config holds configuration parameters for the Stock Strategist behavior.
type Config struct {
	AuditInterval      time.Duration `json:"audit_interval"`
	StagnantThreshold  time.Duration `json:"stagnant_threshold"`
	DiscountPercent    float64       `json:"discount_percent"`
	MaxAllowedDiscount float64       `json:"max_allowed_discount"`
	MinScrapMetal      int           `json:"min_scrap_metal"`
	MinReclaimedMetal  int           `json:"min_reclaimed_metal"`
}

// DefaultConfig returns production-ready strategy defaults.
func DefaultConfig() Config {
	return Config{
		AuditInterval:      1 * time.Hour,
		StagnantThreshold:  14 * 24 * time.Hour, // 14 days
		DiscountPercent:    0.05,                // 5%
		MaxAllowedDiscount: 0.20,                // 20%
		MinScrapMetal:      9,
		MinReclaimedMetal:  3,
	}
}

// BackpackProvider defines inventory details for Stock Strategist.
type BackpackProvider interface {
	GetStock(sku string) int
	GetItemsBySKU(targetSKU string) []uint64
	GetPureStock() currency.PureStock
}

// PriceProvider defines pricedb methods.
type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
	SetPrice(sku string, buy, sell pricedb.Currencies, source pricedb.PricelistChangedSource)
	Watch(sku string)
	Unwatch(sku string)
	GetWatchedSKUs() []string
}

// ConfigProvider defines trading config manager.
type ConfigProvider interface {
	GetConfig() trading.Config
}

// CostBasisProvider defines cost basis store.
type CostBasisProvider interface {
	GetOldestEntry(sku string) (storage.CostBasisEntry, bool)
}

// CraftingProvider defines metal crafting coordinator.
type CraftingProvider interface {
	CondenseMetal(ctx context.Context) (int, error)
	MakeChange(ctx context.Context, targetDefIndex uint32, targetCount int) error
	SmeltClassWeapons(ctx context.Context, class string) ([]uint64, error)
}

// Control returns a behavior.Option to register StockStrategist with the orchestrator.
func Control(
	bp BackpackProvider,
	priceMgr PriceProvider,
	cfgMgr ConfigProvider,
	costBasis CostBasisProvider,
	crafting CraftingProvider,
	cfg Config,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(bp, priceMgr, cfgMgr, costBasis, crafting, o.Bus(), o.Logger(), cfg))
	}
}

// Strategist orchestrates pricing watchlists, dynamic discounts, and crafting cycles.
type Strategist struct {
	config Config
	logger log.Logger
	bus    *bus.Bus

	bp        BackpackProvider
	priceMgr  PriceProvider
	cfgMgr    ConfigProvider
	costBasis CostBasisProvider
	crafting  CraftingProvider

	mu              sync.Mutex
	activeDiscounts map[string]float64 // sku -> applied discounted sell price (metal)
	lastConfigSKUs  map[string]bool
	gcConnected     bool
}

// New constructs a new StockStrategist behavior.
func New(
	bp BackpackProvider,
	priceMgr PriceProvider,
	cfgMgr ConfigProvider,
	costBasis CostBasisProvider,
	crafting CraftingProvider,
	b *bus.Bus,
	logger log.Logger,
	cfg Config,
) *Strategist {
	if cfg.AuditInterval == 0 {
		cfg.AuditInterval = DefaultConfig().AuditInterval
	}

	if cfg.StagnantThreshold == 0 {
		cfg.StagnantThreshold = DefaultConfig().StagnantThreshold
	}

	if cfg.DiscountPercent == 0 {
		cfg.DiscountPercent = DefaultConfig().DiscountPercent
	}

	if cfg.MaxAllowedDiscount == 0 {
		cfg.MaxAllowedDiscount = DefaultConfig().MaxAllowedDiscount
	}

	if cfg.MinScrapMetal == 0 {
		cfg.MinScrapMetal = DefaultConfig().MinScrapMetal
	}

	if cfg.MinReclaimedMetal == 0 {
		cfg.MinReclaimedMetal = DefaultConfig().MinReclaimedMetal
	}

	return &Strategist{
		config:          cfg,
		logger:          logger.With(log.Module(BehaviorName)),
		bus:             b,
		bp:              bp,
		priceMgr:        priceMgr,
		cfgMgr:          cfgMgr,
		costBasis:       costBasis,
		crafting:        crafting,
		activeDiscounts: make(map[string]float64),
		lastConfigSKUs:  make(map[string]bool),
	}
}

// Name returns the unique behavior name.
func (s *Strategist) Name() string {
	return BehaviorName
}

// Run starts the strategist cycles and subscriptions.
func (s *Strategist) Run(ctx context.Context) error {
	s.logger.Info("StockStrategist started")

	sub := s.bus.Subscribe(
		&tf2.BackpackLoadedEvent{},
		&tf2.ConnectedEvent{},
		&tf2.DisconnectedEvent{},
	)
	defer sub.Unsubscribe()

	configCheckTicker := time.NewTicker(5 * time.Second)
	defer configCheckTicker.Stop()

	auditTicker := time.NewTicker(s.config.AuditInterval)
	defer auditTicker.Stop()

	cfg := s.cfgMgr.GetConfig()
	s.mu.Lock()
	for sku := range cfg.Items {
		s.lastConfigSKUs[sku] = true
	}

	s.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case ev, ok := <-sub.C():
			if !ok {
				return nil
			}

			switch ev.(type) {
			case *tf2.BackpackLoadedEvent:
				s.logger.Info("BackpackLoadedEvent received, triggering initial watchlist sync & audit...")
				s.mu.Lock()
				s.gcConnected = true
				s.mu.Unlock()
				s.runAudit(ctx)

			case *tf2.ConnectedEvent:
				s.logger.Info("GC connection established, stock control is ready")
				s.mu.Lock()
				s.gcConnected = true
				s.mu.Unlock()

			case *tf2.DisconnectedEvent:
				s.logger.Warn("GC connection lost, pausing stock control operations")
				s.mu.Lock()
				s.gcConnected = false
				s.mu.Unlock()
			}

		case <-configCheckTicker.C:
			s.checkForConfigUpdates()

		case <-auditTicker.C:
			s.logger.Info("Audit interval elapsed, running scheduled stock audit...")
			s.runAudit(ctx)
		}
	}
}

func (s *Strategist) syncWatchlist() {
	cfg := s.cfgMgr.GetConfig()

	for sku := range cfg.Items {
		s.priceMgr.Watch(sku)
	}

	watched := s.priceMgr.GetWatchedSKUs()
	for _, sku := range watched {
		if sku == currency.SKUKey {
			continue
		}

		if _, exists := cfg.Items[sku]; !exists {
			s.logger.Info("Unwatching obsolete SKU from pricedb", log.String("sku", sku))
			s.priceMgr.Unwatch(sku)
		}
	}
}

func (s *Strategist) checkForConfigUpdates() {
	cfg := s.cfgMgr.GetConfig()

	s.mu.Lock()

	hasChanged := false
	for skuStr := range cfg.Items {
		if !s.lastConfigSKUs[skuStr] {
			hasChanged = true
			break
		}
	}

	if !hasChanged {
		for skuStr := range s.lastConfigSKUs {
			if _, exists := cfg.Items[skuStr]; !exists {
				hasChanged = true
				break
			}
		}
	}

	s.mu.Unlock()

	if hasChanged {
		s.logger.Info("Live configuration update detected, synchronizing watchlist...")
		s.syncWatchlist()

		skus := make([]string, 0, len(cfg.Items)+1)
		for sku := range cfg.Items {
			skus = append(skus, sku)
		}

		skus = append(skus, currency.SKUKey)

		// Immediately notify ListingsSynchronizer of updates
		s.bus.Publish(&listingsync.AuditRequestedEvent{SKUs: skus})

		// Reload cached keys
		s.mu.Lock()

		s.lastConfigSKUs = make(map[string]bool)
		for sku := range cfg.Items {
			s.lastConfigSKUs[sku] = true
		}

		s.mu.Unlock()
	}
}

func (s *Strategist) runAudit(ctx context.Context) {
	s.syncWatchlist()
	s.discountStagnantItems()
	s.coordinateCrafting(ctx)

	cfg := s.cfgMgr.GetConfig()

	skus := make([]string, 0, len(cfg.Items)+1)
	for sku := range cfg.Items {
		skus = append(skus, sku)
	}

	skus = append(skus, currency.SKUKey)

	s.bus.Publish(&listingsync.AuditRequestedEvent{SKUs: skus})
}

func (s *Strategist) discountStagnantItems() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.cfgMgr.GetConfig()
	now := time.Now()

	keyPrice := 0.0
	if kp, ok := s.priceMgr.GetPrice(currency.SKUKey); ok && kp.Sell.Metal > 0 {
		keyPrice = kp.Sell.Metal
	}

	for sku := range cfg.Items {
		price, ok := s.priceMgr.GetPrice(sku)
		if !ok || price.Sell.IsZero() {
			continue
		}

		entry, ok := s.costBasis.GetOldestEntry(sku)
		if !ok {
			continue
		}

		age := now.Sub(entry.Timestamp)

		basePrice, alreadyDiscounted := s.activeDiscounts[sku]

		// If an external update happened, the source is no longer "StockControl".
		// That means a new base price was loaded, so we treat it as the new base price.
		isExternalUpdate := price.Source != string(PricelistChangedSourceStock)
		if isExternalUpdate && alreadyDiscounted {
			s.activeDiscounts[sku] = price.Sell.Metal
			basePrice = price.Sell.Metal
		}

		if age < s.config.StagnantThreshold {
			if alreadyDiscounted {
				s.logger.Info("FIFO item is no longer stagnant, restoring base price",
					log.String("sku", sku),
					log.Float64("base_ref", basePrice),
				)
				delete(s.activeDiscounts, sku)

				// Restore the original base price
				newSellCurrencies := pricedb.Currencies{Keys: price.Sell.Keys, Metal: basePrice}
				s.priceMgr.SetPrice(sku, price.Buy, newSellCurrencies, PricelistChangedSourceStock)
			}

			continue
		}

		var trueBasePrice float64
		if alreadyDiscounted {
			trueBasePrice = basePrice
		} else {
			trueBasePrice = price.Sell.Metal
		}

		discountedMetal := trueBasePrice * (1.0 - s.config.DiscountPercent)

		// FIFO PPU Cost Protection Floor
		selfCostRef := entry.BuyMetal
		if entry.BuyKeys > 0 && keyPrice > 0 {
			selfCostRef += entry.BuyKeys * keyPrice
		}

		minProfitRef := float64(cfg.PPUMinProfitScrap) / 9.0
		minAllowedSellPriceMetal := selfCostRef + minProfitRef

		// Apply max allowed discount boundaries
		maxDiscountedMetal := trueBasePrice * (1.0 - s.config.MaxAllowedDiscount)
		if minAllowedSellPriceMetal < maxDiscountedMetal {
			minAllowedSellPriceMetal = maxDiscountedMetal
		}

		if discountedMetal < minAllowedSellPriceMetal {
			discountedMetal = minAllowedSellPriceMetal
		}

		// Update or apply discount if the current sell price in pricedb is different
		if discountedMetal < price.Sell.Metal || !alreadyDiscounted {
			s.logger.Info("FIFO item is stagnant, applying dynamic discount",
				log.String("sku", sku),
				log.Float64("age_days", age.Hours()/24.0),
				log.Float64("old_sell_ref", price.Sell.Metal),
				log.Float64("new_sell_ref", discountedMetal),
				log.Float64("cost_ref", selfCostRef),
			)

			if !alreadyDiscounted {
				s.activeDiscounts[sku] = trueBasePrice
			}

			newSellCurrencies := pricedb.Currencies{Keys: price.Sell.Keys, Metal: discountedMetal}
			s.priceMgr.SetPrice(sku, price.Buy, newSellCurrencies, PricelistChangedSourceStock)
		}
	}
}

func (s *Strategist) coordinateCrafting(ctx context.Context) {
	s.mu.Lock()
	connected := s.gcConnected
	s.mu.Unlock()

	if !connected {
		s.logger.Debug("Skipping crafting coordination: GC is not connected")
		return
	}

	pureStock := s.bp.GetPureStock()
	scrapCount := pureStock.Scrap
	recCount := pureStock.Reclaimed
	refCount := pureStock.Refined

	s.logger.Debug("Evaluating metal balances for crafting",
		log.Int("scrap", int(scrapCount)),
		log.Int("reclaimed", int(recCount)),
		log.Int("refined", int(refCount)),
	)

	// Smelt duplicates if below thresholds
	if int(scrapCount) < s.config.MinScrapMetal || int(recCount) < s.config.MinReclaimedMetal {
		s.logger.Info("Metal stock below critical minimums. Coordinating smelting...")

		// A. Smelt class weapons duplicates
		classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
		for _, class := range classes {
			if _, err := s.crafting.SmeltClassWeapons(ctx, class); err == nil {
				s.logger.Info(
					"Successfully smelted duplicate class weapons into Scrap Metal",
					log.String("class", class),
				)

				scrapCount++
				if int(scrapCount) >= s.config.MinScrapMetal {
					break
				}
			}
		}

		// B. Smelt Reclaimed or Refined if still below threshold
		if int(scrapCount) < s.config.MinScrapMetal {
			s.logger.Info("Splitting Reclaimed metal into Scrap...")

			if err := s.crafting.MakeChange(ctx, 5000, s.config.MinScrapMetal); err != nil {
				s.logger.Error("Failed to smelt Reclaimed into Scrap", log.Err(err))
			}
		}

		if int(recCount) < s.config.MinReclaimedMetal {
			s.logger.Info("Splitting Refined metal into Reclaimed...")

			if err := s.crafting.MakeChange(ctx, 5001, s.config.MinReclaimedMetal); err != nil {
				s.logger.Error("Failed to smelt Refined into Reclaimed", log.Err(err))
			}
		}
	}

	// Condense low-grade metal if excess exists
	if scrapCount > 18 || recCount > 9 {
		s.logger.Info("Excess low-grade metal detected, condensing inventory slots...")

		if crafts, err := s.crafting.CondenseMetal(ctx); err == nil && crafts > 0 {
			s.logger.Info("Low-grade metals successfully condensed", log.Int("crafts", crafts))
		} else if err != nil {
			s.logger.Error("Failed to condense low-grade metal", log.Err(err))
		}
	}
}
