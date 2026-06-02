// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"testing"

	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	tradingTest "github.com/lemon4ksan/g-man/test/trading"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

func TestTF2TradeTester_WithPricesAndMiddleware_ProcessesTradeSuccessfully(t *testing.T) {
	t.Parallel()

	mockMiddleware := func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			for _, item := range ctx.Offer.ItemsToReceive {
				val, ok := ctx.Get("price_" + item.SKU)
				if ok {
					price, ok := val.(int)
					if ok && price < 10 {
						ctx.Decline("price too low")
						return nil
					}
				}
			}

			return next(ctx)
		}
	}

	tester := NewTF2TradeTester().
		WithPrices(map[string]int{
			"LowValueSKU":  5,
			"HighValueSKU": 50,
		}).
		AddMiddleware(mockMiddleware)

	t.Run("declines_when_condition_met", func(t *testing.T) {
		t.Parallel()

		offer := tradingTest.NewOfferBuilder().
			AddReceiveItem("LowValueSKU", 1).
			Build()

		verdict, err := tester.Run(t.Context(), offer)

		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
	})

	t.Run("passes_through_when_condition_not_met", func(t *testing.T) {
		t.Parallel()

		offer := tradingTest.NewOfferBuilder().
			AddReceiveItem("HighValueSKU", 1).
			Build()

		verdict, err := tester.Run(t.Context(), offer)

		assert.NoError(t, err)
		assert.Equal(t, trading.ActionSkip, verdict.Action)
	})
}

func TestTF2TradeTester_WithTF2PricesAndMiddleware_ProcessesTradeSuccessfully(t *testing.T) {
	t.Parallel()

	mockPriceDBMiddleware := func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			pricesRaw, ok := ctx.Get("prices")
			if !ok {
				return next(ctx)
			}

			priceMap, ok := pricesRaw.(map[string]*pricedb.Price)
			if !ok {
				return next(ctx)
			}

			for _, item := range ctx.Offer.ItemsToReceive {
				p, ok := priceMap[item.SKU]
				if ok && p.Buy.Metal < 10.0 {
					ctx.Decline("metal price too low")
					return nil
				}
			}

			return next(ctx)
		}
	}

	tester := NewTF2TradeTester()
	tester.WithTF2Prices(map[string]*pricedb.Price{
		"LowValueSKU": {
			SKU: "LowValueSKU",
			Buy: pricedb.Currencies{Keys: 0, Metal: 5.0},
		},
		"HighValueSKU": {
			SKU: "HighValueSKU",
			Buy: pricedb.Currencies{Keys: 0, Metal: 50.0},
		},
	})
	tester.AddMiddleware(mockPriceDBMiddleware)

	t.Run("declines_when_metal_price_below_threshold", func(t *testing.T) {
		t.Parallel()

		offer := tradingTest.NewOfferBuilder().
			AddReceiveItem("LowValueSKU", 1).
			Build()

		verdict, err := tester.Run(t.Context(), offer)

		assert.NoError(t, err)
		assert.Equal(t, trading.ActionDecline, verdict.Action)
	})

	t.Run("passes_through_when_metal_price_above_threshold", func(t *testing.T) {
		t.Parallel()

		offer := tradingTest.NewOfferBuilder().
			AddReceiveItem("HighValueSKU", 1).
			Build()

		verdict, err := tester.Run(t.Context(), offer)

		assert.NoError(t, err)
		assert.Equal(t, trading.ActionSkip, verdict.Action)
	})
}
