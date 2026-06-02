// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package trading

import (
	"github.com/lemon4ksan/g-man/test/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

// TF2TradeTester is a specialized wrapper over the generic TradeTester from the core.
// It defines the type T as a PriceDB price map (map[string]*priceb.Price), requiring
// the Price, PPU, and Spell middleware in TF2 to work.
type TF2TradeTester struct {
	*trading.TradeTester[map[string]*pricedb.Price]
}

// NewTF2TradeTester initializes a new instance of TF2TradeTester.
func NewTF2TradeTester() *TF2TradeTester {
	return &TF2TradeTester{
		TradeTester: trading.NewTradeTester[map[string]*pricedb.Price](),
	}
}

// WithTF2Prices passes detailed PriceDB price models inside the generic test engine.
func (t *TF2TradeTester) WithTF2Prices(priceModels map[string]*pricedb.Price) *TF2TradeTester {
	t.WithPriceModels(priceModels)
	return t
}
