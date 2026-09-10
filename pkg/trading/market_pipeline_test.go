// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"errors"
	"testing"

	"github.com/lemon4ksan/aoni"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/rep"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type pipelineMockBP struct {
	items map[uint64]*tf2.Item
	stock map[string]int
}

func (p *pipelineMockBP) GetTotalCount() int {
	return len(p.items)
}

func (p *pipelineMockBP) GetStock(sku string) int {
	return p.stock[sku]
}

func (p *pipelineMockBP) GetItem(id uint64) (*tf2.Item, bool) {
	it, ok := p.items[id]
	return it, ok
}

type pipelineMockMetalMgr struct {
	changeIDs []uint64
	selectErr error
	smeltErr  error
}

func (p *pipelineMockMetalMgr) SelectChange(amount currency.Scrap) ([]uint64, error) {
	if p.selectErr != nil {
		return nil, p.selectErr
	}

	return p.changeIDs, nil
}

func (p *pipelineMockMetalMgr) TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error {
	return p.smeltErr
}

type pipelineMockPartnerInv struct {
	items []*trading.Item
	err   error
}

func (p *pipelineMockPartnerInv) GetPartnerInventory(ctx context.Context, partnerID id.ID) ([]*trading.Item, error) {
	return p.items, p.err
}

type pipelineMockEscrow struct {
	hasEscrow bool
	err       error
}

func (p *pipelineMockEscrow) CheckEscrow(ctx context.Context, offer *trading.TradeOffer) (bool, error) {
	return p.hasEscrow, p.err
}

type pipelineMockBans struct {
	isBanned bool
	details  map[string]string
	err      error
}

func (p *pipelineMockBans) CheckBans(ctx context.Context, partnerID id.ID) (*rep.BanResult, error) {
	return &rep.BanResult{
		IsBanned: p.isBanned,
		Details:  p.details,
	}, p.err
}

type pipelineMockDupeChecker struct {
	duped bool
	err   error
}

func (p *pipelineMockDupeChecker) CheckHistory(
	ctx context.Context,
	assetID uint64,
	mods ...aoni.RequestModifier,
) (backpack.HistoryStatus, error) {
	return backpack.HistoryStatus{
		Recorded: true,
		IsDuped:  p.duped,
	}, p.err
}

// TestMarketTradePipeline_Comprehensive tests end-to-end trade evaluation matching tf2autobot market standards.
func TestMarketTradePipeline_Comprehensive(t *testing.T) {
	t.Parallel()

	schemaFunc := funcMockSchema

	basePrices := map[string]*pricedb.Price{
		currency.SKUKey: {
			SKU:  currency.SKUKey,
			Name: "Mann Co. Supply Crate Key",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 50.5},
		},
		currency.SKURefined: {
			SKU:  currency.SKURefined,
			Name: "Refined Metal",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 1.0},
			Sell: pricedb.Currencies{Keys: 0, Metal: 1.0},
		},
		currency.SKUReclaimed: {
			SKU:  currency.SKUReclaimed,
			Name: "Reclaimed Metal",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.33},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.33},
		},
		currency.SKUScrap: {
			SKU:  currency.SKUScrap,
			Name: "Scrap Metal",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.11},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.11},
		},
		"205;6": { // Rocket Launcher
			SKU:  "205;6",
			Name: "Rocket Launcher",
			Buy:  pricedb.Currencies{Keys: 0, Metal: 0.5},
			Sell: pricedb.Currencies{Keys: 0, Metal: 0.5},
		},
		"30000;6": { // General Unique item
			SKU:  "30000;6",
			Name: "Some Item",
			Buy:  pricedb.Currencies{Keys: 1, Metal: 0.0},
			Sell: pricedb.Currencies{Keys: 1, Metal: 5.0},
		},
	}

	buildTestEngine := func(
		bp BackpackProvider,
		prices map[string]*pricedb.Price,
		stockCfg StockConfig,
		metalMgr MetalChangeManager,
		partnerInv trading.PartnerInventoryProvider,
		escrow trading.EscrowChecker,
		bans ReputationChecker,
		dupe DupeChecker,
		ourSpellScrap currency.Scrap,
	) *engine.Engine {
		e := engine.New()

		// Context injector middleware: sets prices, schema, and spell premium
		e.Use(func(next engine.Handler) engine.Handler {
			return func(ctx *engine.TradeContext) error {
				if prices != nil {
					ctx.Set("prices", prices)
				}

				if schemaFunc != nil {
					ctx.Set("schema", schemaFunc())
				}

				if ourSpellScrap > 0 {
					ctx.Set("our_spell_premium_scrap", ourSpellScrap)
				}

				return next(ctx)
			}
		})

		if escrow != nil {
			e.Use(EscrowMiddleware(escrow, log.Discard))
		}

		if bans != nil {
			e.Use(BanCheckMiddleware(bans, log.Discard))
		}

		e.Use(ItemUsesMiddleware(log.Discard))

		if dupe != nil {
			e.Use(DupeCheckMiddleware(dupe, log.Discard))
		}

		e.Use(StockLimitMiddleware(bp, stockCfg, log.Discard))
		e.Use(SmartCounterMiddleware(nil, metalMgr, bp, partnerInv, log.Discard))

		return e
	}

	t.Run("Scenario 1: Exact Value Match - Accepted", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"30000;6": 5},
			items: map[uint64]*tf2.Item{
				101: {ID: 101, DefIndex: 30000, Quality: 6},
			},
		}
		stockCfg := StockConfig{MaxTotal: 100}
		eng := buildTestEngine(bp, basePrices, stockCfg, &pipelineMockMetalMgr{}, nil, nil, nil, nil, 0)

		// Partner gives 1 key (50 ref) + 5 ref = 55 ref (exact sell price: 1 key 5 ref)
		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToGive:  []*trading.Item{{AssetID: 101, SKU: "30000;6"}},
			ItemsToReceive: []*trading.Item{
				{SKU: currency.SKUKey},
				{SKU: currency.SKURefined},
				{SKU: currency.SKURefined},
				{SKU: currency.SKURefined},
				{SKU: currency.SKURefined},
				{SKU: currency.SKURefined},
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionAccept, verdict.Action)
		assert.Equal(t, reason.AcceptCorrectValue, verdict.Reason)
	})

	t.Run("Scenario 2: Underpayment with Change in Partner Backpack - Countered", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"30000;6": 5},
			items: map[uint64]*tf2.Item{101: {ID: 101, DefIndex: 30000, Quality: 6}},
		}
		// Base prices: "30000;6" Sell: 1 key 5 ref = 55 ref.
		// Partner offers 1 key (50 ref). Partner is missing 5 ref.
		// Partner has 5 ref in inventory.
		partnerInv := &pipelineMockPartnerInv{
			items: []*trading.Item{
				{AssetID: 901, SKU: currency.SKURefined},
				{AssetID: 902, SKU: currency.SKURefined},
				{AssetID: 903, SKU: currency.SKURefined},
				{AssetID: 904, SKU: currency.SKURefined},
				{AssetID: 905, SKU: currency.SKURefined},
			},
		}
		eng := buildTestEngine(
			bp,
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			partnerInv,
			nil,
			nil,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{AssetID: 101, SKU: "30000;6"}},
			ItemsToReceive: []*trading.Item{{SKU: currency.SKUKey}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		// Should counter and add payment from partner inventory
		assert.Equal(t, trading.ActionCounter, verdict.Action)
		assert.Equal(t, reason.AcceptCorrectValue, verdict.Reason)
		decision := verdict.Decision()
		require.NotNil(t, decision.CounterParams)
		assert.NotEmpty(t, decision.CounterParams.ItemsToReceive)
	})

	t.Run("Scenario 3: Underpayment without Change in Partner Backpack - Declined Underpaid", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"205;6": 5},
			items: map[uint64]*tf2.Item{101: {ID: 101, DefIndex: 205, Quality: 6}},
		}
		partnerInv := &pipelineMockPartnerInv{items: nil} // empty inventory
		eng := buildTestEngine(
			bp,
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			partnerInv,
			nil,
			nil,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{AssetID: 101, SKU: "205;6"}},
			ItemsToReceive: nil,
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineUnderpaid, verdict.Reason)
	})

	t.Run("Scenario 4: Overpayment with Bot Change - Countered Adding Change", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{
				"205;6":           5,
				currency.SKUScrap: 10,
			},
			items: map[uint64]*tf2.Item{
				101: {ID: 101, DefIndex: 205, Quality: 6},
				201: {ID: 201, DefIndex: 5000, Quality: 6}, // scrap change
			},
		}
		// Partner paid 1 ref (9 scrap) for a 0.5 ref (4.5 scrap) item -> overpaid!
		metalMgr := &pipelineMockMetalMgr{
			changeIDs: []uint64{201},
		}
		eng := buildTestEngine(bp, basePrices, StockConfig{MaxTotal: 100}, metalMgr, nil, nil, nil, nil, 0)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{AssetID: 101, SKU: "205;6"}},
			ItemsToReceive: []*trading.Item{{SKU: currency.SKURefined}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionCounter, verdict.Action)
		assert.Equal(t, reason.AcceptCorrectValue, verdict.Reason)
		decision := verdict.Decision()
		require.NotNil(t, decision.CounterParams)
		assert.Contains(t, decision.CounterParams.Message, "added the necessary change")
	})

	t.Run("Scenario 5: Overpayment but Bot Cannot Provide Change - Declined No Change", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"205;6": 5},
			items: map[uint64]*tf2.Item{101: {ID: 101, DefIndex: 205, Quality: 6}},
		}
		metalMgr := &pipelineMockMetalMgr{
			selectErr: crafting.ErrNotEnoughChange,
			smeltErr:  errors.New("cannot smelt"),
		}
		eng := buildTestEngine(bp, basePrices, StockConfig{MaxTotal: 100}, metalMgr, nil, nil, nil, nil, 0)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{AssetID: 101, SKU: "205;6"}},
			ItemsToReceive: []*trading.Item{{SKU: currency.SKURefined}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineNoChange, verdict.Reason)
	})

	t.Run("Scenario 6: Spelled Weapon Premium Required - Declined Underpaid", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"205;6": 1},
			items: map[uint64]*tf2.Item{
				101: {ID: 101, DefIndex: 205, Quality: 6},
			},
		}
		// 5 ref spell premium = 45 scrap
		eng := buildTestEngine(
			bp,
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			&pipelineMockPartnerInv{},
			nil,
			nil,
			nil,
			45,
		)

		// Partner offers base weapon price (0.5 ref) for our weapon with Exorcism spell
		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToGive: []*trading.Item{
				{
					AssetID: 101,
					SKU:     "205;6",
					Descriptions: []trading.Description{
						{Value: "Halloween: Exorcism", Color: "7ea9d1"},
					},
				},
			},
			ItemsToReceive: []*trading.Item{
				{SKU: currency.SKUReclaimed}, // only offers reclaimed (3 scrap < base + 45 scrap premium)
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		// Partner did not pay the spell premium -> declined
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineUnderpaid, verdict.Reason)
	})

	t.Run("Scenario 7: High Value Non-Unusual Duped Item - Review Duped", func(t *testing.T) {
		prices := map[string]*pricedb.Price{
			currency.SKUKey: {
				SKU:  currency.SKUKey,
				Name: "Mann Co. Supply Crate Key",
				Buy:  pricedb.Currencies{Keys: 0, Metal: 50.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 50.5},
			},
			"205;11;australium": {
				SKU:  "205;11;australium",
				Name: "Australium Rocket Launcher",
				Buy:  pricedb.Currencies{Keys: 25, Metal: 0},
				Sell: pricedb.Currencies{Keys: 27, Metal: 0},
			},
		}
		dupeChecker := &pipelineMockDupeChecker{duped: true}
		eng := buildTestEngine(
			&pipelineMockBP{},
			prices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			nil,
			nil,
			dupeChecker,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{
				{AssetID: 55555, SKU: "205;11;australium"},
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionReview, verdict.Action)
		assert.Equal(t, tf2reason.ReviewDupedItems, verdict.Reason)
	})

	t.Run("Scenario 8: Dueling Mini-Game Not Full Uses - Declined", func(t *testing.T) {
		eng := buildTestEngine(
			&pipelineMockBP{},
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			nil,
			nil,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{
				{
					SKU: "241;6",
					Descriptions: []trading.Description{
						{Value: "This is a limited use item. Uses: 2", Color: "00a000"},
					},
				},
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineDuelingUses, verdict.Reason)
	})

	t.Run("Scenario 9: Noise Maker Not Full Uses - Declined", func(t *testing.T) {
		eng := buildTestEngine(
			&pipelineMockBP{},
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			nil,
			nil,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{
				{
					SKU: "282;6", // Noise Maker - Werewolf
					Descriptions: []trading.Description{
						{Value: "This is a limited use item. Uses: 18", Color: "00a000"},
					},
				},
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineNoisemakerUses, verdict.Reason)
	})

	t.Run("Scenario 10: Intent Buy Protection (Taking Buy-Only Item) - Declined", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{"30000;6": 5},
			items: map[uint64]*tf2.Item{101: {ID: 101, DefIndex: 30000, Quality: 6}},
		}
		stockCfg := StockConfig{
			Items: map[string]ItemConfig{
				"30000;6": {SKU: "30000;6", EnableBuy: true, EnableSell: false},
			},
		}
		eng := buildTestEngine(bp, basePrices, stockCfg, &pipelineMockMetalMgr{}, nil, nil, nil, nil, 0)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{AssetID: 101, SKU: "30000;6"}},
			ItemsToReceive: []*trading.Item{{SKU: currency.SKUKey}, {SKU: currency.SKUKey}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineIntentBuy, verdict.Reason)
	})

	t.Run("Scenario 11: Intent Sell Protection (Giving Sell-Only Item) - Declined", func(t *testing.T) {
		bp := &pipelineMockBP{stock: map[string]int{"30000;6": 1}}
		stockCfg := StockConfig{
			Items: map[string]ItemConfig{
				"30000;6": {SKU: "30000;6", EnableBuy: false, EnableSell: true},
			},
		}
		eng := buildTestEngine(bp, basePrices, stockCfg, &pipelineMockMetalMgr{}, nil, nil, nil, nil, 0)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToGive:    []*trading.Item{{SKU: currency.SKUKey}},
			ItemsToReceive: []*trading.Item{{SKU: "30000;6"}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineIntentSell, verdict.Reason)
	})

	t.Run("Scenario 12: Understock Drain Attack - Review Understocked", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{currency.SKUKey: 10},
			items: map[uint64]*tf2.Item{
				1: {ID: 1, DefIndex: 5021, Quality: 6},
				2: {ID: 2, DefIndex: 5021, Quality: 6},
				3: {ID: 3, DefIndex: 5021, Quality: 6},
				4: {ID: 4, DefIndex: 5021, Quality: 6},
				5: {ID: 5, DefIndex: 5021, Quality: 6},
				6: {ID: 6, DefIndex: 5021, Quality: 6},
				7: {ID: 7, DefIndex: 5021, Quality: 6},
				8: {ID: 8, DefIndex: 5021, Quality: 6},
			},
		}
		stockCfg := StockConfig{
			MinPerSKU: map[string]int{currency.SKUKey: 5}, // keep at least 5 keys
		}
		eng := buildTestEngine(bp, basePrices, stockCfg, &pipelineMockMetalMgr{}, nil, nil, nil, nil, 0)

		// Partner offers 400 ref for 8 keys (10 - 8 = 2 < MinStock 5)
		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToGive: []*trading.Item{
				{AssetID: 1, SKU: currency.SKUKey},
				{AssetID: 2, SKU: currency.SKUKey},
				{AssetID: 3, SKU: currency.SKUKey},
				{AssetID: 4, SKU: currency.SKUKey},
				{AssetID: 5, SKU: currency.SKUKey},
				{AssetID: 6, SKU: currency.SKUKey},
				{AssetID: 7, SKU: currency.SKUKey},
				{AssetID: 8, SKU: currency.SKUKey},
			},
			ItemsToReceive: []*trading.Item{{SKU: currency.SKURefined}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionReview, verdict.Action)
		assert.Equal(t, reason.ReviewUnderstocked, verdict.Reason)
	})

	t.Run("Scenario 13: Overstock Overflow Attack - Declined Overstocked", func(t *testing.T) {
		bp := &pipelineMockBP{
			stock: map[string]int{currency.SKUKey: 8},
		}
		stockCfg := StockConfig{
			MaxPerSKU: map[string]int{currency.SKUKey: 10}, // max 10 keys
		}
		eng := buildTestEngine(bp, basePrices, stockCfg, &pipelineMockMetalMgr{}, nil, nil, nil, nil, 0)

		// Partner offers 5 keys (8 + 5 = 13 > MaxStock 10)
		offer := &trading.TradeOffer{
			OtherSteamID: id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{
				{SKU: currency.SKUKey},
				{SKU: currency.SKUKey},
				{SKU: currency.SKUKey},
				{SKU: currency.SKUKey},
				{SKU: currency.SKUKey},
			},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.DeclineOverstocked, verdict.Reason)
	})

	t.Run("Scenario 14: Escrow Partner - Declined Escrow", func(t *testing.T) {
		escrow := &pipelineMockEscrow{hasEscrow: true}
		eng := buildTestEngine(
			&pipelineMockBP{},
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			escrow,
			nil,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{{SKU: currency.SKUKey}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.DeclineEscrow, verdict.Reason)
	})

	t.Run("Scenario 15: Banned Partner on SteamRep - Declined Banned", func(t *testing.T) {
		bans := &pipelineMockBans{
			isBanned: true,
			details:  map[string]string{"steamrep.com": "banned"},
		}
		eng := buildTestEngine(
			&pipelineMockBP{},
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			nil,
			bans,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{{SKU: currency.SKUKey}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.DeclineBanned, verdict.Reason)
	})

	t.Run("Scenario 16: Banned Partner on Backpack.tf - Declined Banned Bptf", func(t *testing.T) {
		bans := &pipelineMockBans{
			isBanned: true,
			details:  map[string]string{"backpack.tf": "banned"},
		}
		eng := buildTestEngine(
			&pipelineMockBP{},
			basePrices,
			StockConfig{MaxTotal: 100},
			&pipelineMockMetalMgr{},
			nil,
			nil,
			bans,
			nil,
			0,
		)

		offer := &trading.TradeOffer{
			OtherSteamID:   id.ID(76561198000000001),
			ItemsToReceive: []*trading.Item{{SKU: currency.SKUKey}},
		}
		verdict, err := eng.Process(t.Context(), offer)
		require.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, tf2reason.DeclineBannedBptf, verdict.Reason)
	})
}
