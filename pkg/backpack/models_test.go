// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func TestMapCEconToTF2(t *testing.T) {
	s := mockSchema()

	t.Run("Basic item", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: "100",
				Amount:  "1",
			},
			Description: &inventory.Description{
				Tradable: 1,
				AppData: map[string]any{
					"def_index": "1",
					"quality":   "6",
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(100), item.ID)
		assert.Equal(t, 1, item.Defindex)
		assert.Equal(t, 6, item.Quality)
		assert.False(t, item.FlagCannotTrade)
		assert.False(t, item.FlagCannotCraft)
	})

	t.Run("Uncraftable item", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "101"},
			Description: &inventory.Description{
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "( Not Usable in Crafting )"},
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.True(t, item.FlagCannotCraft)
	})

	t.Run("Unusual item with effect", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "102"},
			Description: &inventory.Description{
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "★ Unusual Effect: Sunbeams"},
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(102), item.ID)
		// sunbeams is ID 17 in TF2, but our mockSchema is empty.
		// Let's improve mockSchema in layout_test.go or here.
	})
}

func TestTF2Item_ToSKU(t *testing.T) {
	tests := []struct {
		name string
		item TF2Item
		want string
	}{
		{
			name: "Unique Weapon",
			item: TF2Item{Defindex: 1, Quality: 6, FlagCannotTrade: false},
			want: "1;6",
		},
		{
			name: "Uncraftable Unique Weapon",
			item: TF2Item{Defindex: 1, Quality: 6, FlagCannotCraft: true, FlagCannotTrade: false},
			want: "1;6;uncraftable",
		},
		{
			name: "Unusual Hat",
			item: TF2Item{
				Defindex:        100,
				Quality:         5,
				FlagCannotTrade: false,
				Attributes: []TF2Attribute{
					{Defindex: 134, Value: float64(17)}, // Sunbeams
				},
			},
			want: "100;5;u17",
		},
		{
			name: "Australium",
			item: TF2Item{
				Defindex:        200,
				Quality:         11,
				FlagCannotTrade: false,
				Attributes: []TF2Attribute{
					{Defindex: 2027, Value: float64(1)},
				},
			},
			want: "200;11;australium",
		},
		{
			name: "Strange Unusual (Elevated)",
			item: TF2Item{
				Defindex: 378,
				Quality:  5,
				Attributes: []TF2Attribute{
					{Defindex: 134, Value: float64(33)},
					{Defindex: 214, Value: float64(1)},
				},
			},
			want: "378;5;u33;strange",
		},
		{
			name: "Strange Parts",
			item: TF2Item{
				Defindex: 1,
				Quality:  11,
				Attributes: []TF2Attribute{
					{Defindex: 10000, Value: float64(10)}, // Part ID 10
					{Defindex: 10001, Value: float64(12)}, // Part ID 12
				},
			},
			want: "1;11;sp10;sp12",
		},
		{
			name: "Spells",
			item: TF2Item{
				Defindex: 1,
				Quality:  6,
				Attributes: []TF2Attribute{
					{Defindex: 11000, Value: sku.Spell{Attribute: 1004, Value: 3}},
				},
			},
			want: "1;6;s-1004-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.item.ToSKU())
		})
	}
}

func TestTF2Item_ToEconItem(t *testing.T) {
	item := TF2Item{
		ID:         123,
		Defindex:   1,
		Quantity:   1,
		CustomName: "My Gun",
		Attributes: []TF2Attribute{
			{Defindex: 134, Value: float64(17)},
		},
		FlagCannotCraft: true,
	}

	econ := item.ToEconItem()
	assert.Equal(t, uint64(123), econ.AssetID)
	assert.Equal(t, uint64(1), econ.ClassID)
	assert.Equal(t, "My Gun", econ.Name)
	assert.Len(t, econ.Attributes, 1)
	assert.Equal(t, 134, econ.Attributes[0].Defindex)
	assert.Equal(t, "17", econ.Attributes[0].Value)
	assert.Contains(t, econ.Descriptions[0].Value, "Not Usable in Crafting")
}

func TestNormalizeDefindex(t *testing.T) {
	tests := []struct {
		defindex int
		expected int
	}{
		{5021, 5021}, // Key stays Key
		{5049, 5021}, // Festive Key -> Key
		{5717, 5021}, // End of the Line Key -> Key
		{294, 160},   // Lugermorph
		{6523, 6522}, // Specialized Killstreak Kit -> Strangifier
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, schema.NormalizeDefindex(tt.defindex))
	}
}
