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

// ErrEmptyInput indicates empty input string was passed to parser.
var ErrEmptyInput = errors.New("currency: empty input")

// Parse parses string representations into a Currency structure (e.g., "2 keys, 1.33 ref", "50 scrap", "10k").
func Parse(input string) (Currency, error) {
	if len(input) == 0 {
		return Currency{}, ErrEmptyInput
	}

	var (
		res      Currency
		foundAny bool
	)

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

		val, err := strconv.ParseFloat(input[numStart:i], 64)
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
		applyCurrencyToken(&res, val, suffix)

		foundAny = true
	}

	if !foundAny {
		return Currency{}, fmt.Errorf("currency: no valid values found in %q", input)
	}

	return res, nil
}

func applyCurrencyToken(res *Currency, val float64, suffix string) {
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

// ParseToScrap parses string input directly into total atomic Scrap units.
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
