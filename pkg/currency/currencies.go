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

// Scrap represents the absolute atomic unit of currency in Team Fortress 2.
// Calculations in Scrap prevent floating-point rounding errors during trades.
type Scrap int

const (
	// ScrapInRec defines the number of individual scrap units contained in one reclaimed metal.
	ScrapInRec = 3
	// ScrapInRef defines the number of individual scrap units contained in one refined metal.
	ScrapInRef = 9
)

// Currency represents a combined balance of keys and refined metal.
// It models item pricing, listing values, and trade session totals.
type Currency struct {
	// Keys represents the count of keys in the balance.
	Keys float64 `json:"keys"`
	// Metal represents the amount of refined metal in the balance.
	Metal float64 `json:"metal"`
}

// New creates and returns a pointer to a new [Currency] instance.
func New(keys, metal float64) *Currency {
	return &Currency{Keys: keys, Metal: metal}
}

// String formats the [Currency] instance into a human-readable string.
// Returns formats such as "1 key, 20.11 ref" or "0 keys, 0 ref".
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

// ToValue calculates the total value of the [Currency] in [Scrap] units.
// It uses the provided key exchange rate in refined units to convert keys.
// Returns an error if keys are present but the key exchange rate is zero or negative.
func (c *Currency) ToValue(keyPriceRef float64) (Scrap, error) {
	if keyPriceRef == 0 && c.Keys != 0 {
		return 0, errors.New("missing conversion rate")
	}

	metalValue := ToScrap(c.Metal)
	if c.Keys != 0 {
		keyPriceScrap := ToScrap(keyPriceRef)
		keyValue := Scrap(math.Round(c.Keys * float64(keyPriceScrap)))

		return metalValue + keyValue, nil
	}

	return metalValue, nil
}

// AddRefined sums multiple refined metal values in floating-point format.
// It converts internally to [Scrap] to eliminate standard floating-point precision errors.
func AddRefined(args ...float64) float64 {
	var total Scrap
	for _, ref := range args {
		total += ToScrap(ref)
	}

	return ToRefined(total)
}

// ScrapToCurrencies converts total [Scrap] units into a [Currency] structure.
// It splits the scrap into keys and remaining metal based on the provided key price in refined.
// If the key price is zero or negative, the result contains only metal.
func ScrapToCurrencies(total Scrap, keyPriceRef float64) *Currency {
	if keyPriceRef <= 0 {
		return New(0, ToRefined(total))
	}

	keyPriceScrap := ToScrap(keyPriceRef)
	keys := int(total) / int(keyPriceScrap)
	leftover := total % Scrap(keyPriceScrap)

	return New(float64(keys), ToRefined(leftover))
}

// ToScrap converts refined metal in floating-point format into [Scrap] units.
func ToScrap(refined float64) Scrap {
	return Scrap(math.Round(refined * float64(ScrapInRef)))
}

// ToRefined converts [Scrap] units into a refined metal floating-point value.
func ToRefined(s Scrap) float64 {
	return float64(s) / float64(ScrapInRef)
}

// FormatRefined formats [Scrap] units into a refined metal string with two decimal places.
// It rounds the last digit up if the trailing remainder is half or greater.
func FormatRefined(s Scrap) string {
	return fmt.Sprintf("%.2f ref", float64(s)/9.0)
}
