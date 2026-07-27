// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import "fmt"

type ValueDiff struct {
	Our      Scrap
	Their    Scrap
	KeyPrice Scrap
}

func NewValueDiff(our, their, keyPrice Scrap) ValueDiff {
	return ValueDiff{
		Our:      our,
		Their:    their,
		KeyPrice: keyPrice,
	}
}

func (v ValueDiff) Diff() Scrap {
	return v.Their - v.Our
}

func (v ValueDiff) IsProfitable() bool {
	return v.Their >= v.Our
}

func (v ValueDiff) MissingRefined() float64 {
	if v.IsProfitable() {
		return 0
	}

	return float64(v.Our-v.Their) / 9.0
}

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
