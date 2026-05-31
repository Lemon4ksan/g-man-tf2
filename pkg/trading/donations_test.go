// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"testing"

	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

func TestDonations_IsJunk_HasSpells(t *testing.T) {
	t.Parallel()

	// 1. IsJunk nil / empty SKU
	assert.True(t, IsJunk(nil))
	assert.True(t, IsJunk(&trading.Item{SKU: ""}))

	// 2. IsJunk item with spells (not junk)
	itemWithSpells := &trading.Item{
		SKU: "5021;6",
		Attributes: []trading.Attribute{
			{Defindex: 1004, Value: "0"},
		},
	}
	assert.False(t, IsJunk(itemWithSpells))

	// 3. IsJunk crate series attribute -> true
	crateItem := &trading.Item{
		SKU: "5022;6;c1",
		Attributes: []trading.Attribute{
			{Defindex: schema.AttrCrateSeries, Value: "1"},
		},
	}
	assert.True(t, IsJunk(crateItem))

	// 4. IsJunk standard weapon -> false
	standardItem := &trading.Item{
		SKU: "5021;6",
	}
	assert.False(t, IsJunk(standardItem))

	// 5. HasSpells nil -> false
	assert.False(t, HasSpells(nil))

	// 6. HasSpells via attributes -> true
	itemSpellAttr := &trading.Item{
		SKU: "5021;6",
		Attributes: []trading.Attribute{
			{Defindex: 1007, Value: "1"},
		},
	}
	assert.True(t, HasSpells(itemSpellAttr))

	// 7. HasSpells via descriptions -> true
	itemSpellDesc := &trading.Item{
		SKU: "5021;6",
		Descriptions: []trading.Description{
			{Value: "Halloween: Squash Rockets"},
		},
	}
	assert.True(t, HasSpells(itemSpellDesc))

	// 8. HasSpells no spells -> false
	assert.False(t, HasSpells(standardItem))
}
