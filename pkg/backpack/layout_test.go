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

// mockSchema returns a minimal Schema instance for testing
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
			UsedByClasses: []string{}, // Metal
		},
	}

	return schema.New(raw)
}

func TestLayoutFilters(t *testing.T) {
	s := mockSchema()

	t.Run("ByQuality", func(t *testing.T) {
		itemUnique := &tf2.Item{Quality: 6}
		itemStrange := &tf2.Item{Quality: 11}

		filterUnique := ByQuality(6)

		assert.True(t, filterUnique(itemUnique, s))
		assert.False(t, filterUnique(itemStrange, s))
	})

	t.Run("ByClass", func(t *testing.T) {
		itemWeapon := &tf2.Item{DefIndex: 1}   // Has Scout, Sniper
		itemMetal := &tf2.Item{DefIndex: 5000} // No classes

		filterScout := ByClass("Scout")
		filterSoldier := ByClass("Soldier")

		assert.True(t, filterScout(itemWeapon, s))
		assert.False(t, filterSoldier(itemWeapon, s))
		assert.False(t, filterScout(itemMetal, s))
	})

	t.Run("IsPure", func(t *testing.T) {
		pureItems := []*tf2.Item{
			{DefIndex: 5000, IsCraftable: true}, // Scrap
			{DefIndex: 5001, IsCraftable: true}, // Rec
			{DefIndex: 5002, IsCraftable: true}, // Ref
			{DefIndex: 5021, IsCraftable: true}, // Key
		}

		notPureItem := &tf2.Item{DefIndex: 123}

		filter := IsPure()

		for _, item := range pureItems {
			assert.True(t, filter(item, s), "Expected defindex %d to be pure", item.DefIndex)
		}

		assert.False(t, filter(notPureItem, s))
	})

	t.Run("BySKU", func(t *testing.T) {
		// Mock schema doesn't have complex SKU logic, but we can test the basic behavior
		item := &tf2.Item{DefIndex: 1, Quality: 6, IsTradable: true, IsCraftable: true}

		filter := BySKU("1;6")

		assert.Equal(t, "1;6", item.GetSKU(s), "checking generated SKU")
		assert.True(t, filter(item, s))

		filterWrong := BySKU("2;6")
		assert.False(t, filterWrong(item, s))
	})
}
