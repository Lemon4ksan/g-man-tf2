// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModels_Currencies(t *testing.T) {
	t.Parallel()

	t.Run("to_metal", func(t *testing.T) {
		c := Currencies{Keys: 2, Metal: 15.5}
		assert.Equal(t, 115.5, c.ToMetal(50.0))
	})

	t.Run("is_zero", func(t *testing.T) {
		assert.True(t, Currencies{Keys: 0, Metal: 0}.IsZero())
		assert.False(t, Currencies{Keys: 1, Metal: 0}.IsZero())
	})

	t.Run("valid", func(t *testing.T) {
		assert.True(t, Currencies{Keys: 0, Metal: 0}.Valid())
		assert.False(t, Currencies{Keys: -1, Metal: 0}.Valid())
		assert.False(t, Currencies{Keys: 0, Metal: -0.1}.Valid())
	})
}

func TestModels_Price(t *testing.T) {
	t.Parallel()

	t.Run("validate", func(t *testing.T) {
		pValid := Price{
			SKU:  "5021;6",
			Buy:  Currencies{Keys: 0, Metal: 50},
			Sell: Currencies{Keys: 0, Metal: 55},
		}
		assert.True(t, pValid.Validate())

		pInvalidSKU := Price{
			SKU:  "",
			Buy:  Currencies{Keys: 0, Metal: 50},
			Sell: Currencies{Keys: 0, Metal: 55},
		}
		assert.False(t, pInvalidSKU.Validate())

		pInvalidBuy := Price{
			SKU:  "5021;6",
			Buy:  Currencies{Keys: -1, Metal: 50},
			Sell: Currencies{Keys: 0, Metal: 55},
		}
		assert.False(t, pInvalidBuy.Validate())
	})

	t.Run("has_profit", func(t *testing.T) {
		p := Price{
			Buy:  Currencies{Keys: 1, Metal: 10},
			Sell: Currencies{Keys: 1, Metal: 15},
		}
		assert.True(t, p.HasProfit(50.0))

		pNoProfit := Price{
			Buy:  Currencies{Keys: 1, Metal: 15},
			Sell: Currencies{Keys: 1, Metal: 10},
		}
		assert.False(t, pNoProfit.HasProfit(50.0))
	})
}
