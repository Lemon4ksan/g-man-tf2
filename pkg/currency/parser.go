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

// Parse converts a formatted text string into a [Currency] instance.
// Supported string patterns include "1.33 ref", "2 keys, 1.33", "50 scrap", or "10k".
// Suffixes are case-insensitive and support abbreviations: "key"/"k" for keys,
// "ref"/"r" for refined, "rec" for reclaimed, and "scr"/"s" for scrap.
// Returns an error if the input string contains no recognizable numeric currency values.
func Parse(input string) (Currency, error) {
	if len(input) == 0 {
		return Currency{}, errors.New("currency: empty input")
	}

	var res Currency

	foundAny := false

	i := 0
	for i < len(input) {
		for i < len(input) && !isDigitOrDot(input[i]) {
			i++
		}

		if i >= len(input) {
			break
		}

		numStart := i
		for i < len(input) && isDigitOrDot(input[i]) {
			i++
		}

		numStr := input[numStart:i]

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
			continue
		}

		for i < len(input) && (input[i] == ' ' || input[i] == '\t') {
			i++
		}

		sufStart := i
		for i < len(input) && isAlpha(input[i]) {
			i++
		}

		suffix := strings.ToLower(input[sufStart:i])

		foundAny = true

		switch {
		case strings.HasPrefix(suffix, "key") || suffix == "k":
			res.Keys += val
		case strings.HasPrefix(suffix, "ref") || suffix == "r" || suffix == "":
			res.Metal = AddRefined(res.Metal, val)
		case strings.HasPrefix(suffix, "rec"):
			scrap := math.Round(val * float64(ScrapInRec))
			res.Metal = AddRefined(res.Metal, scrap/float64(ScrapInRef))
		case strings.HasPrefix(suffix, "scr") || strings.HasPrefix(suffix, "s"):
			scrap := math.Round(val)
			res.Metal = AddRefined(res.Metal, scrap/float64(ScrapInRef))
		default:
			res.Metal = AddRefined(res.Metal, val)
		}
	}

	if !foundAny {
		return Currency{}, fmt.Errorf("currency: no valid values found in %q", input)
	}

	return res, nil
}

// ParseToScrap converts a formatted text string and returns its total value in [Scrap] units.
// It uses the provided key exchange rate in refined units to resolve key values.
// Returns an error if parsing fails or if keys are parsed but the exchange rate is zero or negative.
func ParseToScrap(input string, keyPriceRef float64) (Scrap, error) {
	curr, err := Parse(input)
	if err != nil {
		return 0, err
	}

	return curr.ToValue(keyPriceRef)
}

func isDigitOrDot(c byte) bool {
	return (c >= '0' && c <= '9') || c == '.'
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
