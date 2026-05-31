// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
	tradingtest "github.com/lemon4ksan/g-man/test/trading"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	tf2schema "github.com/lemon4ksan/g-man-tf2/pkg/schema"
	tf2 "github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type complexTestSchemaProvider struct{}

func (m *complexTestSchemaProvider) Get() *tf2schema.Schema { return nil }

type mockEscrowChecker struct {
	hasEscrow bool
}

func (m *mockEscrowChecker) CheckEscrow(ctx context.Context, offer *trading.TradeOffer) (bool, error) {
	return m.hasEscrow, nil
}

func TestComplexTrades_VariousScenarios_ReturnsExpectedVerdicts(t *testing.T) {
	t.Parallel()

	logger := log.Discard

	valueChecker := func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			giveValue := 0
			recvValue := 0

			for _, it := range ctx.Offer.ItemsToGive {
				val, ok := ctx.Get("price_" + it.SKU)
				if ok {
					giveValue += val.(int)
				}
			}

			for _, it := range ctx.Offer.ItemsToReceive {
				val, ok := ctx.Get("price_" + it.SKU)
				if ok {
					recvValue += val.(int)
				}
			}

			if recvValue >= giveValue {
				ctx.Accept(reason.TradeReason("good_value"))
				return nil
			}

			ctx.Decline(reason.TradeReason("low_value"))

			return nil
		}
	}

	t.Run("clean_trade_accepted", func(t *testing.T) {
		t.Parallel()

		bp := backpack.New()
		cache := &mockBackpackCache{items: []*tf2.Item{}}
		setUnexportedField(bp, "cache", cache)
		setUnexportedField(bp, "manager", &complexTestSchemaProvider{})

		tester := NewTF2TradeTester().
			WithPrices(map[string]int{
				"Key": 60,
				"Ref": 1,
			}).
			AddMiddleware(EscrowMiddleware(&mockEscrowChecker{hasEscrow: false}, logger)).
			AddMiddleware(StockLimitMiddleware(bp, StockConfig{MaxTotal: 100, DefaultMax: 100}, logger)).
			AddMiddleware(valueChecker)

		offer := tradingtest.NewOfferBuilder().
			AddGiveItem("Key", 1).
			AddReceiveItem("Ref", 60).
			Build()

		verdict, err := tester.Run(t.Context(), offer)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionAccept, verdict.Action)
	})

	t.Run("escrow_trade_declined", func(t *testing.T) {
		t.Parallel()

		bp := backpack.New()
		cache := &mockBackpackCache{items: []*tf2.Item{}}
		setUnexportedField(bp, "cache", cache)
		setUnexportedField(bp, "manager", &complexTestSchemaProvider{})

		tester := NewTF2TradeTester().
			WithPrices(map[string]int{
				"Key": 60,
				"Ref": 1,
			}).
			AddMiddleware(EscrowMiddleware(&mockEscrowChecker{hasEscrow: true}, logger)).
			AddMiddleware(StockLimitMiddleware(bp, StockConfig{MaxTotal: 100, DefaultMax: 100}, logger)).
			AddMiddleware(valueChecker)

		offer := tradingtest.NewOfferBuilder().
			AddGiveItem("Key", 1).
			AddReceiveItem("Ref", 60).
			Build()

		verdict, err := tester.Run(t.Context(), offer)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.DeclineEscrow, verdict.Reason)
	})

	t.Run("overstock_trade_declined_by_global_limit", func(t *testing.T) {
		t.Parallel()

		bp := backpack.New()
		cache := &mockBackpackCache{items: []*tf2.Item{}}
		setUnexportedField(bp, "cache", cache)
		setUnexportedField(bp, "manager", &complexTestSchemaProvider{})

		tester := NewTF2TradeTester().
			WithPrices(map[string]int{
				"Key": 60,
				"Ref": 1,
			}).
			AddMiddleware(EscrowMiddleware(&mockEscrowChecker{hasEscrow: false}, logger)).
			AddMiddleware(StockLimitMiddleware(bp, StockConfig{MaxTotal: 5, DefaultMax: 10}, logger)).
			AddMiddleware(valueChecker)

		offer := tradingtest.NewOfferBuilder().
			AddReceiveItem("Ref", 6).
			Build()

		verdict, err := tester.Run(t.Context(), offer)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.ReviewOverstocked, verdict.Reason)
	})

	t.Run("underpaid_trade_declined_by_value_checker", func(t *testing.T) {
		t.Parallel()

		bp := backpack.New()
		cache := &mockBackpackCache{items: []*tf2.Item{}}
		setUnexportedField(bp, "cache", cache)
		setUnexportedField(bp, "manager", &complexTestSchemaProvider{})

		tester := NewTF2TradeTester().
			WithPrices(map[string]int{
				"Key": 60,
				"Ref": 1,
			}).
			AddMiddleware(EscrowMiddleware(&mockEscrowChecker{hasEscrow: false}, logger)).
			AddMiddleware(StockLimitMiddleware(bp, StockConfig{MaxTotal: 100, DefaultMax: 100}, logger)).
			AddMiddleware(valueChecker)

		offer := tradingtest.NewOfferBuilder().
			AddGiveItem("Key", 1).
			AddReceiveItem("Ref", 50).
			Build()

		verdict, err := tester.Run(t.Context(), offer)
		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
		assert.Equal(t, reason.TradeReason("low_value"), verdict.Reason)
	})
}
