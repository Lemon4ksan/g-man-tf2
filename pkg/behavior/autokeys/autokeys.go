// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package autokeys provides automatic key trading and scrap metal balancing.
package autokeys

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// BehaviorName is the unique name of this behavior.
const BehaviorName = "tf2_autokeys"

// Autokeys operational phases
const (
	PhaseBuying     = "buying"
	PhaseBankingBuy = "bankingBuy"
	PhaseSelling    = "selling"
	PhaseBanking    = "banking"
	PhaseIdle       = "idle"
)

// Intent constants for listings
const (
	IntentBuy  = "buy"
	IntentSell = "sell"
)

// PricelistChangedSourceAutokeys is the source for pricelist modifications made by Autokeys.
const PricelistChangedSourceAutokeys pricedb.PricelistChangedSource = "Autokeys"

// Config defines the configuration for the Autokeys behavior.
type Config struct {
	MinKeys               int           `json:"min_keys"`
	MaxKeys               int           `json:"max_keys"`
	MinRefs               float64       `json:"min_refs"`
	MaxRefs               float64       `json:"max_refs"`
	EnableBanking         bool          `json:"enable_banking"`
	EnableScrapAdjustment bool          `json:"enable_scrap_adjustment"`
	ScrapAdjustmentValue  int           `json:"scrap_adjustment_value"` // in Scrap units
	CheckInterval         time.Duration `json:"check_interval"`
	WeaponsAsCurrency     bool          `json:"weapons_as_currency"`

	// Precalculated units of currency.Scrap to avoid rounding errors in Go
	MinRefsScrap currency.Scrap `json:"-"`
	MaxRefsScrap currency.Scrap `json:"-"`
}

// OverallStatus contains the active state flags.
type OverallStatus struct {
	IsBuyingKeys          bool
	IsBankingKeys         bool
	AlreadyUpdatedToBank  bool
	AlreadyUpdatedToBuy   bool
	AlreadyUpdatedToSell  bool
	LowPureAlertTriggered bool
}

// OldAmount tracks transaction volumes of the last scan.
type OldAmount struct {
	KeysCanBuy       int
	KeysCanSell      int
	KeysCanBankMin   int
	KeysCanBankMax   int
	CurrentKeysCount int
}

// KeyPrices caches last processed prices.
type KeyPrices struct {
	Buy  pricedb.Currencies
	Sell pricedb.Currencies
}

// State tracks active operational state thread-safely.
type State struct {
	mu           sync.RWMutex
	IsActive     bool
	Status       OverallStatus
	OldAmount    OldAmount
	OldKeyPrices KeyPrices
}

// AlertProvider handles debounced messaging to administrators.
type AlertProvider interface {
	MessageAdmins(ctx context.Context, message string) error
}

// BackpackProvider defines the subset of Backpack methods needed by Autokeys.
type BackpackProvider interface {
	GetPureStock() currency.PureStock
	GetStock(sku string) int
	Cache() backpack.ItemCache
	Schema() backpack.SchemaProvider
}

// PriceProvider defines the subset of pricedb.Manager methods needed by Autokeys.
type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
	SetPrice(sku string, buy, sell pricedb.Currencies, source pricedb.PricelistChangedSource)
}

// Autokeys is the behavior that automatically manages key prices.
type Autokeys struct {
	bp            BackpackProvider
	priceMgr      PriceProvider
	logger        log.Logger
	bus           *bus.Bus
	config        Config
	state         *State
	alertProvider AlertProvider
}

// Register returns a behavior.Option to install Autokeys behavior into Orchestrator.
func Register(
	bp BackpackProvider,
	priceMgr PriceProvider,
	cfg Config,
	alertProvider AlertProvider,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(bp, priceMgr, o.Logger(), o.Bus(), cfg, alertProvider))
	}
}

// New constructs a new Autokeys behavior instance.
func New(
	bp BackpackProvider,
	priceMgr PriceProvider,
	logger log.Logger,
	bus *bus.Bus,
	cfg Config,
	alertProvider AlertProvider,
) *Autokeys {
	cfg.MinRefsScrap = currency.ToScrap(cfg.MinRefs)
	cfg.MaxRefsScrap = currency.ToScrap(cfg.MaxRefs)

	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 5 * time.Minute
	}

	return &Autokeys{
		bp:       bp,
		priceMgr: priceMgr,
		logger:   logger.With(log.Module(BehaviorName)),
		bus:      bus,
		config:   cfg,
		state: &State{
			IsActive: true,
		},
		alertProvider: alertProvider,
	}
}

// Name returns the unique name of the behavior.
func (a *Autokeys) Name() string {
	return BehaviorName
}

// Run starts the background event loop subscribing to inventory changes and fallbacks.
func (a *Autokeys) Run(ctx context.Context) error {
	a.logger.Info("Autokeys behavior started", log.Duration("interval", a.config.CheckInterval))

	sub := a.bus.Subscribe(
		&tf2.BackpackLoadedEvent{},
		&tf2.ItemUpdatedEvent{},
		&tf2.ItemAcquiredEvent{},
		&tf2.ItemRemovedEvent{},
	)
	defer sub.Unsubscribe()

	ticker := time.NewTicker(a.config.CheckInterval)
	defer ticker.Stop()

	// Initial scan at startup
	if err := a.scan(ctx); err != nil {
		a.logger.Error("Initial scan failed", log.Err(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sub.C():
			a.logger.Debug("Inventory change event detected, running scan...")

			if err := a.scan(ctx); err != nil {
				a.logger.Error("Event scan failed", log.Err(err))
			}

		case <-ticker.C:
			a.logger.Debug("Running scheduled periodic scan...")

			if err := a.scan(ctx); err != nil {
				a.logger.Error("Periodic scan failed", log.Err(err))
			}
		}
	}
}

// IsEnabled returns whether the behavior is enabled.
func (a *Autokeys) IsEnabled() bool {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.IsActive
}

// IsActive returns whether the state machine is active in buying or banking.
func (a *Autokeys) IsActive() bool {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.IsActive && (a.state.Status.IsBuyingKeys || a.state.Status.IsBankingKeys)
}

// GetStatus returns the current status.
func (a *Autokeys) GetStatus() string {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()

	if a.state.Status.IsBankingKeys {
		return PhaseBanking
	}

	if a.state.Status.IsBuyingKeys {
		return PhaseBuying
	}

	if a.state.OldAmount.KeysCanSell > 0 && !a.state.Status.IsBuyingKeys && !a.state.Status.IsBankingKeys {
		return PhaseSelling
	}

	return PhaseIdle
}

// scan evaluates balances and triggers updates if state changes.
func (a *Autokeys) scan(ctx context.Context) error {
	// Phase 1: Gather inventory pure balances
	stock := a.bp.GetPureStock()
	currKeys := stock.Keys
	currRef := stock.TotalScrap()

	// Weapons duplication as currency
	if a.config.WeaponsAsCurrency {
		s := a.bp.Schema().Get()
		if s != nil && a.bp.Cache() != nil {
			weaponCounts := make(map[uint32]int)
			for _, item := range a.bp.Cache().GetItems() {
				if !item.IsCraftable || !item.IsTradable {
					continue
				}

				schItem := s.ItemByDef(int(item.DefIndex))
				if schItem == nil || schItem.CraftClass != "weapon" {
					continue
				}

				weaponCounts[item.DefIndex]++
			}

			duplicateWeaponsCount := 0
			for _, count := range weaponCounts {
				if count > 1 {
					duplicateWeaponsCount += (count - 1)
				}
			}

			// Each craftable weapon duplicate counts as 0.5 Scrap
			weaponsScrap := currency.Scrap(float64(duplicateWeaponsCount) * 0.5)
			currRef += weaponsScrap
		}
	}

	// Fetch base market key price
	kp, ok := a.priceMgr.GetPrice(currency.SKUKey)
	if !ok || kp.Buy.Metal <= 0 || kp.Sell.Metal <= 0 {
		a.logger.Warn(
			"Base key price unavailable in pricedb, skipping Autokeys cycle",
			log.String("sku", currency.SKUKey),
		)

		return nil
	}

	// Phase 2: Detect external price changes in pricedb.Manager
	a.state.mu.Lock()

	priceChanged := false
	if kp.Buy.Keys != a.state.OldKeyPrices.Buy.Keys ||
		kp.Buy.Metal != a.state.OldKeyPrices.Buy.Metal ||
		kp.Sell.Keys != a.state.OldKeyPrices.Sell.Keys ||
		kp.Sell.Metal != a.state.OldKeyPrices.Sell.Metal {
		priceChanged = true
		a.state.Status.AlreadyUpdatedToBank = false
		a.state.Status.AlreadyUpdatedToBuy = false
		a.state.Status.AlreadyUpdatedToSell = false

		a.state.OldKeyPrices.Buy = kp.Buy
		a.state.OldKeyPrices.Sell = kp.Sell
	}

	// Phase 3: Evaluate State Machine Phases
	isBuyingKeys := currRef > a.config.MaxRefsScrap && currKeys < a.config.MaxKeys
	isSellingKeys := currRef < a.config.MinRefsScrap && currKeys > a.config.MinKeys
	isBankingKeys := a.config.EnableBanking &&
		currRef >= a.config.MinRefsScrap && currRef <= a.config.MaxRefsScrap &&
		currKeys > a.config.MinKeys
	isBankingBuyKeys := a.config.EnableBanking &&
		currRef > a.config.MinRefsScrap &&
		currKeys <= a.config.MinKeys
	isAlertState := currRef < a.config.MinRefsScrap && currKeys < a.config.MinKeys

	// Phase 4: Math limit Calculations in Scrap Units
	keyBuyPriceScrap := currency.ToScrap(kp.Buy.Metal)
	keySellPriceScrap := currency.ToScrap(kp.Sell.Metal)

	var keysCanBuy, keysCanSell, keysCanBankMin, keysCanBankMax int
	if keyBuyPriceScrap > 0 {
		keysCanBuy = max(int(math.Round(float64(currRef-a.config.MaxRefsScrap)/float64(keyBuyPriceScrap))), 0)
	}

	if keySellPriceScrap > 0 {
		keysCanSell = max(int(math.Round(float64(a.config.MinRefsScrap-currRef)/float64(keySellPriceScrap))), 0)
	}

	if a.config.EnableBanking {
		if keySellPriceScrap > 0 {
			keysCanBankMin = max(int(math.Round(float64(a.config.MaxRefsScrap-currRef)/float64(keySellPriceScrap))), 0)
		}

		if keyBuyPriceScrap > 0 {
			keysCanBankMax = max(int(math.Round(float64(currRef-a.config.MinRefsScrap)/float64(keyBuyPriceScrap))), 0)
		}
	}

	var (
		setMinKeys, setMaxKeys int
		currentPhase           string
	)

	switch {
	case isBuyingKeys:
		currentPhase = PhaseBuying
		setMinKeys = currKeys
		setMaxKeys = currKeys + keysCanBuy

		if setMinKeys < a.config.MinKeys {
			setMinKeys = a.config.MinKeys
		}

		if setMaxKeys > a.config.MaxKeys {
			setMaxKeys = a.config.MaxKeys
		}

	case isBankingBuyKeys:
		currentPhase = PhaseBankingBuy
		setMinKeys = currKeys
		setMaxKeys = currKeys + keysCanBankMax

		if setMinKeys < a.config.MinKeys {
			setMinKeys = a.config.MinKeys
		}

		if setMaxKeys > a.config.MaxKeys {
			setMaxKeys = a.config.MaxKeys
		}

	case isSellingKeys:
		currentPhase = PhaseSelling
		setMinKeys = currKeys - keysCanSell
		setMaxKeys = currKeys

		if setMinKeys < a.config.MinKeys {
			setMinKeys = a.config.MinKeys
		}

		if setMaxKeys > a.config.MaxKeys {
			setMaxKeys = a.config.MaxKeys
		}

	case isBankingKeys:
		currentPhase = PhaseBanking
		setMinKeys = currKeys - keysCanBankMin
		setMaxKeys = currKeys + keysCanBankMax

		if setMinKeys < a.config.MinKeys {
			setMinKeys = a.config.MinKeys
		}

		if setMaxKeys > a.config.MaxKeys {
			setMaxKeys = a.config.MaxKeys
		}

		// Safety range expansion:
		if setMaxKeys-setMinKeys <= 1 {
			setMinKeys--
			setMaxKeys++

			if setMinKeys < 0 {
				setMinKeys = 0
			}
		}

	default:
		currentPhase = PhaseIdle
		setMinKeys = a.config.MinKeys
		setMaxKeys = a.config.MaxKeys
	}

	if setMinKeys >= setMaxKeys {
		setMaxKeys = setMinKeys + 1
	}

	// Phase 5: Scrap price adjustments (Offsets)
	adjBuyScrap := keyBuyPriceScrap
	adjSellScrap := keySellPriceScrap

	if a.config.EnableScrapAdjustment {
		step := currency.Scrap(a.config.ScrapAdjustmentValue)
		if isBuyingKeys || isBankingBuyKeys {
			adjBuyScrap += step
			adjSellScrap += step
		} else if isSellingKeys {
			adjBuyScrap -= step
			adjSellScrap -= step
		}
	}

	if adjBuyScrap <= 0 {
		adjBuyScrap = 1
	}

	if adjSellScrap <= 0 {
		adjSellScrap = 1
	}

	adjBuyRef := currency.ToRefined(adjBuyScrap)
	adjSellRef := currency.ToRefined(adjSellScrap)

	// Phase 6: Debounced alerts for critical pure levels
	if isAlertState {
		if !a.state.Status.LowPureAlertTriggered {
			a.state.Status.LowPureAlertTriggered = true

			msg := fmt.Sprintf(
				"⚠️ [CRITICAL] Autokeys: Low pure alert triggered! Keys: %d, Metal: %s",
				currKeys,
				currency.FormatRefined(currRef),
			)
			a.logger.Warn(
				msg,
				log.Int("curr_keys", currKeys),
				log.Float64("curr_metal_ref", currency.ToRefined(currRef)),
			)

			if a.alertProvider != nil {
				_ = a.alertProvider.MessageAdmins(ctx, msg)
			}
		}
	} else {
		if a.state.Status.LowPureAlertTriggered {
			a.state.Status.LowPureAlertTriggered = false

			msg := fmt.Sprintf(
				"✅ [INFO] Autokeys: Pure levels recovered. Keys: %d, Metal: %s",
				currKeys,
				currency.FormatRefined(currRef),
			)
			a.logger.Info(
				msg,
				log.Int("curr_keys", currKeys),
				log.Float64("curr_metal_ref", currency.ToRefined(currRef)),
			)

			if a.alertProvider != nil {
				_ = a.alertProvider.MessageAdmins(ctx, msg)
			}
		}
	}

	// Phase 7: Redundant transaction filtering
	amountsSame := a.state.OldAmount.KeysCanBuy == keysCanBuy &&
		a.state.OldAmount.KeysCanSell == keysCanSell &&
		a.state.OldAmount.KeysCanBankMin == keysCanBankMin &&
		a.state.OldAmount.KeysCanBankMax == keysCanBankMax &&
		a.state.OldAmount.CurrentKeysCount == currKeys

	flagsSame := a.state.Status.IsBuyingKeys == isBuyingKeys &&
		a.state.Status.IsBankingKeys == (isBankingKeys || isBankingBuyKeys)

	shouldSkipUpdate := amountsSame && flagsSame && !priceChanged
	if !shouldSkipUpdate {
		a.state.OldAmount = OldAmount{
			KeysCanBuy:       keysCanBuy,
			KeysCanSell:      keysCanSell,
			KeysCanBankMin:   keysCanBankMin,
			KeysCanBankMax:   keysCanBankMax,
			CurrentKeysCount: currKeys,
		}
		a.state.Status.IsBuyingKeys = isBuyingKeys
		a.state.Status.IsBankingKeys = (isBankingKeys || isBankingBuyKeys)
	}

	a.state.mu.Unlock()

	// Diagnostic structured logs
	a.logger.Info("Autokeys scan completed",
		log.Int("curr_keys", currKeys),
		log.Float64("curr_metal_ref", currency.ToRefined(currRef)),
		log.Int("set_min_keys", setMinKeys),
		log.Int("set_max_keys", setMaxKeys),
		log.Float64("adj_buy_ref", adjBuyRef),
		log.Float64("adj_sell_ref", adjSellRef),
		log.Int("scrap_offset", int(a.config.ScrapAdjustmentValue)),
		log.String("phase", currentPhase),
	)

	if shouldSkipUpdate {
		a.logger.Debug("Autokeys state identical, skipping transaction update")
		return nil
	}

	// Thread-safe update in local pricedb cache
	buyCurrencies := pricedb.Currencies{Keys: 0, Metal: adjBuyRef}
	sellCurrencies := pricedb.Currencies{Keys: 0, Metal: adjSellRef}
	a.priceMgr.SetPrice(currency.SKUKey, buyCurrencies, sellCurrencies, PricelistChangedSourceAutokeys)

	return nil
}
