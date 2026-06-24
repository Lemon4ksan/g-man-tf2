// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/test/module"
	"github.com/stretchr/testify/assert"

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

	if len(s.itemsByName) != len(raw.Schema.Items) {
		t.Errorf("expected %d itemsByName, got %d", len(raw.Schema.Items), len(s.itemsByName))
	}

	if len(s.attrsByDef) != len(raw.Schema.Attributes) {
		t.Errorf("expected %d attrsByDef, got %d", len(raw.Schema.Attributes), len(s.attrsByDef))
	}

	if len(s.qualByID) != len(raw.Schema.Qualities) {
		t.Errorf("expected %d qualByID, got %d", len(raw.Schema.Qualities), len(s.qualByID))
	}

	if len(s.qualByName) != len(raw.Schema.Qualities) {
		t.Errorf("expected %d qualByName, got %d", len(raw.Schema.Qualities), len(s.qualByName))
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
		{"valid_unique_weapon", &sku.Item{Defindex: 5021, Quality: QualityUnique}, true},
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
		{
			name: "strange_unusual_team_captain_alt",
			item: &trading.Item{
				MarketHashName: "Strange Unusual Team Captain",
				Tradable:       true,
				Descriptions: []trading.Description{
					{Value: "★ Unusual Effect: Burning Flames"},
				},
			},
			expected: "378;5;u13;strange",
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
		quality2      int
		target        int
		output        int
		outputQuality int
		killstreak    int
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
			input:      "Professional Killstreak Kit Scattergun",
			defindex:   6526,
			quality:    QualityUnique,
			target:     5021,
			killstreak: 0,
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

		item := &sku.Item{Defindex: 45, Quality: QualityStrange, Australium: true}
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

func TestCoverage_GetPaintName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Team Spirit", GetPaintName(0xB8383B))
	assert.Equal(t, "", GetPaintName(0x123456))
}

func TestCoverage_IsAustralium_IsNativeFestive(t *testing.T) {
	t.Parallel()
	assert.True(t, IsAustraliumDefindex(13))
	assert.False(t, IsAustraliumDefindex(9999))
	assert.True(t, IsNativeFestive(654))
	assert.False(t, IsNativeFestive(9999))
}

func TestCoverage_IdentifySpell(t *testing.T) {
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

func TestCoverage_SchemaHelpers(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	s := New(raw)

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

func TestCoverage_ModuleWithAndFrom(t *testing.T) {
	t.Parallel()

	cfg := steam.DefaultConfig()

	client, err := steam.NewClient(cfg, WithModule(Config{}))
	if err != nil {
		t.Skipf("steam.NewClient failed, skipping: %v", err)
		return
	}

	sm := From(client)
	assert.NotNil(t, sm)
	assert.Equal(t, ModuleName, sm.Name())
}

func TestCoverage_ManagerCache(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "schema.json")

	sm := NewManager(Config{CachePath: cacheFile})

	err := sm.loadFromCache()
	assert.Error(t, err)

	err = sm.saveToCache()
	assert.NoError(t, err)

	raw := minimalRawSchema()
	sm.schema = New(raw)

	err = sm.saveToCache()
	assert.NoError(t, err)

	sm2 := NewManager(Config{CachePath: cacheFile})
	err = sm2.loadFromCache()
	assert.NoError(t, err)
	assert.NotNil(t, sm2.Get())
	assert.Equal(t, len(sm.schema.Raw.Schema.Items), len(sm2.schema.Raw.Schema.Items))

	err = os.WriteFile(cacheFile, []byte(`{"version":"1"}`), 0o644)
	assert.NoError(t, err)

	sm3 := NewManager(Config{CachePath: cacheFile})
	err = sm3.loadFromCache()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete")

	err = os.WriteFile(cacheFile, []byte(`invalid json`), 0o644)
	assert.NoError(t, err)

	sm4 := NewManager(Config{CachePath: cacheFile})
	err = sm4.loadFromCache()
	assert.Error(t, err)

	sm5 := NewManager(Config{CachePath: ""})
	assert.ErrorContains(t, sm5.loadFromCache(), "not configured")
	assert.Nil(t, sm5.saveToCache())
}

func TestCoverage_MirrorFetch(t *testing.T) {
	t.Parallel()

	sm, _ := setupSchema(t, Config{
		SchemaMirrorURL: "",
	})

	_, err := sm.fetchFromMirror(t.Context())
	assert.ErrorContains(t, err, "not configured")

	sm2, mockAPI2 := setupSchema(t, Config{
		SchemaMirrorURL: "http://mirror/overview",
	})

	mockAPI2.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "overview") {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`{"result":{"qualities":{}}}`)),
				StatusCode: 200,
			}, nil
		}

		if strings.Contains(path, "items") {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				StatusCode: 200,
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res, err := sm2.fetchFromMirror(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestCoverage_StartAuthed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "schema.json")

	raw := minimalRawSchema()
	sObj := New(raw)
	sObj.Version = "1"
	sObj.Time = time.Now()
	data, _ := json.Marshal(sObj)
	_ = os.WriteFile(cacheFile, data, 0o644)

	sm, _ := setupSchema(t, Config{
		CachePath:      cacheFile,
		UpdateInterval: 5 * time.Millisecond,
	})

	authCtx := module.NewAuthContext(7656119)
	ctx, cancel := context.WithCancel(t.Context())

	err := sm.StartAuthed(ctx, authCtx)
	assert.NoError(t, err)

	sm.Bus.Publish(&UpdateRequestedEvent{})

	time.Sleep(15 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestCoverage_CheckExistence_And_Name(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	s := New(raw)

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 9999}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5021, Quality: 2}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 15013, Quality: QualityDecorated, Quality2: QualityStrange}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5021, Quality: QualityUnique, Crateseries: 1}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 99}))

	name1 := s.ItemName(
		&sku.Item{Defindex: 378, Quality: QualityUnusual, Effect: 13, Craftable: true, Tradable: true},
		false,
		false,
		false,
	)
	assert.Contains(t, name1, "Burning Flames")

	name2 := s.ItemName(
		&sku.Item{Defindex: 20000, Quality: QualityUnique, Target: 5021, Craftable: true, Tradable: true},
		false,
		false,
		false,
	)
	assert.Contains(t, name2, "Strangifier Chemistry Set")

	name3 := s.ItemName(
		&sku.Item{
			Defindex:      20006,
			Quality:       QualityUnique,
			Output:        160,
			OutputQuality: QualityCollectors,
			Craftable:     true,
			Tradable:      true,
		},
		false,
		false,
		false,
	)
	assert.Contains(t, name3, "Chemistry Set")

	name4 := s.ItemName(
		&sku.Item{Defindex: 20003, Quality: QualityUnique, Target: 378, Killstreak: 3, Craftable: true, Tradable: true},
		false,
		false,
		false,
	)
	assert.Contains(t, name4, "Professional Killstreak")

	assert.NotNil(t, s.ItemFromName("Burning Flames Team Captain"))
	assert.NotNil(t, s.ItemFromName("Strange Professional Killstreak Scattergun"))
}

func TestCoverage_SKUFromEconItem(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	s := New(raw)

	assert.Equal(t, "unknown", s.SKUFromEconItem(nil))

	assert.Equal(t, "0;0;untradable", s.SKUFromEconItem(&trading.Item{MarketHashName: "Unknown"}))

	decoratedItem := &trading.Item{
		MarketHashName: "Woodsy Widowmaker War Paint Pistol",
		Tradable:       true,
		Tags: []trading.Tag{
			{Category: "Exterior", LocalizedName: "Field-Tested"},
		},
		Descriptions: []trading.Description{
			{Value: "Exterior: Field-Tested"},
			{Value: "( Not Usable in Crafting )"},
			{Value: "★ Unusual Effect: Burning Flames"},
			{Value: "Killstreak Active: Professional"},
			{Value: "Paint Color: Team Spirit"},
			{Value: "Crate Series #1"},
			{Value: "Festivized"},
			{Color: "756b5e", Value: "(Kills: 0)"},
			{Color: "7ea9d1", Value: "Halloween: Gourd Grenades"},
		},
	}

	res := s.SKUFromEconItem(decoratedItem)
	assert.NotEmpty(t, res)

	otherItem := &trading.Item{
		MarketHashName: "Australium Pistol",
		Tradable:       true,
		Descriptions: []trading.Description{
			{Value: "Killstreak Active: Specialized"},
		},
	}
	res2 := s.SKUFromEconItem(otherItem)
	assert.NotEmpty(t, res2)

	otherItem2 := &trading.Item{
		MarketHashName: "Pistol",
		Tradable:       true,
		Descriptions: []trading.Description{
			{Value: "Killstreak Active: Killstreak"},
		},
	}
	res3 := s.SKUFromEconItem(otherItem2)
	assert.NotEmpty(t, res3)

	strangeItem := &trading.Item{
		MarketHashName: "Strange Pistol",
		Tradable:       true,
	}
	res4 := s.SKUFromEconItem(strangeItem)
	assert.NotEmpty(t, res4)
}

func TestCoverage_CheckExistence_Extra(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	raw.Schema.Items = append(raw.Schema.Items, &Item{
		Defindex:    5713,
		Name:        "Spooky Key",
		ItemName:    "Spooky Key",
		ItemClass:   "tool",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    5049,
		Name:        "Festive Winter Crate Key",
		ItemName:    "Festive Winter Crate Key",
		ItemClass:   "tool",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    5791,
		Name:        "Naughty Winter Crate Key 2014",
		ItemName:    "Naughty Winter Crate Key 2014",
		ItemClass:   "tool",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    5734,
		Name:        "Munition Crate",
		ItemName:    "Munition Crate",
		ItemClass:   "supply_crate",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    810,
		Name:        "Exclusive Genuine Item",
		ItemName:    "Exclusive Genuine Item",
		ItemClass:   "tf_wearable",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    831,
		Name:        "Exclusive Genuine Reversed Item",
		ItemName:    "Exclusive Genuine Reversed Item",
		ItemClass:   "tf_wearable",
		ItemQuality: QualityUnique,
	})

	s := New(raw)

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5713, Quality: QualityUnique, Craftable: true}))
	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5713, Quality: QualityUnique, Craftable: false}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5049, Quality: QualityUnique, Craftable: false}))
	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5049, Quality: QualityUnique, Craftable: true}))

	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5791, Quality: QualityUnique, Craftable: false}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5739, Quality: QualityUnusual, Crateseries: 0}))

	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 1}))

	assert.True(t, s.CheckExistence(&sku.Item{Defindex: 5734, Quality: QualityUnique, Crateseries: 82}))
	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5734, Quality: QualityUnique, Crateseries: 83}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 5022, Quality: QualityUnique, Crateseries: 99}))

	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 831, Quality: QualityUnique}))
	assert.False(t, s.CheckExistence(&sku.Item{Defindex: 810, Quality: QualityGenuine}))
}

func TestCoverage_ItemName_Extra(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	raw.Schema.Items = append(raw.Schema.Items, &Item{
		Defindex:    9991,
		Name:        "Base Unusual Item",
		ItemName:    "Base Unusual Item",
		ItemClass:   "tf_wearable",
		ItemQuality: QualityUnusual,
	})

	s := New(raw)

	name1 := s.ItemName(&sku.Item{
		Defindex:  15013,
		Quality:   QualityDecorated,
		Quality2:  QualityStrange,
		Paintkit:  102,
		Wear:      1,
		Craftable: true,
		Tradable:  true,
	}, false, false, false)
	assert.Contains(t, name1, "Strange(e)")

	name2 := s.ItemName(&sku.Item{
		Defindex:  5021,
		Quality:   QualityUnique,
		Quality2:  QualityStrange,
		Craftable: true,
		Tradable:  true,
	}, false, false, false)
	assert.Contains(t, name2, "Unique")

	name3 := s.ItemName(&sku.Item{
		Defindex:  378,
		Quality:   QualityUnusual,
		Effect:    0,
		Craftable: true,
		Tradable:  true,
	}, false, false, false)
	assert.Contains(t, name3, "Unusual")

	name4 := s.ItemName(&sku.Item{
		Defindex:  378,
		Quality:   QualityUnusual,
		Effect:    13,
		Craftable: true,
		Tradable:  true,
	}, false, false, true)
	assert.Contains(t, name4, "Unusual")

	name5 := s.ItemName(&sku.Item{
		Defindex:  9991,
		Quality:   QualityUnusual,
		Craftable: true,
		Tradable:  true,
	}, false, false, false)
	assert.Contains(t, name5, "Unusual")

	nameKS1 := s.ItemName(&sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 1}, false, false, false)
	assert.Contains(t, nameKS1, "Killstreak")

	nameKS2 := s.ItemName(&sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 2}, false, false, false)
	assert.Contains(t, nameKS2, "Specialized Killstreak")

	nameKS3 := s.ItemName(&sku.Item{Defindex: 5021, Quality: QualityUnique, Killstreak: 3}, false, false, false)
	assert.Contains(t, nameKS3, "Professional Killstreak")

	nameTarget := s.ItemName(&sku.Item{
		Defindex:      20000,
		Quality:       QualityUnique,
		Target:        5021,
		Output:        160,
		OutputQuality: QualityVintage,
	}, false, false, false)
	assert.Contains(t, nameTarget, "Vintage")
	assert.Contains(t, nameTarget, "Scattergun")
	assert.Contains(t, nameTarget, "Lugermorph")

	namePipe := s.ItemName(&sku.Item{
		Defindex: 15013,
		Quality:  QualityDecorated,
		Paintkit: 102,
		Wear:     1,
	}, false, true, false)
	assert.Contains(t, namePipe, "Woodsy Widowmaker |")

	nameNoPipe := s.ItemName(&sku.Item{
		Defindex: 15013,
		Quality:  QualityDecorated,
		Paintkit: 102,
		Wear:     1,
	}, false, false, false)
	assert.Contains(t, nameNoPipe, "Woodsy Widowmaker")

	spellName := s.SpellNameFromSKU(sku.Spell{Attribute: 1004, Value: 0})
	assert.Equal(t, "Die Job", spellName)

	spellNameUnknown := s.SpellNameFromSKU(sku.Spell{Attribute: 9999, Value: 0})
	assert.Contains(t, spellNameUnknown, "Unknown Spell")

	nameCraftNum := s.ItemName(&sku.Item{Defindex: 5021, Quality: QualityUnique, Craftnumber: 42}, false, false, false)
	assert.Contains(t, nameCraftNum, "#42")

	raw.Schema.Items = append(raw.Schema.Items, &Item{
		Defindex:    20007,
		Name:        "Chemistry Set",
		ItemName:    "Chemistry Set",
		ItemClass:   "tool",
		ItemQuality: QualityUnique,
	})
	s2 := New(raw)
	nameChem := s2.ItemName(&sku.Item{
		Defindex: 20007,
		Quality:  QualityUnique,
		Target:   647,
		Output:   6522,
	}, false, false, true)
	assert.Contains(t, nameChem, "Series %231")
}

func TestCoverage_ItemFromName_Extra(t *testing.T) {
	t.Parallel()

	raw := minimalRawSchema()
	raw.Schema.Items = append(raw.Schema.Items, &Item{
		Defindex:    9992,
		Name:        "Vintage Tyrolean",
		ItemName:    "Vintage Tyrolean",
		ItemClass:   "tf_wearable",
		ItemQuality: QualityUnique,
	}, &Item{
		Defindex:    9993,
		Name:        "Strange Part: Kills",
		ItemName:    "Strange Part: Kills",
		ItemClass:   "tool",
		ItemQuality: QualityUnique,
	})

	s := New(raw)

	it1 := s.ItemFromName("Strange Part: Kills")
	assert.Equal(t, 9993, it1.Defindex)

	it2 := s.ItemFromName("Strange(e) Woodsy Widowmaker Pistol")
	assert.Equal(t, QualityStrange, it2.Quality2)

	it3 := s.ItemFromName("Uncraftable Untradeable Scattergun")
	assert.False(t, it3.Craftable)
	assert.False(t, it3.Tradable)

	it4 := s.ItemFromName("Unusual Scattergun Unusualifier")
	assert.Equal(t, 9258, it4.Defindex)
	assert.Equal(t, 5021, it4.Target)
	assert.Equal(t, QualityUnusual, it4.Quality)

	assert.Equal(t, 1, s.ItemFromName("Killstreak Scattergun").Killstreak)
	assert.Equal(t, 2, s.ItemFromName("Specialized Killstreak Scattergun").Killstreak)
	assert.Equal(t, 3, s.ItemFromName("Professional Killstreak Scattergun").Killstreak)

	it6 := s.ItemFromName("Australium Festivized Scattergun")
	assert.True(t, it6.Australium)
	assert.True(t, it6.Festivized)

	it7 := s.ItemFromName("Vintage Vintage Tyrolean")
	assert.Equal(t, QualityVintage, it7.Quality)
	assert.Equal(t, 9992, it7.Defindex)

	it8 := s.ItemFromName("Strange Vintage Lugermorph")
	assert.Equal(t, QualityVintage, it8.Quality)
	assert.Equal(t, QualityStrange, it8.Quality2)

	it9 := s.ItemFromName("Burning Flames Scattergun")
	assert.Equal(t, 13, it9.Effect)
	assert.Equal(t, QualityUnusual, it9.Quality)
	assert.Equal(t, 0, it9.Quality2)

	it10 := s.ItemFromName("Woodsy Widowmaker Pistol (Factory New)")
	assert.Equal(t, 102, it10.Paintkit)
	assert.Equal(t, 1, it10.Wear)
	assert.Equal(t, 15013, it10.Defindex)
	assert.Equal(t, QualityDecorated, it10.Quality)
}

func TestCoverage_ManagerErrors(t *testing.T) {
	t.Parallel()

	sm := NewManager(Config{})

	assert.True(t, sm.isRetryable(errors.New("invalid character '<' at line 1")))
	assert.True(t, sm.isRetryable(errors.New("HTTP status 429: Too Many Requests")))
	assert.True(t, sm.isRetryable(errors.New("connection timeout exceeded")))
	assert.True(t, sm.isRetryable(errors.New("connection refused")))
	assert.False(t, sm.isRetryable(errors.New("generic error")))

	assert.True(t, sm.isForbiddenError(errors.New("403 Forbidden")))
	assert.False(t, sm.isForbiddenError(errors.New("generic error")))

	apiErr := &service.SteamAPIError{
		StatusCode: 403,
		Message:    "Forbidden",
	}
	assert.True(t, sm.isForbiddenError(apiErr))

	restErr := &aoni.APIError{
		StatusCode: 403,
	}
	assert.True(t, sm.isForbiddenError(restErr))
}

func TestCoverage_GetItemsGame_Deep(t *testing.T) {
	t.Parallel()

	sm, mockAPI := setupSchema(t, Config{})

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		if strings.Contains(path, "test_items_game") {
			vdf := "\"items_game\"\n{\n\t\"items\"\n\t{\n\t\t\"5022\"\n\t\t{\n\t\t\t\"static_attrs\"\n\t\t\t{\n\t\t\t\t\"set supply crate series\" \"1\"\n\t\t\t}\n\t\t}\n\t}\n}\n"

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(vdf)),
			}, nil
		}

		return nil, fmt.Errorf("unexpected path: %s", path)
	}

	res, err := sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	items := res["items"].(map[string]any)
	assert.Contains(t, items, "5022")

	sm.config.ItemsGameMirrorURL = "http://mock/test_items_game"
	res2, err := sm.getItemsGame(t.Context(), "")
	assert.NoError(t, err)
	assert.NotNil(t, res2)

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("error")),
		}, nil
	}
	_, err = sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.Error(t, err)

	mockAPI.OnRest = func(method, path string, body any) (*http.Response, error) {
		return nil, errors.New("network error")
	}
	_, err = sm.getItemsGame(t.Context(), "http://mock/test_items_game")
	assert.Error(t, err)
}

func TestCoverage_BuildIndices_SpecialEffects(t *testing.T) {
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

func TestCoverage_BuildCrateSeriesList_Deep(t *testing.T) {
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
