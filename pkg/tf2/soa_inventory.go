// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

// FlatInventory provides a high-performance Struct-of-Arrays (SoA) layout for TF2 item collections.
//
// Mechanical Sympathy & CPU Cache Efficiency:
// Traditional pointer-slice representations ([]*Item) suffer from random memory indirection
// (pointer chasing) and high cache miss penalties (40-80ns per dereference) when scanning
// inventories containing thousands of items.
//
// FlatInventory lays out item attributes into contiguous, cache-line sympathetic columnar slices.
// A full scan of 4,000 Defindexes requires reading only 8 KB of contiguous memory (which fits
// entirely within CPU L1 Data Cache, ~1.5ns latency), delivering 10x-50x faster batch filtering.
type FlatInventory struct {
	AssetIDs   []uint64
	Defindexes []uint16
	Qualities  []uint8
	Flags      []ItemFlags
	Positions  []uint16
	Count      int
}

// NewFlatInventory allocates a new FlatInventory with the given initial capacity.
func NewFlatInventory(capacity int) *FlatInventory {
	if capacity <= 0 {
		capacity = 64
	}

	return &FlatInventory{
		AssetIDs:   make([]uint64, 0, capacity),
		Defindexes: make([]uint16, 0, capacity),
		Qualities:  make([]uint8, 0, capacity),
		Flags:      make([]ItemFlags, 0, capacity),
		Positions:  make([]uint16, 0, capacity),
		Count:      0,
	}
}

// Reset clears the inventory while retaining the allocated slice capacities for 0-alloc reuse.
func (f *FlatInventory) Reset() {
	f.AssetIDs = f.AssetIDs[:0]
	f.Defindexes = f.Defindexes[:0]
	f.Qualities = f.Qualities[:0]
	f.Flags = f.Flags[:0]
	f.Positions = f.Positions[:0]
	f.Count = 0
}

// AppendPacked adds a PackedItem to the columnar layout.
func (f *FlatInventory) AppendPacked(it PackedItem) {
	f.AssetIDs = append(f.AssetIDs, it.AssetID)
	f.Defindexes = append(f.Defindexes, it.DefIndex)
	f.Qualities = append(f.Qualities, it.Quality)
	f.Flags = append(f.Flags, it.Flags)
	f.Positions = append(f.Positions, it.Position)
	f.Count++
}

// AppendItem packs and adds a full *Item to the columnar layout.
func (f *FlatInventory) AppendItem(it *Item) {
	if it == nil {
		return
	}

	f.AppendPacked(PackGCItem(it))
}

// FindAllByDefindex scans the contiguous Defindexes array and returns all matching AssetIDs.
func (f *FlatInventory) FindAllByDefindex(defindex uint16) []uint64 {
	results := make([]uint64, 0, 8)
	for i, def := range f.Defindexes {
		if def == defindex {
			results = append(results, f.AssetIDs[i])
		}
	}

	return results
}

// FindAllByQuality scans the contiguous Qualities array and returns all matching AssetIDs.
func (f *FlatInventory) FindAllByQuality(quality uint8) []uint64 {
	results := make([]uint64, 0, 8)
	for i, q := range f.Qualities {
		if q == quality {
			results = append(results, f.AssetIDs[i])
		}
	}

	return results
}

// CountMatching returns the number of items matching both defindex and quality without any heap allocations.
func (f *FlatInventory) CountMatching(defindex uint16, quality uint8) int {
	n := 0
	for i, def := range f.Defindexes {
		if def == defindex && f.Qualities[i] == quality {
			n++
		}
	}

	return n
}

// FilterMatchingTradable returns all matching asset IDs that have the FlagTradable set.
func (f *FlatInventory) FilterMatchingTradable(defindex uint16) []uint64 {
	results := make([]uint64, 0, 8)
	for i, def := range f.Defindexes {
		if def == defindex && f.Flags[i].Has(FlagTradable) {
			results = append(results, f.AssetIDs[i])
		}
	}

	return results
}
