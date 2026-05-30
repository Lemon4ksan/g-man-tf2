// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import "fmt"

// ValueDiff represents the disparity between items offered and items received in a trade.
// It evaluates profitability and calculates missing balances for automated counter-offers.
type ValueDiff struct {
	// Our represents the total value of items given on our side in [Scrap].
	Our Scrap
	// Their represents the total value of items received on their side in [Scrap].
	Their Scrap
	// KeyPrice represents the active key exchange rate in [Scrap] used for calculations.
	KeyPrice Scrap
}

// NewValueDiff creates a new [ValueDiff] instance with the specified values in [Scrap].
func NewValueDiff(our, their, keyPrice Scrap) ValueDiff {
	return ValueDiff{
		Our:      our,
		Their:    their,
		KeyPrice: keyPrice,
	}
}

// Diff returns the raw value difference between their side and our side in [Scrap].
func (v ValueDiff) Diff() Scrap {
	return v.Their - v.Our
}

// IsProfitable returns true if the value of items received is greater than or equal to the items given.
func (v ValueDiff) IsProfitable() bool {
	return v.Their >= v.Our
}

// MissingRefined returns the amount of missing metal in refined floating-point format.
// Returns 0 if the transaction is already profitable.
func (v ValueDiff) MissingRefined() float64 {
	if v.IsProfitable() {
		return 0
	}

	diff := v.Our - v.Their

	return float64(diff) / 9.0
}

// MissingString formats the missing value into a structured currency string.
// Returns strings such as "1 key, 2 ref" or "0.55 ref". Returns "0 ref" if profitable.
func (v ValueDiff) MissingString() string {
	if v.IsProfitable() {
		return "0 ref"
	}

	missingScrap := v.Our - v.Their

	if v.KeyPrice > 0 && missingScrap >= v.KeyPrice {
		keys := int(missingScrap / v.KeyPrice)
		leftoverScrap := missingScrap % v.KeyPrice

		if leftoverScrap == 0 {
			return fmt.Sprintf("%d keys", keys)
		}

		return fmt.Sprintf("%d keys, %s", keys, FormatRefined(leftoverScrap))
	}

	return FormatRefined(missingScrap)
}
