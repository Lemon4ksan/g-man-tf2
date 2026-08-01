// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

var (
	ErrItemNotFound = errors.New("backpack: item not found in inventory")
	ErrSteamAPI     = errors.New("backpack: steam webapi returned error status")
)

type HistoryStatus struct {
	Recorded bool
	IsDuped  bool
}

type DupeChecker interface {
	CheckHistory(ctx context.Context, assetID uint64, mods ...aoni.RequestModifier) (HistoryStatus, error)
}

type TF2Item struct {
	ID              uint64         `json:"id"`
	OriginalID      uint64         `json:"original_id"`
	Defindex        int            `json:"defindex"`
	Level           int            `json:"level"`
	Quality         int            `json:"quality"`
	Inventory       uint32         `json:"inventory"`
	Quantity        int            `json:"quantity"`
	Origin          int            `json:"origin"`
	Style           int            `json:"style,omitempty"`
	FlagCannotTrade bool           `json:"flag_cannot_trade,omitempty"`
	FlagCannotCraft bool           `json:"flag_cannot_craft,omitempty"`
	CustomName      string         `json:"custom_name,omitempty"`
	CustomDesc      string         `json:"custom_desc,omitempty"`
	Attributes      []TF2Attribute `json:"attributes,omitempty"`
	SKU             string         `json:"sku,omitempty"`
}

func PackTF2Item(it *TF2Item) tf2.PackedItem {
	if it == nil {
		return tf2.PackedItem{}
	}

	var flags tf2.ItemFlags
	if !it.FlagCannotTrade {
		flags |= tf2.FlagTradable
	}

	if !it.FlagCannotCraft {
		flags |= tf2.FlagCraftable
	}

	var (
		effect, wear, paintkit, killstreak, paint, quality2, crateseries int
		isAustralium, isFestivized                                       bool
	)

	for _, attr := range it.Attributes {
		switch attr.Defindex {
		case schema.AttrUnusualEffect:
			if val, ok := attr.Value.(float64); ok {
				effect = int(val)
			}
		case schema.AttrWear:
			if val, ok := attr.Value.(float64); ok {
				wear = schema.WearToTier(float32(val))
			}
		case schema.AttrAustralium:
			isAustralium = true
		case schema.AttrPaintkit:
			if val, ok := attr.Value.(float64); ok {
				paintkit = int(val)
			}
		case schema.AttrKillstreak:
			if val, ok := attr.Value.(float64); ok {
				killstreak = int(val)
			}
		case schema.AttrFestivized:
			isFestivized = true
		case schema.AttrPaintColor, schema.AttrPaintColor2:
			if val, ok := attr.Value.(float64); ok {
				paint = int(val)
			}
		case schema.AttrCrateSeries:
			if val, ok := attr.Value.(float64); ok {
				crateseries = int(val)
			}
		case schema.AttrStrangeScore:
			quality2 = schema.QualityStrange
		}
	}

	if isAustralium {
		flags |= tf2.FlagAustralium
	}

	if isFestivized {
		flags |= tf2.FlagFestivized
	}

	if quality2 == schema.QualityStrange {
		flags |= tf2.FlagElevatedStrange
	}

	return tf2.PackedItem{
		AssetID:     it.ID,
		OriginalID:  it.OriginalID,
		Paint:       uint32(paint),
		DefIndex:    uint16(it.Defindex),
		Effect:      uint16(effect),
		Paintkit:    uint16(paintkit),
		Position:    uint16(it.Inventory & 0xFFFF),
		Quality:     uint8(it.Quality),
		Flags:       flags,
		Killstreak:  uint8(killstreak),
		Wear:        uint8(wear),
		CrateSeries: uint8(crateseries),
	}
}

func MapCEconToTF2(econ inventory.CEconItem, s *schema.Schema) TF2Item {
	asset := econ.Asset
	desc := econ.Description

	item := TF2Item{
		ID:              mustParseUint64(asset.AssetID),
		Quantity:        1,
		FlagCannotTrade: desc.Tradable == 0,
	}

	if desc.AppData == nil && len(desc.Tags) == 0 && len(desc.Descriptions) == 0 && desc.Name == "" {
		return item
	}

	if amount, err := strconv.Atoi(asset.Amount); err == nil {
		item.Quantity = amount
	}

	if desc.AppData != nil {
		item.Defindex = desc.AppData.DefIndex
		item.Quality = desc.AppData.Quality
		item.OriginalID = desc.AppData.OriginalID
	}

	if item.Defindex == 0 {
		if s != nil {
			resolveDefindexFromName(&item, &econ, s)
		}

		if item.Defindex == 0 {
			switch {
			case strings.Contains(desc.MarketHashName, "Refined Metal") || strings.Contains(desc.Name, "Refined Metal"):
				item.Defindex = 5002
			case strings.Contains(desc.MarketHashName, "Reclaimed Metal") || strings.Contains(desc.Name, "Reclaimed Metal"):
				item.Defindex = 5001
			case strings.Contains(desc.MarketHashName, "Scrap Metal") || strings.Contains(desc.Name, "Scrap Metal"):
				item.Defindex = 5000
			case strings.Contains(desc.MarketHashName, "Key") || strings.Contains(desc.Name, "Key"):
				item.Defindex = 5021
			}
		}
	}

	if item.Quality == 0 {
		item.Quality = 6 // QualityUnique
	}

	if item.Defindex == 0 || item.Quality == 0 {
		resolveQualityFromTags(&item, &econ, s)
	}

	parseCEconDescriptions(&item, &econ, s)

	if item.Quality == 15 {
		resolvePaintkitFromName(&item, desc.MarketHashName, s)
	}

	hasAustraliumAttr := desc.AppData != nil && desc.AppData.IsAustralium
	if !hasAustraliumAttr && item.Quality == schema.QualityStrange &&
		s != nil && s.IsAustraliumDefindex(item.Defindex) && strings.Contains(desc.MarketHashName, "Australium") {
		hasAustraliumAttr = true
	}

	if hasAustraliumAttr {
		item.Attributes = append(item.Attributes, TF2Attribute{
			Defindex: schema.AttrAustralium,
			Value:    float64(1),
		})
	}

	if strings.Contains(desc.Name, "Festivized") || (s != nil && s.IsNativeFestive(item.Defindex)) {
		item.Attributes = append(item.Attributes, TF2Attribute{
			Defindex: schema.AttrFestivized,
			Value:    float64(1),
		})
	}

	if s != nil {
		item.Defindex = s.NormalizeDefindex(item.Defindex)
	}

	item.SKU = item.ToSKU()

	return item
}

func resolveDefindexFromName(item *TF2Item, econ *inventory.CEconItem, s *schema.Schema) {
	desc := econ.Description
	nameToParse := desc.MarketHashName

	if nameToParse == "" {
		nameToParse = desc.Name
	}

	if nameToParse != "" {
		if parsed := s.ItemFromName(nameToParse); parsed != nil && parsed.Defindex > 0 {
			item.Defindex = parsed.Defindex
			if item.Quality == 0 {
				item.Quality = parsed.Quality
			}
		}
	}
}

func resolveQualityFromTags(item *TF2Item, econ *inventory.CEconItem, s *schema.Schema) {
	if s == nil {
		return
	}

	for _, tag := range econ.Description.Tags {
		if tag.Category == "Quality" && item.Quality == 0 {
			item.Quality = s.QualityIDByName(tag.LocalizedTagName)
		}
	}
}

func parseCEconDescriptions(item *TF2Item, econ *inventory.CEconItem, s *schema.Schema) {
	for _, d := range econ.Description.Descriptions {
		val := d.Value

		if strings.Contains(val, "( Not Usable in Crafting )") {
			item.FlagCannotCraft = true
			continue
		}

		if wearName, ok := strings.CutPrefix(val, "Exterior: "); ok && s != nil {
			if wearID := s.WearByName(wearName); wearID != 0 {
				item.Attributes = append(
					item.Attributes,
					TF2Attribute{Defindex: schema.AttrWear, Value: float64(wearID)},
				)
			}

			continue
		}

		if effectName, ok := strings.CutPrefix(val, "★ Unusual Effect: "); ok && s != nil {
			if effectID := s.EffectIDByName(effectName); effectID != 0 {
				item.Attributes = append(
					item.Attributes,
					TF2Attribute{Defindex: schema.AttrUnusualEffect, Value: float64(effectID)},
				)
			}

			continue
		}

		if strings.Contains(val, "Killstreak Active") || strings.Contains(val, "Killstreaks Active") ||
			strings.HasPrefix(val, "Killstreaker:") || strings.HasPrefix(val, "Sheen:") {
			parseKillstreakAttr(item, val, econ.Description.MarketHashName)
		}

		if paintName, ok := strings.CutPrefix(val, "Paint Color: "); ok && s != nil {
			if paintID := s.PaintDecimalByName(paintName); paintID != 0 {
				item.Attributes = append(
					item.Attributes,
					TF2Attribute{Defindex: schema.AttrPaintColor, Value: float64(paintID)},
				)
			}
		}

		if strings.Contains(val, "Crate Series #") {
			parts := strings.Split(val, "#")
			if len(parts) == 2 {
				if series, err := strconv.Atoi(parts[1]); err == nil {
					item.Attributes = append(
						item.Attributes,
						TF2Attribute{Defindex: schema.AttrCrateSeries, Value: float64(series)},
					)
				}
			}
		}

		if item.Quality != schema.QualityStrange &&
			(strings.Contains(val, "Strange Stat") || strings.Contains(val, "Strange Part")) {
			item.Attributes = append(
				item.Attributes,
				TF2Attribute{Defindex: schema.AttrStrangeScore, Value: float64(1)},
			)
		}

		color := strings.ToLower(d.Color)

		if color == "756b5e" {
			parseStrangePartColorAttr(item, val, s)
		}

		if color == "7ea9d1" && s != nil {
			if spell, ok := s.SpellIDByName(strings.TrimSpace(val)); ok {
				item.Attributes = append(
					item.Attributes,
					TF2Attribute{Defindex: schema.DefSpellProxy + len(item.Attributes), Value: spell},
				)
			}
		}
	}
}

func parseKillstreakAttr(item *TF2Item, val, marketHashName string) {
	ksLevel := 0

	switch {
	case strings.Contains(val, "Professional") || strings.HasPrefix(val, "Killstreaker:") || strings.Contains(marketHashName, "Professional Killstreak"):
		ksLevel = 3
	case strings.Contains(val, "Specialized") || strings.HasPrefix(val, "Sheen:") || strings.Contains(marketHashName, "Specialized Killstreak"):
		ksLevel = 2
	case strings.Contains(val, "Killstreak") || strings.Contains(marketHashName, "Killstreak"):
		ksLevel = 1
	}

	if ksLevel == 0 {
		return
	}

	for i := range item.Attributes {
		if item.Attributes[i].Defindex == schema.AttrKillstreak {
			if curr, ok := item.Attributes[i].Value.(float64); !ok || int(curr) < ksLevel {
				item.Attributes[i].Value = float64(ksLevel)
			}

			return
		}
	}

	item.Attributes = append(item.Attributes, TF2Attribute{Defindex: schema.AttrKillstreak, Value: float64(ksLevel)})
}

func parseStrangePartColorAttr(item *TF2Item, val string, s *schema.Schema) {
	clean := strings.Trim(val, "()")

	before, _, ok := strings.Cut(clean, ":")
	if !ok {
		return
	}

	partName := strings.TrimSpace(before)

	if s != nil {
		partsMap := s.StrangeParts()

		if suffix, found := partsMap[partName]; found {
			if partID, ok := bytesconv.ParseUint64(bytesconv.S2B(strings.TrimPrefix(suffix, "sp"))); ok {
				item.Attributes = append(item.Attributes, TF2Attribute{
					Defindex: schema.DefPartsProxy + len(item.Attributes),
					Value:    float64(partID),
				})

				return
			}
		}

		for name, suffix := range partsMap {
			if strings.Contains(partName, name) {
				if partID, ok := bytesconv.ParseUint64(bytesconv.S2B(strings.TrimPrefix(suffix, "sp"))); ok {
					item.Attributes = append(item.Attributes, TF2Attribute{
						Defindex: schema.DefPartsProxy + len(item.Attributes),
						Value:    float64(partID),
					})

					return
				}
			}
		}
	}

	item.Attributes = append(item.Attributes, TF2Attribute{
		Defindex: schema.DefPartsProxy + len(item.Attributes),
		Value:    float64(0),
	})
}

func resolvePaintkitFromName(item *TF2Item, name string, s *schema.Schema) {
	if s == nil {
		return
	}

	lowerName := strings.ToLower(name)

	for pkName, pkID := range s.PaintKitsByName() {
		if strings.Contains(lowerName, pkName) {
			item.Attributes = append(item.Attributes, TF2Attribute{Defindex: schema.AttrPaintkit, Value: float64(pkID)})

			break
		}
	}
}

func (it *TF2Item) ToSKU() string {
	if it.SKU != "" {
		return it.SKU
	}

	var (
		effect, wear, paintkit, killstreak, paint, quality2, crateseries int
		isAustralium, isFestivized                                       bool
		spells                                                           []sku.Spell
		parts                                                            []int
	)

	for _, attr := range it.Attributes {
		switch attr.Defindex {
		case schema.AttrUnusualEffect:
			if val, ok := attr.Value.(float64); ok {
				effect = int(val)
			}
		case schema.AttrWear:
			if val, ok := attr.Value.(float64); ok {
				wear = schema.WearToTier(float32(val))
			}
		case schema.AttrAustralium:
			isAustralium = true
		case schema.AttrPaintkit:
			if val, ok := attr.Value.(float64); ok {
				paintkit = int(val)
			}
		case schema.AttrKillstreak:
			if val, ok := attr.Value.(float64); ok {
				killstreak = int(val)
			}
		case schema.AttrFestivized:
			isFestivized = true
		case schema.AttrPaintColor, schema.AttrPaintColor2:
			if val, ok := attr.Value.(float64); ok {
				paint = int(val)
			}
		case schema.AttrCrateSeries:
			if val, ok := attr.Value.(float64); ok {
				crateseries = int(val)
			}
		case schema.AttrStrangeScore:
			quality2 = schema.QualityStrange
		}

		if attr.Defindex >= schema.DefSpellProxy && attr.Defindex < schema.DefSpellProxy+100 {
			if spell, ok := attr.Value.(sku.Spell); ok {
				spells = append(spells, spell)
			}
		}

		if attr.Defindex >= schema.DefPartsProxy && attr.Defindex < schema.DefPartsProxy+100 {
			if val, ok := attr.Value.(float64); ok {
				parts = append(parts, int(val))
			}
		}
	}

	return sku.FromObject(&sku.Item{
		Defindex:    it.Defindex,
		Quality:     it.Quality,
		Craftable:   !it.FlagCannotCraft,
		Tradable:    !it.FlagCannotTrade,
		Australium:  isAustralium,
		Effect:      effect,
		Wear:        wear,
		Paintkit:    paintkit,
		Killstreak:  killstreak,
		Festivized:  isFestivized,
		Paint:       paint,
		Quality2:    quality2,
		Crateseries: crateseries,
		Spells:      spells,
		Parts:       parts,
	})
}

func (it *TF2Item) ToEconItem() *trading.Item {
	item := &trading.Item{
		AppID:     440,
		ContextID: 2,
		AssetID:   it.ID,
		ClassID:   uint64(it.Defindex),
		Amount:    int64(it.Quantity),
		Tradable:  !it.FlagCannotTrade,
		Name:      it.CustomName,
		SKU:       it.ToSKU(),
	}

	if len(it.Attributes) > 0 {
		item.Attributes = make([]trading.Attribute, 0, len(it.Attributes))

		for _, attr := range it.Attributes {
			valStr := ""

			var floatVal float64

			switch v := attr.Value.(type) {
			case float64:
				floatVal = v
				valStr = fmt.Sprintf("%g", v)
			case string:
				valStr = v
				floatVal, _ = strconv.ParseFloat(v, 64)
			}

			item.Attributes = append(item.Attributes, trading.Attribute{
				Defindex:   attr.Defindex,
				Value:      valStr,
				FloatValue: floatVal,
			})
		}
	}

	if it.FlagCannotCraft {
		item.Descriptions = append(item.Descriptions, trading.Description{
			Value: "( Not Usable in Crafting )",
			Color: "ff4040",
		})
	}

	if it.CustomDesc != "" {
		item.Descriptions = append(item.Descriptions, trading.Description{Value: it.CustomDesc})
	}

	return item
}

type TF2Attribute struct {
	Defindex   int     `json:"defindex"`
	Value      any     `json:"value"`
	FloatValue float64 `json:"float_value,omitempty"`
}

func mustParseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)

	return v
}
