// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/rep"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// StockConfig defines the inventory limit thresholds for the trading system.
type StockConfig struct {
	// MaxTotal represents the absolute maximum capacity of the bot's inventory across all items.
	MaxTotal int
	// MaxPerSKU maps item SKUs to their specific maximum allowed stock counts.
	MaxPerSKU map[string]int
	// DefaultMax represents the fallback stock limit for items without an explicit entry in MaxPerSKU.
	DefaultMax int
}

// StockLimitMiddleware checks if an incoming trade exceeds total capacity or specific SKU boundaries.
// It reads [StockConfig] limits and cancels the trade with [reason.DeclineOverstocked] if boundaries are violated.
func StockLimitMiddleware(bp *backpack.Backpack, cfg StockConfig, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			if len(ctx.Offer.ItemsToReceive) == 0 {
				return next(ctx)
			}

			currentTotal := bp.GetTotalCount()
			incomingCount := len(ctx.Offer.ItemsToReceive)
			outgoingCount := len(ctx.Offer.ItemsToGive)

			if currentTotal+incomingCount-outgoingCount > cfg.MaxTotal {
				logger.Warn("Trade would exceed total inventory limit",
					log.Int("current", currentTotal),
					log.Int("incoming", incomingCount),
					log.Int("max", cfg.MaxTotal),
				)
				ctx.Decline(reason.ReviewOverstocked)

				return nil
			}

			incomingPerSKU := make(map[string]int)
			for _, it := range ctx.Offer.ItemsToReceive {
				sku := it.SKU
				incomingPerSKU[sku]++
			}

			for sku, count := range incomingPerSKU {
				max, ok := cfg.MaxPerSKU[sku]
				if !ok {
					max = cfg.DefaultMax
				}

				if max <= 0 {
					continue
				}

				currentStock := bp.GetStock(sku)
				if currentStock+count > max {
					logger.Warn("Trade would exceed SKU stock limit",
						log.String("sku", sku),
						log.Int("current", currentStock),
						log.Int("incoming", count),
						log.Int("max", max),
					)
					ctx.Decline(reason.DeclineOverstocked)

					return nil
				}
			}

			return next(ctx)
		}
	}
}

// PriceProvider defines the interface for querying the PriceDB authority.
type PriceProvider interface {
	// GetPrice retrieves the cached price entry for the given SKU, returning false if not found.
	GetPrice(sku string) (*pricedb.Price, bool)
	// Watch registers the given SKU to be included in background price update polling.
	Watch(sku string)
	// Fetch retrieves the latest prices for a slice of SKUs, updating the local cache.
	Fetch(ctx context.Context, skus []string) (map[string]*pricedb.Price, error)
}

// DupeChecker defines the interface for auditing item history to detect duplicated items.
type DupeChecker interface {
	// CheckHistory queries historical tracking databases for the specified asset ID.
	CheckHistory(ctx context.Context, assetID uint64) (backpack.HistoryStatus, error)
}

// ReputationChecker defines the interface for verifying trade partner safety and ban list records.
type ReputationChecker interface {
	// CheckBans audits the specified Steam ID against community ban lists.
	CheckBans(ctx context.Context, partnerID id.ID) (*rep.BanResult, error)
}

// PricerMiddleware enriches the trade context with current item pricing models retrieved from a [PriceProvider].
// It resolves prices, updates watches, and halts evaluation with [tf2reason.ReviewUnpricedItem] if any item is unpriced.
func PricerMiddleware(mgr PriceProvider, schemaProvider func() *schema.Schema, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			ctx.Set("schema", schemaProvider())

			skus := make(map[string]bool)
			for _, item := range append(ctx.Offer.ItemsToGive, ctx.Offer.ItemsToReceive...) {
				pricingSKU := GetPricingSKU(item.SKU)
				skus[pricingSKU] = true
			}

			skuList := make([]string, 0)
			priceMap := make(map[string]*pricedb.Price)

			for sku := range skus {
				if p, ok := mgr.GetPrice(sku); ok {
					priceMap[sku] = p
				} else {
					skuList = append(skuList, sku)
					mgr.Watch(sku)
				}
			}

			if len(skuList) > 0 {
				fetched, err := mgr.Fetch(ctx, skuList)
				if err != nil {
					logger.Warn("Failed to fetch prices from PriceDB", log.Err(err))
					ctx.Review(tf2reason.ReviewPricerDown)
					return err
				}

				maps.Copy(priceMap, fetched)
			}

			// Fallback for painted items: if a painted item is not in priceMap,
			// try to resolve the base SKU and map the base item's price to the painted SKU.
			for _, item := range append(ctx.Offer.ItemsToGive, ctx.Offer.ItemsToReceive...) {
				pricingSKU := GetPricingSKU(item.SKU)
				if _, ok := priceMap[pricingSKU]; !ok {
					if itObj, err := sku.FromString(pricingSKU); err == nil && itObj.Paint != 0 {
						itObj.Paint = 0

						baseSKU := sku.FromObject(itObj)
						if basePrice, ok := mgr.GetPrice(baseSKU); ok {
							priceMap[pricingSKU] = &pricedb.Price{
								SKU:    pricingSKU,
								Name:   basePrice.Name + " (Painted)",
								Buy:    basePrice.Buy,
								Sell:   basePrice.Sell,
								Source: basePrice.Source,
								Time:   basePrice.Time,
							}
							logger.Info("Using base item price as fallback for painted item",
								log.String("painted_sku", pricingSKU),
								log.String("base_sku", baseSKU),
							)
						}
					}
				}
			}

			ctx.Set("prices", priceMap)

			for _, item := range append(ctx.Offer.ItemsToGive, ctx.Offer.ItemsToReceive...) {
				pricingSKU := GetPricingSKU(item.SKU)
				if _, ok := priceMap[pricingSKU]; !ok {
					if isUniqueWeapon(item.SKU, schemaProvider()) {
						continue
					}

					logger.Warn("Item in trade is not priced", log.String("sku", item.SKU))
					ctx.Review(tf2reason.ReviewUnpricedItem)

					return errors.New("unpriced item in trade")
				}
			}

			return next(ctx)
		}
	}
}

// EscrowMiddleware checks whether either trade partner is subject to Steam trade hold restrictions.
// It halts trade evaluation with [reason.DeclineEscrow] if active escrow holds are detected.
func EscrowMiddleware(checker trading.EscrowChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			hasEscrow, err := checker.CheckEscrow(ctx, ctx.Offer)
			if err != nil {
				logger.Warn("Failed to check escrow", log.Err(err))
				ctx.Review(reason.ReviewEscrowCheckFailed)

				return nil
			}

			if hasEscrow {
				logger.Warn("Trade has escrow (trade hold)", log.Uint64("offerID", ctx.Offer.ID))
				ctx.Decline(reason.DeclineEscrow)
				return nil
			}

			return next(ctx)
		}
	}
}

// DupeCheckMiddleware audits the historical records of incoming high-value Unusual items.
// It sets the trade context to a review state with [tf2reason.ReviewDupedItems] if duplicates are found.
func DupeCheckMiddleware(checker DupeChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			for _, item := range ctx.Offer.ItemsToReceive {
				if item.SKU == "" {
					continue
				}

				if isUnusual(item.SKU) {
					logger.Info(
						"Checking history for Unusual item",
						log.String("sku", item.SKU),
						log.Uint64("assetid", item.AssetID),
					)

					status, err := checker.CheckHistory(ctx, item.AssetID)
					if err != nil {
						logger.Warn("Failed to check item history", log.Err(err))
						continue
					}

					if status.Recorded && status.IsDuped {
						logger.Warn("Item is DUPED!", log.Uint64("assetid", item.AssetID))
						ctx.Review(tf2reason.ReviewDupedItems)
					}
				}
			}

			return next(ctx)
		}
	}
}

// BanCheckMiddleware audits the trade partner's reputation using [ReputationChecker].
// It declines trades with [reason.DeclineBanned] or [tf2reason.DeclineBannedBptf] if active bans are found.
func BanCheckMiddleware(bans ReputationChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			res, err := bans.CheckBans(ctx, ctx.Offer.OtherSteamID)
			if err != nil {
				logger.Warn("Failed to check partner bans", log.Err(err))
				return next(ctx)
			}

			if res.IsBanned {
				logger.Warn("Partner is banned!",
					log.String("steamid", ctx.Offer.OtherSteamID.String()),
					log.Any("details", res.Details),
				)

				if _, ok := res.Details["steamrep.com"]; ok {
					ctx.Decline(reason.DeclineBanned)
				} else {
					ctx.Decline(tf2reason.DeclineBannedBptf)
				}

				return nil
			}

			return next(ctx)
		}
	}
}

// SpellPredictor defines the subset of pricedb methods needed for Halloween spell price predictions.
type SpellPredictor interface {
	PredictSpellPrice(ctx context.Context, spells, item string) (*pricedb.SpellPredictionResponse, error)
}

// HalloweenSpellMiddleware computes spell price premiums on spelled weapons and injects them into the trade value.
func HalloweenSpellMiddleware(
	predictor SpellPredictor,
	schemaProvider func() *schema.Schema,
	configProvider func() Config,
	logger log.Logger,
) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			pricesRaw, ok := ctx.Get("prices")
			if !ok {
				return next(ctx)
			}

			priceMap, ok := pricesRaw.(map[string]*pricedb.Price)
			if !ok {
				return next(ctx)
			}

			ourSpellPremium, _ := computePremium(
				ctx,
				ctx.Offer.ItemsToGive,
				priceMap,
				schemaProvider,
				configProvider,
				logger,
				predictor,
			)
			theirSpellPremium, _ := computePremium(
				ctx,
				ctx.Offer.ItemsToReceive,
				priceMap,
				schemaProvider,
				configProvider,
				logger,
				predictor,
			)

			ctx.Set("our_spell_premium_scrap", ourSpellPremium)
			ctx.Set("their_spell_premium_scrap", theirSpellPremium)

			return next(ctx)
		}
	}
}

func computePremium(
	ctx context.Context,
	items []*trading.Item,
	priceMap map[string]*pricedb.Price,
	schemaProvider func() *schema.Schema,
	configProvider func() Config,
	logger log.Logger,
	predictor SpellPredictor,
) (currency.Scrap, error) {
	var totalPremium currency.Scrap
	for _, item := range items {
		pricingSKU := GetPricingSKU(item.SKU)

		p, hasPrice := priceMap[pricingSKU]
		if !hasPrice {
			continue
		}

		var spells []sku.Spell
		for _, desc := range item.Descriptions {
			if strings.EqualFold(desc.Color, "7ea9d1") {
				if spell, ok := schema.IdentifySpell(desc.Value); ok {
					spells = append(spells, spell)
				}
			}
		}

		if len(spells) == 0 {
			continue
		}

		sh := schemaProvider()
		if sh == nil {
			logger.Warn("Schema is not ready, skipping spell premium calculation for item", log.String("sku", item.SKU))
			continue
		}

		var spellNames []string
		for _, s := range spells {
			name := sh.SpellNameFromSKU(s)
			if name != "" && !strings.Contains(name, "Unknown Spell") {
				spellNames = append(spellNames, name)
			}
		}

		if len(spellNames) == 0 {
			continue
		}

		spellsQuery := strings.Join(spellNames, ",")
		logger.Debug("Predicting spell premium for item", log.String("item", p.Name), log.String("spells", spellsQuery))

		var (
			premiumRef      float64
			resolvedFromAPI bool
		)

		prediction, err := predictor.PredictSpellPrice(ctx, spellsQuery, p.Name)
		if err == nil && prediction != nil {
			if premium, ok := prediction.PremiumRanges["mid"]; ok {
				premiumRef = premium.Ref
				resolvedFromAPI = true
			}
		}

		if !resolvedFromAPI {
			logger.Warn("pricedb prediction failed or empty, falling back to static spell premiums",
				log.String("item", p.Name),
				log.String("spells", spellsQuery),
				log.Err(err),
			)

			cfg := configProvider()

			var staticTotal float64
			for _, sName := range spellNames {
				var (
					matchedVal float64
					found      bool
				)

				for k, v := range cfg.FallbackSpellPremiums {
					if strings.EqualFold(k, sName) || strings.EqualFold(strings.TrimPrefix(k, "Halloween: "), sName) {
						matchedVal = v
						found = true
						break
					}
				}

				if !found {
					matchedVal = 2.0 // 2 ref default fallback

					logger.Warn(
						"No custom fallback price configured for spell, using default 2.0 ref",
						log.String("spell", sName),
					)
				}

				staticTotal += matchedVal
			}

			premiumRef = staticTotal
		}

		premiumScrap := currency.ToScrap(premiumRef)
		totalPremium += premiumScrap

		if resolvedFromAPI {
			logger.Info("Applied spell premium for item (API)",
				log.String("item", p.Name),
				log.String("spells", spellsQuery),
				log.Float64("premium_ref", premiumRef),
			)
		} else {
			logger.Info("Applied spell premium for item (Fallback)",
				log.String("item", p.Name),
				log.String("spells", spellsQuery),
				log.Float64("premium_ref", premiumRef),
			)
		}
	}

	return totalPremium, nil
}

// SmartCounterMiddleware calculates transaction value balances and automatically adjusts mismatches.
// If overpaid, it appends change metal from local inventory using [crafting.MetalManager].
// If underpaid, it scans partner inventory using [trading.PartnerInventoryProvider] to request missing change.
func SmartCounterMiddleware(
	cfgManager *ConfigManager,
	metalMgr *crafting.MetalManager,
	bp *backpack.Backpack,
	invProvider trading.PartnerInventoryProvider,
	logger log.Logger,
) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			if err := next(ctx); err != nil {
				return err
			}

			if ctx.Verdict.Action != trading.ActionSkip {
				return nil
			}

			useSeparateKeyRates := false
			if cfgManager != nil {
				useSeparateKeyRates = cfgManager.GetConfig().UseSeparateKeyRates
			}

			diff, err := calculateValueDiff(ctx, useSeparateKeyRates)
			if err != nil {
				return nil //nolint:nilerr
			}

			if diff == 0 {
				ctx.Accept(reason.AcceptCorrectValue)
				return nil
			}

			if diff > 0 {
				changeIDs, err := metalMgr.SelectChange(diff)
				if err != nil {
					if errors.Is(err, crafting.ErrNotEnoughChange) {
						logger.Warn("Not enough metal for change, triggering auto-crafting...")

						if smeltErr := metalMgr.TryToSmeltForChange(ctx, diff); smeltErr == nil {
							return nil
						}

						ctx.Decline(tf2reason.DeclineNoChange)

						return nil
					}

					return err
				}

				ctx.Counter(reason.AcceptCorrectValue, &trading.CounterParams{
					ItemsToGive:    append(ctx.Offer.ItemsToGive, mapIDsToItems(bp, changeIDs)...),
					ItemsToReceive: ctx.Offer.ItemsToReceive,
					Message:        "I've added the necessary change for you!",
				})
			} else if diff < 0 {
				partnerInv, err := invProvider.GetPartnerInventory(ctx, ctx.Offer.OtherSteamID)
				if err != nil {
					logger.Warn("Failed to fetch partner inventory for smart countering", log.Err(err))
					ctx.Review(reason.ReviewPartnerInventoryFetchFailed)
					return nil
				}

				keyPriceVar, _ := ctx.Get("key_price_scrap")
				keyPrice, _ := keyPriceVar.(currency.Scrap)

				needed := -diff

				var sch *schema.Schema
				if val, ok := ctx.Get("schema"); ok {
					if s, ok := val.(*schema.Schema); ok {
						sch = s
					}
				}

				toAdd, ok := FindPartnerCurrency(partnerInv, needed, keyPrice, sch)
				if ok {
					logger.Info("Smart countering: found missing currency in partner inventory",
						log.Int("needed_scrap", int(needed)),
						log.Int("found_items", len(toAdd)),
					)

					ctx.Counter(reason.AcceptCorrectValue, &trading.CounterParams{
						ItemsToGive:    ctx.Offer.ItemsToGive,
						ItemsToReceive: append(ctx.Offer.ItemsToReceive, toAdd...),
						Message:        "You were missing some change, I've added it for you!",
					})
				} else {
					ctx.Decline(tf2reason.DeclineUnderpaid)
				}
			}

			return nil
		}
	}
}

// PPUMiddleware executes Price Protection Unit (PPU) calculations to lock pricing during price crashes.
// It maps cost basis logs from [storage.CostBasisStore] and dynamically caps buy and sell rates inside [engine.TradeContext].
func PPUMiddleware(
	bp *backpack.Backpack,
	cbStore storage.CostBasisStore,
	cfgManager *ConfigManager,
	logger log.Logger,
) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			pricesRaw, ok := ctx.Get("prices")
			if !ok {
				return next(ctx)
			}

			priceMap, ok := pricesRaw.(map[string]*pricedb.Price)
			if !ok {
				return next(ctx)
			}

			var keyPriceRef float64
			if kp, ok := priceMap[currency.SKUKey]; ok {
				keyPriceRef = kp.Buy.Metal
			}

			if keyPriceRef <= 0 {
				keyPriceRef = 50.0
			}

			keyPriceScrap := currency.ToScrap(keyPriceRef)

			cfg := cfgManager.GetConfig()
			holdDuration := cfg.GetPPUHoldDuration()
			gracePeriod := cfg.GetPPUGracePeriod()
			maxStockLimit := cfg.PPUMaxStockLimit
			minProfitScrap := cfg.PPUMinProfitScrap

			uniqueSKUs := make(map[string]bool)
			for _, item := range append(ctx.Offer.ItemsToGive, ctx.Offer.ItemsToReceive...) {
				pricingSKU := GetPricingSKU(item.SKU)
				if pricingSKU != "" &&
					pricingSKU != currency.SKUKey &&
					pricingSKU != currency.SKURefined &&
					pricingSKU != currency.SKUReclaimed &&
					pricingSKU != currency.SKUScrap {
					uniqueSKUs[pricingSKU] = true
				}
			}

			for sku := range uniqueSKUs {
				stockCount := bp.GetStock(sku)

				state, exists := cbStore.GetPPUState(sku)
				if !exists {
					state = storage.PPUState{SKU: sku}
				}

				if stockCount > 0 && state.LastInStockTime.IsZero() {
					state.LastInStockTime = time.Now()
					cbStore.SetPPUState(sku, state)
					logger.Info("Stock self-diagnosis initialized timer", log.String("sku", sku))
				}
			}

			cbStore.Prune(holdDuration)

			for sku := range uniqueSKUs {
				p, hasPrice := priceMap[sku]
				if !hasPrice {
					continue
				}

				state, exists := cbStore.GetPPUState(sku)
				if !exists {
					state = storage.PPUState{SKU: sku}
				}

				if slices.Contains(cfg.PPUExcludeSKUs, sku) {
					if state.IsPartialPriced {
						state.IsPartialPriced = false
						state.ProtectionStarted = time.Time{}
						cbStore.SetPPUState(sku, state)
						logger.Info("PPU price protection deactivated because SKU was excluded", log.String("sku", sku))
					}

					continue
				}

				stockCount := bp.GetStock(sku)

				isProtectedStock := false
				if stockCount > 0 {
					switch {
					case cfg.PPURemoveMaxRestriction || cfg.PPUMaxProtectedUnits == -1:
						isProtectedStock = true
					case cfg.PPUMaxProtectedUnits > 0:
						isProtectedStock = stockCount <= cfg.PPUMaxProtectedUnits
					default:
						isProtectedStock = stockCount <= maxStockLimit
					}
				}

				inStock := isProtectedStock
				inGrace := stockCount == 0 &&
					!state.LastSoldTime.IsZero() &&
					time.Since(state.LastSoldTime) < gracePeriod

				if !inStock && !inGrace {
					if state.IsPartialPriced {
						state.IsPartialPriced = false
						state.ProtectionStarted = time.Time{}
						cbStore.SetPPUState(sku, state)
						logger.Info("PPU price protection deactivated due to stock limits", log.String("sku", sku))
					}

					continue
				}

				entry, hasEntry := cbStore.GetOldestEntry(sku)
				if !hasEntry {
					continue
				}

				baseBuyScrap := currency.Scrap(entry.BuyKeys)*keyPriceScrap + currency.ToScrap(entry.BuyMetal)
				netCostBasisScrap := baseBuyScrap + currency.Scrap(entry.Diff)

				protectedSellPriceScrap := netCostBasisScrap + currency.Scrap(minProfitScrap)
				marketSellPriceScrap := currency.Scrap(p.Sell.Keys)*keyPriceScrap + currency.ToScrap(p.Sell.Metal)

				if marketSellPriceScrap < protectedSellPriceScrap {
					protectedCurrencies := currency.ScrapToCurrencies(
						currency.Scrap(protectedSellPriceScrap),
						keyPriceRef,
					)
					p.Sell.Keys = int(protectedCurrencies.Keys)
					p.Sell.Metal = protectedCurrencies.Metal

					marketBuyPriceScrap := currency.Scrap(p.Buy.Keys)*keyPriceScrap + currency.ToScrap(p.Buy.Metal)
					if marketBuyPriceScrap > netCostBasisScrap {
						cappedBuyCurrencies := currency.ScrapToCurrencies(
							currency.Scrap(netCostBasisScrap),
							keyPriceRef,
						)
						p.Buy.Keys = int(cappedBuyCurrencies.Keys)
						p.Buy.Metal = cappedBuyCurrencies.Metal
					}

					if !state.IsPartialPriced {
						state.IsPartialPriced = true
						state.ProtectionStarted = time.Now()
						cbStore.SetPPUState(sku, state)

						logger.Info("PPU price protection activated",
							log.String("sku", sku),
							log.Int("stock", stockCount),
							log.Float64("protected_sell_ref", currency.ToRefined(protectedSellPriceScrap)),
							log.Float64("net_cost_basis_ref", currency.ToRefined(netCostBasisScrap)),
						)
					}
				} else if state.IsPartialPriced {
					state.IsPartialPriced = false
					state.ProtectionStarted = time.Time{}
					cbStore.SetPPUState(sku, state)
					logger.Info("PPU price protection deactivated, market recovered", log.String("sku", sku))
				}
			}

			ctx.Set("prices", priceMap)

			return next(ctx)
		}
	}
}

// FindPartnerCurrency searches partner items to assemble a combination of currencies covering the specified scrap debt.
// Returns false if the partner's inventory cannot satisfy the required scrap value.
func FindPartnerCurrency(
	items []*trading.Item,
	needed, keyPrice currency.Scrap,
	sch *schema.Schema,
) ([]*trading.Item, bool) {
	var (
		keys      []*trading.Item
		refined   []*trading.Item
		reclaimed []*trading.Item
		scrap     []*trading.Item
		weapons   []*trading.Item
	)

	for _, it := range items {
		switch it.MarketHashName {
		case "Mann Co. Supply Crate Key":
			keys = append(keys, it)
		case "Refined Metal":
			refined = append(refined, it)
		case "Reclaimed Metal":
			reclaimed = append(reclaimed, it)
		case "Scrap Metal":
			scrap = append(scrap, it)
		default:
			if sch != nil && isUniqueWeapon(it.SKU, sch) {
				weapons = append(weapons, it)
			}
		}
	}

	var result []*trading.Item

	remaining := needed

	if keyPrice > 0 {
		for len(keys) > 0 && remaining >= keyPrice {
			result = append(result, keys[0])
			keys = keys[1:]
			remaining -= keyPrice
		}
	}

	for len(refined) > 0 && remaining >= currency.ScrapInRef {
		result = append(result, refined[0])
		refined = refined[1:]
		remaining -= currency.ScrapInRef
	}

	for len(reclaimed) > 0 && remaining >= currency.ScrapInRec {
		result = append(result, reclaimed[0])
		reclaimed = reclaimed[1:]
		remaining -= currency.ScrapInRec
	}

	for len(scrap) > 0 && remaining >= 1 {
		result = append(result, scrap[0])
		scrap = scrap[1:]
		remaining -= 1
	}

	for len(weapons) >= 2 && remaining >= 1 {
		result = append(result, weapons[0], weapons[1])
		weapons = weapons[2:]
		remaining -= 1
	}

	return result, remaining == 0
}

func mapIDsToItems(bp *backpack.Backpack, ids []uint64) []*trading.Item {
	var items []*trading.Item
	for _, id := range ids {
		if it, ok := bp.GetItem(id); ok {
			items = append(items, it.ToEconItem())
		}
	}

	return items
}

func isUnusual(target string) bool {
	it, err := sku.FromString(target)
	if err != nil {
		return false
	}

	return it.Quality == 5
}

// GetPricingSKU normalizes the specified SKU string by stripping transient flags such as Festivized.
// Returns the unmodified SKU string if parsing fails.
func GetPricingSKU(skuStr string) string {
	it, err := sku.FromString(skuStr)
	if err != nil {
		return skuStr
	}

	it.Festivized = false

	return sku.FromObject(it)
}

// calculateValueDiff calculates the difference in value between what we receive and what we give.
// Result > 0: We were overpaid (need change).
// Result < 0: We were underpaid (we should reject or request more).
func calculateValueDiff(ctx *engine.TradeContext, useSeparateKeyRates bool) (currency.Scrap, error) {
	pricesRaw, ok := ctx.Get("prices")
	if !ok {
		return 0, errors.New("prices not found in context")
	}

	priceMap := pricesRaw.(map[string]*pricedb.Price)

	var keyBuyPriceScrap, keySellPriceScrap currency.Scrap
	if keyPrice, ok := priceMap[currency.SKUKey]; ok {
		keyBuyPriceScrap = currency.ToScrap(keyPrice.Buy.Metal)
		keySellPriceScrap = currency.ToScrap(keyPrice.Sell.Metal)
	}

	if keyBuyPriceScrap <= 0 {
		keyBuyPriceScrap = currency.ToScrap(50.0)
	}

	if keySellPriceScrap <= 0 {
		keySellPriceScrap = currency.ToScrap(50.0)
	}

	ctx.Set("key_price_scrap", keyBuyPriceScrap)

	var sch *schema.Schema
	if val, ok := ctx.Get("schema"); ok {
		if s, ok := val.(*schema.Schema); ok {
			sch = s
		}
	}

	var ourTotalScrapVal, theirTotalScrapVal float64

	var ourSpellPremium, theirSpellPremium currency.Scrap
	if val, ok := ctx.Get("our_spell_premium_scrap"); ok {
		ourSpellPremium = val.(currency.Scrap)
	}

	if val, ok := ctx.Get("their_spell_premium_scrap"); ok {
		theirSpellPremium = val.(currency.Scrap)
	}

	ourTotalScrapVal += float64(ourSpellPremium)
	theirTotalScrapVal += float64(theirSpellPremium)

	for _, item := range ctx.Offer.ItemsToGive {
		pricingSKU := GetPricingSKU(item.SKU)

		p, ok := priceMap[pricingSKU]
		if !ok {
			if isUniqueWeapon(item.SKU, sch) {
				ourTotalScrapVal += 0.5
				continue
			}

			ctx.Review(tf2reason.ReviewUnpricedItem)

			return 0, fmt.Errorf("unpriced item in 'give' side: %s", item.SKU)
		}

		keyRate := keyBuyPriceScrap
		if useSeparateKeyRates {
			keyRate = keySellPriceScrap
		}

		val := currency.Scrap(p.Sell.Keys)*keyRate + currency.ToScrap(p.Sell.Metal)
		ourTotalScrapVal += float64(val)
	}

	for _, item := range ctx.Offer.ItemsToReceive {
		pricingSKU := GetPricingSKU(item.SKU)

		p, ok := priceMap[pricingSKU]
		if !ok {
			if isUniqueWeapon(item.SKU, sch) {
				theirTotalScrapVal += 0.5
				continue
			}

			ctx.Review(tf2reason.ReviewUnpricedItem)

			return 0, fmt.Errorf("unpriced item in 'receive' side: %s", item.SKU)
		}

		keyRate := keyBuyPriceScrap
		val := currency.Scrap(p.Buy.Keys)*keyRate + currency.ToScrap(p.Buy.Metal)
		theirTotalScrapVal += float64(val)
	}

	// Overpaying is accepted, underpaying by even 0.5 scrap is rejected
	diffVal := theirTotalScrapVal - ourTotalScrapVal
	diffScrap := currency.Scrap(math.Floor(diffVal))

	ctx.Set("value_diff_scrap", diffScrap)
	ctx.Set("is_profitable", diffScrap >= 0)

	return diffScrap, nil
}

// isUniqueWeapon returns true if the SKU represents a standard craftable Unique weapon.
func isUniqueWeapon(skuStr string, s *schema.Schema) bool {
	if s == nil {
		return false
	}

	item, err := sku.FromString(skuStr)
	if err != nil {
		return false
	}

	if item.Quality != schema.QualityUnique {
		return false
	}

	if !item.Craftable || !item.Tradable {
		return false
	}

	if item.Effect != 0 ||
		item.Killstreak != 0 ||
		item.Festivized ||
		item.Australium ||
		item.Paintkit != 0 ||
		item.Wear != 0 ||
		item.Quality2 != 0 ||
		item.Crateseries != 0 ||
		item.Craftnumber != 0 {
		return false
	}

	sch := s.ItemByDef(item.Defindex)

	return sch != nil &&
		(sch.CraftClass == "weapon" || sch.ItemClass == "weapon" || strings.HasPrefix(sch.ItemClass, "tf_weapon_"))
}
