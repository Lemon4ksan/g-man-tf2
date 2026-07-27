// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// ItemFlags represents packed bitmask flags for trade/craft/quality attributes.
type ItemFlags uint8

// Supported ItemFlags bitmask flags.
const (
	FlagTradable ItemFlags = 1 << iota
	FlagCraftable
	FlagAustralium
	FlagFestivized
	FlagElevatedStrange
)

// Has reports whether the specified flag is set.
func (f ItemFlags) Has(flag ItemFlags) bool {
	return (f & flag) != 0
}

// PackedItem represents a ultra-compact 32-byte value struct without pointers.
// Because it contains zero pointer fields, Go runtime allocates []PackedItem
// with the noscan memory flag, completely skipping GC scanning overhead.
type PackedItem struct {
	AssetID     uint64
	OriginalID  uint64
	Paint       uint32
	DefIndex    uint16
	Effect      uint16
	Paintkit    uint16
	Position    uint16
	Quality     uint8
	Flags       ItemFlags
	Killstreak  uint8
	Wear        uint8
	CrateSeries uint8
	_           uint8
}

// PackGCItem converts a Game Coordinator Item struct into a PackedItem.
func PackGCItem(it *Item) PackedItem {
	if it == nil {
		return PackedItem{}
	}

	var flags ItemFlags
	if it.IsTradable {
		flags |= FlagTradable
	}

	if it.IsCraftable {
		flags |= FlagCraftable
	}

	if it.Australium {
		flags |= FlagAustralium
	}

	if it.Festivized {
		flags |= FlagFestivized
	}

	if it.IsElevated {
		flags |= FlagElevatedStrange
	}

	return PackedItem{
		AssetID:     it.ID,
		OriginalID:  it.OriginalID,
		Paint:       it.PaintPrimary,
		DefIndex:    uint16(it.DefIndex),
		Effect:      uint16(it.Effect),
		Paintkit:    uint16(it.Paintkit),
		Position:    uint16(it.Position()), // Preserves backpack slot position!
		Quality:     uint8(it.Quality),
		Flags:       flags,
		Killstreak:  uint8(it.KillstreakTier),
		Wear:        uint8(schema.WearToTier(it.Wear)),
		CrateSeries: uint8(it.CrateSeries),
	}
}

// ToItem expands a PackedItem back into a full Item struct with exact position restoration.
func (p PackedItem) ToItem(s *schema.Schema) *Item {
	item := &Item{
		ID:             p.AssetID,
		OriginalID:     p.OriginalID,
		DefIndex:       uint32(p.DefIndex),
		Quality:        uint32(p.Quality),
		PaintPrimary:   p.Paint,
		Effect:         uint32(p.Effect),
		Paintkit:       uint32(p.Paintkit),
		Inventory:      uint32(p.Position),
		KillstreakTier: uint32(p.Killstreak),
		Wear:           float32(p.Wear) / 5.0,
		CrateSeries:    uint32(p.CrateSeries),
		IsTradable:     p.Flags.Has(FlagTradable),
		IsCraftable:    p.Flags.Has(FlagCraftable),
		Australium:     p.Flags.Has(FlagAustralium),
		Festivized:     p.Flags.Has(FlagFestivized),
		IsElevated:     p.Flags.Has(FlagElevatedStrange),
	}

	if s != nil {
		item.Fix(s)
		item.SKU = item.GetSKU(s)
	}

	return item
}

// ToSKU generates a standardized SKU string from the packed item attributes.
func (p PackedItem) ToSKU(s *schema.Schema) string {
	quality := int(p.Quality)
	quality2 := 0

	if p.Flags.Has(FlagElevatedStrange) && p.Quality != schema.QualityStrange {
		quality2 = schema.Quality2Strange
	}

	if p.Effect != 0 && p.Quality == schema.QualityStrange && p.Paintkit == 0 {
		quality = schema.QualityUnusual
		quality2 = schema.Quality2Strange
	}

	sItem := &sku.Item{
		Defindex:    int(p.DefIndex),
		Quality:     quality,
		Quality2:    quality2,
		Tradable:    p.Flags.Has(FlagTradable),
		Craftable:   p.Flags.Has(FlagCraftable),
		Killstreak:  int(p.Killstreak),
		Australium:  p.Flags.Has(FlagAustralium),
		Effect:      int(p.Effect),
		Festivized:  p.Flags.Has(FlagFestivized),
		Paintkit:    int(p.Paintkit),
		Wear:        int(p.Wear),
		Crateseries: int(p.CrateSeries),
		Paint:       int(p.Paint),
	}

	if s != nil {
		s.NormalizeItem(sItem)
	}

	return sku.FromObject(sItem)
}

// IsTradable reports whether the item can be traded.
func (p PackedItem) IsTradable() bool {
	return p.Flags.Has(FlagTradable)
}

// IsCraftable reports whether the item is usable in crafting.
func (p PackedItem) IsCraftable() bool {
	return p.Flags.Has(FlagCraftable)
}
