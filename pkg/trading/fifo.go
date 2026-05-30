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
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/storage"
)

// FIFOSubscriber handles the Intake (push) and Outtake (pop) FIFO accounting when trades are accepted.
type FIFOSubscriber struct {
	cbStore  storage.CostBasisStore
	priceMgr *pricedb.Manager
	bus      *bus.Bus
	logger   log.Logger
	wg       sync.WaitGroup
}

// NewFIFOSubscriber creates a new FIFOSubscriber instance.
func NewFIFOSubscriber(
	cbStore storage.CostBasisStore,
	priceMgr *pricedb.Manager,
	eventBus *bus.Bus,
	logger log.Logger,
) *FIFOSubscriber {
	return &FIFOSubscriber{
		cbStore:  cbStore,
		priceMgr: priceMgr,
		bus:      eventBus,
		logger:   logger.With(log.String("module", "fifo_subscriber")),
	}
}

// Start listens for OfferChangedEvents on the event bus and handles accepted trades.
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

// Wait blocks until the subscriber's background goroutines have completed.
func (s *FIFOSubscriber) Wait() {
	s.wg.Wait()
}

// handleAcceptedOffer performs Intake and Outtake calculations on a completed trade offer.
func (s *FIFOSubscriber) handleAcceptedOffer(offer *trading.TradeOffer) {
	tradeIDStr := strconv.FormatUint(offer.ID, 10)
	keyPriceScrap := s.getKeyPriceScrap()

	var ourTotalScrap currency.Scrap
	for _, item := range offer.ItemsToGive {
		if val, isPure := getPureValueScrap(item.SKU, keyPriceScrap); isPure {
			ourTotalScrap += val
		} else {
			// Regular item - lookup sell price in PriceDB
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
		if val, isPure := getPureValueScrap(item.SKU, keyPriceScrap); isPure {
			theirBaseValueScrap += val
		} else {
			receivedRegularItems = append(receivedRegularItems, item)
			// Regular item - lookup buy price in PriceDB
			if p, ok := s.priceMgr.GetPrice(item.SKU); ok {
				itemVal := currency.Scrap(p.Buy.Keys)*keyPriceScrap + currency.ToScrap(p.Buy.Metal)
				theirBaseValueScrap += itemVal
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

	// --- FIFO Push (Intake) ---
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

	// --- FIFO Pop (Outtake & Profit calculations) ---
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
			// Fallback if not found in database (Virtual estimate)
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
}

// getPureValueScrap translates TF2 metal and key SKUs to currency.Scrap.
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
