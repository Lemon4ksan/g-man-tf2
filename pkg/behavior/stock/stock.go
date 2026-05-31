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
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
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
	// AuditInterval defines the duration between scheduled stock audits.
	AuditInterval time.Duration `json:"audit_interval"`
	// ConfigCheckInterval defines the duration between config update checks.
	ConfigCheckInterval time.Duration `json:"config_check_interval"`
	// StagnantThreshold defines the duration an item must remain unsold before it is considered stagnant.
	StagnantThreshold time.Duration `json:"stagnant_threshold"`
	// DiscountPercent defines the percentage deducted from an item's sell price when it is stagnant.
	DiscountPercent float64 `json:"discount_percent"`
	// MaxAllowedDiscount defines the maximum total discount percentage allowed for stagnant items.
	MaxAllowedDiscount float64 `json:"max_allowed_discount"`
	// MinScrapMetal defines the minimum scrap metal reserve threshold.
	MinScrapMetal int `json:"min_scrap_metal"`
	// MinReclaimedMetal defines the minimum reclaimed metal reserve threshold.
	MinReclaimedMetal int `json:"min_reclaimed_metal"`
}

// DefaultConfig returns a [Config] containing production-ready strategist defaults.
func DefaultConfig() Config {
	return Config{
		AuditInterval:       1 * time.Hour,
		ConfigCheckInterval: 5 * time.Second,
		StagnantThreshold:   14 * 24 * time.Hour,
		DiscountPercent:     0.05,
		MaxAllowedDiscount:  0.20,
		MinScrapMetal:       9,
		MinReclaimedMetal:   3,
	}
}

// BackpackProvider defines the subset of inventory methods required to audit local stock.
type BackpackProvider interface {
	// GetStock returns the current stock count for a specific SKU.
	GetStock(sku string) int
	// GetItemsBySKU returns all local item IDs matching the specified SKU.
	GetItemsBySKU(targetSKU string) []uint64
	// GetPureStock retrieves the current keys and metal stock.
	GetPureStock() currency.PureStock
}

// PriceProvider defines the subset of pricedb methods required for pricing calculations.
type PriceProvider interface {
	// GetPrice retrieves the cached price of a given SKU.
	GetPrice(sku string) (*pricedb.Price, bool)
	// SetPrice updates the price and source for a given SKU.
	SetPrice(sku string, buy, sell pricedb.Currencies, source pricedb.PricelistChangedSource)
	// Watch adds a SKU to the background update list.
	Watch(sku string)
	// Unwatch removes a SKU from the background update list.
	Unwatch(sku string)
	// GetWatchedSKUs returns a slice of all currently watched SKUs.
	GetWatchedSKUs() []string
}

// ConfigProvider defines the interface to fetch the current trading configurations.
type ConfigProvider interface {
	// GetConfig returns the active trade settings.
	GetConfig() trading.Config
}

// CostBasisProvider defines the cost basis store interface for price tracking.
type CostBasisProvider interface {
	// GetOldestEntry retrieves the oldest CostBasisEntry for a SKU without removing it.
	GetOldestEntry(sku string) (storage.CostBasisEntry, bool)
}

// CraftingProvider defines the metal crafting coordinator interface.
type CraftingProvider interface {
	// CondenseMetal automatically "compresses" all available metal in the inventory.
	CondenseMetal(ctx context.Context) (int, error)
	// MakeChange smelts higher-grade metal until the target is reached.
	MakeChange(ctx context.Context, targetDefIndex uint32, targetCount int) error
	// SmeltClassWeapons finds duplicate class weapons and smelts them into scrap metal.
	SmeltClassWeapons(ctx context.Context, class string) ([]uint64, error)
}

// Strategist orchestrates pricing watchlists, dynamic discounts, and crafting cycles.
// Use [Control] or [New] to register it with the orchestrator.
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
	activeDiscounts map[string]float64
	lastConfigSKUs  map[string]bool
	gcConnected     bool
}

// Control returns a [behavior.Option] that registers the [Strategist] with the orchestrator.
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

// New constructs a new [Strategist] behavior.
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

	if cfg.ConfigCheckInterval == 0 {
		cfg.ConfigCheckInterval = DefaultConfig().ConfigCheckInterval
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

// Name returns the unique name of the [Strategist] behavior.
func (s *Strategist) Name() string {
	return BehaviorName
}

// Run starts the strategist audit cycles and subscriptions.
// Returns an error if the context is cancelled.
func (s *Strategist) Run(ctx context.Context) error {
	s.logger.Info("StockStrategist started")

	sub := s.bus.Subscribe(
		&tf2.BackpackLoadedEvent{},
		&tf2.ConnectedEvent{},
		&tf2.DisconnectedEvent{},
		&tf2.ItemRemovedEvent{},
	)
	defer sub.Unsubscribe()

	configCheckTicker := time.NewTicker(s.config.ConfigCheckInterval)
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

			case *tf2.ItemRemovedEvent:
				s.logger.Debug("ItemRemovedEvent received, checking if auto-reset is needed")
				s.checkAndResetAutoPrices()
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

		s.bus.Publish(&listingsync.AuditRequestedEvent{SKUs: skus})

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
	s.checkAndResetAutoPrices()
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

func (s *Strategist) checkAndResetAutoPrices() {
	cfg := s.cfgMgr.GetConfig()
	if !cfg.AutoResetToAutopriceOnceSold {
		return
	}

	for sku := range cfg.Items {
		if s.bp.GetStock(sku) == 0 {
			p, ok := s.priceMgr.GetPrice(sku)
			if ok && p.Source == string(pricedb.PricelistChangedSourceManual) {
				s.logger.Info(
					"Stock reached 0 for manually priced item, resetting to autoprice",
					log.String("sku", sku),
				)
				s.priceMgr.SetPrice(sku, p.Buy, p.Sell, pricedb.PricelistChangedSourcePriceDB)
			}
		}
	}
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

		selfCostRef := entry.BuyMetal
		if entry.BuyKeys > 0 && keyPrice > 0 {
			selfCostRef += entry.BuyKeys * keyPrice
		}

		minProfitRef := float64(cfg.PPUMinProfitScrap) / 9.0
		minAllowedSellPriceMetal := selfCostRef + minProfitRef

		maxDiscountedMetal := trueBasePrice * (1.0 - s.config.MaxAllowedDiscount)
		if minAllowedSellPriceMetal < maxDiscountedMetal {
			minAllowedSellPriceMetal = maxDiscountedMetal
		}

		if discountedMetal < minAllowedSellPriceMetal {
			discountedMetal = minAllowedSellPriceMetal
		}

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

	if int(scrapCount) < s.config.MinScrapMetal || int(recCount) < s.config.MinReclaimedMetal {
		s.logger.Info("Metal stock below critical minimums. Coordinating smelting...")

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

	if scrapCount > 18 || recCount > 9 {
		s.logger.Info("Excess low-grade metal detected, condensing inventory slots...")

		if crafts, err := s.crafting.CondenseMetal(ctx); err == nil && crafts > 0 {
			s.logger.Info("Low-grade metals successfully condensed", log.Int("crafts", crafts))
		} else if err != nil {
			s.logger.Error("Failed to condense low-grade metal", log.Err(err))
		}
	}
}
