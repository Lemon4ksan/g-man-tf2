// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"fmt"
	"math"
)

const (
	SKUKey       = "5021;6"
	SKURefined   = "5002;6"
	SKUReclaimed = "5001;6"
	SKUScrap     = "5000;6"
)

// PureStock tracks the counts of pure currency items (Keys, Refined, Reclaimed, Scrap).
type PureStock struct {
	Keys      int
	Refined   int
	Reclaimed int
	Scrap     int
}

// TotalScrap computes the total value of pure metal in atomic integer Scrap units.
func (p PureStock) TotalScrap() Scrap {
	return Scrap((p.Refined * 9) + (p.Reclaimed * 3) + p.Scrap)
}

// TotalRefined returns the total value of pure metal represented in Refined floating-point format.
func (p PureStock) TotalRefined() float64 {
	return ToRefined(p.TotalScrap())
}

// TotalValueScrap calculates the grand total value of both metal and keys in atomic Scrap units.
func (p PureStock) TotalValueScrap(keyPriceRef float64) Scrap {
	if keyPriceRef <= 0 {
		return p.TotalScrap()
	}

	keyPriceScrap := ToScrap(keyPriceRef)
	keysValueScrap := Scrap(p.Keys) * keyPriceScrap

	return keysValueScrap + p.TotalScrap()
}

// FormatStock formats the pure currency stock into human-readable strings (e.g. ["2 keys", "5.33 ref"]).
func (p PureStock) FormatStock() []string {
	var result []string

	if p.Keys > 0 {
		keyStr := "key"
		if p.Keys > 1 {
			keyStr = "keys"
		}

		result = append(result, fmt.Sprintf("%d %s", p.Keys, keyStr))
	}

	totalRef := p.TotalRefined()
	if totalRef > 0 {
		refTruncated := math.Trunc(totalRef*100) / 100
		metalStr := fmt.Sprintf("%.2f ref", refTruncated)

		result = append(result, metalStr)
	}

	return result
}
