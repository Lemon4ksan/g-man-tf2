// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reason contains TF2-specific trade reasons.
package reason

import "github.com/lemon4ksan/g-man/pkg/trading/reason"

// TF2-specific trade reasons.
const (
	ReviewDupedItems   reason.TradeReason = "🟫_DUPED_ITEMS"
	ReviewInvalidValue reason.TradeReason = "🟥_INVALID_VALUE"
	DeclineOverpay     reason.TradeReason = "OVERPAY"
	DeclineUnderpaid   reason.TradeReason = "UNDERPAID"
	DeclineNoChange    reason.TradeReason = "NO_CHANGE"
	DeclineBannedBptf  reason.TradeReason = "BANNED_BPTF"
	ReviewPricerDown   reason.TradeReason = "⬜_PRICER_DOWN"
	ReviewUnpricedItem reason.TradeReason = "⬜_UNPRICED_ITEM"
)
