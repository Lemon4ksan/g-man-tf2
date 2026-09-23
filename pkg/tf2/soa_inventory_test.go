// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatInventory_Operations(t *testing.T) {
	inv := NewFlatInventory(10)
	assert.Equal(t, 0, inv.Count)

	// Append packed items
	inv.AppendPacked(PackedItem{
		AssetID:  1001,
		DefIndex: 5021,
		Quality:  6,
		Flags:    FlagTradable | FlagCraftable,
	})
	inv.AppendPacked(PackedItem{
		AssetID:  1002,
		DefIndex: 5021,
		Quality:  6,
		Flags:    FlagCraftable, // untradable
	})
	inv.AppendPacked(PackedItem{
		AssetID:  1003,
		DefIndex: 340,
		Quality:  11,
		Flags:    FlagTradable,
	})

	assert.Equal(t, 3, inv.Count)

	// FindAllByDefindex
	keys := inv.FindAllByDefindex(5021)
	assert.Equal(t, []uint64{1001, 1002}, keys)

	// FindAllByQuality
	strange := inv.FindAllByQuality(11)
	assert.Equal(t, []uint64{1003}, strange)

	// CountMatching
	count := inv.CountMatching(5021, 6)
	assert.Equal(t, 2, count)

	// FilterMatchingTradable
	tradableKeys := inv.FilterMatchingTradable(5021)
	assert.Equal(t, []uint64{1001}, tradableKeys)

	// Reset
	inv.Reset()
	assert.Equal(t, 0, inv.Count)
	assert.Equal(t, 0, len(inv.AssetIDs))
}

func BenchmarkAoS_vs_SoA_Inventory_4000Items(b *testing.B) {
	const numItems = 4000
	// Setup AoS ([]*Item)
	aos := make([]*Item, numItems)
	soa := NewFlatInventory(numItems)

	for i := 0; i < numItems; i++ {
		def := uint32(i % 500)
		if i%50 == 0 {
			def = 5021 // target key
		}

		item := &Item{
			ID:         uint64(100000 + i),
			DefIndex:   def,
			Quality:    6,
			IsTradable: true,
		}
		aos[i] = item
		soa.AppendItem(item)
	}

	b.Run("AoS_PointerScan", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			count := 0
			for _, it := range aos {
				if it.DefIndex == 5021 && it.Quality == 6 {
					count++
				}
			}

			if count == 0 {
				b.Fatal("unexpected zero count")
			}
		}
	})

	b.Run("SoA_FlatScan", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			count := soa.CountMatching(5021, 6)

			if count == 0 {
				b.Fatal("unexpected zero count")
			}
		}
	})
}
