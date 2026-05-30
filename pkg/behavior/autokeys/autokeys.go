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
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// BehaviorName is the unique identifier for the autokeys behavior.
const BehaviorName = "tf2_autokeys"

const (
	// PhaseBuying represents the key purchase phase where keys are bought with metal.
	PhaseBuying = "buying"
	// PhaseBankingBuy represents the key banking buy phase.
	PhaseBankingBuy = "bankingBuy"
	// PhaseSelling represents the key sale phase where keys are sold for metal.
	PhaseSelling = "selling"
	// PhaseBanking represents the key banking phase where both buy and sell limits are active.
	PhaseBanking = "banking"
	// PhaseIdle represents the inactive phase where no automatic updates are applied.
	PhaseIdle = "idle"
)

const (
	// IntentBuy represents a listing buy intent.
	IntentBuy = "buy"
	// IntentSell represents a listing sell intent.
	IntentSell = "sell"
)

// PricelistChangedSourceAutokeys is the source name applied to pricelist changes made by autokeys.
const PricelistChangedSourceAutokeys pricedb.PricelistChangedSource = "Autokeys"

// Config defines the configuration parameters for key trading and metal balancing.
type Config struct {
	// MinKeys defines the minimum key reserve count.
	MinKeys int `json:"min_keys"`
	// MaxKeys defines the maximum key reserve count.
	MaxKeys int `json:"max_keys"`
	// MinRefs defines the minimum metal reserve count in refined units.
	MinRefs float64 `json:"min_refs"`
	// MaxRefs defines the maximum metal reserve count in refined units.
	MaxRefs float64 `json:"max_refs"`
	// EnableBanking enables parallel buying and selling of keys.
	EnableBanking bool `json:"enable_banking"`
	// EnableScrapAdjustment enables scrap offsets for volatile prices.
	EnableScrapAdjustment bool `json:"enable_scrap_adjustment"`
	// ScrapAdjustmentValue defines the offset value in scrap units.
	ScrapAdjustmentValue int `json:"scrap_adjustment_value"` // in Scrap units
	// CheckInterval defines the delay between scans.
	CheckInterval time.Duration `json:"check_interval"`
	// WeaponsAsCurrency allows counting duplicate craftable weapons as metal value.
	WeaponsAsCurrency bool `json:"weapons_as_currency"`

	// MinRefsScrap represents precalculated minimum refined metal in scrap units.
	MinRefsScrap currency.Scrap `json:"-"`
	// MaxRefsScrap represents precalculated maximum refined metal in scrap units.
	MaxRefsScrap currency.Scrap `json:"-"`
}

// OverallStatus contains the active state flags for the state machine.
type OverallStatus struct {
	// IsBuyingKeys is true when the state machine is actively buying keys.
	IsBuyingKeys bool
	// IsBankingKeys is true when the state machine is actively banking keys.
	IsBankingKeys bool
	// AlreadyUpdatedToBank is true if prices were synchronized for key banking.
	AlreadyUpdatedToBank bool
	// AlreadyUpdatedToBuy is true if prices were synchronized for key buying.
	AlreadyUpdatedToBuy bool
	// AlreadyUpdatedToSell is true if prices were synchronized for key selling.
	AlreadyUpdatedToSell bool
	// LowPureAlertTriggered is true if a low pure alert message was sent to administrators.
	LowPureAlertTriggered bool
}

// OldAmount tracks transactions and inventory states of the previous execution scan.
type OldAmount struct {
	// KeysCanBuy represents the calculated number of keys that can be bought.
	KeysCanBuy int
	// KeysCanSell represents the calculated number of keys that can be sold.
	KeysCanSell int
	// KeysCanBankMin represents the minimum keys available for banking.
	KeysCanBankMin int
	// KeysCanBankMax represents the maximum keys available for banking.
	KeysCanBankMax int
	// CurrentKeysCount represents the last scanned total count of keys in the inventory.
	CurrentKeysCount int
}

// KeyPrices contains the last cached buy and sell prices of keys.
type KeyPrices struct {
	// Buy represents the buy price of keys.
	Buy pricedb.Currencies
	// Sell represents the sell price of keys.
	Sell pricedb.Currencies
}

// State tracks active operational states thread-safely.
type State struct {
	mu sync.RWMutex
	// IsActive is true when the behavior is enabled.
	IsActive bool
	// Status represents the current state flags.
	Status OverallStatus
	// OldAmount represents the last scanned transaction amounts.
	OldAmount OldAmount
	// OldKeyPrices represents the last scanned key prices.
	OldKeyPrices KeyPrices
}

// AlertProvider defines the interface for sending alerts to administrators.
type AlertProvider interface {
	// MessageAdmins sends an alert message to administrators.
	MessageAdmins(ctx context.Context, message string) error
}

// BackpackProvider defines the subset of inventory methods needed by the autokeys behavior.
type BackpackProvider interface {
	// GetPureStock retrieves the current keys and metal stock.
	GetPureStock() currency.PureStock
	// GetStock retrieves the current stock count for a specific SKU.
	GetStock(sku string) int
	// Cache returns the underlying item cache.
	Cache() backpack.ItemCache
	// Schema returns the current schema provider.
	Schema() backpack.SchemaProvider
}

// PriceProvider defines the subset of price management methods needed by the autokeys behavior.
type PriceProvider interface {
	// GetPrice retrieves the cached price for a given SKU.
	GetPrice(sku string) (*pricedb.Price, bool)
	// SetPrice updates the price and source for a given SKU.
	SetPrice(sku string, buy, sell pricedb.Currencies, source pricedb.PricelistChangedSource)
}

// Autokeys manages key prices dynamically based on inventory balances.
// Use [New] to instantiate and configure.
type Autokeys struct {
	bp            BackpackProvider
	priceMgr      PriceProvider
	logger        log.Logger
	bus           *bus.Bus
	config        Config
	state         *State
	alertProvider AlertProvider
}

// Register returns a [behavior.Option] that registers the [Autokeys] behavior with the orchestrator.
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

// New constructs a new [Autokeys] behavior instance.
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

// Name returns the unique name of the [Autokeys] behavior.
func (a *Autokeys) Name() string {
	return BehaviorName
}

// Run starts the background scanning and price modification loops.
// Returns an error if the context is cancelled.
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

// IsEnabled returns true if the behavior is enabled.
func (a *Autokeys) IsEnabled() bool {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.IsActive
}

// IsActive returns true if the state machine is active in buying or banking keys.
func (a *Autokeys) IsActive() bool {
	a.state.mu.RLock()
	defer a.state.mu.RUnlock()
	return a.state.IsActive && (a.state.Status.IsBuyingKeys || a.state.Status.IsBankingKeys)
}

// GetStatus returns the current operational phase.
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

func (a *Autokeys) scan(ctx context.Context) error {
	stock := a.bp.GetPureStock()
	currKeys := stock.Keys
	currRef := stock.TotalScrap()

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

			weaponsScrap := currency.Scrap(float64(duplicateWeaponsCount) * 0.5)
			currRef += weaponsScrap
		}
	}

	kp, ok := a.priceMgr.GetPrice(currency.SKUKey)
	if !ok || kp.Buy.Metal <= 0 || kp.Sell.Metal <= 0 {
		a.logger.Warn(
			"Base key price unavailable in pricedb, skipping Autokeys cycle",
			log.String("sku", currency.SKUKey),
		)

		return nil
	}

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

	isBuyingKeys := currRef > a.config.MaxRefsScrap && currKeys < a.config.MaxKeys
	isSellingKeys := currRef < a.config.MinRefsScrap && currKeys > a.config.MinKeys
	isBankingKeys := a.config.EnableBanking &&
		currRef >= a.config.MinRefsScrap && currRef <= a.config.MaxRefsScrap &&
		currKeys > a.config.MinKeys
	isBankingBuyKeys := a.config.EnableBanking &&
		currRef > a.config.MinRefsScrap &&
		currKeys <= a.config.MinKeys
	isAlertState := currRef < a.config.MinRefsScrap && currKeys < a.config.MinKeys

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

	if isAlertState {
		if !a.state.Status.LowPureAlertTriggered {
			a.state.Status.LowPureAlertTriggered = true

			msg := fmt.Sprintf(
				"[CRITICAL] Low pure alert triggered! Keys: %d, Metal: %s",
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
				"Pure levels recovered. Keys: %d, Metal: %s",
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

	buyCurrencies := pricedb.Currencies{Keys: 0, Metal: adjBuyRef}
	sellCurrencies := pricedb.Currencies{Keys: 0, Metal: adjSellRef}
	a.priceMgr.SetPrice(currency.SKUKey, buyCurrencies, sellCurrencies, PricelistChangedSourceAutokeys)

	return nil
}
