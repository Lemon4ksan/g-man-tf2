// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"math"
	"slices"
	"testing"
)

func TestPureStock_TotalCalculations_ReturnsScrapAndRefinedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		stock     PureStock
		wantScrap Scrap
		wantRef   float64
	}{
		{
			name:      "empty_stock",
			stock:     PureStock{},
			wantScrap: 0,
			wantRef:   0,
		},
		{
			name:      "full_metal_set",
			stock:     PureStock{Refined: 1, Reclaimed: 1, Scrap: 1},
			wantScrap: 13,
			wantRef:   13.0 / 9.0,
		},
		{
			name:      "only_reclaimed",
			stock:     PureStock{Reclaimed: 3},
			wantScrap: 9,
			wantRef:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotScrap := tt.stock.TotalScrap()
			if gotScrap != tt.wantScrap {
				t.Errorf("TotalScrap() = %v, want %v", gotScrap, tt.wantScrap)
			}

			gotRef := tt.stock.TotalRefined()
			if math.Abs(gotRef-tt.wantRef) >= 0.000001 {
				t.Errorf("TotalRefined() = %v, want %v", gotRef, tt.wantRef)
			}
		})
	}
}

func TestPureStock_FormatStock_VariousBalances_FormatsCorrectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stock PureStock
		want  []string
	}{
		{
			name:  "no_keys_no_metal",
			stock: PureStock{},
			want:  nil,
		},
		{
			name:  "one_key",
			stock: PureStock{Keys: 1},
			want:  []string{"1 key"},
		},
		{
			name:  "multiple_keys",
			stock: PureStock{Keys: 5},
			want:  []string{"5 keys"},
		},
		{
			name:  "only_metal",
			stock: PureStock{Refined: 2, Reclaimed: 1},
			want:  []string{"2.33 ref"},
		},
		{
			name:  "keys_and_metal",
			stock: PureStock{Keys: 10, Refined: 1, Scrap: 1},
			want:  []string{"10 keys", "1.11 ref"},
		},
		{
			name:  "rounding_check_truncation",
			stock: PureStock{Scrap: 2},
			want:  []string{"0.22 ref"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.stock.FormatStock()
			if !slices.Equal(got, tt.want) {
				t.Errorf("FormatStock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPureStock_TotalValueScrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		stock        PureStock
		keyPriceRef  float64
		wantValScrap Scrap
	}{
		{
			name:         "empty_stock",
			stock:        PureStock{},
			keyPriceRef:  60.0,
			wantValScrap: 0,
		},
		{
			name:         "zero_or_negative_key_price_ignores_keys",
			stock:        PureStock{Keys: 5, Refined: 2},
			keyPriceRef:  0.0,
			wantValScrap: 18,
		},
		{
			name:         "valid_conversion",
			stock:        PureStock{Keys: 2, Refined: 10},
			keyPriceRef:  60.0,
			wantValScrap: 1170,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.stock.TotalValueScrap(tt.keyPriceRef)
			if got != tt.wantValScrap {
				t.Errorf("TotalValueScrap() = %v, want %v", got, tt.wantValScrap)
			}
		})
	}
}
