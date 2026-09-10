// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reason contains TF2-specific trade decision reason codes.
package reason

import "github.com/lemon4ksan/g-man/pkg/trading/reason"

const (
	ReviewDupedItems      reason.TradeReason = "🟫_DUPED_ITEMS"
	ReviewInvalidValue    reason.TradeReason = "🟥_INVALID_VALUE"
	DeclineOverpay        reason.TradeReason = "OVERPAY"
	DeclineUnderpaid      reason.TradeReason = "UNDERPAID"
	DeclineNoChange       reason.TradeReason = "NO_CHANGE"
	DeclineBannedBptf     reason.TradeReason = "BANNED_BPTF"
	ReviewPricerDown      reason.TradeReason = "⬜_PRICER_DOWN"
	ReviewUnpricedItem    reason.TradeReason = "⬜_UNPRICED_ITEM"
	DeclineIntentBuy      reason.TradeReason = "TAKING_ITEMS_WITH_INTENT_BUY"
	DeclineIntentSell     reason.TradeReason = "GIVING_ITEMS_WITH_INTENT_SELL"
	DeclineDuelingUses    reason.TradeReason = "DUELING_NOT_5_USES"
	DeclineNoisemakerUses reason.TradeReason = "NOISE_MAKER_NOT_25_USES"
)
