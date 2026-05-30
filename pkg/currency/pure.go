// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"fmt"
	"math"
)

const (
	// SKUKey is the canonical SKU string for a Mann Co. Supply Crate Key.
	SKUKey = "5021;6"

	// SKURefined is the canonical SKU string for Refined Metal.
	SKURefined = "5002;6"

	// SKUReclaimed is the canonical SKU string for Reclaimed Metal.
	SKUReclaimed = "5001;6"

	// SKUScrap is the canonical SKU string for Scrap Metal.
	SKUScrap = "5000;6"
)

// PureStock represents the current liquid inventory of keys and metal coins.
type PureStock struct {
	// Keys represents the count of keys in stock.
	Keys int
	// Refined represents the count of Refined Metal in stock.
	Refined int
	// Reclaimed represents the count of Reclaimed Metal in stock.
	Reclaimed int
	// Scrap represents the count of Scrap Metal in stock.
	Scrap int
}

// TotalScrap calculates the total value of metal in [Scrap] units, excluding keys.
func (p PureStock) TotalScrap() Scrap {
	return Scrap((p.Refined * 9) + (p.Reclaimed * 3) + p.Scrap)
}

// TotalRefined calculates the total value of metal in refined floating-point format, excluding keys.
func (p PureStock) TotalRefined() float64 {
	return ToRefined(p.TotalScrap())
}

// FormatStock returns a human-readable string slice representation of the stock.
// Returns outputs like []string{"10 keys", "5.33 ref"}.
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
