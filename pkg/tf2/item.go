// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"slices"
	"strconv"

	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

type EconItemFlag uint32

const (
	EconItemFlagCannotTrade EconItemFlag = 1 << iota
	EconItemFlagCannotBeUsedInCrafting
	EconItemFlagCanBeTradedByFreeAccounts
	EconItemFlagNonEconomy
	EconItemFlagPurchasedAfterStoreCraftabilityChanges2012
	EconItemFlagForceBlueTeam
	EconItemFlagStoreItem
	EconItemFlagPreview
)

func (f EconItemFlag) HasFlag(flag EconItemFlag) bool {
	return (f & flag) != 0
}

const (
	AttrMedalNumber         uint32 = 133
	AttrUnusualEffect       uint32 = 134
	AttrPaintPrimary        uint32 = 142
	AttrCustomTextureLow    uint32 = 152
	AttrCannotTrade         uint32 = 153
	AttrGifterAccountID     uint32 = 186
	AttrCrateSeries         uint32 = 187
	AttrAlwaysTradable      uint32 = 195
	AttrTradableAfter       uint32 = 211
	AttrKillEater           uint32 = 214
	AttrKillEaterScoreValue uint32 = 379
	AttrCustomTextureHigh   uint32 = 227
	AttrCrafterAccountID    uint32 = 228
	AttrCraftNumber         uint32 = 229
	AttrPaintSecondary      uint32 = 261
	AttrStrangePart1        uint32 = 380
	AttrStrangePart1Val     uint32 = 381
	AttrStrangePart2        uint32 = 382
	AttrStrangePart2Val     uint32 = 383
	AttrStrangePart3        uint32 = 384
	AttrStrangePart3Val     uint32 = 385
	AttrCannotCraft         uint32 = 449
	AttrCustomName          uint32 = 500
	AttrCustomDesc          uint32 = 501
	AttrEOTLEarlySupporter  uint32 = 703
	AttrWear                uint32 = 725
	AttrPaintkitSeedLo      uint32 = 866
	AttrPaintkitSeedHi      uint32 = 867
	AttrPaintkit            uint32 = 834
	AttrSpell1              uint32 = 1004
	AttrSpell2              uint32 = 1005
	AttrSpell3              uint32 = 1006
	AttrSpell4              uint32 = 1007
	AttrSpell5              uint32 = 1008
	AttrSpell6              uint32 = 1009
	AttrTarget              uint32 = 2012
	AttrKillstreaker        uint32 = 2013
	AttrSheen               uint32 = 2014
	AttrKillstreakTier      uint32 = 2025
	AttrAustralium          uint32 = 2027
	AttrSeries              uint32 = 2031
	AttrTauntUnusualEffect  uint32 = 2041
	AttrQuestLoanerIDLow    uint32 = 2051
	AttrQuestLoanerIDHigh   uint32 = 2052
	AttrFestivized          uint32 = 2053
)

const (
	KillstreakTierNone uint32 = iota
	KillstreakTierBasic
	KillstreakTierSpecialized
	KillstreakTierProfessional
)

const (
	OriginDrop        uint32 = 0
	OriginAchievement uint32 = 1
	OriginPurchase    uint32 = 2
	OriginStorePromo  uint32 = 5
	OriginSupport     uint32 = 7
	OriginHalloween   uint32 = 12
	OriginForeign     uint32 = 14
	OriginPreview     uint32 = 17
	OriginWorkshop    uint32 = 18
	OriginLoaner      uint32 = 24
)

const (
	QualityNormal     uint32 = 0
	QualityGenuine    uint32 = 1
	QualityVintage    uint32 = 3
	QualityUnusual    uint32 = 5
	QualityUnique     uint32 = 6
	QualityCommunity  uint32 = 7
	QualityValve      uint32 = 8
	QualitySelfMade   uint32 = 9
	QualityCustomized uint32 = 10
	QualityStrange    uint32 = 11
	QualityCompleted  uint32 = 12
	QualityHaunted    uint32 = 13
	QualityCollectors uint32 = 14
	QualityDecorated  uint32 = 15
)

type Item struct {
	ID               uint64
	OriginalID       uint64
	AccountID        uint32
	DefIndex         uint32
	Level            uint32
	Quality          uint32
	Inventory        uint32
	Quantity         uint32
	Origin           uint32
	Flags            EconItemFlag
	Style            uint32
	InUse            bool
	CustomName       string
	CustomDesc       string
	SKU              string
	IsTradable       bool
	TradableAfter    uint32
	IsMarketable     bool
	IsCraftable      bool
	Effect           uint32
	KillstreakTier   uint32
	Australium       bool
	Festivized       bool
	Wear             float32
	Paintkit         uint32
	PaintkitSeed     uint64
	CrateSeries      uint32
	PaintPrimary     uint32
	PaintSecondary   uint32
	ScoreCount       uint32
	CrafterAccountID uint32
	GifterAccountID  uint32
	Sheen            uint32
	Killstreaker     uint32
	CraftNumber      uint32
	Series           uint32
	MedalNumber      uint32
	Target           uint32
	IsElevated       bool
	EarlySupporter   bool
	QuestID          uint64
	IsBuggedLoaner   bool
	Spells           []sku.Spell
	Parts            []uint32
	PartValues       map[uint32]uint32
	HasCustomDecal   bool
	DecalUGCID       uint64
	ImageURL         string
	ImageURLLarge    string
	RecipeComponents []RecipeComponent
}

type RecipeComponent struct {
	SlotIndex        uint32
	DefIndex         uint32
	Quality          uint32
	Flags            uint32
	NumRequired      uint32
	NumFulfilled     uint32
	AttributesString string
}

func (c *RecipeComponent) IsOutput() bool     { return c.Flags&0x01 != 0 }
func (c *RecipeComponent) IsUntradable() bool { return c.Flags&0x02 != 0 }
func (c *RecipeComponent) HasDefIndex() bool  { return c.Flags&0x04 != 0 }
func (c *RecipeComponent) HasQuality() bool   { return c.Flags&0x08 != 0 }
func (c *RecipeComponent) IsComplete() bool   { return c.NumFulfilled >= c.NumRequired }

func (i *Item) Position() uint32 {
	return i.Inventory & 0xFFFF
}

func (i *Item) GetSchema(s *schema.Schema) *schema.Item {
	return s.ItemByDef(int(i.DefIndex))
}

func (i *Item) IsWeapon(s *schema.Schema) bool {
	sch := i.GetSchema(s)
	return sch != nil && sch.CraftClass == "weapon"
}

func (i Item) ToEconItem() *trading.Item {
	item := &trading.Item{
		AppID:          AppID,
		ContextID:      2,
		AssetID:        i.ID,
		ClassID:        uint64(i.DefIndex),
		Amount:         int64(i.Quantity),
		Name:           i.CustomName,
		MarketName:     i.CustomName,
		MarketHashName: i.CustomName,
		Tradable:       i.IsTradable,
		Marketable:     i.IsMarketable,
		SKU:            i.SKU,
	}

	attrs := make([]trading.Attribute, 0, len(i.Spells)+len(i.Parts)+4)

	var floatBuf [24]byte

	addAttr := func(defindex int, val float64) {
		b := strconv.AppendFloat(floatBuf[:0], val, 'g', -1, 64)
		attrs = append(attrs, trading.Attribute{
			Defindex:   defindex,
			Value:      bytesconv.B2S(b),
			FloatValue: val,
		})
	}

	if i.MedalNumber != 0 {
		addAttr(133, float64(i.MedalNumber))
	}

	if i.Effect != 0 {
		addAttr(134, float64(i.Effect))
	}

	if i.PaintPrimary != 0 {
		addAttr(142, float64(i.PaintPrimary))
	}

	if i.DecalUGCID != 0 {
		lo := uint32(i.DecalUGCID & 0xFFFFFFFF)
		hi := uint32(i.DecalUGCID >> 32)

		if lo != 0 {
			addAttr(152, float64(lo))
		}

		if hi != 0 {
			addAttr(227, float64(hi))
		}
	}

	if i.Wear != 0 {
		addAttr(725, float64(i.Wear))
	}

	if i.PaintSecondary != 0 {
		addAttr(261, float64(i.PaintSecondary))
	}

	if i.CrateSeries != 0 {
		addAttr(187, float64(i.CrateSeries))
	}

	if i.CraftNumber != 0 {
		addAttr(229, float64(i.CraftNumber))
	}

	if i.Paintkit != 0 {
		addAttr(834, float64(i.Paintkit))
	}

	if i.PaintkitSeed != 0 {
		lo := uint32(i.PaintkitSeed & 0xFFFFFFFF)
		hi := uint32(i.PaintkitSeed >> 32)

		if lo != 0 {
			addAttr(866, float64(lo))
		}

		if hi != 0 {
			addAttr(867, float64(hi))
		}
	}

	if i.Target != 0 {
		addAttr(2012, float64(i.Target))
	}

	if i.Killstreaker != 0 {
		addAttr(2013, float64(i.Killstreaker))
	}

	if i.Sheen != 0 {
		addAttr(2014, float64(i.Sheen))
	}

	if i.KillstreakTier != 0 {
		addAttr(2025, float64(i.KillstreakTier))
	}

	if i.Australium {
		addAttr(2027, 1.0)
	}

	if i.Series != 0 {
		addAttr(2031, float64(i.Series))
	}

	if i.QuestID != 0 {
		lo := uint32(i.QuestID & 0xFFFFFFFF)
		hi := uint32(i.QuestID >> 32)

		if lo != 0 {
			addAttr(2051, float64(lo))
		}

		if hi != 0 {
			addAttr(2052, float64(hi))
		}
	}

	if i.Festivized {
		addAttr(2053, 1.0)
	}

	if i.EarlySupporter {
		addAttr(703, 1.0)
	}

	if i.IsElevated || i.Quality == 11 {
		addAttr(214, 1.0)
	}

	if i.CrafterAccountID != 0 {
		addAttr(228, float64(i.CrafterAccountID))
	}

	if i.GifterAccountID != 0 {
		addAttr(186, float64(i.GifterAccountID))
	}

	for _, spell := range i.Spells {
		addAttr(spell.Attribute, float64(spell.Value))
	}

	for idx, part := range i.Parts {
		addAttr(10000+idx, float64(part))
	}

	item.Attributes = attrs

	return item
}

func (i Item) ToSKUObject() *sku.Item {
	quality := int(i.Quality)
	quality2 := 0

	if i.IsElevated && i.Quality != QualityStrange {
		quality2 = schema.Quality2Strange
	}

	if i.Effect != 0 && i.Quality == 11 && i.Paintkit == 0 {
		quality = schema.QualityUnusual
		quality2 = schema.Quality2Strange
	}

	return &sku.Item{
		Defindex:    int(i.DefIndex),
		Quality:     quality,
		Quality2:    quality2,
		Tradable:    i.IsTradable,
		Craftable:   i.IsCraftable,
		Killstreak:  int(i.KillstreakTier),
		Australium:  i.Australium,
		Effect:      int(i.Effect),
		Festivized:  i.Festivized,
		Paintkit:    int(i.Paintkit),
		Seed:        int(i.PaintkitSeed),
		Wear:        schema.WearToTier(i.Wear),
		Craftnumber: int(i.CraftNumber),
		Crateseries: int(i.CrateSeries),
		Target:      int(i.Target),
		Paint:       int(i.PaintPrimary),
		Spells:      i.Spells,
		Parts: func() []int {
			p := make([]int, len(i.Parts))
			for idx, v := range i.Parts {
				p[idx] = int(v)
			}

			return p
		}(),
	}
}

func (i *Item) GetSKU(s *schema.Schema) string {
	if i.SKU != "" {
		return i.SKU
	}

	return s.SKUFromItem(i.ToSKUObject())
}

func (i *Item) Fix(s *schema.Schema) {
	sch := i.GetSchema(s)
	if sch == nil {
		return
	}

	i.ImageURL = sch.ImageURL
	i.ImageURLLarge = sch.ImageURLLarge

	for _, attr := range sch.Attributes {
		if attr.Name == "cannot trade" || attr.Class == "cannot_trade" {
			if attr.Value == 1 {
				i.IsTradable = false
				i.IsMarketable = false
			}
		}

		if attr.Name == "cannot craft" || attr.Class == "cannot_craft" {
			if attr.Value == 1 {
				i.IsCraftable = false
			}
		}
	}

	if (i.DefIndex >= 5726 && i.DefIndex <= 5733) ||
		(i.DefIndex >= 5743 && i.DefIndex <= 5751) ||
		(i.DefIndex >= 5793 && i.DefIndex <= 5801) {
		i.DefIndex = 6527
	}

	strangifiers := []uint32{
		5661, 5721, 5722, 5723, 5724, 5725, 5753, 5754, 5755, 5756, 5757, 5758, 5759, 5783, 5784, 5804,
	}
	if slices.Contains(strangifiers, i.DefIndex) {
		i.DefIndex = 6522
	}

	if i.DefIndex >= 20001 && i.DefIndex <= 20009 {
		i.DefIndex = 20000
	}

	if sch.ItemClass == "supply_crate" && i.CrateSeries == 0 {
		for _, attr := range sch.Attributes {
			if attr.Name == "set supply crate series" {
				i.CrateSeries = uint32(attr.Value)
				break
			}
		}
	}
}
