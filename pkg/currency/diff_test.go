// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package currency

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValueDiff_Metrics_CalculatesExpectedValues(t *testing.T) {
	t.Parallel()

	var keyPrice Scrap = 540

	tests := []struct {
		name           string
		ourScrap       Scrap
		theirScrap     Scrap
		keyPrice       Scrap
		wantProfitable bool
		wantMissingRef float64
		wantMissingStr string
	}{
		{
			name:           "profitable_trade",
			ourScrap:       100,
			theirScrap:     120,
			keyPrice:       keyPrice,
			wantProfitable: true,
			wantMissingRef: 0,
			wantMissingStr: "0 ref",
		},
		{
			name:           "equal_trade",
			ourScrap:       100,
			theirScrap:     100,
			keyPrice:       keyPrice,
			wantProfitable: true,
			wantMissingRef: 0,
			wantMissingStr: "0 ref",
		},
		{
			name:           "unprofitable_small_metal_diff",
			ourScrap:       15,
			theirScrap:     10,
			keyPrice:       keyPrice,
			wantProfitable: false,
			wantMissingRef: 5.0 / 9.0,
			wantMissingStr: "0.56 ref",
		},
		{
			name:           "unprofitable_one_key_missing",
			ourScrap:       keyPrice + 10,
			theirScrap:     10,
			keyPrice:       keyPrice,
			wantProfitable: false,
			wantMissingStr: "1 keys",
		},
		{
			name:           "unprofitable_keys_and_metal_missing",
			ourScrap:       (keyPrice * 2) + 18,
			theirScrap:     0,
			keyPrice:       keyPrice,
			wantProfitable: false,
			wantMissingStr: "2 keys, 2.00 ref",
		},
		{
			name:           "unprofitable_nearly_one_key_missing",
			ourScrap:       keyPrice - 1,
			theirScrap:     0,
			keyPrice:       keyPrice,
			wantProfitable: false,
			wantMissingStr: "59.89 ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diff := NewValueDiff(tt.ourScrap, tt.theirScrap, tt.keyPrice)

			if diff.IsProfitable() != tt.wantProfitable {
				t.Errorf("IsProfitable() = %v, want %v", diff.IsProfitable(), tt.wantProfitable)
			}

			gotMissingRef := diff.MissingRefined()
			if tt.wantMissingRef > 0 &&
				(gotMissingRef < tt.wantMissingRef-0.0001 || gotMissingRef > tt.wantMissingRef+0.0001) {
				t.Errorf("MissingRefined() = %v, want %v", gotMissingRef, tt.wantMissingRef)
			}

			if diff.MissingString() != tt.wantMissingStr {
				t.Errorf("MissingString() = %v, want %v", diff.MissingString(), tt.wantMissingStr)
			}
		})
	}

	t.Run("zero_key_price_missing_string", func(t *testing.T) {
		vd := NewValueDiff(18, 9, 0)
		assert.False(t, vd.IsProfitable())
		assert.Equal(t, "1.00 ref", vd.MissingString())
	})

	t.Run("profitable_cases", func(t *testing.T) {
		vd := NewValueDiff(10, 20, 0)
		assert.True(t, vd.IsProfitable())
		assert.Equal(t, 0.0, vd.MissingRefined())
		assert.Equal(t, "0 ref", vd.MissingString())
		assert.Equal(t, Scrap(10), vd.Diff())
	})
}
