// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
)

// TF2TradeTester provides a fluent, isolated mock context for executing and auditing middleware pipelines.
type TF2TradeTester struct {
	engine *engine.Engine
	logger log.Logger
}

// NewTF2TradeTester constructs a new [TF2TradeTester] with discarded logging and an empty middleware chain.
func NewTF2TradeTester() *TF2TradeTester {
	return &TF2TradeTester{
		engine: engine.New(),
		logger: log.Discard,
	}
}

// WithPrices sets up a mock pricer middleware injecting static prices into the execution context.
func (t *TF2TradeTester) WithPrices(prices map[string]int) *TF2TradeTester {
	t.engine.Use(func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			for sku, price := range prices {
				ctx.Set("price_"+sku, price)
			}

			return next(ctx)
		}
	})

	return t
}

// AddMiddleware appends a custom middleware stage to the testing execution chain.
func (t *TF2TradeTester) AddMiddleware(m engine.Middleware) *TF2TradeTester {
	t.engine.Use(m)
	return t
}

// Run processes the trade offer through the configured middleware pipeline.
// Returns the resulting [engine.Verdict] or an error if any of the pipeline stages fail.
func (t *TF2TradeTester) Run(ctx context.Context, offer *trading.TradeOffer) (engine.Verdict, error) {
	verdict, err := t.engine.Process(ctx, offer)
	if err != nil {
		return engine.Verdict{}, err
	}

	return *verdict, nil
}
