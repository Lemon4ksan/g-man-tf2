// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Scrap represents the absolute atomic integer unit of currency in Team Fortress 2.
// Using Scrap eliminates floating-point rounding errors during financial trade valuations.
type Scrap int

const (
	ScrapInRec = 3
	ScrapInRef = 9
)

// ErrMissingConversionRate indicates key conversion rate in Refined is missing or zero.
var ErrMissingConversionRate = errors.New("currency: missing conversion rate")

// Currency represents a mixed TF2 balance composed of Keys and Refined metal.
type Currency struct {
	Keys  float64 `json:"keys"`
	Metal float64 `json:"metal"`
}

// New creates a new Currency instance with the given keys and metal amounts.
func New(keys, metal float64) *Currency {
	return &Currency{Keys: keys, Metal: metal}
}

// String formats the Currency into a human-readable string (e.g. "2 keys, 1.33 ref").
func (c *Currency) String() string {
	if c.Keys == 0 && c.Metal == 0 {
		return "0 keys, 0 ref"
	}

	var parts []string

	if c.Keys != 0 {
		kStr := fmt.Sprintf("%g key", c.Keys)
		if c.Keys != 1 {
			kStr += "s"
		}

		parts = append(parts, kStr)
	}

	if c.Metal != 0 || len(parts) == 0 {
		scrap := ToScrap(c.Metal)
		refined := float64(scrap) / 9.0
		rounded := math.Round(refined*100) / 100
		metalStr := strconv.FormatFloat(rounded, 'f', -1, 64)
		parts = append(parts, metalStr+" ref")
	}

	return strings.Join(parts, ", ")
}

// ToValue converts keys and metal balance into atomic integer Scrap units.
func (c *Currency) ToValue(keyPriceRef float64) (Scrap, error) {
	if keyPriceRef == 0 && c.Keys != 0 {
		return 0, ErrMissingConversionRate
	}

	metalValue := ToScrap(c.Metal)
	if c.Keys != 0 {
		keyPriceScrap := ToScrap(keyPriceRef)
		keyValue := Scrap(math.Round(c.Keys * float64(keyPriceScrap)))

		return metalValue + keyValue, nil
	}

	return metalValue, nil
}

// AddRefined sums floating-point refined metal amounts safely via atomic Scrap space.
func AddRefined(args ...float64) float64 {
	var total Scrap
	for _, ref := range args {
		total += ToScrap(ref)
	}

	return ToRefined(total)
}

// ScrapToCurrencies breaks down atomic Scrap units into integer keys and remaining refined metal.
func ScrapToCurrencies(total Scrap, keyPriceRef float64) *Currency {
	if keyPriceRef <= 0 {
		return New(0, ToRefined(total))
	}

	keyPriceScrap := ToScrap(keyPriceRef)
	keys := int(total) / int(keyPriceScrap)
	leftover := total % Scrap(keyPriceScrap)

	return New(float64(keys), ToRefined(leftover))
}

// ToScrap converts floating-point refined metal into atomic integer Scrap units.
func ToScrap(refined float64) Scrap {
	return Scrap(math.Round(refined * float64(ScrapInRef)))
}

// ToRefined converts atomic Scrap units into floating-point refined metal.
func ToRefined(s Scrap) float64 {
	return float64(s) / float64(ScrapInRef)
}

// FormatRefined formats atomic Scrap units into a string formatted to two decimal places.
func FormatRefined(s Scrap) string {
	return fmt.Sprintf("%.2f ref", float64(s)/9.0)
}
