// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func mockSchemaForLayout() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      1,
			ItemQuality:   6,
			CraftClass:    "weapon",
			ItemClass:     "weapon",
			UsedByClasses: []string{"Scout", "Sniper"},
		},
		{
			Defindex:    5000,
			CraftClass:  "tool",
			ItemClass:   "tool",
			ItemQuality: 6,
		},
		{
			Defindex:    5021,
			CraftClass:  "tool",
			ItemClass:   "tool",
			ItemQuality: 6,
		},
		{
			Defindex:    6000,
			CraftClass:  "hat",
			ItemClass:   "tf_wearable",
			ItemQuality: 6,
		},
		{
			Defindex:    7000,
			ItemName:    "Taunt: Laugh",
			ItemClass:   "tf_wearable_taunt",
			ItemQuality: 5,
		},
		{
			Defindex:    8000,
			ItemClass:   "supply_crate",
			ItemQuality: 6,
		},
	}

	return schema.New(raw)
}

func TestLayoutFilters(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()

	t.Run("not_and_or_filters", func(t *testing.T) {
		itemUnique := &tf2.Item{Quality: 6, DefIndex: 1}

		filterNot := Not(ByQuality(6))
		assert.False(t, filterNot(itemUnique, s))

		filterAnd := And(ByQuality(6), ByClass("Scout"))
		assert.True(t, filterAnd(itemUnique, s))

		filterOr := Or(ByQuality(11), ByQuality(6))
		assert.True(t, filterOr(itemUnique, s))
	})

	t.Run("basic_filters", func(t *testing.T) {
		itemUnique := &tf2.Item{Quality: 6, DefIndex: 1, IsTradable: true}
		itemStrange := &tf2.Item{Quality: 11, DefIndex: 1, IsTradable: false}

		assert.True(t, ByQuality(6)(itemUnique, s))
		assert.False(t, ByQuality(6)(itemStrange, s))

		assert.True(t, ByClass("Scout")(itemUnique, s))
		assert.False(t, ByClass("Medic")(itemUnique, s))

		assert.True(t, IsTradable()(itemUnique, s))
		assert.False(t, IsTradable()(itemStrange, s))
	})

	t.Run("is_weapon_filters", func(t *testing.T) {
		itemWeapon := &tf2.Item{DefIndex: 1}
		itemTool := &tf2.Item{DefIndex: 5000}

		assert.True(t, IsWeapon()(itemWeapon, s))
		assert.False(t, IsWeapon()(itemTool, s))
	})

	t.Run("is_cosmetic_filters", func(t *testing.T) {
		itemHat := &tf2.Item{DefIndex: 6000}
		itemTaunt := &tf2.Item{DefIndex: 7000}

		assert.True(t, IsCosmetic()(itemHat, s))
		assert.False(t, IsCosmetic()(itemTaunt, s))
	})

	t.Run("is_taunt_filters", func(t *testing.T) {
		itemTaunt := &tf2.Item{DefIndex: 7000}
		itemHat := &tf2.Item{DefIndex: 6000}

		assert.True(t, IsTaunt()(itemTaunt, s))
		assert.False(t, IsTaunt()(itemHat, s))
	})

	t.Run("is_crate_filters", func(t *testing.T) {
		itemCrate := &tf2.Item{DefIndex: 8000}
		itemHat := &tf2.Item{DefIndex: 6000}

		assert.True(t, IsCrate()(itemCrate, s))
		assert.False(t, IsCrate()(itemHat, s))
	})

	t.Run("is_tool_filters", func(t *testing.T) {
		itemTool := &tf2.Item{DefIndex: 5000}
		itemHat := &tf2.Item{DefIndex: 6000}

		assert.True(t, IsTool()(itemTool, s))
		assert.False(t, IsTool()(itemHat, s))
	})

	t.Run("is_action_and_toy_filters", func(t *testing.T) {
		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{
				Defindex:  233,
				ItemName:  "Secret Saxton",
				Name:      "Gift - 1 Player",
				ItemClass: "tf_wearable",
			},
			{
				Defindex:  280,
				ItemName:  "Noise Maker - Black Cat",
				Name:      "Halloween Noise Maker - Black Cat",
				ItemClass: "tf_wearable",
			},
			{
				Defindex:  30880,
				ItemName:  "Pocket Saxton",
				Name:      "Pocket Saxton",
				ItemClass: "tf_wearable",
			},
		}
		sCustom := schema.New(raw)

		itemSaxton := &tf2.Item{DefIndex: 233}
		itemNoise := &tf2.Item{DefIndex: 280}
		itemPocket := &tf2.Item{DefIndex: 30880}

		assert.True(t, IsAction()(itemSaxton, sCustom))
		assert.False(t, IsCosmetic()(itemSaxton, sCustom))

		assert.True(t, IsAction()(itemNoise, sCustom))
		assert.False(t, IsCosmetic()(itemNoise, sCustom))

		assert.False(t, IsAction()(itemPocket, sCustom))
		assert.True(t, IsCosmetic()(itemPocket, sCustom))
	})

	t.Run("default_layout_and_priority_coverage", func(t *testing.T) {
		layout := DefaultLayout()
		assert.NotEmpty(t, layout.Sections)

		assert.Equal(t, 1, GetPurePriority(5021, s))
		assert.Equal(t, 2, GetPurePriority(5002, s))
		assert.Equal(t, 3, GetPurePriority(5001, s))
		assert.Equal(t, 4, GetPurePriority(5000, s))
		assert.Equal(t, 5, GetPurePriority(1234, s))

		assert.Equal(t, 1, GetQualityPriority(6))
		assert.Equal(t, 2, GetQualityPriority(11))
	})
}

func TestLayoutSorters(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()

	t.Run("currency_sorter", func(t *testing.T) {
		itemKey := &tf2.Item{ID: 1, DefIndex: 5021}
		itemRef := &tf2.Item{ID: 2, DefIndex: 5002}
		itemRef2 := &tf2.Item{ID: 3, DefIndex: 5002}

		assert.True(t, CurrencySorter(itemKey, itemRef, s) < 0)
		assert.True(t, CurrencySorter(itemRef, itemKey, s) > 0)
		assert.True(t, CurrencySorter(itemRef, itemRef2, s) < 0)
	})

	t.Run("weapons_sorter", func(t *testing.T) {
		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{
				Defindex:      1,
				ItemQuality:   6,
				CraftClass:    "weapon",
				ItemClass:     "tf_weapon_scattergun",
				UsedByClasses: []string{"Scout"},
			},
			{
				Defindex:      3,
				ItemQuality:   6,
				CraftClass:    "weapon",
				ItemClass:     "tf_weapon_rocketlauncher",
				UsedByClasses: []string{"Soldier"},
			},
			{
				Defindex:      7,
				ItemQuality:   6,
				CraftClass:    "weapon",
				ItemClass:     "tf_weapon_wrench",
				UsedByClasses: []string{"Engineer"},
			},
			{
				Defindex:      13,
				ItemQuality:   6,
				CraftClass:    "weapon",
				ItemClass:     "tf_weapon_shotgun",
				UsedByClasses: []string{"Engineer"},
			},
			{
				Defindex:      22,
				ItemQuality:   6,
				CraftClass:    "weapon",
				ItemClass:     "tf_weapon_pistol",
				UsedByClasses: []string{"Scout"},
			},
		}
		sWeapons := schema.New(raw)

		itemScout := &tf2.Item{ID: 1, DefIndex: 1, Quality: 6}
		itemSoldier := &tf2.Item{ID: 2, DefIndex: 3, Quality: 6}
		itemStrangeScout := &tf2.Item{ID: 3, DefIndex: 1, Quality: 11}

		assert.True(t, WeaponsSorter(itemScout, itemStrangeScout, sWeapons) < 0)
		assert.True(t, WeaponsSorter(itemScout, itemSoldier, sWeapons) < 0)
	})

	t.Run("cosmetics_sorter", func(t *testing.T) {
		itemUnique := &tf2.Item{ID: 1, DefIndex: 6000, Quality: 6}
		itemStrange := &tf2.Item{ID: 2, DefIndex: 6000, Quality: 11}

		assert.True(t, CosmeticsSorter(itemUnique, itemStrange, s) < 0)
	})

	t.Run("defindex_sorter", func(t *testing.T) {
		itemA := &tf2.Item{ID: 1, DefIndex: 5000, Quality: 6}
		itemB := &tf2.Item{ID: 2, DefIndex: 5021, Quality: 6}

		assert.True(t, DefindexSorter(itemA, itemB, s) < 0)
	})
}

func TestGetSlotPriority(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 9, CraftClass: "weapon", ItemClass: "tf_weapon_shotgun"},
		{Defindex: 1178, CraftClass: "weapon", ItemClass: "tf_weapon_flamethrower"},
		{Defindex: 22, CraftClass: "weapon", ItemClass: "tf_weapon_pistol"},
		{Defindex: 7, CraftClass: "weapon", ItemClass: "tf_weapon_wrench"},
		{Defindex: 25, CraftClass: "weapon", ItemClass: "tf_weapon_pda"},
		{Defindex: 999, CraftClass: "hat", ItemClass: "tf_wearable"},
	}
	s := schema.New(raw)

	tests := []struct {
		name     string
		defindex uint32
		expected int
	}{
		{"primary_shotgun", 9, 1},
		{"dragons_fury", 1178, 1},
		{"pistol_secondary", 22, 2},
		{"wrench_melee", 7, 3},
		{"pda_builder", 25, 4},
		{"non_weapon_hat", 999, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &tf2.Item{DefIndex: tt.defindex}
			assert.Equal(t, tt.expected, GetSlotPriority(item, s))
		})
	}
}

func TestLayout_IsPure(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()

	assert.True(t, IsPure()(&tf2.Item{DefIndex: 5021}, s))
	assert.True(t, IsPure()(&tf2.Item{DefIndex: 5002}, s))
	assert.True(t, IsPure()(&tf2.Item{DefIndex: 5001}, s))
	assert.True(t, IsPure()(&tf2.Item{DefIndex: 5000}, s))
	assert.False(t, IsPure()(&tf2.Item{DefIndex: 1}, s))
}

func TestLayout_GetClassPriority(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 10, UsedByClasses: []string{}},
		{Defindex: 11, UsedByClasses: []string{"Scout", "Soldier"}},
		{Defindex: 1, UsedByClasses: []string{"Scout"}},
		{Defindex: 2, UsedByClasses: []string{"Soldier"}},
		{Defindex: 3, UsedByClasses: []string{"Pyro"}},
		{Defindex: 4, UsedByClasses: []string{"Demoman"}},
		{Defindex: 5, UsedByClasses: []string{"Heavy"}},
		{Defindex: 6, UsedByClasses: []string{"Engineer"}},
		{Defindex: 7, UsedByClasses: []string{"Medic"}},
		{Defindex: 8, UsedByClasses: []string{"Sniper"}},
		{Defindex: 9, UsedByClasses: []string{"Spy"}},
		{Defindex: 12, UsedByClasses: []string{"CustomClass"}},
	}
	s := schema.New(raw)

	assert.Equal(t, 12, GetClassPriority(&tf2.Item{DefIndex: 999}, s))
	assert.Equal(t, 12, GetClassPriority(&tf2.Item{DefIndex: 10}, s))
	assert.Equal(t, 10, GetClassPriority(&tf2.Item{DefIndex: 11}, s))

	assert.Equal(t, 1, GetClassPriority(&tf2.Item{DefIndex: 1}, s))
	assert.Equal(t, 2, GetClassPriority(&tf2.Item{DefIndex: 2}, s))
	assert.Equal(t, 3, GetClassPriority(&tf2.Item{DefIndex: 3}, s))
	assert.Equal(t, 4, GetClassPriority(&tf2.Item{DefIndex: 4}, s))
	assert.Equal(t, 5, GetClassPriority(&tf2.Item{DefIndex: 5}, s))
	assert.Equal(t, 6, GetClassPriority(&tf2.Item{DefIndex: 6}, s))
	assert.Equal(t, 7, GetClassPriority(&tf2.Item{DefIndex: 7}, s))
	assert.Equal(t, 8, GetClassPriority(&tf2.Item{DefIndex: 8}, s))
	assert.Equal(t, 9, GetClassPriority(&tf2.Item{DefIndex: 9}, s))
	assert.Equal(t, 11, GetClassPriority(&tf2.Item{DefIndex: 12}, s))
}

func TestLayout_WeaponsSorter_Detailed(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 1, CraftClass: "weapon", UsedByClasses: []string{"Scout"}},
		{Defindex: 2, CraftClass: "weapon", UsedByClasses: []string{"Scout"}},
	}
	s := schema.New(raw)

	a := &tf2.Item{ID: 10, DefIndex: 1, Quality: 6}
	b := &tf2.Item{ID: 20, DefIndex: 2, Quality: 6}
	assert.True(t, WeaponsSorter(a, b, s) < 0)

	e := &tf2.Item{ID: 10, DefIndex: 1, Quality: 3}
	f := &tf2.Item{ID: 20, DefIndex: 1, Quality: 13}
	assert.True(t, WeaponsSorter(e, f, s) < 0)
}

func TestLayout_CosmeticsSorter_Detailed(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 1, UsedByClasses: []string{"Scout"}},
		{Defindex: 2, UsedByClasses: []string{"Scout"}},
	}
	s := schema.New(raw)

	a := &tf2.Item{ID: 10, DefIndex: 1, Quality: 6}
	b := &tf2.Item{ID: 20, DefIndex: 2, Quality: 6}
	assert.True(t, CosmeticsSorter(a, b, s) < 0)

	e := &tf2.Item{ID: 10, DefIndex: 1, Quality: 3}
	f := &tf2.Item{ID: 20, DefIndex: 1, Quality: 13}
	assert.True(t, CosmeticsSorter(e, f, s) < 0)
}

func TestLayout_DefindexSorter_Detailed(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()

	e := &tf2.Item{ID: 10, DefIndex: 1, Quality: 3}
	f := &tf2.Item{ID: 20, DefIndex: 1, Quality: 13}
	assert.True(t, DefindexSorter(e, f, s) < 0)
}
