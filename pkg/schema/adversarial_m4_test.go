// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func schemaForM4Adversarial() *Schema {
	raw := minimalRawSchema()

	// Ensure all required items for M4 adversarial testing are present in the mock schema
	additionalItems := []*Item{
		{
			Defindex:      13,
			Name:          "TF_WEAPON_SCATTERGUN",
			ItemName:      "Scattergun",
			ItemClass:     "tf_weapon_scattergun",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:      18,
			Name:          "TF_WEAPON_ROCKETLAUNCHER",
			ItemName:      "Rocket Launcher",
			ItemClass:     "tf_weapon_rocketlauncher",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Soldier"},
		},
		{
			Defindex:      14,
			Name:          "TF_WEAPON_SNIPERRIFLE",
			ItemName:      "Sniper Rifle",
			ItemClass:     "tf_weapon_sniperrifle",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Sniper"},
		},
		{
			Defindex:      194,
			Name:          "TF_WEAPON_KNIFE",
			ItemName:      "Knife",
			ItemClass:     "tf_weapon_knife",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Spy"},
		},
		{
			Defindex:      197,
			Name:          "TF_WEAPON_WRENCH",
			ItemName:      "Wrench",
			ItemClass:     "tf_weapon_wrench",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Engineer"},
		},
		{
			Defindex:      202,
			Name:          "TF_WEAPON_MINIGUN",
			ItemName:      "Minigun",
			ItemClass:     "tf_weapon_minigun",
			ItemQuality:   QualityUnique,
			UsedByClasses: []string{"Heavy"},
		},
		{
			Defindex:    444,
			Name:        "The Pomson 6000",
			ItemName:    "Pomson 6000",
			ItemClass:   "tf_weapon_drg_pomson",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    440,
			Name:        "Disciplinary Action",
			ItemName:    "Disciplinary Action",
			ItemClass:   "tf_weapon_riding_crop",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    6523,
			Name:        "Specialized Killstreak Kit",
			ItemName:    "Specialized Killstreak Kit",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    6524,
			Name:        "Specialized Killstreak Kit Fabricator Item",
			ItemName:    "Specialized Killstreak Kit Fabricator",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20001,
			Name:        "Cosmetic Strangifier Recipe 1 Rare",
			ItemName:    "Strangifier Chemistry Set",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20004,
			Name:        "Killstreak Kit Fabricator",
			ItemName:    "Killstreak Kit Fabricator",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20005,
			Name:        "Cosmetic Strangifier Recipe 2",
			ItemName:    "Strangifier Chemistry Set",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    5022,
			Name:        "Mann Co. Supply Crate Series 1",
			ItemName:    "Mann Co. Supply Crate",
			ItemClass:   "supply_crate",
			ItemQuality: QualityUnique,
			Attributes: []ItemAttribute{
				{Class: "supply_crate_series", Value: 1},
			},
		},
	}

	raw.Schema.Items = append(raw.Schema.Items, additionalItems...)

	// Ensure ParticleEffects map Burning Flames (13) and Sunbeams (17)
	raw.Schema.AttributeControlledAttachedParticles = append(
		raw.Schema.AttributeControlledAttachedParticles,
		&ParticleEffect{ID: 13, Name: "Burning Flames"},
		&ParticleEffect{ID: 17, Name: "Sunbeams"},
	)

	for _, it := range raw.Schema.Items {
		if it.Defindex == 20000 {
			it.Name = "Chemistry Set"
			it.ItemName = "Chemistry Set"
		}
	}

	return New(raw)
}

// ============================================================================
// CHALLENGE 1: Australium Strange Naming Parity & Stress Testing
// ============================================================================

func TestAdversarial_AustraliumStrangeNaming(t *testing.T) {
	s := schemaForM4Adversarial()

	weapons := []struct {
		defindex int
		name     string
	}{
		{202, "Minigun"},
		{13, "Scattergun"},
		{18, "Rocket Launcher"},
		{14, "Sniper Rifle"},
		{194, "Knife"},
		{197, "Wrench"},
	}

	t.Run("StandardVsSCM_StrangeAustraliumWeapons", func(t *testing.T) {
		for _, w := range weapons {
			item := &sku.Item{
				Defindex:   w.defindex,
				Quality:    QualityStrange, // 11
				Australium: true,
				Craftable:  true,
				Tradable:   true,
			}

			// Standard format (scmFormat = false) MUST prepend "Strange "
			standardName := s.ItemName(item, false, false, false)
			expectedStandard := "Strange Australium " + w.name
			assert.Equal(t, expectedStandard, standardName,
				"Standard display name must be 'Strange Australium %s' for defindex %d", w.name, w.defindex)

			// SCM format (scmFormat = true) MUST omit "Strange "
			scmName := s.ItemName(item, false, false, true)
			expectedSCM := "Australium " + w.name
			assert.Equal(t, expectedSCM, scmName,
				"SCM name must omit 'Strange ' and be 'Australium %s' for defindex %d", w.name, w.defindex)
		}
	})

	t.Run("StandardVsSCM_NonAustraliumStrangeWeapons", func(t *testing.T) {
		for _, w := range weapons {
			item := &sku.Item{
				Defindex:   w.defindex,
				Quality:    QualityStrange, // 11
				Australium: false,
				Craftable:  true,
				Tradable:   true,
			}

			// For non-Australium strange weapons, BOTH formats must contain "Strange "
			standardName := s.ItemName(item, false, false, false)
			assert.Equal(t, "Strange "+w.name, standardName)

			scmName := s.ItemName(item, false, false, true)
			assert.Equal(t, "Strange "+w.name, scmName)
		}
	})

	t.Run("StandardVsSCM_NonStrangeAustraliumWeapons", func(t *testing.T) {
		// Unique Australium (Quality 6)
		uniqueAust := &sku.Item{
			Defindex:   202,
			Quality:    QualityUnique, // 6
			Australium: true,
			Craftable:  true,
			Tradable:   true,
		}
		assert.Equal(t, "Australium Minigun", s.ItemName(uniqueAust, false, false, false),
			"Unique Australium in standard format should not have 'Strange'")
		assert.Equal(t, "Australium Minigun", s.ItemName(uniqueAust, false, false, true),
			"Unique Australium in SCM format should be 'Australium Minigun'")

		// Vintage Australium (Quality 3)
		vintageAust := &sku.Item{
			Defindex:   13,
			Quality:    QualityVintage, // 3
			Australium: true,
			Craftable:  true,
			Tradable:   true,
		}
		assert.Equal(t, "Vintage Australium Scattergun", s.ItemName(vintageAust, false, false, false))
		assert.Equal(t, "Vintage Australium Scattergun", s.ItemName(vintageAust, false, false, true))

		// Genuine Australium (Quality 1)
		genuineAust := &sku.Item{
			Defindex:   18,
			Quality:    QualityGenuine, // 1
			Australium: true,
			Craftable:  true,
			Tradable:   true,
		}
		assert.Equal(t, "Genuine Australium Rocket Launcher", s.ItemName(genuineAust, false, false, false))
		assert.Equal(t, "Genuine Australium Rocket Launcher", s.ItemName(genuineAust, false, false, true))
	})

	t.Run("StandardVsSCM_UnusualAustraliums", func(t *testing.T) {
		// Unusual Australium with particle effect (Effect 13 = Burning Flames)
		unusualAust := &sku.Item{
			Defindex:   202,
			Quality:    QualityUnusual, // 5
			Effect:     13,             // Burning Flames
			Australium: true,
			Craftable:  true,
			Tradable:   true,
		}

		// Standard format: Effect name + Australium + ItemName
		standardName := s.ItemName(unusualAust, false, false, false)
		assert.Equal(t, "Burning Flames Australium Minigun", standardName)

		// SCM format: "Unusual Australium Minigun"
		scmName := s.ItemName(unusualAust, false, false, true)
		assert.Equal(t, "Unusual Australium Minigun", scmName)

		// Unusual Australium without effect (Effect 0)
		noEffectAust := &sku.Item{
			Defindex:   13,
			Quality:    QualityUnusual, // 5
			Effect:     0,
			Australium: true,
			Craftable:  true,
			Tradable:   true,
		}
		assert.Equal(t, "Unusual Australium Scattergun", s.ItemName(noEffectAust, false, false, false))
		assert.Equal(t, "Unusual Australium Scattergun", s.ItemName(noEffectAust, false, false, true))
	})

	t.Run("KillstreakAndFestivized_AustraliumCombinations", func(t *testing.T) {
		comboItem := &sku.Item{
			Defindex:   202,
			Quality:    QualityStrange,
			Australium: true,
			Killstreak: 3, // Professional
			Festivized: true,
			Craftable:  true,
			Tradable:   true,
		}

		// Standard: "Strange Festivized Professional Killstreak Australium Minigun"
		assert.Equal(t, "Strange Festivized Professional Killstreak Australium Minigun",
			s.ItemName(comboItem, false, false, false))

		// SCM: "Festivized Professional Killstreak Australium Minigun"
		assert.Equal(t, "Festivized Professional Killstreak Australium Minigun",
			s.ItemName(comboItem, false, false, true))
	})

	t.Run("ItemFromName_RoundTripAustralium", func(t *testing.T) {
		parsed := s.ItemFromName("Strange Australium Minigun")
		require.NotNil(t, parsed)
		assert.Equal(t, 202, parsed.Defindex)
		assert.Equal(t, QualityStrange, parsed.Quality)
		assert.True(t, parsed.Australium)

		// Format back to standard and SCM
		assert.Equal(t, "Strange Australium Minigun", s.ItemName(parsed, false, false, false))
		assert.Equal(t, "Australium Minigun", s.ItemName(parsed, false, false, true))
	})
}

// ============================================================================
// CHALLENGE 2: Chemistry Sets (20000..20007) Never Have Crate Tag Appended
// ============================================================================

func TestAdversarial_ChemistrySets_NoCrateSeriesTag(t *testing.T) {
	s := schemaForM4Adversarial()

	chemDefindexes := []int{20000, 20001, 20002, 20003, 20004, 20005, 20006, 20007}
	testSeries := []int{1, 2, 3, 5, 42, 60, 100}

	t.Run("AllDefindexes_20000_to_20007_StandardFormat_NeverHasSeriesHash", func(t *testing.T) {
		for _, def := range chemDefindexes {
			for _, series := range testSeries {
				item := &sku.Item{
					Defindex:    def,
					Quality:     QualityUnique,
					Crateseries: series,
					Craftable:   true,
					Tradable:    true,
				}

				name := s.ItemName(item, false, false, false)
				assert.False(
					t,
					strings.Contains(name, fmt.Sprintf("#%d", series)),
					"Chemistry Set defindex %d with Crateseries %d must NOT contain '#%d', got: %s",
					def,
					series,
					series,
					name,
				)
				assert.False(t, strings.HasSuffix(name, fmt.Sprintf("#%d", series)),
					"Chemistry Set defindex %d must not have '#%d' suffix", def, series)
			}
		}
	})

	t.Run("StrangifierChemistrySets_TargetAndOutputFormatting", func(t *testing.T) {
		// Strangifier Chemistry Set (20000) with Target Pomson 6000 (444) and Output Strangifier (6522)
		item := &sku.Item{
			Defindex:      20000,
			Quality:       QualityUnique,
			Target:        444,
			Output:        6522,
			OutputQuality: QualityUnique,
			Crateseries:   42, // Non-zero crate series must be ignored in standard format!
			Craftable:     true,
			Tradable:      true,
		}

		standardName := s.ItemName(item, false, false, false)
		assert.Equal(t, "Pomson 6000 Strangifier Chemistry Set", standardName,
			"Standard name must format target and not include any crate series")
		assert.False(t, strings.Contains(standardName, "#42"))

		// SCM format: Strangifier Chemistry Sets with Output 6522 resolve series from target
		scmName := s.ItemName(item, false, false, true)
		// Target 444 is not in strangifierChemistrySetSeries, so no series appended on SCM
		assert.Equal(t, "Pomson 6000 Strangifier Chemistry Set", scmName)

		// Target 440 (Disciplinary Action) is mapped to Series 2 on SCM
		itemSeries2 := &sku.Item{
			Defindex:      20000,
			Quality:       QualityUnique,
			Target:        440,
			Output:        6522,
			OutputQuality: QualityUnique,
			Crateseries:   99, // Should NOT leak into standard format
			Craftable:     true,
			Tradable:      true,
		}
		assert.Equal(t, "Disciplinary Action Strangifier Chemistry Set", s.ItemName(itemSeries2, false, false, false))
		assert.Equal(
			t,
			"Disciplinary Action Strangifier Chemistry Set Series %232",
			s.ItemName(itemSeries2, false, false, true),
		)
	})

	t.Run("CollectorsChemistrySets_StandardAndSCMFormat", func(t *testing.T) {
		// Collector's Chemistry Set (20006)
		item := &sku.Item{
			Defindex:      20006,
			Quality:       QualityUnique,
			Output:        444,
			OutputQuality: QualityCollectors,
			Crateseries:   50, // Erroneous series must NOT appear
			Craftable:     true,
			Tradable:      true,
		}

		standardName := s.ItemName(item, false, false, false)
		assert.False(t, strings.Contains(standardName, "#50"),
			"Collector's Chemistry Set must never include '#50', got: %s", standardName)

		scmName := s.ItemName(item, false, false, true)
		assert.False(t, strings.Contains(scmName, "Series %23"),
			"Collector's Chemistry Set on SCM must never append crate series, got: %s", scmName)
		assert.False(t, strings.Contains(scmName, "#50"))
	})

	t.Run("SupplyCrates_LegitimateSeriesAppended", func(t *testing.T) {
		// Mann Co Supply Crate (5022) with supply_crate_series attribute MUST include series
		crate := &sku.Item{
			Defindex:    5022,
			Quality:     QualityUnique,
			Crateseries: 1,
			Craftable:   true,
			Tradable:    true,
		}

		assert.Equal(t, "Mann Co. Supply Crate #1", s.ItemName(crate, false, false, false),
			"Real supply crate must include '#1'")
		assert.Equal(t, "Mann Co. Supply Crate Series %231", s.ItemName(crate, false, false, true),
			"Real supply crate on SCM must include 'Series %231'")
	})

	t.Run("ParseChemistrySet_StripsSeriesFromInput", func(t *testing.T) {
		parsed := s.ItemFromName("Sandman Strangifier Chemistry Set Series #1")
		require.NotNil(t, parsed)
		assert.Equal(t, 20000, parsed.Defindex)
		assert.Equal(t, 0, parsed.Crateseries, "Crateseries must be 0 after stripping series tag")
	})
}

// ============================================================================
// CHALLENGE 3: Low-Craft Numbers (#1-100) vs Craft Numbers > 100
// ============================================================================

func TestAdversarial_LowCraftNumbers(t *testing.T) {
	s := schemaForM4Adversarial()

	t.Run("LowCraftNumbers_1_42_100_ItemNameFormatting", func(t *testing.T) {
		testNumbers := []int{1, 42, 100}
		for _, num := range testNumbers {
			item := &sku.Item{
				Defindex:    378, // Team Captain
				Quality:     QualityUnique,
				Craftnumber: num,
				Craftable:   true,
				Tradable:    true,
			}

			name := s.ItemName(item, false, false, false)
			expected := fmt.Sprintf("Team Captain #%d", num)
			assert.Equal(t, expected, name, "Low-craft number %d must format as '#%d'", num, num)
		}
	})

	t.Run("LowCraftNumbers_SKU_Roundtrip", func(t *testing.T) {
		testCases := []struct {
			num      int
			skuTag   string
			expected string
		}{
			{1, ";n1", "378;6;n1"},
			{42, ";n42", "378;6;n42"},
			{100, ";n100", "378;6;n100"},
		}

		for _, tc := range testCases {
			item := &sku.Item{
				Defindex:    378,
				Quality:     6,
				Craftable:   true,
				Tradable:    true,
				Craftnumber: tc.num,
			}

			// Encode
			encoded := sku.FromObject(item)
			assert.Equal(t, tc.expected, encoded, "SKU must contain %s", tc.skuTag)

			// Decode
			decoded, err := sku.FromString(encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.num, decoded.Craftnumber)
			assert.Equal(t, 378, decoded.Defindex)
			assert.Equal(t, 6, decoded.Quality)

			// Re-encode
			reEncoded := sku.FromObject(decoded)
			assert.Equal(t, tc.expected, reEncoded)
		}
	})

	t.Run("CraftNumbers_GreaterThan100_Behavior", func(t *testing.T) {
		// When specified explicitly in name query:
		explicitParsed := s.ItemFromName("Team Captain #1337")
		require.NotNil(t, explicitParsed)
		assert.Equal(t, 1337, explicitParsed.Craftnumber,
			"Explicitly specified craft number #1337 in name must be parsed")

		explicit101 := s.ItemFromName("Team Captain #101")
		require.NotNil(t, explicit101)
		assert.Equal(t, 101, explicit101.Craftnumber,
			"Explicitly specified craft number #101 in name must be parsed")

		// When NOT specified in name query:
		normalParsed := s.ItemFromName("Team Captain")
		require.NotNil(t, normalParsed)
		assert.Equal(t, 0, normalParsed.Craftnumber,
			"Unspecified craft number must default to 0")

		// SKU roundtrip for craft numbers > 100
		item1337 := &sku.Item{
			Defindex:    378,
			Quality:     6,
			Craftable:   true,
			Tradable:    true,
			Craftnumber: 1337,
		}
		encoded1337 := sku.FromObject(item1337)
		assert.Equal(t, "378;6;n1337", encoded1337)

		decoded1337, err := sku.FromString(encoded1337)
		require.NoError(t, err)
		assert.Equal(t, 1337, decoded1337.Craftnumber)
	})
}

// ============================================================================
// CHALLENGE 4: Recipe Targets & Outputs in SKU Round-Trip
// ============================================================================

func TestAdversarial_RecipeTargetsAndOutputs_SKU(t *testing.T) {
	testCases := []struct {
		name        string
		item        sku.Item
		expectedSKU string
	}{
		{
			name: "Strangifier Chemistry Set",
			item: sku.Item{
				Defindex:      20000,
				Quality:       QualityUnique,
				Craftable:     true,
				Tradable:      true,
				Target:        30000,
				Output:        6522,
				OutputQuality: 6,
			},
			expectedSKU: "20000;6;td-30000;od-6522;oq-6",
		},
		{
			name: "Collector's Chemistry Set",
			item: sku.Item{
				Defindex:      20006,
				Quality:       QualityUnique,
				Craftable:     true,
				Tradable:      true,
				Output:        444,
				OutputQuality: 14,
			},
			expectedSKU: "20006;6;od-444;oq-14",
		},
		{
			name: "Killstreak Kit Fabricator",
			item: sku.Item{
				Defindex:      20002,
				Quality:       QualityUnique,
				Craftable:     true,
				Tradable:      true,
				Target:        202,
				Output:        6523,
				OutputQuality: 6,
			},
			expectedSKU: "20002;6;td-202;od-6523;oq-6",
		},
		{
			name: "Specialized Kit Fabricator",
			item: sku.Item{
				Defindex:      20003,
				Quality:       QualityUnique,
				Craftable:     true,
				Tradable:      true,
				Target:        13,
				Output:        6524,
				OutputQuality: 6,
			},
			expectedSKU: "20003;6;td-13;od-6524;oq-6",
		},
		{
			name: "Professional Kit Fabricator",
			item: sku.Item{
				Defindex:      20004,
				Quality:       QualityUnique,
				Craftable:     true,
				Tradable:      true,
				Target:        18,
				Output:        6526,
				OutputQuality: 6,
			},
			expectedSKU: "20004;6;td-18;od-6526;oq-6",
		},
		{
			name: "Strangifier Item with Target",
			item: sku.Item{
				Defindex:  6522,
				Quality:   QualityUnique,
				Craftable: true,
				Tradable:  true,
				Target:    30000,
			},
			expectedSKU: "6522;6;td-30000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Serialization from Object
			serialized := sku.FromObject(&tc.item)
			assert.Equal(t, tc.expectedSKU, serialized, "FromObject must produce expected SKU")

			// 2. Deserialization from String
			decoded, err := sku.FromString(serialized)
			require.NoError(t, err, "FromString must decode without error")
			assert.Equal(t, tc.item.Defindex, decoded.Defindex)
			assert.Equal(t, tc.item.Quality, decoded.Quality)
			assert.Equal(t, tc.item.Target, decoded.Target)
			assert.Equal(t, tc.item.Output, decoded.Output)
			assert.Equal(t, tc.item.OutputQuality, decoded.OutputQuality)

			// 3. Re-serialization idempotency
			reSerialized := sku.FromObject(decoded)
			assert.Equal(t, tc.expectedSKU, reSerialized, "Round-trip re-serialization must match")
		})
	}
}
