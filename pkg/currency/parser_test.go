// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import "testing"

func TestParse_FormattingPatterns_ReturnsCurrencyObject(t *testing.T) {
	t.Parallel()

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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			curr, err := Parse(tc.input)
			if err != nil {
				t.Errorf("Input %s failed: %v", tc.input, err)
				return
			}

			if curr.String() != tc.expected {
				t.Errorf("Input %s: expected %s, got %s", tc.input, tc.expected, curr.String())
			}
		})
	}
}
