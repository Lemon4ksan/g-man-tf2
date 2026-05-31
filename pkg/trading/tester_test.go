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
