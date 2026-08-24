// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests_test

import (
	"fmt"
	"testing"

	"github.com/lemon4ksan/foundation/generic"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// mockItemCache implements backpack.ItemCache and ForEachItem interface for benchmarking.
type mockItemCache struct {
	items []*tf2.Item
	slots int
}

func (m *mockItemCache) GetItems() []*tf2.Item {
	return m.items
}

func (m *mockItemCache) GetItem(id uint64) (*tf2.Item, bool) {
	for _, item := range m.items {
		if item.ID == id {
			return item, true
		}
	}

	return nil, false
}

func (m *mockItemCache) GetMaxSlots() int {
	return m.slots
}

func (m *mockItemCache) ForEachItem(fn func(*tf2.Item) bool) {
	for _, item := range m.items {
		if !fn(item) {
			break
		}
	}
}

type mockSchemaProvider struct {
	s *schema.Schema
}

func (m *mockSchemaProvider) Get() *schema.Schema {
	return m.s
}

func generateBenchmarkBackpack(itemCount int) *mockItemCache {
	items := make([]*tf2.Item, 0, itemCount)

	for i := range itemCount {
		assetID := uint64(1000000000 + i)

		var (
			defIndex    uint32
			quality     uint32 = schema.QualityUnique
			isTradable         = true
			isCraftable        = true
			skuStr      string
		)

		switch {
		case i < 200: // 200 Keys
			defIndex = schema.DefKey
			skuStr = "5021;6"
		case i < 1000: // 800 Refined
			defIndex = schema.DefRefined
			skuStr = "5002;6"
		case i < 1200: // 200 Reclaimed
			defIndex = schema.DefReclaimed
			skuStr = "5001;6"
		case i < 1400: // 200 Scrap
			defIndex = schema.DefScrap
			skuStr = "5000;6"
		case i < 1800: // 400 Weapons
			defIndex = uint32(200 + (i % 50))
			skuStr = fmt.Sprintf("%d;6", defIndex)
		default: // 200 Unusuals / Complex Cosmetics
			defIndex = uint32(30000 + (i % 100))
			quality = schema.QualityUnusual
			skuStr = fmt.Sprintf("%d;5;u13", defIndex)
		}

		item := &tf2.Item{
			ID:          assetID,
			OriginalID:  assetID,
			DefIndex:    defIndex,
			Quality:     quality,
			IsTradable:  isTradable,
			IsCraftable: isCraftable,
			SKU:         skuStr,
			Inventory:   uint32(i + 1),
			Spells:      []sku.Spell{},
			Parts:       []uint32{},
		}

		items = append(items, item)
	}

	return &mockItemCache{
		items: items,
		slots: 3000,
	}
}

func BenchmarkBackpack_GetPureStock_2000Items(b *testing.B) {
	cache := generateBenchmarkBackpack(2000)
	bp := backpack.NewWithDeps(cache, &mockSchemaProvider{}, make(generic.Set[uint64]))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stock := bp.GetPureStock()
		_ = stock
	}
}

func BenchmarkBackpack_GetStock_2000Items(b *testing.B) {
	cache := generateBenchmarkBackpack(2000)
	sch := schema.New(&schema.Raw{})
	bp := backpack.NewWithDeps(cache, &mockSchemaProvider{s: sch}, make(generic.Set[uint64]))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		count := bp.GetStock("5002;6")
		_ = count
	}
}

func BenchmarkBackpack_GetAssetIDs_2000Items(b *testing.B) {
	cache := generateBenchmarkBackpack(2000)
	sch := schema.New(&schema.Raw{})
	bp := backpack.NewWithDeps(cache, &mockSchemaProvider{s: sch}, make(generic.Set[uint64]))

	b.ReportAllocs()

	for b.Loop() {
		ids := bp.GetAssetIDs("5002;6")
		_ = ids
	}
}

func BenchmarkBackpack_FindCraftableItems_2000Items(b *testing.B) {
	cache := generateBenchmarkBackpack(2000)
	bp := backpack.NewWithDeps(cache, &mockSchemaProvider{}, make(generic.Set[uint64]))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items := bp.FindCraftableItems(schema.DefRefined, 3)
		_ = items
	}
}
