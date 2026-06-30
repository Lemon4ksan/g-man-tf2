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
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("basic_mapping_and_parsing", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: "100",
				Amount:  "1",
			},
			Description: inventory.Description{
				Tradable: 1,
				AppData: map[string]any{
					"def_index": "13",
					"quality":   "6",
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(100), item.ID)
		assert.Equal(t, 13, item.Defindex)
		assert.Equal(t, 6, item.Quality)
		assert.False(t, item.FlagCannotTrade)
		assert.False(t, item.FlagCannotCraft)
	})

	t.Run("invalid_amount_parsing", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: "101",
				Amount:  "not_a_number",
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.Equal(t, 1, item.Quantity)
	})

	t.Run("original_id_as_float", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "102"},
			Description: inventory.Description{
				AppData: map[string]any{
					"original_id": float64(54321),
				},
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(54321), item.OriginalID)
	})

	t.Run("original_id_as_string", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "103"},
			Description: inventory.Description{
				AppData: map[string]any{
					"original_id": "12345",
				},
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(12345), item.OriginalID)
	})

	t.Run("fallback_defindex_from_market_hash_name", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "104"},
			Description: inventory.Description{
				Name:           "Paint Can",
				MarketHashName: "Paint Can",
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.Equal(t, 13, item.Defindex)
	})

	t.Run("descriptions_parsing", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "105"},
			Description: inventory.Description{
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "( Not Usable in Crafting )"},
					{Value: "Exterior: Field-Tested"},
					{Value: "★ Unusual Effect: Sunbeams"},
					{Value: "Killstreak Active: Specialized"},
					{Value: "Paint Color: Sunbeams Skin"},
					{Value: "Crate Series #85"},
					{Value: "Strange Stat: Kills: 0"},
					{Value: "Strange Part: Robots Destroyed: 0", Color: "756b5e"},
					{Value: "Exorcism", Color: "7ea9d1"},
				},
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.True(t, item.FlagCannotCraft)
		assert.NotEmpty(t, item.Attributes)
	})

	t.Run("unparseable_appdata_strings", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "106"},
			Description: inventory.Description{
				AppData: map[string]any{
					"def_index":   "not_a_number",
					"quality":     "not_a_number",
					"original_id": "not_a_number",
				},
			},
		}
		item := mapCEconToTF2(econ, s)
		assert.Equal(t, 0, item.Defindex)
		assert.Equal(t, 0, item.Quality)
		assert.Equal(t, uint64(0), item.OriginalID)
	})
}

func TestTF2Item_ToSKU(t *testing.T) {
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
			want: "100;15;w3;pk10",
		},
		{
			name: "paint_and_crate_series",
			item: TF2Item{
				Defindex: 5021,
				Quality:  6,
				Attributes: []TF2Attribute{
					{Defindex: 142, Value: float64(12345)},
					{Defindex: 187, Value: float64(10)},
				},
			},
			want: "5021;6;c10;p12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.item.ToSKU())
		})
	}
}

func TestTF2Item_ToEconItem(t *testing.T) {
	t.Parallel()

	t.Run("basic_conversion_and_description", func(t *testing.T) {
		item := TF2Item{
			ID:         123,
			Defindex:   1,
			Quantity:   1,
			CustomName: "My Gun",
			CustomDesc: "Custom Desc",
			Attributes: []TF2Attribute{
				{Defindex: 134, Value: float64(17)},
				{Defindex: 135, Value: "string_value"},
			},
			FlagCannotCraft: true,
		}

		econ := item.ToEconItem()
		assert.Equal(t, uint64(123), econ.AssetID)
		assert.Equal(t, uint64(1), econ.ClassID)
		assert.Equal(t, "My Gun", econ.Name)
		assert.Len(t, econ.Attributes, 2)
		assert.Equal(t, 134, econ.Attributes[0].Defindex)
		assert.Equal(t, "17", econ.Attributes[0].Value)
		assert.Equal(t, "string_value", econ.Attributes[1].Value)
		assert.Contains(t, econ.Descriptions[0].Value, "Not Usable in Crafting")
		assert.Contains(t, econ.Descriptions[1].Value, "Custom Desc")
	})
}

func TestModels_CEconToTF2_EdgeCases(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 13, ItemQuality: 11, ItemName: "Australium Scattergun"},
	}
	s := schema.New(raw)

	t.Run("australium_detection_by_name", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "100"},
			Description: inventory.Description{
				MarketHashName: "Australium Scattergun",
				AppData: map[string]any{
					"def_index": "13",
					"quality":   "11",
				},
			},
		}

		item := mapCEconToTF2(econ, s)

		hasAustralium := false
		for _, attr := range item.Attributes {
			if attr.Defindex == schema.AttrAustralium {
				hasAustralium = true
				break
			}
		}

		assert.True(t, hasAustralium)
	})

	t.Run("festive_detection_by_name", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "100"},
			Description: inventory.Description{
				Name: "Festivized Scattergun",
				AppData: map[string]any{
					"def_index": "13",
					"quality":   "6",
				},
			},
		}

		item := mapCEconToTF2(econ, s)

		hasFestive := false
		for _, attr := range item.Attributes {
			if attr.Defindex == schema.AttrFestivized {
				hasFestive = true
				break
			}
		}

		assert.True(t, hasFestive)
	})

	t.Run("strange_part_and_spells_parsing", func(t *testing.T) {
		rawCustom := &schema.Raw{}
		rawCustom.Schema.Items = []*schema.Item{
			{Defindex: 13, ItemQuality: 11},
		}
		sCustom := schema.New(rawCustom)

		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "100"},
			Description: inventory.Description{
				AppData: map[string]any{
					"def_index": "13",
					"quality":   "11",
				},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "Halloween: Team Spirit Footprints", Color: "7ea9d1"},
					{Value: "Kills: 0", Color: "756b5e"},
				},
			},
		}

		item := mapCEconToTF2(econ, sCustom)
		hasSpell := false

		hasPart := false
		for _, attr := range item.Attributes {
			if attr.Defindex >= schema.DefSpellProxy {
				hasSpell = true
			}

			if attr.Defindex >= schema.DefPartsProxy && attr.Defindex < schema.DefSpellProxy {
				hasPart = true
			}
		}

		assert.True(t, hasSpell)
		assert.True(t, hasPart)
	})
}

func TestModels_ToSKU_Comprehensive(t *testing.T) {
	t.Parallel()

	item := TF2Item{
		Defindex: 1,
		Quality:  6,
		Attributes: []TF2Attribute{
			{Defindex: schema.AttrUnusualEffect, Value: float64(17)},
			{Defindex: schema.AttrWear, Value: float64(0.2)},
			{Defindex: schema.AttrAustralium, Value: float64(1)},
			{Defindex: schema.AttrPaintkit, Value: float64(10)},
			{Defindex: schema.AttrKillstreak, Value: float64(3)},
			{Defindex: schema.AttrFestivized, Value: float64(1)},
			{Defindex: schema.AttrPaintColor, Value: float64(142)},
			{Defindex: schema.AttrCrateSeries, Value: float64(85)},
			{Defindex: schema.AttrStrangeScore, Value: float64(1)},
			{Defindex: schema.DefSpellProxy, Value: sku.Spell{Attribute: 1004, Value: 3}},
			{Defindex: schema.DefPartsProxy, Value: float64(10)},
		},
	}

	skuStr := item.ToSKU()
	assert.NotEmpty(t, skuStr)
	assert.Contains(t, skuStr, "u17")
	assert.Contains(t, skuStr, "australium")
	assert.Contains(t, skuStr, "pk10")
	assert.Contains(t, skuStr, "kt-3")
	assert.Contains(t, skuStr, "festive")
	assert.Contains(t, skuStr, "p142")
	assert.Contains(t, skuStr, "c85")
	assert.Contains(t, skuStr, "s-1004-3")
	assert.Contains(t, skuStr, "sp10")
}

func TestMapCEconToTF2_GlitchedInventory(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("strange_festivized_prof_ks_holy_mackerel_with_spell", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: "5244338466",
				Amount:  "1",
			},
			Description: inventory.Description{
				Tradable:       1,
				MarketHashName: "Strange Festivized Professional Killstreak Holy Mackerel",
				AppData: map[string]any{
					"def_index": "221",
					"quality":   "11",
				},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "(Player Hits: 0)", Color: "756b5e"},
					{Value: "Festivized", Color: "ffd700"},
					{Value: "Halloween: Exorcism (spell only active during event)", Color: "7ea9d1"},
					{Value: "Killstreaker: Cerebral Discharge", Color: "7ea9d1"},
					{Value: "Sheen: Team Shine", Color: "7ea9d1"},
					{Value: "Killstreaks Active", Color: "7ea9d1"},
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.Equal(t, uint64(5244338466), item.ID)
		assert.Equal(t, 221, item.Defindex)
		assert.Equal(t, 11, item.Quality)

		hasKS3 := false

		hasSpell := false
		for _, attr := range item.Attributes {
			if attr.Defindex == schema.AttrKillstreak && attr.Value == float64(3) {
				hasKS3 = true
			}

			if attr.Defindex >= schema.DefSpellProxy {
				hasSpell = true
			}
		}

		assert.True(t, hasKS3, "Should correctly extract Professional Killstreak (Tier 3) from community description")
		assert.True(t, hasSpell, "Should correctly extract Halloween Spell from community description")
	})

	t.Run("prof_ks_three_rune_blade_with_spell", func(t *testing.T) {
		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: "261014391",
				Amount:  "1",
			},
			Description: inventory.Description{
				Tradable:       1,
				MarketHashName: "Professional Killstreak Three-Rune Blade",
				AppData: map[string]any{
					"def_index": "457",
					"quality":   "6",
				},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "On Hit: Bleed for 5 seconds", Color: "7ea9d1"},
					{Value: "Halloween: Exorcism (spell only active during event)", Color: "7ea9d1"},
					{Value: "Killstreaker: Flames", Color: "7ea9d1"},
					{Value: "Sheen: Team Shine", Color: "7ea9d1"},
					{Value: "Killstreaks Active", Color: "7ea9d1"},
				},
			},
		}

		item := mapCEconToTF2(econ, s)
		assert.Equal(t, 457, item.Defindex)

		hasKS3 := false
		for _, attr := range item.Attributes {
			if attr.Defindex == schema.AttrKillstreak && attr.Value == float64(3) {
				hasKS3 = true
			}
		}

		assert.True(t, hasKS3, "Should extract Professional Killstreak (Tier 3)")
	})
}

func TestStrangeIsotopeFlameThrower_SKURoundtrip(t *testing.T) {
	t.Parallel()

	targetSKU := "208;11;u702;w3;pk417;kt-3;festive"

	// 1. Verify SKU parsing via sku.FromString
	itemObj, err := sku.FromString(targetSKU)
	assert.NoError(t, err, "Should successfully parse target SKU string")

	assert.Equal(t, 208, itemObj.Defindex, "Defindex must be 208 (Flame Thrower)")
	assert.Equal(t, 11, itemObj.Quality, "Quality must be 11 (Strange)")
	assert.Equal(t, 702, itemObj.Effect, "Effect must be 702 (Isotope)")
	assert.Equal(t, 3, itemObj.Wear, "Wear must be 3 (Field-Tested)")
	assert.Equal(t, 417, itemObj.Paintkit, "Paintkit must be 417 (Team Serviced)")
	assert.Equal(t, 3, itemObj.Killstreak, "Killstreak must be 3 (Professional)")
	assert.True(t, itemObj.Festivized, "Item must be Festivized")

	// 2. Verify idempotency via sku.FromObject
	generatedSKU := sku.FromObject(itemObj)
	assert.Equal(t, targetSKU, generatedSKU, "Generated SKU must exactly match target SKU")

	// 3. Verify mapCEconToTF2 parsing for Community inventory format
	s := mockSchemaForCoverage()
	econ := inventory.CEconItem{
		Asset: inventory.Asset{AssetID: "9999999", Amount: "1"},
		Description: inventory.Description{
			Tradable:       1,
			MarketHashName: "Strange Isotope Festivized Professional Killstreak Team Serviced Flame Thrower (Field-Tested)",
			AppData: map[string]any{
				"def_index": "208",
				"quality":   "11",
			},
			Descriptions: []struct {
				Value string `json:"value"`
				Color string `json:"color,omitempty"`
			}{
				{Value: "★ Unusual Effect: Isotope", Color: "ffd700"},
				{Value: "Festivized", Color: "ffd700"},
				{Value: "Killstreaker: Incinerator", Color: "7ea9d1"},
				{Value: "Sheen: Deadly Daffodil", Color: "7ea9d1"},
				{Value: "Killstreaks Active", Color: "7ea9d1"},
			},
		},
	}

	tfItem := mapCEconToTF2(econ, s)
	assert.Equal(t, 208, tfItem.Defindex)
	assert.Equal(t, 11, tfItem.Quality)
}
