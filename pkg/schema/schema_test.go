// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func minimalRawSchema() *Raw {
	items := []*Item{
		{
			Defindex:      5021,
			Name:          "Scattergun",
			ItemName:      "Scattergun",
			ItemClass:     "weapon",
			ItemQuality:   QualityUnique,
			ProperName:    false,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:     378,
			Name:         "Team Captain",
			ItemName:     "Team Captain",
			ItemClass:    "tf_wearable",
			ItemQuality:  QualityUnique,
			Capabilities: &Capabilities{Paintable: true},
		},
		{Defindex: 15013, Name: "Pistol", ItemName: "Pistol", ItemClass: "weapon", ItemQuality: QualityDecorated},
		{
			Defindex:    16189,
			Name:        "Paintkit 102",
			ItemName:    "Woodsy Widowmaker War Paint",
			ItemClass:   "tool",
			ItemQuality: QualityDecorated,
		},
		{
			Defindex:    5022,
			Name:        "Crate 1",
			ItemName:    "Mann Co. Supply Crate",
			ItemClass:   "supply_crate",
			ItemQuality: QualityUnique,
		},
		{Defindex: 160, Name: "Lugermorph", ItemName: "Lugermorph", ItemClass: "weapon", ItemQuality: QualityVintage},
		{
			Defindex:    294,
			Name:        "Promo Lugermorph",
			ItemName:    "Promo Lugermorph",
			ItemClass:   "weapon",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:      38,
			Name:          "Scout Weapon",
			ItemName:      "Scout Weapon",
			ItemClass:     "weapon",
			ItemQuality:   QualityUnique,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:     100,
			Name:         "Cosmetic",
			ItemName:     "Cosmetic",
			ItemClass:    "tf_wearable",
			ItemQuality:  QualityUnique,
			Capabilities: &Capabilities{Paintable: true},
		},
		{
			Defindex:    5739,
			Name:        "Seriesless Crate",
			ItemName:    "Seriesless Crate",
			ItemClass:   "supply_crate",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    6526,
			Name:        "Professional Killstreak Kit",
			ItemName:    "Professional Killstreak Kit",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{Defindex: 6522, Name: "Strangifier", ItemName: "Strangifier", ItemClass: "tool", ItemQuality: QualityUnique},
		{
			Defindex:    20006,
			Name:        "Collector's Chemistry Set",
			ItemName:    "Collector's Chemistry Set",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20000,
			Name:        "Strangifier Chemistry Set",
			ItemName:    "Strangifier Chemistry Set",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20003,
			Name:        "Professional Killstreak Kit Fabricator",
			ItemName:    "Professional Killstreak Kit Fabricator",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    20002,
			Name:        "Specialized Killstreak Kit Fabricator",
			ItemName:    "Specialized Killstreak Kit Fabricator",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{Defindex: 9258, Name: "Unusualifier", ItemName: "Unusualifier", ItemClass: "tool", ItemQuality: QualityUnique},
		{
			Defindex:    5713,
			Name:        "Spooky Key",
			ItemName:    "Spooky Key",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    5049,
			Name:        "Festive Winter Crate Key",
			ItemName:    "Festive Winter Crate Key",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    5791,
			Name:        "Naughty Winter Crate Key 2014",
			ItemName:    "Naughty Winter Crate Key 2014",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    5734,
			Name:        "Munition Crate",
			ItemName:    "Munition Crate",
			ItemClass:   "supply_crate",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    810,
			Name:        "Exclusive Genuine Item",
			ItemName:    "Exclusive Genuine Item",
			ItemClass:   "tf_wearable",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    831,
			Name:        "Exclusive Genuine Reversed Item",
			ItemName:    "Exclusive Genuine Reversed Item",
			ItemClass:   "tf_wearable",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    9991,
			Name:        "Base Unusual Item",
			ItemName:    "Base Unusual Item",
			ItemClass:   "tf_wearable",
			ItemQuality: QualityUnusual,
		},
		{
			Defindex:    20007,
			Name:        "Chemistry Set",
			ItemName:    "Chemistry Set",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
		{
			Defindex:    2093,
			Name:        "Name Tag",
			ItemName:    "Name Tag",
			ItemClass:   "tool",
			ItemQuality: 0,
		},
		{
			Defindex:    12345,
			Name:        "Strange Part: Robots Destroyed",
			ItemName:    "Strange Part: Robots Destroyed",
			ItemClass:   "tool",
			ItemQuality: QualityUnique,
		},
	}

	qualities := map[string]int{
		"Normal": 0, "Genuine": 1, "Vintage": 3, "Unusual": 5, "Unique": 6, "Community": 7,
		"Valve": 8, "Self-Made": 9, "Customized": 10, "Strange": 11, "Completed": 12,
		"Haunted": 13, "Collector's": 14, "Decorated": 15,
	}

	qualityNames := map[string]string{
		"Normal":      "Normal",
		"Genuine":     "Genuine",
		"Vintage":     "Vintage",
		"Unusual":     "Unusual",
		"Unique":      "Unique",
		"Community":   "Community",
		"Valve":       "Valve",
		"Self-Made":   "Self-Made",
		"Customized":  "Customized",
		"Strange":     "Strange",
		"Completed":   "Completed",
		"Haunted":     "Haunted",
		"Collector's": "Collector's",
		"Decorated":   "Decorated",
	}

	particles := []*ParticleEffect{
		{ID: 13, Name: "Burning Flames"},
		{ID: 33, Name: "Orbiting Fire"},
		{ID: 103, Name: "Ether Trail"},
		{ID: 141, Name: "Fragmenting Reality"},
	}

	paintKits := map[string]string{
		"102":   "Woodsy Widowmaker",
		"15013": "Pistol Skin",
	}

	itemsGame := map[string]any{
		"items": map[string]any{
			"5022": map[string]any{
				"static_attrs": map[string]any{
					"set supply crate series": map[string]any{
						"value": float64(1),
					},
				},
			},
		},
		"recipes": map[string]any{
			"3": `
			"3"
			{
				"name" "Smelt Weapons"
				"disabled" "1"
				"premium_only" "1"
				"all_same_class" "1"
				"all_same_slot" "1"
				"category" "commonitem"
				"input_items"
				{
					"2"
					{
						"conditions"
						{
							"1"
							{
								"field" "defindex"
								"value" "5021"
							}
						}
					}
				}
				"output_items"
				{
					"1"
					{
						"conditions"
						{
							"1"
							{
								"field" "name"
								"value" "Scrap"
							}
						}
					}
				}
			}`,
		},
	}

	killEater := []*KillEaterScoreType{
		{Type: 0, TypeName: "Kills"},
		{Type: 1, TypeName: "Kill Assists"},
		{Type: 97, TypeName: "Something Excluded"},
	}

	return &Raw{
		Schema: struct {
			Items                                []*Item               `json:"items"`
			Attributes                           []*AttributeSchema    `json:"attributes"`
			Qualities                            map[string]int        `json:"qualities"`
			QualityNames                         map[string]string     `json:"qualityNames"`
			OriginNames                          []*OriginName         `json:"originNames"`
			ItemSets                             []*ItemSet            `json:"item_sets"`
			AttributeControlledAttachedParticles []*ParticleEffect     `json:"attribute_controlled_attached_particles"`
			ItemLevels                           []*ItemLevel          `json:"item_levels"`
			KillEaterScoreTypes                  []*KillEaterScoreType `json:"kill_eater_score_types"`
			StringLookups                        []*StringLookup       `json:"string_lookups"`
			PaintKits                            map[string]string     `json:"paintkits"`
		}{
			Items:                                items,
			Qualities:                            qualities,
			QualityNames:                         qualityNames,
			AttributeControlledAttachedParticles: particles,
			PaintKits:                            paintKits,
			KillEaterScoreTypes:                  killEater,
		},
		ItemsGame: itemsGame,
	}
}

func TestSchema_New_ValidRawPayload_IndexesCorrectly(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()

	s := New(raw)
	if s == nil {
		t.Fatal("New returned nil")
	}

	if len(s.itemsByDef) != len(raw.Schema.Items) {
		t.Errorf("expected %d itemsByDef, got %d", len(raw.Schema.Items), len(s.itemsByDef))
	}

	if len(s.itemsByName) != len(raw.Schema.Items)-1 {
		t.Errorf("expected %d itemsByName, got %d", len(raw.Schema.Items)-1, len(s.itemsByName))
	}

	expectedEff := 0
	for _, p := range raw.Schema.AttributeControlledAttachedParticles {
		if p.Name != "" {
			expectedEff++
		}
	}

	if len(s.effByID) < expectedEff {
		t.Errorf("expected at least %d effByID, got %d", expectedEff, len(s.effByID))
	}
}

func TestSchema_ItemByDef_ExistingItem_ReturnsItem(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	item := s.ItemByDef(5022)
	if item == nil {
		t.Fatal("item 5022 not found")
	}

	if item.Defindex != 5022 {
		t.Errorf("expected defindex 5022, got %d", item.Defindex)
	}
}

func TestSchema_ItemByName_VariousCases_ExpectedResult(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	item := s.ItemByName("Mann Co. Supply Crate")
	if item == nil {
		t.Fatal("item not found")
	}

	if item.Defindex != 5022 {
		t.Errorf("expected defindex 5022, got %d", item.Defindex)
	}

	item = s.ItemByName("mann co. supply crate")
	if item == nil {
		t.Error("case insensitive lookup failed")
	}
}

func TestSchema_QualityByIDAndName_StandardQualities_MatchesIDs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name string
		id   int
		want string
	}{
		{"unique", 6, "Unique"},
		{"strange", 11, "Strange"},
		{"unusual", 5, "Unusual"},
		{"genuine", 1, "Genuine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if name := s.QualityByID(tt.id); name != tt.want {
				t.Errorf("QualityByID(%d): expected %s, got %s", tt.id, tt.want, name)
			}

			if id := s.QualityIDByName(tt.want); id != tt.id {
				t.Errorf("QualityIDByName(%s): expected %d, got %d", tt.want, tt.id, id)
			}
		})
	}

	t.Run("unknown_id", func(t *testing.T) {
		t.Parallel()

		if name := s.QualityByID(99); name != "" {
			t.Errorf("expected empty for unknown id, got %s", name)
		}
	})

	t.Run("unknown_name", func(t *testing.T) {
		t.Parallel()

		if id := s.QualityIDByName("nonexistent"); id != 0 {
			t.Errorf("expected 0, got %d", id)
		}
	})
}

func TestSchema_EffectByIDAndName_StandardEffects_MatchesIDs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name string
		id   int
		want string
	}{
		{"orbiting_fire", 33, "Orbiting Fire"},
		{"ether_trail", 103, "Ether Trail"},
		{"fragmenting_reality", 141, "Fragmenting Reality"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if name := s.EffectByID(tt.id); name != tt.want {
				t.Errorf("EffectByID(%d): expected %s, got %s", tt.id, tt.want, name)
			}

			if id := s.EffectIDByName(tt.want); id != tt.id {
				t.Errorf("EffectIDByName(%s): expected %d, got %d", tt.want, tt.id, id)
			}

			if id := s.EffectIDByName(strings.ToLower(tt.want)); id != tt.id {
				t.Errorf("Case insensitive EffectIDByName failed for %s", tt.want)
			}
		})
	}

	t.Run("unknown_effect", func(t *testing.T) {
		t.Parallel()

		if name := s.EffectByID(999); name != "" {
			t.Errorf("expected empty for unknown effect, got %s", name)
		}
	})
}

func TestSchema_SpellIDByName_VariousSpellNames_ExpectedIDs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name       string
		expected   sku.Spell
		shouldFind bool
	}{
		{"exorcism", sku.Spell{Attribute: 1009, Value: 1}, true},
		{"voices_from_below", sku.Spell{Attribute: 1006, Value: 1}, true},
		{"spectral_spectrum", sku.Spell{Attribute: 1004, Value: 3}, true},
		{"voices_case_insensitive", sku.Spell{Attribute: 1006, Value: 1}, true},
		{"nonexistent", sku.Spell{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var query string
			switch tt.name {
			case "exorcism":
				query = "Exorcism"
			case "voices_from_below":
				query = "Voices from Below"
			case "spectral_spectrum":
				query = "Spectral Spectrum"
			case "voices_case_insensitive":
				query = "voices from below"
			default:
				query = "Nonexistent"
			}

			spell, ok := s.SpellIDByName(query)
			if ok != tt.shouldFind {
				t.Errorf("SpellIDByName(%s) ok = %v, want %v", query, ok, tt.shouldFind)
			}

			if ok && spell != tt.expected {
				t.Errorf("SpellIDByName(%s) = %+v, want %+v", query, spell, tt.expected)
			}
		})
	}
}

func TestSchema_SkinByIDAndName_ValidAndInvalid_MatchesIDs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	t.Run("valid_skin_by_id", func(t *testing.T) {
		t.Parallel()

		if name := s.SkinByID(15013); name != "Pistol Skin" {
			t.Errorf("expected Pistol Skin, got %s", name)
		}
	})

	t.Run("invalid_skin_by_id", func(t *testing.T) {
		t.Parallel()

		if name := s.SkinByID(999); name != "" {
			t.Errorf("expected empty, got %s", name)
		}
	})

	t.Run("valid_skin_by_name", func(t *testing.T) {
		t.Parallel()

		if id := s.SkinIDByName("Pistol Skin"); id != 15013 {
			t.Errorf("expected 15013, got %d", id)
		}
	})

	t.Run("case_insensitive_by_name", func(t *testing.T) {
		t.Parallel()

		if id := s.SkinIDByName("pistol skin"); id != 15013 {
			t.Errorf("case insensitive failed, got %d", id)
		}
	})
}

func TestSchema_CheckExistence_VariousItems_ValidatesAvailability(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name     string
		item     *sku.Item
		expected bool
	}{
		{
			"valid_unique_weapon",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: true, Tradable: true},
			true,
		},
		{"invalid_quality_for_weapon", &sku.Item{Defindex: 5021, Quality: 0}, false},
		{"valid_crate_with_series", &sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 1}, true},
		{
			"invalid_crate_with_extra_attrs",
			&sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 1, Killstreak: 1},
			false,
		},
		{"valid_seriesless_crate", &sku.Item{Defindex: 5739, Quality: QualityUnique}, true},
		{
			"invalid_seriesless_crate_with_series",
			&sku.Item{Defindex: 5739, Quality: QualityUnique, Crateseries: 5},
			false,
		},
		{"unusual_without_effect", &sku.Item{Defindex: 5739, Quality: QualityUnusual, Crateseries: 0}, false},
		{"non_existent_item", &sku.Item{Defindex: 9999, Quality: QualityUnique}, false},
		{
			"invalid_unusual_with_quality2",
			&sku.Item{Defindex: 15013, Quality: QualityDecorated, Quality2: QualityStrange},
			false,
		},
		{"invalid_unique_crate_series", &sku.Item{Defindex: 5021, Quality: QualityUnique, Crateseries: 1}, false},
		{"crate_invalid_series", &sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 99}, false},
		{"spooky_key_craftable", &sku.Item{Defindex: 5713, Quality: QualityUnique, Craftable: true}, false},
		{"spooky_key_uncraftable", &sku.Item{Defindex: 5713, Quality: QualityUnique, Craftable: false}, true},
		{"festive_key_uncraftable", &sku.Item{Defindex: 5049, Quality: QualityUnique, Craftable: false}, false},
		{"festive_key_craftable", &sku.Item{Defindex: 5049, Quality: QualityUnique, Craftable: true}, true},
		{"naughty_key_uncraftable", &sku.Item{Defindex: 5791, Quality: QualityUnique, Craftable: false}, true},
		{"munition_crate_correct_series", &sku.Item{Defindex: 5734, Quality: QualityUnique, Crateseries: 82}, true},
		{"munition_crate_incorrect_series", &sku.Item{Defindex: 5734, Quality: QualityUnique, Crateseries: 83}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := s.CheckExistence(tt.item)
			if result != tt.expected {
				t.Errorf("CheckExistence() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSchema_ItemName_EdgeCases_ReturnsFormatedNames(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name      string
		item      *sku.Item
		scmFormat bool
		expected  string
	}{
		{
			name: "basic_crate",
			item: &sku.Item{
				Defindex:    5022,
				Quality:     QualityUnique,
				Crateseries: 1,
				Craftable:   true,
				Tradable:    true,
			},
			expected: "Mann Co. Supply Crate #1",
		},
		{
			name: "specialized_killstreak",
			item: &sku.Item{
				Defindex:    5022,
				Quality:     QualityUnique,
				Crateseries: 1,
				Killstreak:  2,
				Craftable:   true,
				Tradable:    true,
			},
			expected: "Specialized Killstreak Mann Co. Supply Crate #1",
		},
		{
			name:     "strange_weapon",
			item:     &sku.Item{Defindex: 5021, Quality: QualityStrange, Craftable: true, Tradable: true},
			expected: "Strange Scattergun",
		},
		{
			name:     "unusual_weapon_without_scm_format",
			item:     &sku.Item{Defindex: 5021, Quality: QualityUnusual, Effect: 33, Craftable: true, Tradable: true},
			expected: "Orbiting Fire Scattergun",
		},
		{
			name:      "unusual_weapon_with_scm_format",
			item:      &sku.Item{Defindex: 5021, Quality: QualityUnusual, Effect: 33, Craftable: true, Tradable: true},
			scmFormat: true,
			expected:  "Unusual Scattergun",
		},
		{
			name: "australium",
			item: &sku.Item{
				Defindex:   5021,
				Quality:    QualityUnique,
				Australium: true,
				Craftable:  true,
				Tradable:   true,
			},
			expected: "Australium Scattergun",
		},
		{
			name:     "non_craftable",
			item:     &sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: false, Tradable: true},
			expected: "Non-Craftable Scattergun",
		},
		{
			name:     "non_tradable",
			item:     &sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: true, Tradable: false},
			expected: "Non-Tradable Scattergun",
		},
		{
			name: "festivized",
			item: &sku.Item{
				Defindex:   5021,
				Quality:    QualityUnique,
				Festivized: true,
				Craftable:  true,
				Tradable:   true,
			},
			expected: "Festivized Scattergun",
		},
		{
			name: "craft_number",
			item: &sku.Item{
				Defindex:    5021,
				Quality:     QualityUnique,
				Craftnumber: 42,
				Craftable:   true,
				Tradable:    true,
			},
			expected: "Scattergun #42",
		},
		{
			name: "strange_unusual_elevated",
			item: &sku.Item{
				Defindex:  378,
				Quality:   QualityUnusual,
				Quality2:  11,
				Effect:    33,
				Craftable: true,
				Tradable:  true,
			},
			expected: "Strange Orbiting Fire Team Captain",
		},
		{
			name:     "kit_target",
			item:     &sku.Item{Defindex: 6526, Quality: QualityUnique, Target: 5021, Craftable: true, Tradable: true},
			expected: "Scattergun Professional Killstreak Kit",
		},
		{
			name: "wear_factory_new_skin",
			item: &sku.Item{
				Defindex:  15013,
				Quality:   QualityDecorated,
				Paintkit:  102,
				Wear:      1,
				Craftable: true,
				Tradable:  true,
			},
			expected: "Woodsy Widowmaker Pistol (Factory New)",
		},
		{
			name: "spells",
			item: &sku.Item{
				Defindex:  5021,
				Quality:   QualityUnique,
				Craftable: true,
				Tradable:  true,
				Spells:    []sku.Spell{{Attribute: 1009, Value: 1}, {Attribute: 1004, Value: 3}},
			},
			expected: "Scattergun (Spell: Exorcism) (Spell: Spectral Spectrum)",
		},
		{
			name: "strange_parts",
			item: &sku.Item{
				Defindex:  5021,
				Quality:   QualityStrange,
				Craftable: true,
				Tradable:  true,
				Parts:     []int{0},
			},
			expected: "Strange Scattergun (Kills: 0)",
		},
		{
			name: "strange_elevated_decorated_skin",
			item: &sku.Item{
				Defindex:  15013,
				Quality:   QualityDecorated,
				Quality2:  11,
				Paintkit:  102,
				Wear:      1,
				Craftable: true,
				Tradable:  true,
			},
			expected: "Strange(e) Woodsy Widowmaker Pistol (Factory New)",
		},
		{
			name: "strange_quality2_with_quality_unique",
			item: &sku.Item{
				Defindex:  5021,
				Quality:   QualityUnique,
				Quality2:  QualityStrange,
				Craftable: true,
				Tradable:  true,
			},
			expected: "Strange Unique Scattergun",
		},
		{
			name:     "unusual_quality_without_effect_with_scm",
			item:     &sku.Item{Defindex: 378, Quality: QualityUnusual, Effect: 0, Craftable: true, Tradable: true},
			expected: "Unusual Team Captain",
		},
		{
			name:      "unusual_quality_with_effect_with_scm",
			item:      &sku.Item{Defindex: 378, Quality: QualityUnusual, Effect: 13, Craftable: true, Tradable: true},
			scmFormat: true,
			expected:  "Unusual Team Captain",
		},
		{
			name:     "unusual_quality_defined_in_raw_schema",
			item:     &sku.Item{Defindex: 9991, Quality: QualityUnusual, Craftable: true, Tradable: true},
			expected: "Unusual Base Unusual Item",
		},
		{
			name:     "basic_killstreak",
			item:     &sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 1, Craftable: true, Tradable: true},
			expected: "Killstreak Scattergun",
		},
		{
			name:     "professional_killstreak",
			item:     &sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 3, Craftable: true, Tradable: true},
			expected: "Professional Killstreak Scattergun",
		},
		{
			name: "chemistry_set_output_quality",
			item: &sku.Item{
				Defindex:      20007,
				Quality:       QualityUnique,
				Output:        160,
				OutputQuality: QualityCollectors,
				Craftable:     true,
				Tradable:      true,
			},
			expected: "Collector's Lugermorph Chemistry Set",
		},
		{
			name: "chemistry_set_vdf_series_formatting",
			item: &sku.Item{
				Defindex:  20007,
				Quality:   QualityUnique,
				Target:    647,
				Output:    6522,
				Craftable: true,
				Tradable:  true,
			},
			scmFormat: true,
			expected:  "Strangifier Chemistry Set Series %231",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			name := s.ItemName(tt.item, true, false, tt.scmFormat)
			if name != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, name)
			}
		})
	}
}

func TestSchema_ItemFromName_EdgeCases_ParsesNames(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name     string
		expected *sku.Item
	}{
		{
			"scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: true, Tradable: true},
		},
		{
			"strange_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityStrange, Craftable: true, Tradable: true},
		},
		{
			"mann_co_supply_crate_1",
			&sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 1, Craftable: true, Tradable: true},
		},
		{
			"orbiting_fire_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnusual, Effect: 33, Craftable: true, Tradable: true},
		},
		{
			"specialized_killstreak_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 2, Craftable: true, Tradable: true},
		},
		{
			"australium_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Australium: true, Craftable: true, Tradable: true},
		},
		{
			"non_craftable_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: false, Tradable: true},
		},
		{
			"non_tradable_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Craftable: true, Tradable: false},
		},
		{
			"festivized_scattergun",
			&sku.Item{Defindex: 5021, Quality: QualityUnique, Festivized: true, Craftable: true, Tradable: true},
		},
		{
			"team_captain_1337",
			&sku.Item{Defindex: 378, Quality: QualityUnique, Craftnumber: 1337, Craftable: true, Tradable: true},
		},
		{
			"professional_killstreak_kit_scattergun",
			&sku.Item{Defindex: 6526, Quality: QualityUnique, Target: 5021, Craftable: true, Tradable: true},
		},
		{
			"woodsy_widowmaker_pistol_field_tested",
			&sku.Item{
				Defindex:  15013,
				Quality:   QualityDecorated,
				Paintkit:  102,
				Wear:      3,
				Craftable: true,
				Tradable:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var query string
			switch tt.name {
			case "scattergun":
				query = "Scattergun"
			case "strange_scattergun":
				query = "Strange Scattergun"
			case "mann_co_supply_crate_1":
				query = "Mann Co. Supply Crate #1"
			case "orbiting_fire_scattergun":
				query = "Orbiting Fire Scattergun"
			case "specialized_killstreak_scattergun":
				query = "Specialized Killstreak Scattergun"
			case "australium_scattergun":
				query = "Australium Scattergun"
			case "non_craftable_scattergun":
				query = "Non-Craftable Scattergun"
			case "non_tradable_scattergun":
				query = "Non-Tradable Scattergun"
			case "festivized_scattergun":
				query = "Festivized Scattergun"
			case "team_captain_1337":
				query = "Team Captain #1337"
			case "professional_killstreak_kit_scattergun":
				query = "Professional Killstreak Kit Scattergun"
			case "woodsy_widowmaker_pistol_field_tested":
				query = "Woodsy Widowmaker Pistol (Field-Tested)"
			}

			item := s.ItemFromName(query)

			if item.Defindex != tt.expected.Defindex ||
				item.Quality != tt.expected.Quality ||
				item.Killstreak != tt.expected.Killstreak ||
				item.Craftable != tt.expected.Craftable ||
				item.Tradable != tt.expected.Tradable ||
				item.Australium != tt.expected.Australium ||
				item.Festivized != tt.expected.Festivized ||
				item.Craftnumber != tt.expected.Craftnumber ||
				item.Target != tt.expected.Target ||
				item.Wear != tt.expected.Wear {
				t.Errorf("ItemFromName(%q) mismatch.\nExpected: %+v\nGot: %+v", query, tt.expected, item)
			}
		})
	}
}

func TestSchema_SkuFromName_VariousItems_GeneratesSKUs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name     string
		expected string
	}{
		{"scattergun", "5021;6"},
		{"strange_scattergun", "5021;11"},
		{"non_craftable_scattergun", "5021;6;uncraftable"},
		{"specialized_killstreak_scattergun", "5021;6;kt-2"},
		{"orbiting_fire_team_captain", "378;5;u33"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var query string
			switch tt.name {
			case "scattergun":
				query = "Scattergun"
			case "strange_scattergun":
				query = "Strange Scattergun"
			case "non_craftable_scattergun":
				query = "Non-Craftable Scattergun"
			case "specialized_killstreak_scattergun":
				query = "Specialized Killstreak Scattergun"
			case "orbiting_fire_team_captain":
				query = "Orbiting Fire Team Captain"
			}

			skuStr := s.SkuFromName(query)
			if skuStr != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, skuStr)
			}
		})
	}
}

func TestSchema_CrateSeriesList_ValidPayload_MapsCorrectly(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	series := s.CrateSeriesList()
	if val, ok := series[5022]; !ok || val != 1 {
		t.Errorf("expected series 1 for def 5022, got %v", val)
	}

	if _, ok := series[5739]; ok {
		t.Errorf("did not expect def 5739 (seriesless) to be in series list")
	}
}

func TestSchema_CraftableWeaponsSchema_ValidPayload_ReturnsWeapons(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())
	weapons := s.CraftableWeaponsSchema()

	if len(weapons) != 2 {
		t.Errorf("expected 2 weapons, got %d", len(weapons))
	}

	foundScattergun := false
	for _, w := range weapons {
		if w.Defindex == 5021 {
			foundScattergun = true
			break
		}
	}

	if !foundScattergun {
		t.Error("scattergun not found in craftable weapons")
	}
}

func TestSchema_WeaponsForCraftingByClass_ScoutAndDemoman_MatchesExpectations(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	skus := s.WeaponsForCraftingByClass("Scout")
	if len(skus) != 2 || skus[0] != "5021;6" || skus[1] != "38;6" {
		t.Errorf("expected[5021;6 38;6], got %v", skus)
	}

	skusDemo := s.WeaponsForCraftingByClass("Demoman")
	if len(skusDemo) != 0 {
		t.Errorf("expected[], got %v", skusDemo)
	}
}

func TestSchema_UnusualEffects_ValidPayload_ReturnsEffects(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())
	effects := s.UnusualEffects()

	if len(effects) < 4 {
		t.Errorf("expected at least 4 effects, got %d", len(effects))
	}

	found := false
	for _, e := range effects {
		if e.Name == "Orbiting Fire" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Orbiting Fire not found in effects list")
	}
}

func TestSchema_PaintableItemDefindexes_ValidPayload_ReturnsItemIDs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())
	paintable := s.PaintableItemDefindexes()

	if len(paintable) == 0 {
		t.Fatal("expected at least 1 paintable item")
	}

	if !slices.Contains(paintable, 378) {
		t.Error("Team Captain (378) not found in paintable item defindexes")
	}
}

func createMockSchema() *Schema {
	items := []*Item{
		{Defindex: 13, Name: "TF_WEAPON_SCATTERGUN", ItemClass: "tf_weapon_scattergun"},
		{Defindex: 200, Name: "Upgradeable TF_WEAPON_SCATTERGUN", ItemClass: "tf_weapon_scattergun"},
		{Defindex: 5020, ItemName: "Mann Co. Supply Crate Key"},
		{Defindex: 212, ItemName: "Lugermorph"},
		{Defindex: 5726, ItemName: "Killstreak Kit"},
		{Defindex: 851, Name: "AWPer Hand", ItemName: "AWPer Hand", CraftClass: "weapon"},
		{Defindex: 801, Name: "Promo AWPer Hand", ItemName: "AWPer Hand", CraftClass: ""},
		{Defindex: 5022, ItemClass: "supply_crate"},
		{Defindex: 100, ItemName: "Team Captain"},
	}

	raw := &Raw{}
	raw.Schema.Items = items

	s := &Schema{
		Raw:             raw,
		itemsByDef:      make(map[int]*Item),
		crateSeriesList: map[int]int{5022: 42},
	}

	for _, item := range items {
		s.itemsByDef[item.Defindex] = item
	}

	return s
}

func TestSchema_IsPromoItem_VariousItems_ValidatesPromoStatus(t *testing.T) {
	t.Parallel()

	s := &Schema{}

	tests := []struct {
		name     string
		item     *Item
		expected bool
	}{
		{
			name:     "valid_promo_item",
			item:     &Item{Name: "Promo AWPer Hand", CraftClass: ""},
			expected: true,
		},
		{
			name:     "promo_prefix_with_craft_class",
			item:     &Item{Name: "Promo Hat", CraftClass: "hat"},
			expected: false,
		},
		{
			name:     "empty_craft_class_no_promo_prefix",
			item:     &Item{Name: "AWPer Hand", CraftClass: ""},
			expected: false,
		},
		{
			name:     "regular_item",
			item:     &Item{Name: "Scattergun", CraftClass: "weapon"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := s.IsPromoItem(tt.item); got != tt.expected {
				t.Errorf("IsPromoItem() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSchema_NormalizeItem_VariousItems_NormalizesExpectedly(t *testing.T) {
	t.Parallel()

	s := createMockSchema()

	tests := []struct {
		name     string
		input    sku.Item
		expected sku.Item
	}{
		{
			name:     "unknown_item",
			input:    sku.Item{Defindex: 99999},
			expected: sku.Item{Defindex: 99999},
		},
		{
			name:     "upgradeable_weapon_fix",
			input:    sku.Item{Defindex: 13},
			expected: sku.Item{Defindex: 200},
		},
		{
			name:     "key_standardization",
			input:    sku.Item{Defindex: 5049},
			expected: sku.Item{Defindex: 5021},
		},
		{
			name:     "lugermorph_standardization",
			input:    sku.Item{Defindex: 294},
			expected: sku.Item{Defindex: 160},
		},
		{
			name:     "grouping_killstreak_kits",
			input:    sku.Item{Defindex: 6520},
			expected: sku.Item{Defindex: 6527},
		},
		{
			name:     "promo_to_non_promo",
			input:    sku.Item{Defindex: 801, Quality: QualityUnique},
			expected: sku.Item{Defindex: 851, Quality: QualityUnique},
		},
		{
			name:     "non_promo_to_promo",
			input:    sku.Item{Defindex: 851, Quality: QualityGenuine},
			expected: sku.Item{Defindex: 801, Quality: QualityGenuine},
		},
		{
			name:     "crate_series_assignment",
			input:    sku.Item{Defindex: 5022},
			expected: sku.Item{Defindex: 5022, Crateseries: 42},
		},
		{
			name: "strange_unusual_cosmetic",
			input: sku.Item{
				Defindex: 100,
				Effect:   13,
				Quality:  QualityStrange,
				Paintkit: 0,
			},
			expected: sku.Item{
				Defindex: 100,
				Effect:   13,
				Quality:  QualityUnusual,
				Quality2: QualityStrange,
				Paintkit: 0,
			},
		},
		{
			name: "unusual_weapon_skin",
			input: sku.Item{
				Defindex: 100,
				Effect:   701,
				Quality:  QualityUnusual,
				Paintkit: 100,
			},
			expected: sku.Item{
				Defindex: 100,
				Effect:   701,
				Quality:  QualityDecorated,
				Paintkit: 100,
			},
		},
		{
			name: "strange_weapon_skin_with_effect",
			input: sku.Item{
				Defindex: 100,
				Effect:   701,
				Quality:  QualityStrange,
				Quality2: QualityStrange,
				Paintkit: 100,
			},
			expected: sku.Item{
				Defindex: 100,
				Effect:   701,
				Quality:  QualityDecorated,
				Quality2: QualityStrange,
				Paintkit: 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s.NormalizeItem(&tt.input)

			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("\n        Got:\n        %+v\n        Want:\n        %+v", tt.input, tt.expected)
			}
		})
	}
}

func TestSchema_Getters_StandardTypes_MatchesExpected(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	t.Run("qualities", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Unique", s.QualityByID(6))
		assert.Equal(t, 6, s.QualityIDByName("Unique"))
		assert.NotEmpty(t, s.Qualities())
	})

	t.Run("effects", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Orbiting Fire", s.EffectByID(33))
		assert.Equal(t, 33, s.EffectIDByName("Orbiting Fire"))
		assert.NotEmpty(t, s.ParticleEffects())
	})

	t.Run("skins", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Woodsy Widowmaker", s.SkinByID(102))
		assert.Equal(t, 102, s.SkinIDByName("Woodsy Widowmaker"))
		assert.NotEmpty(t, s.PaintKits())
	})

	t.Run("items", func(t *testing.T) {
		t.Parallel()

		item := s.ItemByDef(5021)
		assert.NotNil(t, item)
		assert.Equal(t, "Scattergun", item.ItemName)

		itemByName := s.ItemByName("Scattergun")
		assert.Equal(t, item, itemByName)
	})
}

func TestSchema_SKUFromEconItem_Variations_GeneratesSKUs(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		name     string
		item     *trading.Item
		expected string
	}{
		{
			name: "basic_unique_weapon",
			item: &trading.Item{
				MarketHashName: "Scattergun",
				Tradable:       true,
				Descriptions:   []trading.Description{},
			},
			expected: "5021;6",
		},
		{
			name: "non_craftable_unique_weapon",
			item: &trading.Item{
				MarketHashName: "Scattergun",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "( Not Usable in Crafting )"},
				},
			},
			expected: "5021;6;uncraftable",
		},
		{
			name: "strange_unusual_hat_with_effect",
			item: &trading.Item{
				MarketHashName: "Strange Unusual Team Captain",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "★ Unusual Effect: Orbiting Fire"},
				},
			},
			expected: "378;5;u33;strange",
		},
		{
			name: "strange_unusual_team_captain",
			item: &trading.Item{
				MarketHashName: "Strange Unusual Team Captain",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "★ Unusual Effect: Burning Flames"},
				},
			},
			expected: "378;5;u13;strange",
		},
		{
			name: "item_with_halloween_spell",
			item: &trading.Item{
				MarketHashName: "Scattergun",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "Halloween: Exorcism", Color: "7ea9d1"},
				},
			},
			expected: "5021;6;s-1009-1",
		},
		{
			name: "item_with_multiple_halloween_spells",
			item: &trading.Item{
				MarketHashName: "Scattergun",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "Halloween: Exorcism", Color: "7ea9d1"},
					{Value: "Halloween: Spectral Spectrum (paint)", Color: "7ea9d1"},
				},
			},
			expected: "5021;6;s-1009-1;s-1004-3",
		},
		{
			name: "decorated_weapon_skin_with_wear",
			item: &trading.Item{
				MarketHashName: "Woodsy Widowmaker Pistol (Factory New)",
				Tradable:       true,
				Descriptions:   []trading.Description{},
			},
			expected: "15013;15;w1;pk102",
		},
		{
			name: "unusual_decorated_weapon",
			item: &trading.Item{
				MarketHashName: "Unusual Woodsy Widowmaker Pistol (Minimal Wear)",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "★ Unusual Effect: Orbiting Fire"},
				},
			},
			expected: "15013;15;u33;w2;pk102",
		},
		{
			name: "war_paint",
			item: &trading.Item{
				MarketHashName: "Woodsy Widowmaker War Paint (Field-Tested)",
				Tradable:       true,
				Descriptions:   []trading.Description{},
			},
			expected: "16189;15;w3;pk102",
		},
		{
			name: "decorated_weapon_with_tags",
			item: &trading.Item{
				MarketHashName: "Woodsy Widowmaker Pistol (Factory New)",
				Tradable:       true,
				Tags: []trading.Tag{
					{Category: "Quality", LocalizedName: "Decorated Weapon"},
					{Category: "Exterior", LocalizedName: "Factory New"},
				},
			},
			expected: "15013;15;w1;pk102",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := s.SKUFromEconItem(tt.item)
			if got != tt.expected {
				t.Errorf("SKUFromEconItem() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSchema_ItemFromName_MoreVariations_ParsesNames(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	tests := []struct {
		input         string
		defindex      int
		quality       int
		target        int
		output        int
		outputQuality int
	}{
		{
			input:    "The Scattergun",
			defindex: 5021,
			quality:  QualityUnique,
		},
		{
			input:    "Strange The Scattergun",
			defindex: 5021,
			quality:  QualityStrange,
		},
		{
			input:    "Unusual The Team Captain",
			defindex: 378,
			quality:  QualityUnusual,
		},
		{
			input:    "Non-Craftable The Scattergun",
			defindex: 5021,
			quality:  QualityUnique,
		},
		{
			input:    "Unusualifier Scattergun",
			defindex: 9258,
			quality:  QualityUnusual,
			target:   5021,
		},
		{
			input:    "Professional Killstreak Kit Fabricator Scattergun",
			defindex: 20003,
			quality:  QualityUnique,
			target:   5021,
			output:   6526,
		},
		{
			input:    "Specialized Killstreak Kit Fabricator Scattergun",
			defindex: 20002,
			quality:  QualityUnique,
			target:   5021,
			output:   6523,
		},
		{
			input:         "Collector's Chemistry Set Scattergun",
			defindex:      20006,
			quality:       QualityUnique,
			output:        5021,
			outputQuality: 14,
		},
		{
			input:         "Strangifier Chemistry Set Scattergun",
			defindex:      20000,
			quality:       QualityUnique,
			target:        5021,
			output:        6522,
			outputQuality: QualityUnique,
		},
		{
			input:    "Strangifier Scattergun",
			defindex: 6522,
			quality:  QualityUnique,
			target:   5021,
		},
		{
			input:    "Professional Killstreak Kit Scattergun",
			defindex: 6526,
			quality:  QualityUnique,
			target:   5021,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := s.ItemFromName(tt.input)
			if got == nil {
				t.Fatalf("failed to parse %q", tt.input)
			}

			assert.Equal(t, tt.defindex, got.Defindex, "Defindex mismatch for %s", tt.input)
			assert.Equal(t, tt.quality, got.Quality, "Quality mismatch for %s", tt.input)

			if tt.target != 0 {
				assert.Equal(t, tt.target, got.Target, "Target mismatch for %s", tt.input)
			}

			if tt.output != 0 {
				assert.Equal(t, tt.output, got.Output, "Output mismatch for %s", tt.input)
			}

			if tt.outputQuality != 0 {
				assert.Equal(t, tt.outputQuality, got.OutputQuality, "OutputQuality mismatch for %s", tt.input)
			}
		})
	}
}

func TestSchema_NormalizeItem_AdvancedCases_NormalizesAttributes(t *testing.T) {
	t.Parallel()

	s := createMockSchema()

	t.Run("strange_unusual_decoration", func(t *testing.T) {
		t.Parallel()

		item := &sku.Item{
			Defindex: 100,
			Quality:  QualityStrange,
			Effect:   13,
			Paintkit: 102,
		}
		s.NormalizeItem(item)

		assert.Equal(t, QualityDecorated, item.Quality, "Should be Decorated (15)")
		assert.Equal(t, QualityStrange, item.Quality2, "Should have Elevated Strange (11)")
	})

	t.Run("australium_normalization", func(t *testing.T) {
		t.Parallel()

		item := &sku.Item{Defindex: 45}
		assert.True(t, s.IsAustraliumDefindex(item.Defindex))
	})
}

func TestSchema_StrangeParts_ValidCounters_MapsToSKUSuffix(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	s.Raw.Schema.KillEaterScoreTypes = append(s.Raw.Schema.KillEaterScoreTypes, &KillEaterScoreType{
		Type: 10, TypeName: "Airborne Enemies Killed",
	})

	parts := s.StrangeParts()
	assert.Equal(t, "sp10", parts["Airborne Enemies Killed"])
}

func TestSchema_ItemByNameWithThe_SpecialCases_FindsItems(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	item := s.ItemByNameWithThe("The Scattergun")
	assert.NotNil(t, item)
	assert.Equal(t, 5021, item.Defindex)

	s.Raw.Schema.Items = append(s.Raw.Schema.Items, &Item{
		Defindex: 0, ItemName: "Scattergun", ItemQuality: 0,
	})
	s.buildIndices()

	itemStock := s.ItemByName("Scattergun")
	assert.Equal(t, 5021, itemStock.Defindex)
}

func TestSchema_WearToTier_VariousWearLevels_ReturnsTier(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, WearToTier(0))
	assert.Equal(t, WearFactoryNew, WearToTier(0.05))
	assert.Equal(t, WearMinimalWear, WearToTier(0.12))
	assert.Equal(t, WearFieldTested, WearToTier(0.25))
	assert.Equal(t, WearWellWorn, WearToTier(0.40))
	assert.Equal(t, WearBattleScarred, WearToTier(0.85))
}

func TestSchema_GetPaintName_ValidAndInvalidColors_ExpectedOutput(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Team Spirit", GetPaintName(0xB8383B))
	assert.Equal(t, "", GetPaintName(0x123456))
}

func TestSchema_IsAustralium_And_IsNativeFestive_ChecksStatus(t *testing.T) {
	t.Parallel()
	assert.True(t, IsAustraliumDefindex(13))
	assert.False(t, IsAustraliumDefindex(9999))
	assert.True(t, IsNativeFestive(654))
	assert.False(t, IsNativeFestive(9999))
}

func TestSchema_IdentifySpell_StandardSpells_MatchesAttributeIDs(t *testing.T) {
	t.Parallel()

	s1, ok1 := IdentifySpell("Halloween: Sentry Quad-Pumpkins")
	assert.True(t, ok1)
	assert.Equal(t, 1007, s1.Attribute)

	s2, ok2 := IdentifySpell("weapon spell: Sentry Quad-Pumpkins")
	assert.True(t, ok2)
	assert.Equal(t, 1007, s2.Attribute)

	_, ok3 := IdentifySpell("Sentry Quad-Pumpkins (Extra)")
	assert.False(t, ok3)

	s4, ok4 := IdentifySpell("die job")
	assert.True(t, ok4)
	assert.Equal(t, 1004, s4.Attribute)

	_, ok5 := IdentifySpell("non-existent-spell")
	assert.False(t, ok5)
}

func TestSchema_ItemAttribute_UnmarshalJSON_Variations(t *testing.T) {
	t.Parallel()

	var attr1 ItemAttribute

	err := json.Unmarshal([]byte(`{"class": "test", "value": 1.5}`), &attr1)
	assert.NoError(t, err)
	assert.Equal(t, 1.5, attr1.Value)

	var attr2 ItemAttribute

	err = json.Unmarshal([]byte(`{"class": "test", "value": 42}`), &attr2)
	assert.NoError(t, err)
	assert.Equal(t, 42.0, attr2.Value)

	var attr3 ItemAttribute

	err = json.Unmarshal([]byte(`{"class": "test", "value": "some_string"}`), &attr3)
	assert.NoError(t, err)
	assert.Equal(t, "some_string", attr3.ValueString)

	var attr4 ItemAttribute

	err = json.Unmarshal([]byte(`{"class": "test", "value": {}}`), &attr4)
	assert.NoError(t, err)

	var attr5 ItemAttribute

	err = json.Unmarshal([]byte(`{"class": "test", "value": `), &attr5)
	assert.Error(t, err)
}

func TestSchema_RecipeBlock_ParseRecipeBlock(t *testing.T) {
	t.Parallel()

	vdf := `
	"3"
	{
		"name" "Smelt Weapons"
		"disabled" "1"
		"premium_only" "1"
		"all_same_class" "1"
		"all_same_slot" "1"
		"category" "commonitem"
		"input_items"
		{
			"2"
			{
				"conditions"
				{
					"1"
					{
						"field" "defindex"
						"value" "5021"
					}
				}
			}
		}
		"output_items"
		{
			"1"
			{
				"conditions"
				{
					"1"
					{
						"field" "name"
						"value" "Scrap"
					}
				}
			}
		}
		"tool"
		{
			"input"
			{
				"tool_input"
				{
					"lootlist_name" "all_weapons"
					"quality" "strange"
					"counts"
					{
						"1" "3"
					}
				}
			}
		}
	}`

	r := parseRecipeBlock(3, vdf)
	require.NotNil(t, r)
	assert.Equal(t, "Smelt Weapons", r.Name)
	assert.True(t, r.Disabled)
	assert.True(t, r.PremiumAccountOnly)
	assert.True(t, r.RequiresAllSameClass)
	assert.True(t, r.RequiresAllSameSlot)
	assert.Equal(t, RecipeCategoryCommonItems, r.Category)

	require.Len(t, r.InputItems, 1)
	assert.Equal(t, 5021, r.InputItems[0].DefIndex)

	require.Len(t, r.OutputItems, 1)
	assert.Equal(t, "Scrap", r.OutputItems[0].Name)
}

func TestSchema_CoverHelpers_And_Getters(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	s := New(raw)

	assert.Nil(t, s.AttributeByDef(9999))

	assert.Equal(t, "Team Spirit", s.PaintNameByDecimal(0xB8383B))
	assert.Equal(t, "", s.PaintNameByDecimal(0))
	assert.Equal(t, "#123456", s.PaintNameByDecimal(0x123456))

	assert.Zero(t, s.PaintDecimalByName("Unknown Paint"))

	assert.Nil(t, s.ItemBySKU("invalid sku"))
	assert.NotNil(t, s.ItemBySKU("5021;6"))

	effects := s.UnusualEffects()
	assert.NotEmpty(t, effects)

	paints := s.Paints()
	assert.NotNil(t, paints)

	paintable := s.PaintableItemDefindexes()
	assert.Contains(t, paintable, 378)

	assert.NotEmpty(t, s.PaintKitsByName())
	assert.NotEmpty(t, s.PaintKits())

	assert.Equal(t, "", s.SKUFromItem(nil))

	item := &sku.Item{Defindex: 5021, Quality: 6, Craftable: true, Tradable: true}
	assert.Equal(t, "5021;6", s.SKUFromItem(item))

	jsonMap := s.ToJSON()
	assert.NotNil(t, jsonMap)
	assert.Equal(t, s.Version, jsonMap["version"])

	assert.NotEmpty(t, s.CrateSeriesList())

	assert.Nil(t, s.WeaponsForCraftingByClass("InvalidClass"))

	assert.NotEmpty(t, s.CraftableWeaponsForTrading())

	assert.NotEmpty(t, s.UncraftableWeaponsForTrading())
}

func TestSchema_BuildIndices_SpecialEffects(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	raw.Schema.AttributeControlledAttachedParticles = append(raw.Schema.AttributeControlledAttachedParticles,
		&ParticleEffect{ID: 991, Name: "Eerie Orbiting Fire"},
		&ParticleEffect{ID: 992, Name: "Nether Trail"},
		&ParticleEffect{ID: 993, Name: "Refragmenting Reality"},
		&ParticleEffect{ID: 994, Name: ""},
	)

	s := New(raw)
	assert.Equal(t, 33, s.effByName["orbiting fire"])
	assert.Equal(t, 103, s.effByName["ether trail"])
	assert.Equal(t, 141, s.effByName["fragmenting reality"])
}

func TestSchema_BuildCrateSeriesList_Deep(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	raw.Schema.Items = append(raw.Schema.Items, &Item{
		Defindex: 9981,
		Name:     "Direct Attribute Crate",
		ItemName: "Direct Attribute Crate",
		Attributes: []ItemAttribute{
			{Name: "set supply crate series", Value: 45.0},
		},
	})

	itemsGame := raw.ItemsGame["items"].(map[string]any)
	itemsGame["invalid_defindex"] = map[string]any{}
	itemsGame["9982"] = "not a map"
	itemsGame["9983"] = map[string]any{
		"static_attrs": map[string]any{
			"set supply crate series": int(77),
		},
	}
	itemsGame["9984"] = map[string]any{
		"static_attrs": map[string]any{
			"set supply crate series": map[string]any{
				"value": float64(88),
			},
		},
	}
	itemsGame["9985"] = map[string]any{
		"static_attrs": map[string]any{
			"set supply crate series": "invalid type",
		},
	}

	s := New(raw)
	assert.Equal(t, 45, s.crateSeriesList[9981])
	assert.Equal(t, 77, s.crateSeriesList[9983])
	assert.Equal(t, 88, s.crateSeriesList[9984])
	assert.NotContains(t, s.crateSeriesList, 9985)
}

func TestSchema_ItemFromName_AdditionalComplexCases(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	itPart := s.ItemFromName("Strange Part: Robots Destroyed")
	require.NotNil(t, itPart)
	assert.Equal(t, 12345, itPart.Defindex)

	assert.False(t, s.ItemFromName("untradeable Scattergun").Tradable)
	assert.False(t, s.ItemFromName("untradable Scattergun").Tradable)
	assert.False(t, s.ItemFromName("non-tradeable Scattergun").Tradable)
	assert.False(t, s.ItemFromName("non-tradable Scattergun").Tradable)

	itUnusual := s.ItemFromName("unusualifier")
	assert.Equal(t, 9258, itUnusual.Defindex)
	assert.Equal(t, 0, itUnusual.Target)

	itChem := s.ItemFromName("Chemistry Set")
	assert.Equal(t, 20006, itChem.Defindex)
	assert.Equal(t, 0, itChem.Output)

	itFestiveChem := s.ItemFromName("Festive Chemistry Set")
	assert.Equal(t, 20007, itFestiveChem.Defindex)

	assert.Equal(t, 5068, s.ItemFromName("Salvaged Mann Co. Supply Crate").Defindex)
	assert.Equal(t, 5660, s.ItemFromName("Select Reserve Mann Co. Supply Crate").Defindex)
}

func TestSchema_CheckExistence_FallbackAndExclusiveQualities(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 831, Quality: QualityUnique}))
	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 810, Quality: QualityGenuine}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 9991, Quality: QualityStrange, Effect: 13}))

	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5021, Quality: QualityGenuine}))
	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5021, Quality: QualityVintage}))
	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5021, Quality: QualityStrange}))

	rawStrange := &Raw{}
	rawStrange.Schema.Items = []*Item{
		{Defindex: 100, ItemQuality: QualityStrange},
	}
	sStrange := New(rawStrange)
	assert.False(t, sStrange.CheckExistence(&sku.Item{Defindex: 100, Quality: QualityUnusual}))
}

func TestSchema_ItemName_AdvancedScmFormatting(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	itDecorated := &sku.Item{
		Defindex:  15013,
		Quality:   QualityDecorated,
		Paintkit:  102,
		Wear:      1,
		Effect:    13,
		Craftable: true,
		Tradable:  true,
	}
	name := s.ItemName(itDecorated, true, false, true)
	assert.Contains(t, name, "Unusual")

	itChem := &sku.Item{
		Defindex:  20007,
		Quality:   QualityUnique,
		Output:    6522,
		Target:    647,
		Craftable: true,
		Tradable:  true,
	}
	nameChem := s.ItemName(itChem, true, false, true)
	assert.Contains(t, nameChem, "Series %231")
}

func TestSchema_Capabilities_AllCapabilitiesChecked(t *testing.T) {
	t.Parallel()

	c := &Capabilities{
		Paintable:           true,
		Nameable:            true,
		CanStrangify:        true,
		CanUseStrangeParts:  true,
		CanKillstreakify:    true,
		CanGiftWrap:         true,
		CanCollect:          true,
		CanConsume:          true,
		PaintableTeamColors: true,
	}
	assert.True(t, c.HasCapability("paintable"))
	assert.True(t, c.HasCapability("nameable"))
	assert.True(t, c.HasCapability("can_use_strange_parts"))
	assert.True(t, c.HasCapability("can_strangify"))
	assert.True(t, c.HasCapability("can_killstreakify"))
	assert.True(t, c.HasCapability("can_gift_wrap"))
	assert.True(t, c.HasCapability("can_collect"))
	assert.True(t, c.HasCapability("can_consume"))
	assert.True(t, c.HasCapability("paintable_team_colors"))
	assert.False(t, c.HasCapability("invalid"))
}

func TestSchema_Unification(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	s := New(raw)

	t.Run("indices_creation", func(t *testing.T) {
		assert.Equal(t, len(raw.Schema.Items), len(s.itemsByDef))
		assert.Equal(t, "Scattergun", s.ItemByDef(5021).ItemName)
	})

	t.Run("quality_helpers", func(t *testing.T) {
		assert.Equal(t, "Unique", s.QualityByID(6))
		assert.Equal(t, 6, s.QualityIDByName("Unique"))
	})

	t.Run("recipe_management", func(t *testing.T) {
		recipe := s.GetRecipe(3)
		require.NotNil(t, recipe)
		assert.Equal(t, "Smelt Weapons", recipe.Name)
		assert.True(t, recipe.Disabled)

		all := s.GetAllRecipes()
		assert.NotEmpty(t, all)
	})
}

func TestSchema_Item_HelperMethods(t *testing.T) {
	t.Parallel()

	it := &Item{
		Flags:       FlagCannotTrade,
		LoadoutSlot: LoadoutHead,
		CraftClass:  "weapon",
		ItemClass:   "tf_weapon_scattergun",
	}

	assert.False(t, it.IsTradableByFlags())
	assert.True(t, it.IsCraftableByFlags())
	assert.True(t, it.HasFlag(FlagCannotTrade))
	assert.Equal(t, LoadoutHead, it.GetLoadoutSlot())
	assert.True(t, it.IsWeapon())
	assert.True(t, it.IsCosmetic())
	assert.False(t, it.IsTaunt())
	assert.False(t, it.IsTool())

	itTaunt := &Item{LoadoutSlot: LoadoutTaunt}
	assert.True(t, itTaunt.IsTaunt())

	itTool := &Item{ItemClass: "tool"}
	assert.True(t, itTool.IsTool())

	itNoSlot := &Item{LoadoutSlot: 0}
	assert.Equal(t, LoadoutInvalid, itNoSlot.GetLoadoutSlot())
}

func TestCapabilities_Helpers(t *testing.T) {
	t.Parallel()

	var capNil *Capabilities
	assert.False(t, capNil.HasCapability("paintable"))
	assert.False(t, capNil.CanApplyTool("paint"))

	c := &Capabilities{
		Paintable:           true,
		Nameable:            true,
		CanStrangify:        true,
		CanUseStrangeParts:  true,
		CanKillstreakify:    true,
		CanGiftWrap:         true,
		CanCollect:          true,
		CanConsume:          true,
		PaintableTeamColors: true,
	}

	assert.True(t, c.HasCapability("paintable"))
	assert.True(t, c.HasCapability("nameable"))
	assert.True(t, c.HasCapability("can_use_strange_parts"))
	assert.True(t, c.HasCapability("can_strangify"))
	assert.True(t, c.HasCapability("can_killstreakify"))
	assert.True(t, c.HasCapability("can_gift_wrap"))
	assert.True(t, c.HasCapability("can_collect"))
	assert.True(t, c.HasCapability("can_consume"))
	assert.True(t, c.HasCapability("paintable_team_colors"))
	assert.False(t, c.HasCapability("invalid_cap"))

	assert.True(t, c.CanApplyTool("paint"))
	assert.True(t, c.CanApplyTool("nametag"))
	assert.True(t, c.CanApplyTool("desctag"))
	assert.True(t, c.CanApplyTool("strangifier"))
	assert.True(t, c.CanApplyTool("strange-part"))
	assert.True(t, c.CanApplyTool("killstreak"))
	assert.True(t, c.CanApplyTool("gift-wrap"))
	assert.False(t, c.CanApplyTool("invalid"))
}

func TestSchema_QualityNameAndID(t *testing.T) {
	t.Parallel()

	s := &Schema{
		qualByID:   map[int]string{6: "Unique"},
		qualByName: map[string]int{"unique": 6},
	}

	assert.Equal(t, "Unique", s.QualityName(6))
	assert.Equal(t, 6, s.QualityID("Unique"))
	assert.Equal(t, -1, s.QualityID("Nonexistent"))

	var sNil *Schema
	assert.Equal(t, "", sNil.QualityName(6))
	assert.Equal(t, -1, sNil.QualityID("Unique"))
}

func TestSchema_PaintKits_And_Weapons(t *testing.T) {
	t.Parallel()

	it := &Item{
		Capabilities: &Capabilities{CanCustomizeTexture: true},
	}
	assert.True(t, it.IsPaintKitWeapon())
	assert.True(t, it.ValidatePaintKit(102))
	assert.False(t, it.ValidatePaintKit(-1))

	itNoCap := &Item{}
	assert.False(t, itNoCap.IsPaintKitWeapon())
	assert.False(t, itNoCap.ValidatePaintKit(102))

	s := &Schema{}

	paints := []int{18, 60, 76, 78}
	for _, p := range paints {
		res := s.GetSupportedWeaponsForPaintkit(p)
		assert.NotEmpty(t, res)
	}
}

func TestRecipeCategory_Parse(t *testing.T) {
	t.Parallel()

	assert.Equal(t, RecipeCategoryCraftingItems, parseRecipeCategory("crafting"))
	assert.Equal(t, RecipeCategoryCommonItems, parseRecipeCategory("commonitem"))
	assert.Equal(t, RecipeCategoryRareItems, parseRecipeCategory("rareitem"))
	assert.Equal(t, RecipeCategorySpecial, parseRecipeCategory("special"))
	assert.Equal(t, RecipeCategoryCraftingItems, parseRecipeCategory("unknown"))
}

func TestSchema_ParseRecipeVDFLine_Variations(t *testing.T) {
	t.Parallel()

	k, v := parseRecipeVDFLine(`"key" "value"`)
	assert.Equal(t, "key", k)
	assert.Equal(t, "value", v)

	k2, v2 := parseRecipeVDFLine(`invalid`)
	assert.Empty(t, k2)
	assert.Empty(t, v2)

	k3, v3 := parseRecipeVDFLine(`"key`)
	assert.Empty(t, k3)
	assert.Empty(t, v3)

	k4, v4 := parseRecipeVDFLine(`"key"`)
	assert.Equal(t, "key", k4)
	assert.Empty(t, v4)

	k5, v5 := parseRecipeVDFLine(`"key" "value`)
	assert.Equal(t, "key", k5)
	assert.Empty(t, v5)
}

func TestSchema_ItemFromEconItem_FullEnrichment(t *testing.T) {
	t.Parallel()

	s := New(minimalRawSchema())

	s.Raw.Schema.KillEaterScoreTypes = append(s.Raw.Schema.KillEaterScoreTypes, &KillEaterScoreType{
		Type: 10, TypeName: "Scouts Killed",
	})

	item := &trading.Item{
		MarketName: "Unusual Team Captain",
		Tradable:   true,
		Descriptions: []trading.Description{
			{Value: "Exterior: factory new"},
			{Value: "★ Unusual Effect: Orbiting Fire"},
			{Value: "Crate Series #42"},
			{Value: "Paint Color: Woodsy Widowmaker"},
			{Value: "Festivized"},
			{Value: "(Scouts Killed: 100)", Color: "756b5e"},
			{Value: "Exorcism", Color: "7ea9d1"},
		},
	}

	res := s.ItemFromEconItem(item)
	require.NotNil(t, res)
	assert.Equal(t, 378, res.Defindex)
	assert.Equal(t, 1, res.Wear)
	assert.Equal(t, 33, res.Effect)
	assert.Equal(t, 42, res.Crateseries)
	assert.True(t, res.Festivized)
	assert.Contains(t, res.Parts, 10)
	assert.Equal(t, 100, res.PartValues[10])
	assert.NotEmpty(t, res.Spells)
}

func TestSchema_CamelCase_And_StyleSuffix(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "TfWeaponScattergun", toCamelCase("tf_weapon_scattergun"))
	assert.Equal(t, "Scattergun", stripStyleSuffix("Scattergun_Style2"))
}

func TestSchema_StaticHelpers(t *testing.T) {
	t.Parallel()

	s := &Schema{}
	assert.Equal(t, 5021, s.NormalizeDefindex(5049))
	assert.True(t, s.IsAustraliumDefindex(13))
	assert.True(t, s.IsNativeFestive(654))
}
