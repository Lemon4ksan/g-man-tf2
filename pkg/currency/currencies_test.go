// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency_test

import (
	"math"
	"testing"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
)

func TestToScrap_VariousValues_ReturnsCorrectScrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		refined float64
		want    currency.Scrap
	}{
		{"one_scrap", 0.11, 1},
		{"three_scrap", 0.33, 3},
		{"nine_scrap", 1.00, 9},
		{"ten_scrap", 1.11, 10},
		{"twenty_one_scrap", 2.33, 21},
		{"four_hundred_fifty_scrap", 50.00, 450},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := currency.ToScrap(tt.refined)
			if got != tt.want {
				t.Errorf("ToScrap(%v) = %v, want %v", tt.refined, got, tt.want)
			}
		})
	}
}

func TestToRefined_VariousValues_ReturnsCorrectRefined(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scrap currency.Scrap
		want  float64
	}{
		{"one_scrap", 1, 1.0 / 9.0},
		{"three_scrap", 3, 3.0 / 9.0},
		{"nine_scrap", 9, 1.0},
		{"ten_scrap", 10, 10.0 / 9.0},
		{"thirteen_scrap", 13, 13.0 / 9.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := currency.ToRefined(tt.scrap)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("ToRefined(%v) = %v, want %v", tt.scrap, got, tt.want)
			}
		})
	}
}

func TestCurrency_String_VariousBalances_ReturnsCorrectString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    *currency.Currency
		want string
	}{
		{
			name: "single_key_and_metal",
			c:    currency.New(1, 20.11),
			want: "1 key, 20.11 ref",
		},
		{
			name: "multiple_keys",
			c:    currency.New(5, 0),
			want: "5 keys",
		},
		{
			name: "only_metal",
			c:    currency.New(0, 15.33),
			want: "15.33 ref",
		},
		{
			name: "zero_value",
			c:    currency.New(0, 0),
			want: "0 keys, 0 ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.c.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrency_ToValue_ConversionRates_ReturnsExpectedScrap(t *testing.T) {
	t.Parallel()

	const conv = 50.0

	tests := []struct {
		name    string
		keys    float64
		metal   float64
		want    currency.Scrap
		wantErr bool
	}{
		{"one_key_only", 1, 0, 450, false},
		{"one_key_with_metal", 1, 1.11, 460, false},
		{"half_key", 0.5, 0, 225, false},
		{"only_metal_no_keys", 0, 15.33, 138, false},
		{"error_missing_conversion", 1, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := currency.New(tt.keys, tt.metal)

			conversion := conv
			if tt.wantErr {
				conversion = 0
			}

			got, err := c.ToValue(conversion)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("ToValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScrapToCurrencies_VariousScrapValues_ReturnsCurrencyBalances(t *testing.T) {
	t.Parallel()

	const conv = 50.0

	tests := []struct {
		name     string
		scrap    currency.Scrap
		keyPrice float64
		wantK    float64
		wantM    float64
	}{
		{"exactly_one_key", 450, conv, 1, 0},
		{"one_key_and_one_scrap", 451, conv, 1, 1.0 / 9.0},
		{"less_than_one_key", 100, conv, 0, 100.0 / 9.0},
		{"multiple_keys", 1000, conv, 2, 100.0 / 9.0},
		{"zero_key_price", 100, 0, 0, 100.0 / 9.0},
		{"negative_key_price", 100, -5.0, 0, 100.0 / 9.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := currency.ScrapToCurrencies(tt.scrap, tt.keyPrice)
			if got.Keys != tt.wantK || math.Abs(got.Metal-tt.wantM) > 1e-9 {
				t.Errorf("ScrapToCurrencies() = %v keys, %v metal; want %v keys, %v metal",
					got.Keys, got.Metal, tt.wantK, tt.wantM)
			}
		})
	}
}

func TestAddRefined_MultipleFloatValues_SumsWithoutPrecisionLoss(t *testing.T) {
	t.Parallel()

	res := currency.AddRefined(1.11, 2.22, 0.11)

	want := 31.0 / 9.0
	if math.Abs(res-want) > 1e-9 {
		t.Errorf("AddRefined() = %v, want %v", res, want)
	}
}
