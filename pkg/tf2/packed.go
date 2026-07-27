// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

type ItemFlags uint8

const (
	FlagTradable ItemFlags = 1 << iota
	FlagCraftable
	FlagAustralium
	FlagFestivized
	FlagElevatedStrange
)

func (f ItemFlags) Has(flag ItemFlags) bool {
	return (f & flag) != 0
}

// PackedItem represents an ultra-compact 32-byte struct without pointers for zero GC overhead.
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
		Position:    uint16(it.Position()),
		Quality:     uint8(it.Quality),
		Flags:       flags,
		Killstreak:  uint8(it.KillstreakTier),
		Wear:        uint8(schema.WearToTier(it.Wear)),
		CrateSeries: uint8(it.CrateSeries),
	}
}

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

func (p PackedItem) ToSKU(s *schema.Schema) string {
	sItem := sku.GetItem()
	defer sku.ReleaseItem(sItem)

	quality := int(p.Quality)
	quality2 := 0

	if p.Flags.Has(FlagElevatedStrange) && p.Quality != schema.QualityStrange {
		quality2 = schema.Quality2Strange
	}

	if p.Effect != 0 && p.Quality == schema.QualityStrange && p.Paintkit == 0 {
		quality = schema.QualityUnusual
		quality2 = schema.Quality2Strange
	}

	sItem.Defindex = int(p.DefIndex)
	sItem.Quality = quality
	sItem.Quality2 = quality2
	sItem.Tradable = p.Flags.Has(FlagTradable)
	sItem.Craftable = p.Flags.Has(FlagCraftable)
	sItem.Killstreak = int(p.Killstreak)
	sItem.Australium = p.Flags.Has(FlagAustralium)
	sItem.Effect = int(p.Effect)
	sItem.Festivized = p.Flags.Has(FlagFestivized)
	sItem.Paintkit = int(p.Paintkit)
	sItem.Wear = int(p.Wear)
	sItem.Crateseries = int(p.CrateSeries)
	sItem.Paint = int(p.Paint)

	if s != nil {
		s.NormalizeItem(sItem)
	}

	return sku.FromObject(sItem)
}

func (p PackedItem) IsTradable() bool  { return p.Flags.Has(FlagTradable) }
func (p PackedItem) IsCraftable() bool { return p.Flags.Has(FlagCraftable) }
