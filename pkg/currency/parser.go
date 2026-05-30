// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var currencyRegex = regexp.MustCompile(`([0-9]*\.?[0-9]+)\s*([a-zA-Z]*)`)

// Parse converts a formatted text string into a [Currency] instance.
// Supported string patterns include "1.33 ref", "2 keys, 1.33", "50 scrap", or "10k".
// Suffixes are case-insensitive and support abbreviations: "key"/"k" for keys,
// "ref"/"r" for refined, "rec" for reclaimed, and "scr"/"s" for scrap.
// Returns an error if the input string contains no recognizable numeric currency values.
func Parse(input string) (*Currency, error) {
	input = strings.ToLower(input)
	input = strings.ReplaceAll(input, ",", "")

	matches := currencyRegex.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("currency: could not parse input %q", input)
	}

	res := &Currency{}
	foundAny := false

	for _, match := range matches {
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}

		suffix := match[2]
		foundAny = true

		switch {
		case strings.HasPrefix(suffix, "key") || suffix == "k":
			res.Keys += val

		case strings.HasPrefix(suffix, "ref") || suffix == "r" || suffix == "":
			res.Metal = AddRefined(res.Metal, val)

		case strings.HasPrefix(suffix, "rec"):
			scrap := math.Round(val * float64(ScrapInRec))
			metalFromRec := scrap / float64(ScrapInRef)
			res.Metal = AddRefined(res.Metal, metalFromRec)

		case strings.HasPrefix(suffix, "scr") || strings.HasPrefix(suffix, "s"):
			scrap := math.Round(val)
			metalFromScrap := scrap / float64(ScrapInRef)
			res.Metal = AddRefined(res.Metal, metalFromScrap)

		default:
			res.Metal = AddRefined(res.Metal, val)
		}
	}

	if !foundAny {
		return nil, fmt.Errorf("currency: no valid values found in %q", input)
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
