// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"1.33 ref", "1.33 ref"},
		{"2 keys 1.33 ref", "2 keys, 1.33 ref"},
		{"10k 5r", "10 keys, 5 ref"},
		{"3 rec", "1 ref"},
		{"9 scrap", "1 ref"},
		{"1.5 keys", "1.5 keys"},
		{"1.5 rec", "0.56 ref"},   // 1.5 * 3 = 4.5 -> 5 scrap -> 0.56 ref
		{"0.5 scrap", "0.11 ref"}, // 0.5 -> 1 scrap -> 0.11 ref
		{"1 key, 2.33ref", "1 key, 2.33 ref"},
		{"5 keys, 10 scrap", "5 keys, 1.11 ref"},
	}

	for _, tc := range cases {
		curr, err := Parse(tc.input)
		if err != nil {
			t.Errorf("Input %s failed: %v", tc.input, err)
			continue
		}

		if curr.String() != tc.expected {
			t.Errorf("Input %s: expected %s, got %s", tc.input, tc.expected, curr.String())
		}
	}
}
