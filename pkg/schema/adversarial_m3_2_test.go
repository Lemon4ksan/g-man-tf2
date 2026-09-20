// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/trading"
)

// ============================================================================
// OBJECTIVE 1: Defindex Corruption Fix & Legacy Parity (pkg/schema/ids.go)
// ============================================================================

func TestAdversarial_DefindexCorruptionAndParity(t *testing.T) {
	s := New(minimalRawSchema())

	t.Run("KillstreakKits_NotCorruptedTo6522", func(t *testing.T) {
		// Specialized Killstreak Kit (6523) must retain 6523 and NEVER normalize to 6522
		assert.Equal(t, 6523, NormalizeDefindex(6523), "Specialized Killstreak Kit must remain 6523")
		assert.Equal(t, 6523, s.NormalizeDefindex(6523), "s.NormalizeDefindex(6523) must remain 6523")

		// Professional Killstreak Kit (6526) must retain 6526 and NEVER normalize to 6522
		assert.Equal(t, 6526, NormalizeDefindex(6526), "Professional Killstreak Kit must remain 6526")
		assert.Equal(t, 6526, s.NormalizeDefindex(6526), "s.NormalizeDefindex(6526) must remain 6526")
	})

	t.Run("Strangifiers_All16NormalizeTo6522", func(t *testing.T) {
		strangifierDefs := []int{
			5661, // Pomson 6000 Strangifier
			5721, // Pretty Boy's Pocket Pistol Strangifier
			5722, // Phlogistinator Strangifier
			5723, // Cleaner's Carbine Strangifier
			5724, // Private Eye Strangifier
			5725, // Big Chief Strangifier
			5753, // Air Strike Strangifier
			5754, // Classic Strangifier
			5755, // Manmelter Strangifier
			5756, // Vaccinator Strangifier
			5757, // Widowmaker Strangifier
			5758, // Anger Strangifier
			5759, // Apparition's Aspect Strangifier
			5783, // Cow Mangler 5000 Strangifier
			5784, // Third Degree Strangifier
			5804, // Righteous Bison Strangifier
		}
		require.Len(t, strangifierDefs, 16, "Must test exactly 16 Strangifiers")

		for _, def := range strangifierDefs {
			assert.Equal(t, 6522, NormalizeDefindex(def), "Strangifier %d must normalize to 6522", def)
			assert.Equal(t, 6522, s.NormalizeDefindex(def), "s.NormalizeDefindex(%d) must normalize to 6522", def)
		}
	})

	t.Run("BasicKillstreakKits_All26NormalizeTo6527", func(t *testing.T) {
		kitDefs := []int{
			5726, // Rocket Launcher
			5727, // Scattergun
			5728, // Sniper Rifle
			5729, // Shotgun
			5730, // Ubersaw
			5731, // GRU
			5732, // Spy-cicle
			5733, // Axtinguisher
			5743, // Sticky Launcher
			5744, // Minigun
			5745, // Direct Hit
			5746, // Huntsman
			5747, // Backburner
			5748, // Backscatter
			5749, // Kritzkrieg
			5750, // Ambassador
			5751, // Frontier Justice
			5793, // Flaregun
			5794, // Wrench
			5795, // Revolver
			5796, // Machina
			5797, // Baby Face Blaster
			5798, // Huo Long Heatmaker
			5799, // Loose Cannon
			5800, // Vaccinator
			5801, // Air Strike
		}
		require.Len(t, kitDefs, 26, "Must test exactly 26 Basic Killstreak Kits")

		for _, def := range kitDefs {
			assert.Equal(t, 6527, NormalizeDefindex(def), "Basic KS Kit %d must normalize to 6527", def)
			assert.Equal(t, 6527, s.NormalizeDefindex(def), "s.NormalizeDefindex(%d) must normalize to 6527", def)
		}
	})

	t.Run("ChemistrySets_StrangifierRecipesNormalizeTo20000", func(t *testing.T) {
		chemDefs := []int{
			20001, // Cosmetic Strangifier Recipe 1 Rare
			20005, // Cosmetic Strangifier Recipe 2
			20008, // Rebuild Strange Weapon Recipe
			20009, // Cosmetic Strangifier Recipe 3
		}
		require.Len(t, chemDefs, 4, "Must test exactly 4 Strangifier Chemistry Sets")

		for _, def := range chemDefs {
			assert.Equal(t, 20000, NormalizeDefindex(def), "Chemistry Set %d must normalize to 20000", def)
			assert.Equal(t, 20000, s.NormalizeDefindex(def), "s.NormalizeDefindex(%d) must normalize to 20000", def)
		}
	})

	t.Run("ChemistrySets_NonStrangifiersMustNotNormalizeTo20000", func(t *testing.T) {
		nonStrangifierChem := []int{
			20002, // Specialized Killstreak Kit Fabricator
			20003, // Professional Killstreak Kit Fabricator
			20006, // Collector's Chemistry Set
			20007, // Festive Collector's Chemistry Set
		}
		for _, def := range nonStrangifierChem {
			assert.NotEqual(t, 20000, NormalizeDefindex(def), "Item %d must NOT normalize to 20000", def)
			assert.Equal(t, def, NormalizeDefindex(def), "Item %d must preserve its defindex", def)
			assert.Equal(t, def, s.NormalizeDefindex(def), "s.NormalizeDefindex(%d) must preserve its defindex", def)
		}
	})

	t.Run("StockpileCrate_5738NormalizesTo5737", func(t *testing.T) {
		assert.Equal(t, 5737, NormalizeDefindex(5738), "Crate 5738 must normalize to 5737")
		assert.Equal(t, 5737, s.NormalizeDefindex(5738), "s.NormalizeDefindex(5738) must normalize to 5737")
		// Assert 5737 itself stays 5737
		assert.Equal(t, 5737, NormalizeDefindex(5737), "Crate 5737 must remain 5737")
	})

	t.Run("OtherPreviouslyCorruptedIDs_Preserved", func(t *testing.T) {
		// Strange Filters & Medals that were previously mismapped
		preserved := []int{6520, 6521, 6530, 6531, 6532, 6534, 11051, 11052}
		for _, def := range preserved {
			assert.Equal(t, def, NormalizeDefindex(def), "Defindex %d must retain itself", def)
			assert.NotEqual(t, 6522, NormalizeDefindex(def), "Defindex %d must not be 6522", def)
			assert.NotEqual(t, 6527, NormalizeDefindex(def), "Defindex %d must not be 6527", def)
		}
	})

	t.Run("StandardItemIDs_RemainUnchanged", func(t *testing.T) {
		unmodified := []int{0, -1, 13, 160, 200, 378, 5021, 6522, 6527, 20000, 999999}
		for _, def := range unmodified {
			assert.Equal(t, def, NormalizeDefindex(def))
		}
	})
}

// ============================================================================
// OBJECTIVE 2: Decorated Weapon War Paint Retention & Quality (pkg/schema/schema.go)
// ============================================================================

func setupDecoratedSchema() *Schema {
	raw := minimalRawSchema()
	if raw.Schema.PaintKits == nil {
		raw.Schema.PaintKits = make(map[string]string)
	}
	// Add paintkits:
	// Carpet Bomber Mk.II (ID 279)
	// Carpet Bomber (ID 123) - shorter prefix to challenge sort order!
	// Civic Duty (ID 205)
	raw.Schema.PaintKits["279"] = "Carpet Bomber Mk.II"
	raw.Schema.PaintKits["123"] = "Carpet Bomber"
	raw.Schema.PaintKits["205"] = "Civic Duty"

	// Register items
	raw.Schema.Items = append(raw.Schema.Items,
		&Item{
			Defindex:    15002,
			Name:        "Scattergun",
			ItemName:    "Scattergun",
			ItemClass:   "tf_weapon_scattergun",
			ItemQuality: QualityDecorated,
		},
		&Item{
			Defindex:    15003,
			Name:        "Shotgun",
			ItemName:    "Shotgun",
			ItemClass:   "tf_weapon_shotgun",
			ItemQuality: QualityDecorated,
		},
		&Item{
			Defindex:    200,
			Name:        "Scattergun",
			ItemName:    "Scattergun",
			ItemClass:   "tf_weapon_scattergun",
			ItemQuality: QualityUnique,
		},
	)

	// Add effects
	raw.Schema.AttributeControlledAttachedParticles = append(raw.Schema.AttributeControlledAttachedParticles,
		&ParticleEffect{ID: 701, Name: "Hot"},
		&ParticleEffect{ID: 702, Name: "Isotope"},
		&ParticleEffect{ID: 703, Name: "Cool"},
	)

	return New(raw)
}

func TestAdversarial_DecoratedWeaponWarPaintRetentionAndQuality(t *testing.T) {
	s := setupDecoratedSchema()

	t.Run("Unique_CarpetBomberMkII_Scattergun", func(t *testing.T) {
		item := &trading.Item{
			MarketHashName: "Carpet Bomber Mk.II Scattergun (Field-Tested)",
			Tradable:       true,
			Tags: []trading.Tag{
				{Category: "Exterior", LocalizedName: "Field-Tested"},
			},
		}

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)

		// Assert Paintkit is retained (279) and NOT stripped or matched to shorter "Carpet Bomber" (123)
		assert.Equal(t, 279, skuItem.Paintkit, "Must match longest paint kit 'Carpet Bomber Mk.II' (279), not 'Carpet Bomber' (123)")
		// Assert Wear is Field-Tested (3)
		assert.Equal(t, 3, skuItem.Wear)
		// Assert weapon name / defindex is preserved (Scattergun skin defindex 15002, NOT generic war paint tool 16189)
		assert.Equal(t, 15002, skuItem.Defindex, "Decorated weapon must retain weapon defindex 15002")
		// Assert Quality is Decorated (15)
		assert.Equal(t, QualityDecorated, skuItem.Quality)
		assert.Equal(t, 0, skuItem.Quality2)
	})

	t.Run("Strange_CarpetBomberMkII_Scattergun_Quality15AndQuality211", func(t *testing.T) {
		item := &trading.Item{
			MarketHashName: "Strange Carpet Bomber Mk.II Scattergun (Factory New)",
			Tradable:       true,
			Tags: []trading.Tag{
				{Category: "Quality", LocalizedName: "Strange"},
				{Category: "Exterior", LocalizedName: "Factory New"},
			},
		}

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)

		// 1. Paintkit is retained and not stripped
		assert.Equal(t, 279, skuItem.Paintkit, "Paintkit must be retained as 279")
		// 2. Weapon defindex is preserved
		assert.Equal(t, 15002, skuItem.Defindex, "Weapon defindex must be preserved as 15002")
		// 3. Wear is Factory New (1)
		assert.Equal(t, 1, skuItem.Wear)
		// 4. Quality == 15 (QualityDecorated) and Quality2 == 11 (QualityStrange)
		assert.Equal(t, QualityDecorated, skuItem.Quality, "Strange Decorated weapon must have Quality == 15 (QualityDecorated)")
		assert.Equal(t, QualityStrange, skuItem.Quality2, "Strange Decorated weapon must have Quality2 == 11 (QualityStrange)")
	})

	t.Run("Unusual_CarpetBomberMkII_Scattergun_RetainsPaintkitAndEffect", func(t *testing.T) {
		item := &trading.Item{
			MarketHashName: "Unusual Carpet Bomber Mk.II Scattergun (Minimal Wear)",
			Tradable:       true,
			Tags: []trading.Tag{
				{Category: "Quality", LocalizedName: "Unusual"},
				{Category: "Exterior", LocalizedName: "Minimal Wear"},
			},
			Descriptions: []trading.Description{
				{Value: "★ Unusual Effect: Hot"},
			},
		}

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)

		// 1. Paintkit is retained
		assert.Equal(t, 279, skuItem.Paintkit, "Paintkit must be retained as 279")
		// 2. Weapon defindex is preserved
		assert.Equal(t, 15002, skuItem.Defindex)
		// 3. Wear is Minimal Wear (2)
		assert.Equal(t, 2, skuItem.Wear)
		// 4. Unusual Effect is 701 (Hot)
		assert.Equal(t, 701, skuItem.Effect, "Unusual Effect Hot must be 701")
		// 5. Quality
		assert.Equal(t, QualityDecorated, skuItem.Quality, "Unusual weapon skin quality is 15")
	})

	t.Run("StrangeUnusual_CarpetBomberMkII_Scattergun", func(t *testing.T) {
		item := &trading.Item{
			MarketHashName: "Strange Unusual Carpet Bomber Mk.II Scattergun (Field-Tested)",
			Tradable:       true,
			Tags: []trading.Tag{
				{Category: "Quality", LocalizedName: "Strange"},
				{Category: "Exterior", LocalizedName: "Field-Tested"},
			},
			Descriptions: []trading.Description{
				{Value: "★ Unusual Effect: Isotope"},
			},
		}

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)

		assert.Equal(t, 279, skuItem.Paintkit, "Paintkit must be retained as 279")
		assert.Equal(t, 15002, skuItem.Defindex, "Defindex must be 15002")
		assert.Equal(t, 3, skuItem.Wear, "Wear must be Field-Tested (3)")
		assert.Equal(t, 702, skuItem.Effect, "Effect must be Isotope (702)")
		assert.Equal(t, QualityDecorated, skuItem.Quality, "Quality must be 15")
		assert.Equal(t, QualityStrange, skuItem.Quality2, "Quality2 must be 11")
	})

	t.Run("ItemFromName_DecoratedWeaponWithPrefixes", func(t *testing.T) {
		// Non-Strange decorated weapon from name
		parsed := s.ItemFromName("Carpet Bomber Mk.II Scattergun (Field-Tested)")
		require.NotNil(t, parsed)
		assert.Equal(t, 279, parsed.Paintkit)
		assert.Equal(t, 15002, parsed.Defindex)
		assert.Equal(t, 3, parsed.Wear)
		assert.Equal(t, QualityDecorated, parsed.Quality)

		// Prefix order stress test: Carpet Bomber vs Carpet Bomber Mk.II
		parsedShorter := s.ItemFromName("Carpet Bomber Scattergun (Field-Tested)")
		require.NotNil(t, parsedShorter)
		assert.Equal(t, 123, parsedShorter.Paintkit, "Should match 123 for Carpet Bomber")
	})

	t.Run("EmpiricalFinding_BattleScarredWearAndCaseSensitivityBug", func(t *testing.T) {
		// Empirical verification of bug in ids.go:333 and schema.go:52:
		// Steam Community Market & Steam Inventory tags use canonical "Battle-Scarred" (with hyphen)
		// and capitalized tags like "Field-Tested".
		// 1. WearByName fails on canonical capitalized tags because it lacks strings.ToLower
		assert.Equal(t, 0, s.WearByName("Field-Tested"), "BUG: WearByName('Field-Tested') returns 0 due to case-sensitivity")
		assert.Equal(t, 3, s.WearByName("field-tested"), "WearByName('field-tested') returns 3 only when lowercase")

		// 2. WearByName fails on canonical "Battle-Scarred" even if lowercase because of missing hyphen
		assert.Equal(t, 0, s.WearByName("battle-scarred"), "BUG: WearByName('battle-scarred') returns 0 because wears map has '(battle scarred)'")
		assert.Equal(t, 5, s.WearByName("battle scarred"), "WearByName returns 5 only without hyphen")

		// 3. Cascading impact on Decorated weapons:
		// Canonical market name "(Battle-Scarred)" fails wear detection in ItemFromName
		parsedCanonical := s.ItemFromName("Carpet Bomber Mk.II Scattergun (Battle-Scarred)")
		require.NotNil(t, parsedCanonical)
		assert.Equal(t, 0, parsedCanonical.Wear, "BUG: Wear is not extracted from canonical '(Battle-Scarred)'")
		assert.Equal(t, 0, parsedCanonical.Paintkit, "BUG: Paintkit is 0 because wear was not detected")

		// Conversely, unhyphenated "(battle scarred)" succeeds
		parsedUnhyphenated := s.ItemFromName("Carpet Bomber Mk.II Scattergun (battle scarred)")
		require.NotNil(t, parsedUnhyphenated)
		assert.Equal(t, 5, parsedUnhyphenated.Wear)
		assert.Equal(t, 279, parsedUnhyphenated.Paintkit)
	})
}

// ============================================================================
// OBJECTIVE 3: EconItem Defindex Extraction (pkg/schema/schema.go)
// ============================================================================

func TestAdversarial_EconItemDefindexExtraction(t *testing.T) {
	raw := minimalRawSchema()
	raw.Schema.Items = append(raw.Schema.Items,
		&Item{Defindex: 560, Name: "Item 560", ItemName: "Item 560"},
		&Item{Defindex: 30217, Name: "Item 30217", ItemName: "Item 30217"},
		&Item{Defindex: 5661, Name: "Pomson Strangifier", ItemName: "Pomson Strangifier"},
		&Item{Defindex: 5726, Name: "Rocket Launcher Kit", ItemName: "Rocket Launcher Kit"},
	)
	s := New(raw)

	t.Run("DefindexFromAppData_IgnoresClassID", func(t *testing.T) {
		item := &trading.Item{
			ClassID: 99999999, // Bogus ClassID
			Descriptions: []trading.Description{
				{
					Value: "Some descriptive text",
					AppData: &struct {
						Defindex int `json:"def_index,string"`
					}{Defindex: 560},
				},
			},
		}

		def := s.DefindexFromEconItem(item)
		assert.Equal(t, 560, def, "DefindexFromEconItem must return 560 from AppData, NOT ClassID 99999999")

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)
		assert.Equal(t, 560, skuItem.Defindex, "ItemFromEconItem must return 560, NOT ClassID 99999999")
	})

	t.Run("DefindexFromActionURL_IgnoresClassID", func(t *testing.T) {
		item := &trading.Item{
			ClassID: 88888888, // Bogus ClassID
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "http://wiki.teamfortress.com/scripts/itemredirect.php?id=30217&lang=en_US",
				},
			},
		}

		def := s.DefindexFromEconItem(item)
		assert.Equal(t, 30217, def, "DefindexFromEconItem must return 30217 from Action URL, NOT ClassID 88888888")

		skuItem := s.ItemFromEconItem(item)
		require.NotNil(t, skuItem)
		assert.Equal(t, 30217, skuItem.Defindex, "ItemFromEconItem must return 30217, NOT ClassID 88888888")
	})

	t.Run("ActionURL_AdversarialQueryParameters", func(t *testing.T) {
		// Case A: id is second parameter
		itemA := &trading.Item{
			ClassID: 11111111,
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "http://wiki.teamfortress.com/scripts/itemredirect.php?lang=en_US&id=30217",
				},
			},
		}
		assert.Equal(t, 30217, s.DefindexFromEconItem(itemA), "Must parse 'id' when it is not the first query param")

		// Case B: id is surrounded by other parameters
		itemB := &trading.Item{
			ClassID: 22222222,
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "https://wiki.teamfortress.com/scripts/itemredirect.php?utm_source=steam&id=560&extra=123",
				},
			},
		}
		assert.Equal(t, 560, s.DefindexFromEconItem(itemB), "Must parse 'id' when surrounded by params")

		// Case C: Non-numeric id
		itemC := &trading.Item{
			ClassID: 33333333,
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "https://wiki.teamfortress.com/scripts/itemredirect.php?id=invalid_text",
				},
			},
		}
		assert.Equal(t, 0, s.DefindexFromEconItem(itemC), "Non-numeric id must return 0, not ClassID")

		// Case D: Negative or zero id
		itemD := &trading.Item{
			ClassID: 44444444,
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "https://wiki.teamfortress.com/scripts/itemredirect.php?id=-10",
				},
			},
		}
		assert.Equal(t, 0, s.DefindexFromEconItem(itemD), "Negative id must return 0, not ClassID")
	})

	t.Run("DefindexExtraction_NormalizesDefindexes", func(t *testing.T) {
		// Strangifier 5661 in Action URL -> should normalize to 6522
		itemStrangifier := &trading.Item{
			ClassID: 55555555,
			Actions: []trading.Action{
				{
					Name: "Item Wiki Page...",
					Link: "http://wiki.teamfortress.com/scripts/itemredirect.php?id=5661",
				},
			},
		}
		assert.Equal(t, 6522, s.DefindexFromEconItem(itemStrangifier), "Pomson Strangifier 5661 must normalize to 6522")

		// Basic Kit 5726 in AppData -> should normalize to 6527
		itemKit := &trading.Item{
			ClassID: 66666666,
			Descriptions: []trading.Description{
				{
					AppData: &struct {
						Defindex int `json:"def_index,string"`
					}{Defindex: 5726},
				},
			},
		}
		assert.Equal(t, 6527, s.DefindexFromEconItem(itemKit), "Basic KS Kit 5726 must normalize to 6527")
	})

	t.Run("DefindexExtraction_FallbackChainAndNilSafety", func(t *testing.T) {
		// Nil item
		assert.Equal(t, 0, s.DefindexFromEconItem(nil))
		assert.Nil(t, s.ItemFromEconItem(nil))

		// Item with ClassID but unknown name and no AppData/Actions -> returns 0, never ClassID
		unknownItem := &trading.Item{
			ClassID:        987654321,
			MarketHashName: "NonExistentTF2Item_ABCD_1234",
		}
		assert.Equal(t, 0, s.DefindexFromEconItem(unknownItem), "Unresolved defindex must return 0, NEVER ClassID")
		unknownSKU := s.ItemFromEconItem(unknownItem)
		require.NotNil(t, unknownSKU)
		assert.Equal(t, 0, unknownSKU.Defindex, "skuItem must have defindex 0, NEVER ClassID")

		// Item with empty Descriptions slice and empty Actions slice
		emptyMetadataItem := &trading.Item{
			ClassID:      12345,
			Descriptions: []trading.Description{},
			Actions:      []trading.Action{},
		}
		assert.Equal(t, 0, s.DefindexFromEconItem(emptyMetadataItem))

		// Item with Description having nil AppData
		nilAppDataItem := &trading.Item{
			ClassID: 54321,
			Descriptions: []trading.Description{
				{Value: "No AppData here", AppData: nil},
			},
		}
		assert.Equal(t, 0, s.DefindexFromEconItem(nilAppDataItem))
	})
}
