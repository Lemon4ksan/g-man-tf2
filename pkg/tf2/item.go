// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"fmt"
	"slices"

	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// EconItemFlag represents bitmask flags for Steam Econ items.
type EconItemFlag uint32

const (
	// EconItemFlagCannotTrade indicates the item cannot be traded.
	EconItemFlagCannotTrade EconItemFlag = 1 << iota
	// EconItemFlagCannotBeUsedInCrafting indicates the item cannot be used in crafting.
	EconItemFlagCannotBeUsedInCrafting
	// EconItemFlagCanBeTradedByFreeAccounts indicates the item can be traded even if the account is not premium.
	EconItemFlagCanBeTradedByFreeAccounts
	// EconItemFlagNonEconomy indicates the item is a non-economy item.
	EconItemFlagNonEconomy
	// EconItemFlagPurchasedAfterStoreCraftabilityChanges2012 relates to store items bought after the 2012 craftability update.
	EconItemFlagPurchasedAfterStoreCraftabilityChanges2012
	// EconItemFlagForceBlueTeam indicates the item is forced to blue team.
	EconItemFlagForceBlueTeam
	// EconItemFlagStoreItem indicates the item was bought from the Mann Co. Store.
	EconItemFlagStoreItem
	// EconItemFlagPreview indicates the item is a preview item from the store.
	EconItemFlagPreview
)

// HasFlag checks if the provided flag is set in the bitmask.
func (f EconItemFlag) HasFlag(flag EconItemFlag) bool {
	return (f & flag) != 0
}

// Attributes list configuration from items_game.txt.
const (
	// AttrMedalNumber represents a registration number on tournament medals.
	AttrMedalNumber uint32 = 133
	// AttrUnusualEffect represents the Unusual particle effect identifier.
	AttrUnusualEffect uint32 = 134
	// AttrPaintPrimary represents the primary team paint color decimal value.
	AttrPaintPrimary uint32 = 142
	// AttrCustomTextureLow represents the low texture ID for custom decals.
	AttrCustomTextureLow uint32 = 152
	// AttrCannotTrade represents the non-tradable restriction.
	AttrCannotTrade uint32 = 153
	// AttrGifterAccountID represents the gifter account ID attribute.
	AttrGifterAccountID uint32 = 186
	// AttrCrateSeries represents the supply crate series number attribute.
	AttrCrateSeries uint32 = 187
	// AttrAlwaysTradable represents the attribute overriding standard tradable constraints.
	AttrAlwaysTradable uint32 = 195
	// AttrTradableAfter represents the unix timestamp limitation for trades.
	AttrTradableAfter uint32 = 211
	// AttrKillEater represents a Strange part counter tracking presence or score type.
	AttrKillEater uint32 = 214
	// AttrKillEaterScoreValue represents the main Strange score counter value.
	AttrKillEaterScoreValue uint32 = 379
	// AttrCustomTextureHigh represents the high texture ID for custom decals.
	AttrCustomTextureHigh uint32 = 227
	// AttrCrafterAccountID represents the account ID of the crafter.
	AttrCrafterAccountID uint32 = 228
	// AttrCraftNumber represents the craft index numbering attribute.
	AttrCraftNumber uint32 = 229
	// AttrPaintSecondary represents the secondary team paint color decimal value.
	AttrPaintSecondary uint32 = 261
	// AttrStrangePart1 represents the first applied Strange Part slot.
	AttrStrangePart1 uint32 = 380
	// AttrStrangePart1Val represents the first applied Strange Part counter value.
	AttrStrangePart1Val uint32 = 381
	// AttrStrangePart2 represents the second applied Strange Part slot.
	AttrStrangePart2 uint32 = 382
	// AttrStrangePart2Val represents the second applied Strange Part counter value.
	AttrStrangePart2Val uint32 = 383
	// AttrStrangePart3 represents the third applied Strange Part slot.
	AttrStrangePart3 uint32 = 384
	// AttrStrangePart3Val represents the third applied Strange Part counter value.
	AttrStrangePart3Val uint32 = 385
	// AttrCannotCraft represents the non-craftable restriction.
	AttrCannotCraft uint32 = 449
	// AttrCustomName represents a Name Tag customization in TF2.
	AttrCustomName uint32 = 500
	// AttrCustomDesc represents a Description Tag customization in TF2.
	AttrCustomDesc uint32 = 501
	// AttrEOTLEarlySupporter represents the End of the Line supporter tag.
	AttrEOTLEarlySupporter uint32 = 703
	// AttrWear represents the skin pattern wear float value.
	AttrWear uint32 = 725
	// AttrPaintkitSeedLo represents the low 32 bits of the custom paintkit seed.
	AttrPaintkitSeedLo uint32 = 866
	// AttrPaintkitSeedHi represents the high 32 bits of the custom paintkit seed.
	AttrPaintkitSeedHi uint32 = 867
	// AttrPaintkit represents the skin pattern index identifier.
	AttrPaintkit uint32 = 834
	// AttrSpell1 represents the first applied Halloween spell effect slot.
	AttrSpell1 uint32 = 1004
	// AttrSpell2 represents the second applied Halloween spell effect slot.
	AttrSpell2 uint32 = 1005
	// AttrSpell3 represents the third applied Halloween spell effect slot.
	AttrSpell3 uint32 = 1006
	// AttrSpell4 represents the fourth applied Halloween spell effect slot.
	AttrSpell4 uint32 = 1007
	// AttrSpell5 represents the fifth applied Halloween spell effect slot.
	AttrSpell5 uint32 = 1008
	// AttrSpell6 represents the sixth applied Halloween spell effect slot.
	AttrSpell6 uint32 = 1009
	// AttrTarget represents the target item defindex for tools.
	AttrTarget uint32 = 2012
	// AttrKillstreaker represents the professional killstreak eye effect index.
	AttrKillstreaker uint32 = 2013
	// AttrSheen represents the specialized killstreak weapon sheen effect index.
	AttrSheen uint32 = 2014
	// AttrKillstreakTier represents the active killstreak tier level.
	AttrKillstreakTier uint32 = 2025
	// AttrAustralium represents the Australium weapon quality attribute flag.
	AttrAustralium uint32 = 2027
	// AttrSeries represents the secondary series counter (e.g. Duck Journal).
	AttrSeries uint32 = 2031
	// AttrTauntUnusualEffect represents the Unusual particle effect for taunts.
	AttrTauntUnusualEffect uint32 = 2041
	// AttrQuestLoanerIDLow represents the low 32-bit of the linked contract ID.
	AttrQuestLoanerIDLow uint32 = 2051
	// AttrQuestLoanerIDHigh represents the high 32-bit of the linked contract ID.
	AttrQuestLoanerIDHigh uint32 = 2052
	// AttrFestivized represents the festivizer modification attribute flag.
	AttrFestivized uint32 = 2053
)

// Killstreak tiers configuration.
const (
	// KillstreakTierNone indicates no active killstreak.
	KillstreakTierNone uint32 = iota
	// KillstreakTierBasic indicates standard killstreak kit.
	KillstreakTierBasic
	// KillstreakTierSpecialized indicates specialized killstreak kit with weapon sheen.
	KillstreakTierSpecialized
	// KillstreakTierProfessional indicates professional killstreak kit with weapon sheen and eye effects.
	KillstreakTierProfessional
)

// Item Origins configuration.
const (
	// OriginDrop represents a random item drop.
	OriginDrop uint32 = 0
	// OriginAchievement represents an achievement unlock award.
	OriginAchievement uint32 = 1
	// OriginPurchase represents an item purchased from store.
	OriginPurchase uint32 = 2
	// OriginStorePromo represents store promotion item.
	OriginStorePromo uint32 = 5
	// OriginSupport represents item granted by Steam Support.
	OriginSupport uint32 = 7
	// OriginHalloween represents a Halloween cauldron or drop reward.
	OriginHalloween uint32 = 12
	// OriginForeign represents an unverified source.
	OriginForeign uint32 = 14
	// OriginPreview represents a temporary store preview item.
	OriginPreview uint32 = 17
	// OriginWorkshop represents a Steam Workshop contribution award.
	OriginWorkshop uint32 = 18
	// OriginLoaner represents a temporary contract loaner item.
	OriginLoaner uint32 = 24
)

// Quality configuration values.
const (
	// QualityNormal represents Stock quality items.
	QualityNormal uint32 = 0
	// QualityGenuine represents Genuine quality promo items.
	QualityGenuine uint32 = 1
	// QualityVintage represents Vintage quality items.
	QualityVintage uint32 = 3
	// QualityUnusual represents Unusual quality cosmetic items.
	QualityUnusual uint32 = 5
	// QualityUnique represents Unique quality standard items.
	QualityUnique uint32 = 6
	// QualityCommunity represents Community quality contribution items.
	QualityCommunity uint32 = 7
	// QualityValve represents Valve developer quality items.
	QualityValve uint32 = 8
	// QualitySelfMade represents Self-Made creator quality items.
	QualitySelfMade uint32 = 9
	// QualityCustomized represents Customized quality items.
	QualityCustomized uint32 = 10
	// QualityStrange represents Strange quality counting items.
	QualityStrange uint32 = 11
	// QualityCompleted represents Completed quality items.
	QualityCompleted uint32 = 12
	// QualityHaunted represents Haunted quality Halloween items.
	QualityHaunted uint32 = 13
	// QualityCollectors represents Collector's quality compiled items.
	QualityCollectors uint32 = 14
	// QualityDecorated represents Decorated quality skins.
	QualityDecorated uint32 = 15
)

// Item represents a parsed and normalized TF2 item with its economic and technical metadata.
type Item struct {
	// ID represents the unique asset identifier of the item.
	ID uint64
	// OriginalID represents the permanent origin identifier assigned upon creation.
	OriginalID uint64
	// AccountID represents the Steam Account ID of the current owner.
	AccountID uint32
	// DefIndex represents the item definition index from the schema.
	DefIndex uint32
	// Level represents the cosmetic item level.
	Level uint32
	// Quality represents the quality ID of the item.
	Quality uint32
	// Inventory represents the raw inventory positioning bits.
	Inventory uint32
	// Quantity represents the stack size of the item.
	Quantity uint32
	// Origin represents the item drop, craft, or purchase source ID.
	Origin uint32
	// Flags represents the bitmask restrictions applied to the item.
	Flags EconItemFlag
	// Style represents the selected cosmetic style index.
	Style uint32
	// InUse indicates whether the item is currently equipped or in use.
	InUse bool

	// CustomName represents the custom text applied by a name tag.
	CustomName string
	// CustomDesc represents the custom text applied by a description tag.
	CustomDesc string
	// SKU represents the canonical SKU string for pricing.
	SKU string

	// IsTradable indicates whether the item can be traded.
	IsTradable bool
	// TradableAfter represents the Unix timestamp after which the item can be traded.
	// If 0, the item can be traded immediately.
	TradableAfter uint32
	// IsMarketable indicates whether the item can be listed on the Steam Market.
	IsMarketable bool
	// IsCraftable indicates whether the item can be used in crafting recipes.
	IsCraftable bool

	// Effect represents the Unusual particle effect ID.
	Effect uint32
	// KillstreakTier represents the active killstreak tier index.
	KillstreakTier uint32
	// Australium indicates whether the item is an Australium variant.
	Australium bool
	// Festivized indicates whether a festivizer has been applied.
	Festivized bool
	// Wear represents the skin wear fraction value.
	Wear float32
	// Paintkit represents the skin pattern ID.
	Paintkit uint32
	// PaintkitSeed represents the skin pattern seed.
	PaintkitSeed uint64
	// CrateSeries represents the series number for supply crates.
	CrateSeries uint32
	// PaintPrimary represents the applied paint color ID.
	PaintPrimary uint32
	// PaintSecondary represents the secondary paint color ID.
	PaintSecondary uint32
	// ScoreCount represents the number of kills on the item.
	ScoreCount uint32
	// CrafterAccountID represents the account ID of the crafter.
	CrafterAccountID uint32
	// GifterAccountID represents the account ID of the gifter.
	GifterAccountID uint32

	// Sheen represents the killstreak sheen effect index.
	Sheen uint32
	// Killstreaker represents the professional killstreak eye effect index.
	Killstreaker uint32
	// CraftNumber represents the limited edition craft number.
	CraftNumber uint32
	// Series represents the secondary series ID.
	Series uint32
	// MedalNumber represents the number assigned to tournament medals.
	MedalNumber uint32
	// Target represents the defindex of the target item.
	Target uint32
	// IsElevated indicates whether the item is an elevated strange cosmetic.
	IsElevated bool
	// EarlySupporter indicates whether the item has the early supporter tag.
	EarlySupporter bool
	// QuestID represents the 64-bit ID of the linked contract.
	QuestID uint64
	// IsBuggedLoaner indicates whether the loaner is exceptionally tradable.
	IsBuggedLoaner bool
	// Spells contains the list of applied Halloween spells.
	Spells []sku.Spell
	// Parts contains the list of Strange Part IDs.
	Parts []uint32
	// PartValues maps Strange Part IDs to their counter values.
	PartValues map[uint32]uint32
	// HasCustomDecal indicates whether the item has a custom decal (applied picture).
	HasCustomDecal bool
	// DecalUGCID represents the reconstructed 64-bit UGC ID of the custom decal image.
	DecalUGCID uint64

	// ImageURL represents the URL of the small backpack icon for the relevant item.
	ImageURL string
	// ImageURLLarge represents the URL of the large backpack image for the relevant item.
	ImageURLLarge string

	// RecipeComponents contains the parsed dynamic recipe components (fabricator/chemistry set inputs/outputs).
	RecipeComponents []RecipeComponent
}

// RecipeComponent represents a single ingredient or output slot of a dynamic recipe item (fabricator, chemistry set).
type RecipeComponent struct {
	// SlotIndex is the attribute definition index (2000-2009) identifying this component slot.
	SlotIndex uint32
	// DefIndex is the required item definition index (0 if not specified).
	DefIndex uint32
	// Quality is the required item quality (0 if not specified).
	Quality uint32
	// Flags is the component_flags bitmask.
	Flags uint32
	// NumRequired is how many items are needed for this component.
	NumRequired uint32
	// NumFulfilled is how many items have already been contributed.
	NumFulfilled uint32
	// AttributesString is the raw encoded attribute requirements.
	AttributesString string
}

// IsOutput returns true if this component is an output (reward) slot.
func (c *RecipeComponent) IsOutput() bool {
	return c.Flags&0x01 != 0
}

// IsUntradable returns true if the produced item will be untradable.
func (c *RecipeComponent) IsUntradable() bool {
	return c.Flags&0x02 != 0
}

// HasDefIndex returns true if this component specifies a required item definition.
func (c *RecipeComponent) HasDefIndex() bool {
	return c.Flags&0x04 != 0
}

// HasQuality returns true if this component specifies a required quality.
func (c *RecipeComponent) HasQuality() bool {
	return c.Flags&0x08 != 0
}

// IsComplete returns true if all required items have been fulfilled.
func (c *RecipeComponent) IsComplete() bool {
	return c.NumFulfilled >= c.NumRequired
}

// Position returns the item's slot index in the backpack.
func (i *Item) Position() uint32 {
	return i.Inventory & 0xFFFF
}

// GetSchema returns the [schema.Item] metadata for the item.
func (i *Item) GetSchema(s *schema.Schema) *schema.Item {
	return s.ItemByDef(int(i.DefIndex))
}

// IsWeapon returns true if the item is classified as a weapon.
func (i *Item) IsWeapon(s *schema.Schema) bool {
	sch := i.GetSchema(s)
	return sch != nil && sch.CraftClass == "weapon"
}

// ToEconItem maps the item fields into a universal exchange [trading.Item] format.
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

	attrs := make([]trading.Attribute, 0)
	addAttr := func(defindex int, val float64) {
		attrs = append(attrs, trading.Attribute{
			Defindex:   defindex,
			Value:      fmt.Sprintf("%g", val),
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

// ToSKUObject converts the item parameters to a [sku.Item] representation.
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

// GetSKU retrieves or generates the standard SKU string for the item.
func (i *Item) GetSKU(s *schema.Schema) string {
	if i.SKU != "" {
		return i.SKU
	}

	return s.SKUFromItem(i.ToSKUObject())
}

// Fix applies schema-based normalizations and defindex overrides to the item.
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
