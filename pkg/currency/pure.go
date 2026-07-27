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

type PureStock struct {
	Keys      int
	Refined   int
	Reclaimed int
	Scrap     int
}

func (p PureStock) TotalScrap() Scrap {
	return Scrap((p.Refined * 9) + (p.Reclaimed * 3) + p.Scrap)
}

func (p PureStock) TotalRefined() float64 {
	return ToRefined(p.TotalScrap())
}

func (p PureStock) TotalValueScrap(keyPriceRef float64) Scrap {
	if keyPriceRef <= 0 {
		return p.TotalScrap()
	}

	keyPriceScrap := ToScrap(keyPriceRef)
	keysValueScrap := Scrap(p.Keys) * keyPriceScrap

	return keysValueScrap + p.TotalScrap()
}

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
