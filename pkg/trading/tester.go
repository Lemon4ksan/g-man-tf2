// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"github.com/lemon4ksan/g-man/pkg/test/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

type TF2TradeTester struct {
	*trading.TradeTester[map[string]*pricedb.Price]
}

func NewTF2TradeTester() *TF2TradeTester {
	return &TF2TradeTester{
		TradeTester: trading.NewTradeTester[map[string]*pricedb.Price](),
	}
}

func (t *TF2TradeTester) WithTF2Prices(priceModels map[string]*pricedb.Price) *TF2TradeTester {
	t.WithPriceModels(priceModels)

	return t
}
