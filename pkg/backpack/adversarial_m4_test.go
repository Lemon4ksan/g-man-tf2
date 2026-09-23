// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func backpackTestSchema() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 13, ItemName: "Scattergun", ItemQuality: 6, UsedByClasses: []string{"Scout"}},
		{Defindex: 202, ItemName: "Minigun", ItemQuality: 6, UsedByClasses: []string{"Heavy"}},
		{Defindex: 378, ItemName: "Team Captain", ItemQuality: 6},
		{Defindex: 121, ItemName: "Gentle Manne's Service Medal", ItemQuality: 6},
		{Defindex: 444, ItemName: "Pomson 6000", ItemQuality: 6},
		{Defindex: 6522, ItemName: "Strangifier", ItemQuality: 6},
		{Defindex: 20000, ItemName: "Strangifier Chemistry Set", ItemQuality: 6},
		{Defindex: 20006, ItemName: "Collector's Chemistry Set", ItemQuality: 6},
		{Defindex: 30000, ItemName: "The All-Father", ItemQuality: 6},
	}

	return schema.New(raw)
}

// ============================================================================
// CHALLENGE 5: Halloween Spells CEcon Deserialization & Robustness (1004..1009)
// ============================================================================

func TestAdversarial_HalloweenSpells_CEconDeserialization(t *testing.T) {
	s := backpackTestSchema()

	spellCases := []struct {
		name              string
		descriptionValue  string
		expectedAttribute int
		expectedValue     int
	}{
		// 1004: Halloween Paint / Voice
		{"Die Job", "Halloween: Die Job (paint)", 1004, 0},
		{"Chromatic Corruption", "Halloween: Chromatic Corruption (paint)", 1004, 1},
		{"Putrescent Pigmentation", "Halloween: Putrescent Pigmentation (paint)", 1004, 2},
		{"Spectral Spectrum", "Halloween: Spectral Spectrum (paint)", 1004, 3},
		{"Sinister Staining", "Halloween: Sinister Staining (paint)", 1004, 4},
		// 1005: Halloween Footprints
		{"Team Spirit Footprints", "Halloween: Team Spirit Footprints", 1005, 1},
		{"Headless Horseshoes", "Halloween: Headless Horseshoes", 1005, 2},
		{"Corpse Gray Footprints", "Halloween: Corpse Gray Footprints", 1005, 3100495},
		{"Violent Violet Footprints", "Halloween: Violent Violet Footprints", 1005, 5322826},
		{"Rotten Orange Footprints", "Halloween: Rotten Orange Footprints", 1005, 13595446},
		{"Bruised Purple Footprints", "Halloween: Bruised Purple Footprints", 1005, 8208497},
		{"Gangreen Footprints", "Halloween: Gangreen Footprints", 1005, 8421376},
		// 1006: Voices from Below
		{"Voices from Below", "Halloween: Voices from Below", 1006, 1},
		// 1007: Pumpkin Bombs & Projectiles
		{"Pumpkin Bombs", "Halloween: Pumpkin Bombs", 1007, 1},
		{"Gourd Grenades", "Halloween: Gourd Grenades", 1007, 1},
		{"Squash Rockets", "Halloween: Squash Rockets", 1007, 1},
		{"Sentry Quad-Pumpkins", "Halloween: Sentry Quad-Pumpkins", 1007, 1},
		// 1008: Halloween Flames
		{"Halloween Fire", "Halloween: Halloween Fire", 1008, 1},
		{"Spectral Flame", "Halloween: Spectral Flame", 1008, 1},
		// 1009: Exorcism
		{"Exorcism", "Halloween: Exorcism", 1009, 1},
	}

	for _, sc := range spellCases {
		t.Run(sc.name, func(t *testing.T) {
			econ := inventory.CEconItem{
				Asset: inventory.Asset{AssetID: "111", Amount: "1"},
				Description: inventory.Description{
					Tradable:       1,
					MarketHashName: "Minigun",
					AppData:        &inventory.AppData{DefIndex: 202, Quality: 6},
					Descriptions: []struct {
						Value string `json:"value"`
						Color string `json:"color,omitempty"`
					}{
						{Value: sc.descriptionValue, Color: "7ea9d1"},
					},
				},
			}

			// Map CEconItem to TF2Item
			item := MapCEconToTF2(econ, s)
			assert.Equal(t, 202, item.Defindex)
			assert.Equal(t, 6, item.Quality)

			// Find spell attribute in TF2Item
			var foundSpell *sku.Spell
			for _, attr := range item.Attributes {
				if attr.Defindex >= schema.DefSpellProxy {
					if sp, ok := attr.Value.(sku.Spell); ok {
						foundSpell = &sp
						break
					}
				}
			}

			require.NotNil(t, foundSpell, "Spell attribute must be present in TF2Item")
			assert.Equal(t, sc.expectedAttribute, foundSpell.Attribute)
			assert.Equal(t, sc.expectedValue, foundSpell.Value)

			// SKU serialization must contain ;s-<attr>-<val>
			expectedSKUPart := fmt.Sprintf(";s-%d-%d", sc.expectedAttribute, sc.expectedValue)
			itemSKU := item.ToSKU()
			assert.Contains(t, itemSKU, expectedSKUPart,
				"ToSKU() must contain spell tag %s, got: %s", expectedSKUPart, itemSKU)

			// ToEconItem conversion must retain spell as trading.Attribute
			econItem := item.ToEconItem()
			require.NotNil(t, econItem)

			var foundEconAttr *trading.Attribute
			for _, attr := range econItem.Attributes {
				if attr.Defindex == sc.expectedAttribute {
					foundEconAttr = &attr
					break
				}
			}

			require.NotNil(t, foundEconAttr, "ToEconItem must map spell attribute %d", sc.expectedAttribute)
			assert.Equal(t, sc.expectedAttribute, foundEconAttr.Defindex)
			assert.Equal(t, strconv.Itoa(sc.expectedValue), foundEconAttr.Value)
			assert.Equal(t, float64(sc.expectedValue), foundEconAttr.FloatValue)
		})
	}

	t.Run("MultiSpelledItem_AllSpellsPreserved", func(t *testing.T) {
		// Hat with 3 spells: Spectral Spectrum (1004-3) + Voices from Below (1006-1) + Exorcism (1009-1)
		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "222", Amount: "1"},
			Description: inventory.Description{
				Tradable:       1,
				MarketHashName: "The All-Father",
				AppData:        &inventory.AppData{DefIndex: 30000, Quality: 6},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "Halloween: Spectral Spectrum (paint)", Color: "7ea9d1"},
					{Value: "Halloween: Voices from Below", Color: "7ea9d1"},
					{Value: "Halloween: Exorcism", Color: "7ea9d1"},
				},
			},
		}

		item := MapCEconToTF2(econ, s)
		assert.Equal(t, 30000, item.Defindex)

		itemSKU := item.ToSKU()
		assert.Contains(t, itemSKU, ";s-1004-3")
		assert.Contains(t, itemSKU, ";s-1006-1")
		assert.Contains(t, itemSKU, ";s-1009-1")

		econItem := item.ToEconItem()
		require.Len(t, econItem.Attributes, 3)
		assert.Equal(t, 1004, econItem.Attributes[0].Defindex)
		assert.Equal(t, 1006, econItem.Attributes[1].Defindex)
		assert.Equal(t, 1009, econItem.Attributes[2].Defindex)
	})

	t.Run("MalformedSpellAttributes_NoPanics", func(t *testing.T) {
		malformedCases := []struct {
			name  string
			value string
			color string
		}{
			{"EmptyValue", "", "7ea9d1"},
			{"WhitespaceOnly", "   \t\n  ", "7ea9d1"},
			{"UnknownSpell", "Halloween: Ultra Spooky Phantom Beam", "7ea9d1"},
			{"NonSpellContentWithSpellColor", "Strange Score: 100", "7ea9d1"},
			{"NullBytes", "\x00\x01\x02\xff", "7ea9d1"},
			{"ExtremelyLongString", strings.Repeat("A", 10000), "7ea9d1"},
			{"SpecialCharacters", "<script>alert('xss')</script>", "7ea9d1"},
			{"EmojiAndUnicode", "🎃👻💀 Halloween Spell", "7ea9d1"},
		}

		for _, mc := range malformedCases {
			t.Run(mc.name, func(t *testing.T) {
				econ := inventory.CEconItem{
					Asset: inventory.Asset{AssetID: "333", Amount: "1"},
					Description: inventory.Description{
						Tradable:       1,
						MarketHashName: "Scattergun",
						AppData:        &inventory.AppData{DefIndex: 13, Quality: 6},
						Descriptions: []struct {
							Value string `json:"value"`
							Color string `json:"color,omitempty"`
						}{
							{Value: mc.value, Color: mc.color},
						},
					},
				}

				// Must not panic
				assert.NotPanics(t, func() {
					item := MapCEconToTF2(econ, s)
					_ = item.ToSKU()
					_ = item.ToEconItem()
				})

				// With nil schema, must also not panic
				assert.NotPanics(t, func() {
					item := MapCEconToTF2(econ, nil)
					_ = item.ToSKU()
					_ = item.ToEconItem()
				})
			})
		}
	})

	t.Run("TF2Item_ToEconItem_And_ToSKU_TypeResilience", func(t *testing.T) {
		corruptItems := []*TF2Item{
			{
				ID: 1, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: schema.DefSpellProxy, Value: nil},
				},
			},
			{
				ID: 2, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: schema.DefSpellProxy, Value: (*sku.Spell)(nil)},
				},
			},
			{
				ID: 3, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: schema.DefSpellProxy, Value: "malformed_string"},
				},
			},
			{
				ID: 4, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: schema.DefSpellProxy, Value: 12345},
				},
			},
			{
				ID: 5, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: schema.DefSpellProxy, Value: struct{ A int }{A: 1}},
				},
			},
			{
				ID: 6, Defindex: 13, Quality: 6,
				Attributes: []TF2Attribute{
					{Defindex: 1004, Value: nil},
					{Defindex: 1005, Value: "invalid"},
					{Defindex: 1009, Value: float64(1)},
				},
			},
		}

		for idx, it := range corruptItems {
			t.Run(fmt.Sprintf("CorruptItem_%d", idx+1), func(t *testing.T) {
				assert.NotPanics(t, func() {
					_ = it.ToSKU()
				}, "ToSKU() must not panic on corrupt attribute types")

				assert.NotPanics(t, func() {
					econItem := it.ToEconItem()
					require.NotNil(t, econItem)
				}, "ToEconItem() must not panic on corrupt attribute types")
			})
		}
	})
}

// ============================================================================
// CHALLENGE 6: Backpack Craft Numbers & Recipe Target/Output Deserialization
// ============================================================================

func TestAdversarial_Backpack_CraftNumbersAndRecipeTargets(t *testing.T) {
	s := backpackTestSchema()

	t.Run("CraftNumber_Descriptions_Parsing", func(t *testing.T) {
		testNumbers := []struct {
			desc        string
			defindex    int
			expectedSKU string
		}{
			{"Item #1", 378, "378;6;n1"},
			{"Item #42", 378, "378;6;n42"},
			{"Item #100", 378, "378;6;n100"},
			{"Item #101", 378, "378;6;n101"},
			{"Item #1337", 378, "378;6;n1337"},
			// Defindex 121 (Gentle Manne's Service Medal) MUST NOT parse Item # as Craft Number
			{"Item #1", 121, "121;6"},
			{"Item #42", 121, "121;6"},
			{"Item #100", 121, "121;6"},
			{"Item #1337", 121, "121;6"},
		}

		for _, tc := range testNumbers {
			econ := inventory.CEconItem{
				Asset: inventory.Asset{AssetID: "555", Amount: "1"},
				Description: inventory.Description{
					Tradable: 1,
					AppData:  &inventory.AppData{DefIndex: tc.defindex, Quality: 6},
					Descriptions: []struct {
						Value string `json:"value"`
						Color string `json:"color,omitempty"`
					}{
						{Value: tc.desc},
					},
				},
			}

			item := MapCEconToTF2(econ, s)
			assert.Equal(t, tc.expectedSKU, item.ToSKU(),
				"Item with %s for defindex %d must match expected SKU %s", tc.desc, tc.defindex, tc.expectedSKU)
		}
	})

	t.Run("RecipeTargetAndOutput_TF2Item_ToSKU_Types", func(t *testing.T) {
		// Test with float64 attributes (as produced by json.Unmarshal)
		tfItemFloat := TF2Item{
			ID:       1,
			Defindex: 20000,
			Quality:  6,
			Attributes: []TF2Attribute{
				{Defindex: schema.AttrTarget, Value: float64(444)},
				{Defindex: schema.AttrOutput, Value: float64(6522)},
				{Defindex: schema.AttrOutputQuality, Value: float64(6)},
			},
		}
		assert.Equal(t, "20000;6;td-444;od-6522;oq-6", tfItemFloat.ToSKU())

		// Test with int attributes
		tfItemInt := TF2Item{
			ID:       2,
			Defindex: 20000,
			Quality:  6,
			Attributes: []TF2Attribute{
				{Defindex: schema.AttrTarget, Value: 444},
				{Defindex: schema.AttrOutput, Value: 6522},
				{Defindex: schema.AttrOutputQuality, Value: 6},
			},
		}
		assert.Equal(t, "20000;6;td-444;od-6522;oq-6", tfItemInt.ToSKU())

		// Test Collector's Chemistry Set with float64
		collectorFloat := TF2Item{
			ID:       3,
			Defindex: 20006,
			Quality:  6,
			Attributes: []TF2Attribute{
				{Defindex: schema.AttrOutput, Value: float64(444)},
				{Defindex: schema.AttrOutputQuality, Value: float64(14)},
			},
		}
		assert.Equal(t, "20006;6;od-444;oq-14", collectorFloat.ToSKU())
	})

	t.Run("ChemistrySet_FromDescriptions_OutputParsing", func(t *testing.T) {
		// Strangifier recipe output description
		econStrangifier := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "777", Amount: "1"},
			Description: inventory.Description{
				Tradable:       1,
				MarketHashName: "Strangifier Chemistry Set",
				AppData:        &inventory.AppData{DefIndex: 20000, Quality: 6},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "You will receive all of the following outputs once all of the inputs are fulfilled."},
					{Value: "Pomson 6000 Strangifier"},
				},
			},
		}

		itemStrangifier := MapCEconToTF2(econStrangifier, s)
		assert.Equal(t, "20000;6;td-444;od-6522;oq-6", itemStrangifier.ToSKU())

		// Collector's recipe output description
		econCollector := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: "778", Amount: "1"},
			Description: inventory.Description{
				Tradable:       1,
				MarketHashName: "Collector's Chemistry Set",
				AppData:        &inventory.AppData{DefIndex: 20006, Quality: 6},
				Descriptions: []struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{
					{Value: "You will receive all of the following outputs once all of the inputs are fulfilled."},
					{Value: "Collector's Pomson 6000"},
				},
			},
		}

		itemCollector := MapCEconToTF2(econCollector, s)
		assert.Equal(t, "20006;6;od-444;oq-14", itemCollector.ToSKU())
	})
}
