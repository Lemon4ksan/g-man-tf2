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
	"strings"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
	"github.com/lemon4ksan/foundation/async/log"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/rep"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

var (
	ErrUnpricedItemInTrade = errors.New("unpriced item in trade")
	ErrPricesNotFound      = errors.New("prices not found in context")
)

type StockConfig struct {
	MaxTotal   int
	MaxPerSKU  map[string]int
	DefaultMax int
}

type BackpackProvider interface {
	GetTotalCount() int
	GetStock(sku string) int
	GetItem(id uint64) (*tf2.Item, bool)
}

func StockLimitMiddleware(bp BackpackProvider, cfg StockConfig, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			if len(ctx.Offer.ItemsToReceive) == 0 {
				return next(ctx)
			}

			if bp.GetTotalCount()+len(ctx.Offer.ItemsToReceive)-len(ctx.Offer.ItemsToGive) > cfg.MaxTotal {
				ctx.Decline(reason.ReviewOverstocked)
				return nil
			}

			incomingPerSKU := make(map[string]int, len(ctx.Offer.ItemsToReceive))
			for _, it := range ctx.Offer.ItemsToReceive {
				incomingPerSKU[it.SKU]++
			}

			for skuStr, count := range incomingPerSKU {
				maxStock, ok := cfg.MaxPerSKU[skuStr]
				if !ok {
					maxStock = cfg.DefaultMax
				}

				if maxStock > 0 && bp.GetStock(skuStr)+count > maxStock {
					ctx.Decline(reason.DeclineOverstocked)
					return nil
				}
			}

			return next(ctx)
		}
	}
}

type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
	Watch(sku string)
	Fetch(ctx context.Context, skus []string) (map[string]*pricedb.Price, error)
}

type DupeChecker interface {
	CheckHistory(ctx context.Context, assetID uint64, mods ...aoni.RequestModifier) (backpack.HistoryStatus, error)
}

type ReputationChecker interface {
	CheckBans(ctx context.Context, partnerID id.ID) (*rep.BanResult, error)
}

func PricerMiddleware(mgr PriceProvider, schemaProvider func() *schema.Schema, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			ctx.Set("schema", schemaProvider())

			totalItems := len(ctx.Offer.ItemsToGive) + len(ctx.Offer.ItemsToReceive)
			skus := make(map[string]bool, totalItems)

			for _, item := range ctx.Offer.ItemsToGive {
				skus[sku.ToPricingSKU(item.SKU)] = true
			}

			for _, item := range ctx.Offer.ItemsToReceive {
				skus[sku.ToPricingSKU(item.SKU)] = true
			}

			skuList := make([]string, 0, len(skus))
			priceMap := make(map[string]*pricedb.Price, len(skus))

			for skuStr := range skus {
				if p, ok := mgr.GetPrice(skuStr); ok {
					priceMap[skuStr] = p
				} else {
					skuList = append(skuList, skuStr)
					mgr.Watch(skuStr)
				}
			}

			if len(skuList) > 0 {
				fetched, err := mgr.Fetch(ctx, skuList)
				if err != nil {
					ctx.Review(tf2reason.ReviewPricerDown)
					return err
				}

				maps.Copy(priceMap, fetched)
			}

			for _, item := range ctx.Offer.ItemsToGive {
				enrichPaintedFallback(item.SKU, priceMap, mgr, logger)
			}

			for _, item := range ctx.Offer.ItemsToReceive {
				enrichPaintedFallback(item.SKU, priceMap, mgr, logger)
			}

			ctx.Set("prices", priceMap)

			for _, item := range ctx.Offer.ItemsToGive {
				if err := verifyItemPriced(item, priceMap, schemaProvider, logger, ctx); err != nil {
					return err
				}
			}

			for _, item := range ctx.Offer.ItemsToReceive {
				if err := verifyItemPriced(item, priceMap, schemaProvider, logger, ctx); err != nil {
					return err
				}
			}

			return next(ctx)
		}
	}
}

func enrichPaintedFallback(itemSKU string, priceMap map[string]*pricedb.Price, mgr PriceProvider, logger log.Logger) {
	pricingSKU := sku.ToPricingSKU(itemSKU)
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
			}
		}
	}
}

func verifyItemPriced(
	item *trading.Item,
	priceMap map[string]*pricedb.Price,
	schemaProvider func() *schema.Schema,
	logger log.Logger,
	ctx *engine.TradeContext,
) error {
	pricingSKU := sku.ToPricingSKU(item.SKU)
	if _, ok := priceMap[pricingSKU]; !ok {
		if isUniqueWeapon(item.SKU, schemaProvider()) {
			return nil
		}

		ctx.Review(tf2reason.ReviewUnpricedItem)

		return ErrUnpricedItemInTrade
	}

	return nil
}

func EscrowMiddleware(checker trading.EscrowChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			hasEscrow, err := checker.CheckEscrow(ctx, ctx.Offer)
			if err != nil {
				ctx.Review(reason.ReviewEscrowCheckFailed)
				return nil //nolint:nilerr
			}

			if hasEscrow {
				ctx.Decline(reason.DeclineEscrow)
				return nil
			}

			return next(ctx)
		}
	}
}

func DupeCheckMiddleware(checker DupeChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			for _, item := range ctx.Offer.ItemsToReceive {
				if item.SKU != "" && isUnusual(item.SKU) {
					status, err := checker.CheckHistory(ctx, item.AssetID)
					if err == nil && status.Recorded && status.IsDuped {
						ctx.Review(tf2reason.ReviewDupedItems)
					}
				}
			}

			return next(ctx)
		}
	}
}

func BanCheckMiddleware(bans ReputationChecker, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			res, err := bans.CheckBans(ctx, ctx.Offer.OtherSteamID)
			if err != nil {
				return next(ctx)
			}

			if res.IsBanned {
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

type MetalChangeManager interface {
	SelectChange(amount currency.Scrap) ([]uint64, error)
	TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error
}

func SmartCounterMiddleware(
	cfgManager *ConfigManager,
	metalMgr MetalChangeManager,
	bp BackpackProvider,
	invProvider trading.PartnerInventoryProvider,
	logger log.Logger,
) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			if err := next(ctx); err != nil {
				return err
			}

			if ctx.Verdict.Action == trading.ActionDecline &&
				ctx.Verdict.Reason != tf2reason.DeclineUnderpaid &&
				ctx.Verdict.Reason != tf2reason.ReviewInvalidValue {
				return nil
			}

			useSeparateKeyRates := false
			if cfgManager != nil {
				useSeparateKeyRates = cfgManager.GetConfig().UseSeparateKeyRates
			}

			diff, err := calculateValueDiff(ctx, useSeparateKeyRates)
			if err != nil {
				ctx.Review(tf2reason.ReviewInvalidValue)
				return err
			}

			if diff == 0 {
				ctx.Accept(reason.AcceptCorrectValue)
				return nil
			}

			if diff > 0 {
				return handlePositiveDiffCounter(ctx, metalMgr, bp, diff)
			}

			return handleNegativeDiffCounter(ctx, invProvider, diff)
		}
	}
}

func handlePositiveDiffCounter(
	ctx *engine.TradeContext,
	metalMgr MetalChangeManager,
	bp BackpackProvider,
	diff currency.Scrap,
) error {
	changeIDs, err := metalMgr.SelectChange(diff)
	if err != nil {
		if errors.Is(err, crafting.ErrNotEnoughChange) {
			if smeltErr := metalMgr.TryToSmeltForChange(ctx, diff); smeltErr == nil {
				ctx.Decline(tf2reason.DeclineNoChange)
				return nil
			}
		}

		ctx.Decline(tf2reason.DeclineNoChange)

		return nil
	}

	ctx.Counter(reason.AcceptCorrectValue, &trading.CounterParams{
		ItemsToGive:    append(ctx.Offer.ItemsToGive, mapIDsToItems(bp, changeIDs)...),
		ItemsToReceive: ctx.Offer.ItemsToReceive,
		Message:        "I've added the necessary change for you!",
	})

	return nil
}

func handleNegativeDiffCounter(
	ctx *engine.TradeContext,
	invProvider trading.PartnerInventoryProvider,
	diff currency.Scrap,
) error {
	partnerInv, err := invProvider.GetPartnerInventory(ctx, ctx.Offer.OtherSteamID)
	if err != nil {
		ctx.Review(reason.ReviewPartnerInventoryFetchFailed)
		return nil //nolint:nilerr
	}

	keyPriceVar, _ := ctx.Get("key_price_scrap").Value()
	keyPrice, _ := keyPriceVar.(currency.Scrap)

	var sch *schema.Schema
	if val, ok := ctx.Get("schema").Value(); ok {
		if s, ok := val.(*schema.Schema); ok {
			sch = s
		}
	}

	toAdd, ok := FindPartnerCurrency(partnerInv, -diff, keyPrice, sch)
	if ok {
		ctx.Counter(reason.AcceptCorrectValue, &trading.CounterParams{
			ItemsToGive:    ctx.Offer.ItemsToGive,
			ItemsToReceive: append(ctx.Offer.ItemsToReceive, toAdd...),
			Message:        "You were missing some change, I've added it for you!",
		})
	} else {
		ctx.Decline(tf2reason.DeclineUnderpaid)
	}

	return nil
}

func FindPartnerCurrency(
	items []*trading.Item,
	needed, keyPrice currency.Scrap,
	sch *schema.Schema,
) ([]*trading.Item, bool) {
	n := len(items)

	keys := make([]*trading.Item, 0, n)
	refined := make([]*trading.Item, 0, n)
	reclaimed := make([]*trading.Item, 0, n)
	scrap := make([]*trading.Item, 0, n)
	weapons := make([]*trading.Item, 0, n)

	for _, it := range items {
		switch it.SKU {
		case currency.SKUKey:
			keys = append(keys, it)
		case currency.SKURefined:
			refined = append(refined, it)
		case currency.SKUReclaimed:
			reclaimed = append(reclaimed, it)
		case currency.SKUScrap:
			scrap = append(scrap, it)
		default:
			if sch != nil && isUniqueWeapon(it.SKU, sch) {
				weapons = append(weapons, it)
			}
		}
	}

	result := make([]*trading.Item, 0, 16)
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

func mapIDsToItems(bp BackpackProvider, ids []uint64) []*trading.Item {
	items := make([]*trading.Item, 0, len(ids))

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

func calculateValueDiff(ctx *engine.TradeContext, useSeparateKeyRates bool) (currency.Scrap, error) {
	pricesRaw, ok := ctx.Get("prices").Value()
	if !ok {
		return 0, ErrPricesNotFound
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
	if val, ok := ctx.Get("schema").Value(); ok {
		if s, ok := val.(*schema.Schema); ok {
			sch = s
		}
	}

	var ourTotalScrapVal, theirTotalScrapVal float64

	for _, item := range ctx.Offer.ItemsToGive {
		pricingSKU := sku.ToPricingSKU(item.SKU)
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
		pricingSKU := sku.ToPricingSKU(item.SKU)
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

	diffVal := theirTotalScrapVal - ourTotalScrapVal
	diffScrap := currency.Scrap(math.Floor(diffVal))

	ctx.Set("value_diff_scrap", diffScrap)
	ctx.Set("is_profitable", diffScrap >= 0)

	return diffScrap, nil
}

func isUniqueWeapon(skuStr string, s *schema.Schema) bool {
	if s == nil {
		return false
	}

	item, err := sku.FromString(skuStr)
	if err != nil {
		return false
	}
	defer sku.ReleaseItem(item)

	if item.Quality != schema.QualityUnique || !item.Craftable || !item.Tradable {
		return false
	}

	if item.Effect != 0 || item.Killstreak != 0 || item.Festivized || item.Australium ||
		item.Paintkit != 0 || item.Wear != 0 || item.Quality2 != 0 || item.Crateseries != 0 || item.Craftnumber != 0 {
		return false
	}

	sch := s.ItemByDef(item.Defindex)

	return sch != nil &&
		(sch.CraftClass == "weapon" || sch.ItemClass == "weapon" || strings.HasPrefix(sch.ItemClass, "tf_weapon_"))
}

func IsJunk(it *trading.Item) bool {
	if it == nil || it.SKU == "" {
		return true
	}

	if HasSpells(it) {
		return false
	}

	for _, attr := range it.Attributes {
		if attr.Defindex == schema.AttrCrateSeries {
			return true
		}
	}

	return false
}

func HasSpells(it *trading.Item) bool {
	if it == nil {
		return false
	}

	for _, attr := range it.Attributes {
		if attr.Defindex >= 1004 && attr.Defindex <= 1009 {
			return true
		}
	}

	for _, desc := range it.Descriptions {
		if _, ok := schema.IdentifySpell(desc.Value); ok {
			return true
		}
	}

	return false
}

// SpellPredictor defines the subset of pricedb methods needed for Halloween spell price predictions.
type SpellPredictor interface {
	PredictSpellPrice(
		ctx context.Context,
		spells, item string,
		mods ...aoni.RequestModifier,
	) (*pricedb.SpellPredictionResponse, error)
}

// HalloweenSpellMiddleware computes spell price premiums on spelled weapons and injects them into the trade value context.
func HalloweenSpellMiddleware(
	predictor SpellPredictor,
	schemaProvider func() *schema.Schema,
	configProvider func() Config,
	logger log.Logger,
) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			pricesRaw, ok := ctx.Get("prices").Value()
			if !ok {
				return next(ctx)
			}

			priceMap, ok := pricesRaw.(map[string]*pricedb.Price)
			if !ok {
				return next(ctx)
			}

			ourSpellPremium, _ := computeSpellPremium(
				ctx, ctx.Offer.ItemsToGive, priceMap, schemaProvider, configProvider, logger, predictor,
			)
			theirSpellPremium, _ := computeSpellPremium(
				ctx, ctx.Offer.ItemsToReceive, priceMap, schemaProvider, configProvider, logger, predictor,
			)

			ctx.Set("our_spell_premium_scrap", ourSpellPremium)
			ctx.Set("their_spell_premium_scrap", theirSpellPremium)

			return next(ctx)
		}
	}
}

func computeSpellPremium(
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
		pricingSKU := sku.ToPricingSKU(item.SKU)

		p, hasPrice := priceMap[pricingSKU]
		if !hasPrice {
			continue
		}

		var spells []sku.Spell
		for _, desc := range item.Descriptions {
			if bytesconv.EqualFoldASCII(desc.Color, "7ea9d1") {
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
			cfg := configProvider()

			var staticTotal float64

			for _, sName := range spellNames {
				matchedVal := 2.0
				for k, v := range cfg.FallbackSpellPremiums {
					if strings.EqualFold(k, sName) || strings.EqualFold(strings.TrimPrefix(k, "Halloween: "), sName) {
						matchedVal = v
						break
					}
				}

				staticTotal += matchedVal
			}

			premiumRef = staticTotal
		}

		totalPremium += currency.ToScrap(premiumRef)
	}

	return totalPremium, nil
}
