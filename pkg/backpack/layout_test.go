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

func mockSchema() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      1,
			ItemQuality:   6,
			UsedByClasses: []string{"Scout", "Sniper"},
		},
		{
			Defindex:      5000,
			UsedByClasses: []string{},
		},
	}

	return schema.New(raw)
}

func TestLayoutFilters_StandardFilters_ExpectedMatching(t *testing.T) {
	t.Parallel()

	s := mockSchema()

	t.Run("by_quality", func(t *testing.T) {
		itemUnique := &tf2.Item{Quality: 6}
		itemStrange := &tf2.Item{Quality: 11}

		filterUnique := ByQuality(6)

		assert.True(t, filterUnique(itemUnique, s))
		assert.False(t, filterUnique(itemStrange, s))
	})

	t.Run("by_class", func(t *testing.T) {
		itemWeapon := &tf2.Item{DefIndex: 1}
		itemMetal := &tf2.Item{DefIndex: 5000}

		filterScout := ByClass("Scout")
		filterSoldier := ByClass("Soldier")

		assert.True(t, filterScout(itemWeapon, s))
		assert.False(t, filterSoldier(itemWeapon, s))
		assert.False(t, filterScout(itemMetal, s))
	})

	t.Run("is_pure", func(t *testing.T) {
		pureItems := []*tf2.Item{
			{DefIndex: 5000, IsCraftable: true},
			{DefIndex: 5001, IsCraftable: true},
			{DefIndex: 5002, IsCraftable: true},
			{DefIndex: 5021, IsCraftable: true},
		}

		notPureItem := &tf2.Item{DefIndex: 123}

		filter := IsPure()

		for _, item := range pureItems {
			assert.True(t, filter(item, s), "Expected defindex %d to be pure", item.DefIndex)
		}

		assert.False(t, filter(notPureItem, s))
	})

	t.Run("by_sku", func(t *testing.T) {
		item := &tf2.Item{DefIndex: 1, Quality: 6, IsTradable: true, IsCraftable: true}

		filter := BySKU("1;6")

		assert.Equal(t, "1;6", item.GetSKU(s), "checking generated SKU")
		assert.True(t, filter(item, s))

		filterWrong := BySKU("2;6")
		assert.False(t, filterWrong(item, s))
	})

	t.Run("noisemakers_and_secret_saxton", func(t *testing.T) {
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

		// Pocket Saxton is a regular toy cosmetic
		assert.False(t, IsAction()(itemPocket, sCustom))
		assert.True(t, IsCosmetic()(itemPocket, sCustom))
	})
}
