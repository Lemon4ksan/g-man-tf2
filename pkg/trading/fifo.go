// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// FIFOSubscriber monitors accepted trade events on the bus to record cost basis and calculate transaction profits.
// It registers with [bus.Bus] and writes records to the [storage.CostBasisStore].
type FIFOSubscriber struct {
	cbStore    storage.CostBasisStore
	statsStore storage.TradeProfitStore // Can be nil
	priceMgr   *pricedb.Manager
	bus        *bus.Bus
	logger     log.Logger
	wg         sync.WaitGroup
}

// NewFIFOSubscriber constructs a new [FIFOSubscriber] linked to the specified store, statsStore, manager, and event bus.
func NewFIFOSubscriber(
	cbStore storage.CostBasisStore,
	statsStore storage.TradeProfitStore,
	priceMgr *pricedb.Manager,
	eventBus *bus.Bus,
	logger log.Logger,
) *FIFOSubscriber {
	return &FIFOSubscriber{
		cbStore:    cbStore,
		statsStore: statsStore,
		priceMgr:   priceMgr,
		bus:        eventBus,
		logger:     logger.With(log.String("module", "fifo_subscriber")),
	}
}

// Start launches the background event loop listening for accepted trade event notifications.
// The consumer loop terminates when the provided [context.Context] is cancelled.
func (s *FIFOSubscriber) Start(ctx context.Context) {
	sub := s.bus.Subscribe(&web.OfferChangedEvent{})
	s.logger.Info("FIFO subscriber started listening for trade events")

	s.wg.Go(func() {
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.C():
				if !ok {
					return
				}

				event, ok := ev.(*web.OfferChangedEvent)
				if !ok {
					continue
				}

				if event.Offer.State == trading.OfferStateAccepted && event.OldState != trading.OfferStateAccepted {
					s.logger.Info(
						"Processing accepted trade for FIFO accounting",
						log.Uint64("offer_id", event.Offer.ID),
					)
					s.handleAcceptedOffer(event.Offer)
				}
			}
		}
	})
}

// Wait blocks until the subscriber background goroutines have successfully terminated.
func (s *FIFOSubscriber) Wait() {
	s.wg.Wait()
}

func (s *FIFOSubscriber) handleAcceptedOffer(offer *trading.TradeOffer) {
	for _, item := range offer.ItemsToReceive {
		if itObj, err := sku.FromString(item.SKU); err == nil && itObj.Paint != 0 {
			origPaint := itObj.Paint
			itObj.Paint = 0

			baseSKU := sku.FromObject(itObj)
			if basePrice, ok := s.priceMgr.GetPrice(baseSKU); ok {
				buyPrice := basePrice.Buy
				sellPrice := basePrice.Sell

				// Default premium of 0.5 ref if not found in options
				premium := PaintPremium{Keys: 0, Metal: 0.5}
				if pName := schema.GetPaintName(uint32(origPaint)); pName != "" {
					if prem, ok := PaintPremiums[pName]; ok {
						premium = prem
					}
				}

				// Apply markup to sell price
				sellPrice.Keys += premium.Keys
				sellPrice.Metal += premium.Metal

				// Normalize metal to keys if metal exceeds key price
				keyPriceRef := s.getKeyPriceRef()
				if sellPrice.Metal >= keyPriceRef && keyPriceRef > 0 {
					truncKeys := int(sellPrice.Metal / keyPriceRef)
					sellPrice.Keys += truncKeys
					sellPrice.Metal -= float64(truncKeys) * keyPriceRef
				}

				buyPrice.Keys = 0
				buyPrice.Metal = 0.11

				s.priceMgr.SetPrice(item.SKU, buyPrice, sellPrice, pricedb.PricelistChangedSourcePaintMarkup)
				s.logger.Info("Auto-pricing painted item with markup",
					log.String("painted_sku", item.SKU),
					log.String("base_sku", baseSKU),
					log.String("paint_name", schema.GetPaintName(uint32(origPaint))),
					log.Float64("markup_ref", premium.Metal),
					log.Int("markup_keys", premium.Keys),
					log.Int("sell_keys", sellPrice.Keys),
					log.Float64("sell_metal", sellPrice.Metal),
				)
			}
		}
	}

	tradeIDStr := strconv.FormatUint(offer.ID, 10)
	keyPriceScrap := s.getKeyPriceScrap()

	var (
		ourTotalScrap currency.Scrap
		netKeys       int
		netMetalScrap currency.Scrap
	)

	for _, item := range offer.ItemsToGive {
		if item.SKU == currency.SKUKey {
			netKeys--
		} else {
			switch item.SKU {
			case currency.SKURefined:
				netMetalScrap -= 9
			case currency.SKUReclaimed:
				netMetalScrap -= 3
			case currency.SKUScrap:
				netMetalScrap -= 1
			}
		}

		if val, isPure := getPureValueScrap(item.SKU, keyPriceScrap); isPure {
			ourTotalScrap += val
		} else {
			if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
				itemVal := currency.Scrap(p.Sell.Keys)*keyPriceScrap + currency.ToScrap(p.Sell.Metal)
				ourTotalScrap += itemVal
			} else {
				s.logger.Warn(
					"Unpriced item given in accepted trade, skipping value calculation",
					log.String("sku", item.SKU),
				)
			}
		}
	}

	var (
		theirBaseValueScrap  currency.Scrap
		receivedRegularItems []*trading.Item
	)

	for _, item := range offer.ItemsToReceive {
		if item.SKU == currency.SKUKey {
			netKeys++
		} else {
			switch item.SKU {
			case currency.SKURefined:
				netMetalScrap += 9
			case currency.SKUReclaimed:
				netMetalScrap += 3
			case currency.SKUScrap:
				netMetalScrap += 1
			}
		}

		if val, isPure := getPureValueScrap(item.SKU, keyPriceScrap); isPure {
			theirBaseValueScrap += val
		} else {
			receivedRegularItems = append(receivedRegularItems, item)
			if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
				theirBaseValueScrap += currency.Scrap(p.Buy.Keys)*keyPriceScrap + currency.ToScrap(p.Buy.Metal)
			} else {
				s.logger.Warn(
					"Unpriced item received in accepted trade, skipping value calculation",
					log.String("sku", item.SKU),
				)
			}
		}
	}

	var itemDiff int
	if regularItems := len(receivedRegularItems); regularItems > 0 {
		totalDiff := ourTotalScrap - theirBaseValueScrap
		itemDiff = int(totalDiff) / regularItems
	}

	for _, item := range receivedRegularItems {
		var buyKeys, buyMetal float64

		if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
			buyKeys, buyMetal = float64(p.Buy.Keys), p.Buy.Metal
		}

		entry := storage.CostBasisEntry{
			SKU:        item.SKU,
			BuyKeys:    buyKeys,
			BuyMetal:   buyMetal,
			Diff:       itemDiff,
			TradeID:    tradeIDStr,
			Timestamp:  time.Now(),
			IsEstimate: false,
		}

		s.cbStore.Push(item.SKU, entry)
		s.logger.Info("FIFO Intake pushed entry",
			log.String("sku", item.SKU),
			log.Float64("buy_keys", buyKeys),
			log.Float64("buy_metal", buyMetal),
			log.Int("diff_scrap", itemDiff),
			log.String("trade_id", tradeIDStr),
		)
	}

	var (
		totalFIFOProfitScrap currency.Scrap
		anyEstimateUsed      bool
	)

	for _, item := range offer.ItemsToGive {
		if _, isPure := getPureValueScrap(item.SKU, keyPriceScrap); isPure {
			continue
		}

		var (
			netCostBasisScrap currency.Scrap
			isEstimate        bool
		)

		entry, popped := s.cbStore.Pop(item.SKU)
		if popped {
			baseBuyScrap := currency.Scrap(entry.BuyKeys)*keyPriceScrap + currency.ToScrap(entry.BuyMetal)
			netCostBasisScrap = baseBuyScrap + currency.Scrap(entry.Diff)
			isEstimate = entry.IsEstimate
		} else {
			if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
				netCostBasisScrap = currency.Scrap(p.Buy.Keys)*keyPriceScrap + currency.ToScrap(p.Buy.Metal)
			}

			isEstimate = true
		}

		var actualSellPriceScrap currency.Scrap
		if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
			actualSellPriceScrap = currency.Scrap(p.Sell.Keys)*keyPriceScrap + currency.ToScrap(p.Sell.Metal)
		}

		profitScrap := actualSellPriceScrap - netCostBasisScrap
		totalFIFOProfitScrap += profitScrap

		if isEstimate {
			anyEstimateUsed = true
		}

		state, exists := s.cbStore.GetPPUState(item.SKU)
		if !exists {
			state = storage.PPUState{SKU: item.SKU}
		}

		state.LastSoldTime = time.Now()
		s.cbStore.SetPPUState(item.SKU, state)

		s.logger.Info("FIFO Outtake popped entry",
			log.String("sku", item.SKU),
			log.Int("profit_scrap", int(profitScrap)),
			log.Float64("profit_ref", currency.ToRefined(profitScrap)),
			log.Bool("is_estimate", isEstimate),
			log.Float64("cost_basis_ref", currency.ToRefined(netCostBasisScrap)),
			log.Float64("sell_price_ref", currency.ToRefined(actualSellPriceScrap)),
			log.String("trade_id", tradeIDStr),
		)
	}

	if s.statsStore != nil {
		logEntry := storage.TradeProfitLog{
			TradeID:         tradeIDStr,
			Timestamp:       time.Now(),
			NetKeys:         netKeys,
			NetMetalRef:     float64(netMetalScrap) / 9.0,
			FIFOProfitScrap: int(totalFIFOProfitScrap),
			IsEstimate:      anyEstimateUsed,
		}
		if err := s.statsStore.Push(logEntry); err != nil {
			s.logger.Error("Failed to record trade profit stats log", log.Err(err))
		} else {
			s.logger.Info("Recorded trade profit stats log",
				log.String("trade_id", tradeIDStr),
				log.Int("net_keys", netKeys),
				log.Float64("net_metal_ref", float64(netMetalScrap)/9.0),
				log.Int("fifo_profit_scrap", int(totalFIFOProfitScrap)),
			)
		}
	}
}

func getPureValueScrap(sku string, keyPriceScrap currency.Scrap) (currency.Scrap, bool) {
	switch sku {
	case currency.SKUKey:
		return keyPriceScrap, true
	case currency.SKURefined:
		return 9, true
	case currency.SKUReclaimed:
		return 3, true
	case currency.SKUScrap:
		return 1, true
	default:
		return 0, false
	}
}

func (s *FIFOSubscriber) getKeyPriceScrap() currency.Scrap {
	var keyPriceRef float64
	if kp, ok := s.priceMgr.GetPrice(currency.SKUKey); ok {
		keyPriceRef = kp.Buy.Metal
	}

	if keyPriceRef <= 0 {
		keyPriceRef = 50.0
	}

	return currency.ToScrap(keyPriceRef)
}

func (s *FIFOSubscriber) getKeyPriceRef() float64 {
	var keyPriceRef float64
	if kp, ok := s.priceMgr.GetPrice(currency.SKUKey); ok {
		keyPriceRef = kp.Buy.Metal
	}

	if keyPriceRef <= 0 {
		keyPriceRef = 50.0
	}

	return keyPriceRef
}

// PaintPremium represents the markup price addition in keys and refined metal.
type PaintPremium struct {
	Keys  int
	Metal float64
}

// PaintPremiums defines the default markups applied to items with TF2 paints.
var PaintPremiums = map[string]PaintPremium{
	"Pink as Hell":                               {Keys: 1, Metal: 10.0},
	"A Distinctive Lack of Hue":                  {Keys: 1, Metal: 10.0},
	"The Bitter Taste of Defeat and Lime":        {Keys: 1, Metal: 10.0},
	"Team Spirit":                                {Keys: 1, Metal: 10.0},
	"After Eight":                                {Keys: 1, Metal: 5.0},
	"An Extraordinary Abundance of Tinge":        {Keys: 1, Metal: 5.0},
	"An Air of Debonair":                         {Keys: 0, Metal: 30.0},
	"A Mann's Mint":                              {Keys: 0, Metal: 15.0},
	"Dark Salmon Injustice":                      {Keys: 0, Metal: 15.0},
	"Australium Gold":                            {Keys: 0, Metal: 15.0},
	"Balaclavas Are Forever":                     {Keys: 0, Metal: 15.0},
	"Cream Spirit":                               {Keys: 0, Metal: 15.0},
	"Operator's Overalls":                        {Keys: 0, Metal: 15.0},
	"The Value of Teamwork":                      {Keys: 0, Metal: 15.0},
	"Waterlogged Lab Coat":                       {Keys: 0, Metal: 15.0},
	"A Deep Commitment to Purple":                {Keys: 0, Metal: 7.0},
	"Color No. 216-190-216":                      {Keys: 0, Metal: 7.0},
	"Noble Hatter's Violet":                      {Keys: 0, Metal: 7.0},
	"Mann Co. Orange":                            {Keys: 0, Metal: 6.0},
	"Aged Moustache Grey":                        {Keys: 0, Metal: 5.0},
	"Drably Olive":                               {Keys: 0, Metal: 5.0},
	"Indubitably Green":                          {Keys: 0, Metal: 5.0},
	"The Color of a Gentlemann's Business Pants": {Keys: 0, Metal: 5.0},
	"A Color Similar to Slate":                   {Keys: 0, Metal: 5.0},
	"Zepheniah's Greed":                          {Keys: 0, Metal: 4.0},
	"Peculiarly Drab Tincture":                   {Keys: 0, Metal: 3.0},
	"Radigan Conagher Brown":                     {Keys: 0, Metal: 2.0},
	"Muskelmannbraun":                            {Keys: 0, Metal: 2.0},
	"Ye Olde Rustic Colour":                      {Keys: 0, Metal: 2.0},
}
