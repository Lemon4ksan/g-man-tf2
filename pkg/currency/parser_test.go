// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_FormattingPatterns_ReturnsCurrencyObject(t *testing.T) {
	t.Parallel()

	t.Run("success_cases", func(t *testing.T) {
		cases := []struct {
			name     string
			input    string
			expected string
		}{
			{"standard_metal", "1.33 ref", "1.33 ref"},
			{"standard_keys_and_metal", "2 keys 1.33 ref", "2 keys, 1.33 ref"},
			{"abbreviated_keys_and_refined", "10k 5r", "10 keys, 5 ref"},
			{"reclaimed_conversion", "3 rec", "1 ref"},
			{"scrap_conversion", "9 scrap", "1 ref"},
			{"float_keys", "1.5 keys", "1.5 keys"},
			{"float_reclaimed", "1.5 rec", "0.56 ref"},
			{"float_scrap", "0.5 scrap", "0.11 ref"},
			{"compressed_input", "1 key, 2.33ref", "1 key, 2.33 ref"},
			{"large_input", "5 keys, 10 scrap", "5 keys, 1.11 ref"},
			{"unknown_suffix_fallback", "10 xyz", "10 ref"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				curr, err := Parse(tc.input)
				require.NoError(t, err, "Input %s failed", tc.input)
				assert.Equal(t, tc.expected, curr.String())
			})
		}
	})

	t.Run("error_cases", func(t *testing.T) {
		cases := []struct {
			name  string
			input string
		}{
			{"empty_input", ""},
			{"no_numeric_values", "abc"},
			{"nan_values", "NaN keys"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := Parse(tc.input)
				assert.Error(t, err)
			})
		}
	})
}

func TestParseToScrap(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		val, err := ParseToScrap("2 keys, 1.5 ref", 50.0)
		require.NoError(t, err)
		assert.Equal(t, Scrap(914), val)
	})

	t.Run("parse_error", func(t *testing.T) {
		_, err := ParseToScrap("invalid", 50.0)
		assert.Error(t, err)
	})

	t.Run("conversion_error", func(t *testing.T) {
		_, err := ParseToScrap("2 keys", 0.0)
		assert.Error(t, err)
	})
}
