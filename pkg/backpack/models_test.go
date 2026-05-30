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

func mockSchemaWithSunbeams() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      1,
			ItemQuality:   6,
			UsedByClasses: []string{"Scout", "Sniper"},
		},
	}
	raw.Schema.AttributeControlledAttachedParticles = []*schema.ParticleEffect{
		{ID: 17, Name: "Sunbeams"},
	}

	return schema.New(raw)
}

func TestMapCEconToTF2_EconMapping_ReturnsNormalizedTF2Item(t *testing.T) {
	t.Parallel()

	s := mockSchemaWithSunbeams()

	t.Run("basic_item", func(t *testing.T) {
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

	t.Run("uncraftable_item", func(t *testing.T) {
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

	t.Run("unusual_item_with_effect", func(t *testing.T) {
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
		assert.Len(t, item.Attributes, 1)
		assert.Equal(t, schema.AttrUnusualEffect, item.Attributes[0].Defindex)
		assert.Equal(t, float64(17), item.Attributes[0].Value)
	})
}

func TestTF2Item_ToSKU_AttributesAndQualities_GeneratesCorrectSKU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item TF2Item
		want string
	}{
		{
			name: "unique_weapon",
			item: TF2Item{Defindex: 1, Quality: 6, FlagCannotTrade: false},
			want: "1;6",
		},
		{
			name: "uncraftable_unique_weapon",
			item: TF2Item{Defindex: 1, Quality: 6, FlagCannotCraft: true, FlagCannotTrade: false},
			want: "1;6;uncraftable",
		},
		{
			name: "unusual_hat",
			item: TF2Item{
				Defindex:        100,
				Quality:         5,
				FlagCannotTrade: false,
				Attributes: []TF2Attribute{
					{Defindex: 134, Value: float64(17)},
				},
			},
			want: "100;5;u17",
		},
		{
			name: "australium",
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
			name: "strange_unusual_elevated",
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
			name: "strange_parts",
			item: TF2Item{
				Defindex: 1,
				Quality:  11,
				Attributes: []TF2Attribute{
					{Defindex: 10000, Value: float64(10)},
					{Defindex: 10001, Value: float64(12)},
				},
			},
			want: "1;11;sp10;sp12",
		},
		{
			name: "spells",
			item: TF2Item{
				Defindex: 1,
				Quality:  6,
				Attributes: []TF2Attribute{
					{Defindex: 11000, Value: sku.Spell{Attribute: 1004, Value: 3}},
				},
			},
			want: "1;6;s-1004-3",
		},
		{
			name: "decorated_skin_paintkit_and_wear",
			item: TF2Item{
				Defindex: 100,
				Quality:  15,
				Attributes: []TF2Attribute{
					{Defindex: 834, Value: float64(10)},
					{Defindex: 725, Value: float64(0.2)},
				},
			},
			want: "100;15;w1;pk10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.item.ToSKU())
		})
	}
}

func TestTF2Item_ToEconItem_ValidTF2Item_MapsCorrectly(t *testing.T) {
	t.Parallel()

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

func TestNormalizeDefindex_VariousInputs_ReturnsCanonicalID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		defindex int
		expected int
	}{
		{"key_remains_key", 5021, 5021},
		{"festive_key_normalizes_to_key", 5049, 5021},
		{"eotl_key_normalizes_to_key", 5717, 5021},
		{"lugermorph_normalizes", 294, 160},
		{"killstreak_kit_normalizes_to_strangifier", 6523, 6522},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, schema.NormalizeDefindex(tt.defindex))
		})
	}
}
