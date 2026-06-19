// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import "github.com/lemon4ksan/miyako/bus"

// PricelistUpdatedEvent is published when a price in the database is set or changed.
type PricelistUpdatedEvent struct {
	bus.BaseEvent
	SKU     string     `json:"sku"`
	Buy     Currencies `json:"buy"`
	Sell    Currencies `json:"sell"`
	OldBuy  Currencies `json:"old_buy"`
	OldSell Currencies `json:"old_sell"`
	Source  string     `json:"source"`
}
