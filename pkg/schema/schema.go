// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/internal/stringpool"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// ============================================================
// SECTION 0: GLOBAL VARIABLES & DEBUG LOGGING
// ============================================================

var enableDebugSchema = os.Getenv("DEBUG_SCHEMA") == "true"

func debugLog(v ...any) {
	if enableDebugSchema {
		log.Println(v...)
	}
}

var (
	wearNamesStatic = [...]string{"Factory New", "Minimal Wear", "Field-Tested", "Well-Worn", "Battle Scarred"}

	staticWearsTable = [...]struct {
		str string
		val int
	}{
		{"(factory new)", WearFactoryNew},
		{"(minimal wear)", WearMinimalWear},
		{"(field-tested)", WearFieldTested},
		{"(well-worn)", WearWellWorn},
		{"(battle scarred)", WearBattleScarred},
	}

	staticKillstreaksTable = [...]struct {
		phrase string
		value  int
	}{
		{"professional killstreak", 3},
		{"specialized killstreak", 2},
		{"killstreak", 1},
	}
)

var nameBufferPool = sync.Pool{
	New: func() any {
		b := new(bytes.Buffer)
		b.Grow(128)

		return b
	},
}

// ============================================================
// SECTION 1: TYPES & STRUCT DEFINITIONS
// ============================================================

type Raw struct {
	Schema struct {
		Items                                []*Item               `json:"items"`
		Attributes                           []*AttributeSchema    `json:"attributes"`
		Qualities                            map[string]int        `json:"qualities"`
		QualityNames                         map[string]string     `json:"qualityNames"`
		OriginNames                          []*OriginName         `json:"originNames"`
		ItemSets                             []*ItemSet            `json:"item_sets"`
		AttributeControlledAttachedParticles []*ParticleEffect     `json:"attribute_controlled_attached_particles"`
		ItemLevels                           []*ItemLevel          `json:"item_levels"`
		KillEaterScoreTypes                  []*KillEaterScoreType `json:"kill_eater_score_types"`
		StringLookups                        []*StringLookup       `json:"string_lookups"`
		PaintKits                            map[string]string     `json:"paintkits"`
	} `json:"schema"`

	ItemsGame map[string]any `json:"items_game"`
}

type Item struct {
	Capabilities  *Capabilities   `json:"capabilities,omitempty"`
	UsedByClasses []string        `json:"used_by_classes,omitempty"`
	Attributes    []ItemAttribute `json:"attributes,omitempty"`
	Name          string          `json:"name"`
	ItemName      string          `json:"item_name"`
	ItemTypeName  string          `json:"item_type_name,omitempty"`
	ItemClass     string          `json:"item_class"`
	CraftClass    string          `json:"craft_class,omitempty"`
	ImageURL      string          `json:"image_url,omitempty"`
	ImageURLLarge string          `json:"image_url_large,omitempty"`
	ItemSlot      string          `json:"item_slot,omitempty"`
	Defindex      int             `json:"defindex"`
	ItemQuality   int             `json:"item_quality"`
	MinIlevel     int             `json:"min_ilevel,omitempty"`
	MaxIlevel     int             `json:"max_ilevel,omitempty"`
	Flags         int             `json:"flags,omitempty"`
	Origin        int             `json:"origin,omitempty"`
	LoadoutSlot   int             `json:"loadoutslot,omitempty"`
	ProperName    bool            `json:"proper_name"`
}

type Capabilities struct {
	Nameable            bool `json:"nameable"`
	Paintable           bool `json:"paintable"`
	CanCraft            bool `json:"can_craft_if_purchased"`
	Decodable           bool `json:"decodable"`
	CanCustomizeTexture bool `json:"can_customize_texture"`
	Usable              bool `json:"usable"`
	UsableGC            bool `json:"usable_gc"`
	UsableOutOfGame     bool `json:"usable_out_of_game"`
	CanGiftWrap         bool `json:"can_gift_wrap"`
	CanCollect          bool `json:"can_collect"`
	CanCraftCount       bool `json:"can_craft_count"`
	CanCraftMark        bool `json:"can_craft_mark"`
	CanBeRestored       bool `json:"can_be_restored"`
	StrangeParts        bool `json:"strange_parts"`
	CanUseStrangeParts  bool `json:"can_use_strange_parts"`
	CanStrangify        bool `json:"can_strangify"`
	CanKillstreakify    bool `json:"can_killstreakify"`
	CanConsume          bool `json:"can_consume"`
	PaintableTeamColors bool `json:"paintable_team_colors"`
}

type ItemAttribute struct {
	Name        string  `json:"name"`
	Class       string  `json:"class"`
	Value       float64 `json:"value"`
	ValueString string  `json:"value_string,omitempty"`
}

type AttributeSchema struct {
	Defindex        int    `json:"defindex"`
	Name            string `json:"name"`
	AttributeClass  string `json:"attribute_class"`
	Description     string `json:"description_string"`
	DescriptionFmt  string `json:"description_format"`
	EffectType      string `json:"effect_type"`
	Hidden          bool   `json:"hidden"`
	StoredAsInteger bool   `json:"stored_as_integer"`
}

type ParticleEffect struct {
	ID               int    `json:"id"`
	System           string `json:"system"`
	AttachToRootbone bool   `json:"attach_to_rootbone"`
	Name             string `json:"name"`
}

type KillEaterScoreType struct {
	Type      int    `json:"type"`
	TypeName  string `json:"type_name"`
	LevelData string `json:"level_data"`
}

type ItemSet struct {
	ItemSet    string          `json:"item_set"`
	Name       string          `json:"name"`
	Items      []string        `json:"items"`
	Attributes []ItemAttribute `json:"attributes"`
}

type RecipeCategory int

const (
	RecipeCategoryCraftingItems RecipeCategory = 0
	RecipeCategoryCommonItems   RecipeCategory = 1
	RecipeCategoryRareItems     RecipeCategory = 2
	RecipeCategorySpecial       RecipeCategory = 3
)

type RecipeDefinition struct {
	DefIndex             int                `json:"defindex"`
	Name                 string             `json:"name"`
	Disabled             bool               `json:"disabled"`
	RequiresAllSameClass bool               `json:"require_all_same_class"`
	RequiresAllSameSlot  bool               `json:"require_all_same_slot"`
	PremiumAccountOnly   bool               `json:"premium_account_only"`
	Category             RecipeCategory     `json:"category"`
	InputItems           []RecipeInputItem  `json:"input_items"`
	OutputItems          []RecipeOutputItem `json:"output_items"`
}

type RecipeInputItem struct {
	DefIndex     int    `json:"defindex"`
	Name         string `json:"name,omitempty"`
	Count        int    `json:"count"`
	Slot         int    `json:"slot"`
	Class        string `json:"class"`
	LootlistName string `json:"lootlist_name,omitempty"`
	Quality      string `json:"quality,omitempty"`
}

type RecipeOutputItem struct {
	DefIndex     int    `json:"defindex"`
	Name         string `json:"name,omitempty"`
	Count        int    `json:"count"`
	LootlistName string `json:"lootlist_name,omitempty"`
}

type OriginName struct {
	Origin int    `json:"origin"`
	Name   string `json:"name"`
}

type ItemLevel struct {
	Name   string `json:"name"`
	Levels []struct {
		Level         int    `json:"level"`
		RequiredScore int    `json:"required_score"`
		Name          string `json:"name"`
	} `json:"levels"`
}

type StringLookup struct {
	TableName string `json:"table_name"`
	Strings   []struct {
		Index  int    `json:"index"`
		String string `json:"string"`
	} `json:"strings"`
}

type WeaponOption struct {
	Defindex uint32
	Name     string
}

// ============================================================
// SECTION 2: ITEM & CAPABILITIES METHODS
// ============================================================

func (it *Item) InternStrings() {
	if it == nil {
		return
	}

	it.Name = stringpool.Intern(it.Name)
	it.ItemName = stringpool.Intern(it.ItemName)
	it.ItemClass = stringpool.Intern(it.ItemClass)
	it.CraftClass = stringpool.Intern(it.CraftClass)
	it.ImageURL = stringpool.Intern(it.ImageURL)
	it.ImageURLLarge = stringpool.Intern(it.ImageURLLarge)
	it.ItemSlot = stringpool.Intern(it.ItemSlot)

	for i, cls := range it.UsedByClasses {
		it.UsedByClasses[i] = stringpool.Intern(cls)
	}
}

func (it *Item) IsTradableByFlags() bool  { return it.Flags&FlagCannotTrade == 0 }
func (it *Item) IsCraftableByFlags() bool { return it.Flags&FlagCannotBeUsedInCrafting == 0 }
func (it *Item) HasFlag(flag int) bool    { return it.Flags&flag != 0 }
func (it *Item) GetLoadoutSlot() int {
	if it.LoadoutSlot != 0 {
		return it.LoadoutSlot
	}

	return LoadoutInvalid
}
func (it *Item) IsWeapon() bool { return it.CraftClass == "weapon" }
func (it *Item) IsCosmetic() bool {
	return it.LoadoutSlot == LoadoutHead || it.LoadoutSlot == LoadoutMisc || it.LoadoutSlot == LoadoutMisc2
}

func (it *Item) IsTaunt() bool {
	return it.LoadoutSlot >= LoadoutTaunt && it.LoadoutSlot <= LoadoutTaunt8
}
func (it *Item) IsTool() bool { return it.ItemClass == "tool" }
func (it *Item) IsPaintKitWeapon() bool {
	return it.Capabilities != nil && it.Capabilities.CanCustomizeTexture
}
func (it *Item) ValidatePaintKit(id int) bool { return it.IsPaintKitWeapon() && id > 0 }

func (it *Item) UnmarshalJSON(data []byte) error {
	type Alias Item

	var aux struct {
		Alias
		DefIndexAlt int `json:"def_index"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*it = Item(aux.Alias)
	if it.Defindex == 0 && aux.DefIndexAlt != 0 {
		it.Defindex = aux.DefIndexAlt
	}

	return nil
}

func containsFoldASCII(s, substr string) bool {
	nSub := len(substr)

	nS := len(s)
	if nSub == 0 || nSub > nS {
		return false
	}

	limit := nS - nSub
	for i := 0; i <= limit; i++ {
		if bytesconv.EqualFoldASCII(s[i:i+nSub], substr) {
			return true
		}
	}

	return false
}

func isExcludedPattern(name string) bool {
	return (containsFoldASCII(name, "medal") && !containsFoldASCII(name, "Gentle Manne's Service Medal")) ||
		containsFoldASCII(name, "tournament") ||
		containsFoldASCII(name, "etf2l") ||
		containsFoldASCII(name, "ugc ") ||
		containsFoldASCII(name, "rgl.gg") ||
		containsFoldASCII(name, "asiafortress")
}

func (s *Schema) indexItem(item *Item) {
	if item == nil {
		return
	}

	lowName := strings.ToLower(generic.Coalesce(item.ItemName, item.Name))

	s.itemsByDef[item.Defindex] = item

	if lowName == "" || (item.ItemName == "Name Tag" && item.Defindex == 2093) {
		return
	}

	if _, exists := s.itemsByName[lowName]; !exists {
		s.itemsByName[lowName] = item
	}

	stripped := strings.TrimPrefix(lowName, "the ")
	if _, exists := s.itemsByNameStripped[stripped]; !exists {
		s.itemsByNameStripped[stripped] = item
	}
}

func (s *Schema) ItemByNameWithThe(loweredName string) *Item {
	if s == nil {
		return nil
	}

	loweredName = strings.ToLower(loweredName)

	if s.itemsByNameStripped != nil {
		if item, ok := s.itemsByNameStripped[loweredName]; ok {
			return item
		}
	}

	if s.itemsByName != nil {
		if item, ok := s.itemsByName[loweredName]; ok {
			return item
		}
	}

	stripped := strings.TrimPrefix(loweredName, "the ")

	if s.itemsByNameStripped != nil {
		if item, ok := s.itemsByNameStripped[stripped]; ok {
			return item
		}
	}

	if s.itemsByName != nil {
		if item, ok := s.itemsByName[stripped]; ok {
			return item
		}
	}

	withThe := "the " + stripped
	if s.itemsByName != nil {
		if item, ok := s.itemsByName[withThe]; ok {
			return item
		}
	}

	if s.itemsByNameStripped != nil {
		if item, ok := s.itemsByNameStripped[withThe]; ok {
			return item
		}
	}

	return nil
}

func (c *Capabilities) HasCapability(cap string) bool {
	if c == nil {
		return false
	}

	switch cap {
	case "paintable":
		return c.Paintable
	case "nameable":
		return c.Nameable
	case "decodable":
		return c.Decodable
	case "can_customize_texture":
		return c.CanCustomizeTexture
	case "usable":
		return c.Usable
	case "can_gift_wrap":
		return c.CanGiftWrap
	case "can_collect":
		return c.CanCollect
	case "can_use_strange_parts", "strange_parts":
		return c.CanUseStrangeParts || c.StrangeParts
	case "can_strangify":
		return c.CanStrangify
	case "can_killstreakify":
		return c.CanKillstreakify
	case "can_consume":
		return c.CanConsume
	case "paintable_team_colors":
		return c.PaintableTeamColors
	default:
		return false
	}
}

func (c *Capabilities) CanApplyTool(toolType string) bool {
	if c == nil {
		return false
	}

	switch toolType {
	case "paint":
		return c.Paintable || c.PaintableTeamColors
	case "nametag", "desctag":
		return c.Nameable
	case "strangifier":
		return c.CanStrangify
	case "strange-part":
		return c.CanUseStrangeParts || c.StrangeParts
	case "killstreak":
		return c.CanKillstreakify
	case "gift-wrap":
		return c.CanGiftWrap
	default:
		return false
	}
}

func (a *ItemAttribute) UnmarshalJSON(data []byte) error {
	type Alias ItemAttribute

	var aux struct {
		Alias
		DynamicValue any `json:"value"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*a = ItemAttribute(aux.Alias)

	switch v := aux.DynamicValue.(type) {
	case float64:
		a.Value = v
	case int:
		a.Value = float64(v)
	case string:
		a.ValueString = v
	}

	return nil
}

// ============================================================
// SECTION 3: SCHEMA CONSTRUCTOR & INDEXING
// ============================================================

type Schema struct {
	Version string
	Raw     *Raw
	Time    time.Time

	itemsByDef          map[int]*Item
	itemsByName         map[string]*Item
	itemList            []*Item
	killEaterTypesByID  map[int]string
	attrsByDef          map[int]*AttributeSchema
	qualByID            map[int]string
	qualByName          map[string]int
	effByID             map[int]string
	effByName           map[string]int
	paintKitByID        map[int]string
	paintKitByName      map[string]int
	paintByDecimal      map[int]string
	paintByName         map[string]int
	crateSeriesList     map[int]int
	itemsByNameStripped map[string]*Item
	spellsByName        map[string]sku.Spell
	spellsByID          map[string]string
	strangePartsCache   map[string]string

	craftableWeapons             []*Item
	craftableWeaponsForTrading   []string
	uncraftableWeaponsForTrading []string
	weaponsForCraftingByClass    map[string][]string
	unusualEffectsCache          []struct {
		Name string
		ID   int
	}
	paintableItemDefindexesCache []int
	recipes                      map[int]*RecipeDefinition
}

func New(raw *Raw) *Schema {
	s := &Schema{Raw: raw}
	s.buildIndices()

	return s
}

func (s *Schema) buildIndices() {
	numItems := len(s.Raw.Schema.Items)
	numAttrs := len(s.Raw.Schema.Attributes)
	numQual := len(s.Raw.Schema.Qualities)

	s.itemList = s.Raw.Schema.Items
	s.killEaterTypesByID = make(map[int]string, len(s.Raw.Schema.KillEaterScoreTypes))

	for _, p := range s.Raw.Schema.KillEaterScoreTypes {
		s.killEaterTypesByID[p.Type] = stringpool.Intern(p.TypeName)
	}

	s.itemsByDef = make(map[int]*Item, numItems)
	s.itemsByName = make(map[string]*Item, numItems)
	s.itemsByNameStripped = make(map[string]*Item, numItems)
	s.attrsByDef = make(map[int]*AttributeSchema, numAttrs)
	s.qualByID = make(map[int]string, numQual)
	s.qualByName = make(map[string]int, numQual)
	s.effByID = make(map[int]string, len(s.Raw.Schema.AttributeControlledAttachedParticles))
	s.effByName = make(map[string]int, len(s.Raw.Schema.AttributeControlledAttachedParticles))
	s.paintKitByID = make(map[int]string, len(s.Raw.Schema.PaintKits))
	s.paintKitByName = make(map[string]int, len(s.Raw.Schema.PaintKits))
	s.paintByDecimal = make(map[int]string, 32)
	s.paintByName = make(map[string]int, 32)

	for _, item := range s.Raw.Schema.Items {
		item.InternStrings()
		s.indexItem(item)
	}

	for _, attr := range s.Raw.Schema.Attributes {
		attr.Name = stringpool.Intern(attr.Name)
		attr.AttributeClass = stringpool.Intern(attr.AttributeClass)
		s.attrsByDef[attr.Defindex] = attr
	}

	s.indexQualities()
	s.indexEffects()
	s.indexPaints()

	s.crateSeriesList = s.buildCrateSeriesList()
	s.buildSpellIndices()
	s.indexWeapons()
	s.buildRecipes()

	s.strangePartsCache = s.buildStrangePartsCache()
}

func (s *Schema) indexQualities() {
	for qType, id := range s.Raw.Schema.Qualities {
		if name, ok := s.Raw.Schema.QualityNames[qType]; ok {
			internedName := stringpool.Intern(name)
			s.qualByID[id] = internedName
			s.qualByName[strings.ToLower(internedName)] = id
		}
	}

	if len(s.qualByName) == 0 {
		fallbackQualities := map[int]string{
			0: "Normal", 1: "Genuine", 3: "Vintage", 5: "Unusual",
			6: "Unique", 7: "Community", 8: "Valve", 9: "Self-Made",
			10: "Customized", 11: "Strange", 12: "Completed",
			13: "Haunted", 14: "Collector's", 15: "Decorated Weapon",
		}

		for id, name := range fallbackQualities {
			internedName := stringpool.Intern(name)
			s.qualByID[id] = internedName
			s.qualByName[strings.ToLower(internedName)] = id
		}
	}
}

func (s *Schema) indexEffects() {
	seenEffects := make(map[string]bool)

	for _, eff := range s.Raw.Schema.AttributeControlledAttachedParticles {
		if eff.Name == "" || seenEffects[eff.Name] {
			continue
		}

		internedName := stringpool.Intern(eff.Name)
		s.effByID[eff.ID] = internedName
		s.effByName[strings.ToLower(internedName)] = eff.ID
		seenEffects[eff.Name] = true

		switch eff.Name {
		case "Eerie Orbiting Fire":
			s.effByName["orbiting fire"] = 33
			s.effByID[33] = "Orbiting Fire"
		case "Nether Trail":
			s.effByName["ether trail"] = 103
			s.effByID[103] = "Ether Trail"
		case "Refragmenting Reality":
			s.effByName["fragmenting reality"] = 141
			s.effByID[141] = "Fragmenting Reality"
		}
	}
}

func (s *Schema) indexPaints() {
	for idStr, name := range s.Raw.Schema.PaintKits {
		if id, err := strconv.Atoi(idStr); err == nil {
			internedName := stringpool.Intern(name)
			s.paintKitByID[id] = internedName
			s.paintKitByName[strings.ToLower(internedName)] = id
		}
	}

	for _, it := range s.Raw.Schema.Items {
		if strings.Contains(it.Name, "Paint Can") && it.Name != "Paint Can" && len(it.Attributes) > 0 {
			decimal := int(it.Attributes[0].Value)
			internedName := stringpool.Intern(it.ItemName)
			s.paintByDecimal[decimal] = internedName
			s.paintByName[strings.ToLower(internedName)] = decimal
		}
	}

	s.paintByDecimal[5801378] = "Legacy Paint"
	s.paintByName["legacy paint"] = 5801378
}

func (s *Schema) indexWeapons() {
	s.craftableWeapons = make([]*Item, 0)
	for _, it := range s.Raw.Schema.Items {
		if _, ok := weaponsToExclude[it.Defindex]; ok {
			continue
		}

		if it.ItemQuality == QualityUnique && it.CraftClass == "weapon" {
			s.craftableWeapons = append(s.craftableWeapons, it)
		}
	}

	s.craftableWeaponsForTrading = make([]string, 0, len(s.craftableWeapons))
	s.uncraftableWeaponsForTrading = make([]string, 0)

	for _, it := range s.craftableWeapons {
		s.craftableWeaponsForTrading = append(s.craftableWeaponsForTrading, fmt.Sprintf("%d;6", it.Defindex))
		if _, ok := excludeUncraftable[it.Defindex]; !ok {
			s.uncraftableWeaponsForTrading = append(
				s.uncraftableWeaponsForTrading,
				fmt.Sprintf("%d;6;uncraftable", it.Defindex),
			)
		}
	}

	s.weaponsForCraftingByClass = make(map[string][]string)

	for _, class := range Classes {
		var classWeapons []string

		for _, it := range s.craftableWeapons {
			if slices.Contains(it.UsedByClasses, class) {
				classWeapons = append(classWeapons, fmt.Sprintf("%d;6", it.Defindex))
			}
		}

		s.weaponsForCraftingByClass[class] = classWeapons
	}

	s.unusualEffectsCache = make([]struct {
		Name string
		ID   int
	}, 0, len(s.effByID))

	for id, name := range s.effByID {
		s.unusualEffectsCache = append(s.unusualEffectsCache, struct {
			Name string
			ID   int
		}{name, id})
	}

	s.paintableItemDefindexesCache = make([]int, 0)
	for _, it := range s.Raw.Schema.Items {
		if it.Capabilities != nil && it.Capabilities.Paintable {
			s.paintableItemDefindexesCache = append(s.paintableItemDefindexesCache, it.Defindex)
		}
	}
}

func (s *Schema) buildSpellIndices() {
	s.spellsByName = make(map[string]sku.Spell)
	s.spellsByID = make(map[string]string)

	for name, spell := range SpellDefinitions {
		lowerName := strings.ToLower(name)
		s.spellsByName[lowerName] = spell

		idKey := fmt.Sprintf("%d-%d", spell.Attribute, spell.Value)
		s.spellsByID[idKey] = stringpool.Intern(name)

		if spellObj, ok := IdentifySpell(lowerName); ok {
			s.spellsByName[lowerName] = spellObj
		}
	}
}

func (s *Schema) buildStrangePartsCache() map[string]string {
	partsToExclude := map[string]bool{
		"Ubers": true, "Kill Assists": true, "Sentry Kills": true,
		"Sodden Victims": true, "Spies Shocked": true, "Heads Taken": true,
		"Humiliations": true, "Gifts Given": true, "Deaths Feigned": true,
		"Buildings Sapped": true, "Tickle Fights Won": true, "Opponents Flattened": true,
		"Food Items Eaten": true, "Banners Deployed": true, "Seconds Cloaked": true,
		"Health Dispensed to Teammates": true, "Teammates Teleported": true,
		"KillEaterEvent_UniquePlayerKills": true, "Points Scored": true,
		"Double Donks": true, "Teammates Whipped": true, "Wrangled Sentry Kills": true,
		"Carnival Kills": true, "Carnival Underworld Kills": true, "Carnival Games Won": true,
		"Contracts Completed": true, "Contract Points": true, "Contract Bonus Points": true,
		"Times Performed": true, "Kills and Assists during Invasion Event": true,
		"Kills and Assists on 2Fort Invasion": true, "Kills and Assists on Probed": true,
		"Kills and Assists on Byre": true, "Kills and Assists on Watergate": true,
		"Souls Collected": true, "Merasmissions Completed": true,
		"Halloween Transmutes Performed": true, "Power Up Canteens Used": true,
		"Contract Points Earned": true, "Contract Points Contributed To Friends": true,
	}

	m := make(map[string]string)

	if s.Raw != nil {
		for _, p := range s.Raw.Schema.KillEaterScoreTypes {
			if partsToExclude[p.TypeName] || p.Type == 0 || p.Type == 97 {
				continue
			}

			m[p.TypeName] = fmt.Sprintf("sp%d", p.Type)
		}
	}

	for typeID, typeName := range StrangePartsMap {
		if partsToExclude[typeName] || typeID == 0 || typeID == 97 {
			continue
		}

		if _, exists := m[typeName]; !exists {
			m[typeName] = fmt.Sprintf("sp%d", typeID)
		}
	}

	return m
}

func (s *Schema) buildRecipes() {
	if s.Raw == nil || s.Raw.ItemsGame == nil {
		return
	}

	recipesRaw, ok := s.Raw.ItemsGame["recipes"].(map[string]any)
	if !ok {
		return
	}

	s.recipes = make(map[int]*RecipeDefinition)

	for defindexStr, rawBlock := range recipesRaw {
		defindex, err := strconv.Atoi(defindexStr)
		if err != nil {
			continue
		}

		blockStr, ok := rawBlock.(string)
		if !ok {
			continue
		}

		recipe := parseRecipeBlock(defindex, blockStr)
		if recipe != nil {
			s.recipes[defindex] = recipe
		}
	}
}

func (s *Schema) buildCrateSeriesList() map[int]int {
	series := make(map[int]int)

	if s.Raw == nil {
		return series
	}

	for _, it := range s.Raw.Schema.Items {
		if it.Attributes != nil {
			for _, attr := range it.Attributes {
				if attr.Name == "set supply crate series" {
					series[it.Defindex] = int(it.Attributes[0].Value)
					break
				}
			}
		}
	}

	if s.Raw.ItemsGame != nil {
		if items, ok := s.Raw.ItemsGame["items"].(map[string]any); ok {
			for defindexStr, item := range items {
				defindex, err := strconv.Atoi(defindexStr)
				if err != nil || series[defindex] != 0 {
					continue
				}

				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}

				if staticAttrs, ok := itemMap["static_attrs"].(map[string]any); ok {
					if val, ok := staticAttrs["set supply crate series"]; ok {
						switch v := val.(type) {
						case float64:
							series[defindex] = int(v)
						case int:
							series[defindex] = v
						case map[string]any:
							if vv, ok := v["value"]; ok {
								if f, ok := vv.(float64); ok {
									series[defindex] = int(f)
								}
							}
						}
					}
				}
			}
		}
	}

	return series
}

// ============================================================
// SECTION 4: LOOKUP & QUERY API
// ============================================================

func (s *Schema) ItemCount() int {
	if s == nil {
		return 0
	}

	return len(s.itemList)
}

func (s *Schema) ItemByDef(def int) *Item                     { return s.itemsByDef[def] }
func (s *Schema) ItemByName(name string) *Item                { return s.itemsByName[strings.ToLower(name)] }
func (s *Schema) AttributeByDef(def int) *AttributeSchema     { return s.attrsByDef[def] }
func (s *Schema) QualityByID(id int) string                   { return s.qualByID[id] }
func (s *Schema) QualityIDByName(name string) int             { return s.qualByName[strings.ToLower(name)] }
func (s *Schema) EffectByID(id int) string                    { return s.effByID[id] }
func (s *Schema) EffectIDByName(name string) int              { return s.effByName[strings.ToLower(name)] }
func (s *Schema) SkinByID(id int) string                      { return s.paintKitByID[id] }
func (s *Schema) SkinIDByName(name string) int                { return s.paintKitByName[strings.ToLower(name)] }
func (s *Schema) PaintDecimalByName(name string) int          { return s.paintByName[strings.ToLower(name)] }
func (s *Schema) Qualities() map[string]int                   { return s.qualByName }
func (s *Schema) ParticleEffects() map[string]int             { return s.effByName }
func (s *Schema) PaintKitsByName() map[string]int             { return s.paintKitByName }
func (s *Schema) PaintKits() map[string]int                   { return s.paintKitByName }
func (s *Schema) Paints() map[string]int                      { return s.paintByName }
func (s *Schema) PaintableItemDefindexes() []int              { return s.paintableItemDefindexesCache }
func (s *Schema) CraftableWeaponsSchema() []*Item             { return s.craftableWeapons }
func (s *Schema) WeaponsForCraftingByClass(c string) []string { return s.weaponsForCraftingByClass[c] }
func (s *Schema) CraftableWeaponsForTrading() []string        { return s.craftableWeaponsForTrading }
func (s *Schema) UncraftableWeaponsForTrading() []string      { return s.uncraftableWeaponsForTrading }
func (s *Schema) CrateSeriesList() map[int]int                { return s.crateSeriesList }

func (s *Schema) QualityName(qualityID int) string {
	if s == nil {
		return ""
	}

	return s.qualByID[qualityID]
}

func (s *Schema) QualityID(name string) int {
	if s == nil {
		return -1
	}

	if id, ok := s.qualByName[strings.ToLower(name)]; ok {
		return id
	}

	return -1
}

func (s *Schema) PaintNameByDecimal(decimal int) string {
	if name, ok := s.paintByDecimal[decimal]; ok {
		return name
	}

	if name, ok := StandardPaints[uint32(decimal)]; ok {
		return name
	}

	if decimal == 0 {
		return ""
	}

	return fmt.Sprintf("#%06X", decimal)
}

func (s *Schema) ItemBySKU(itemSku string) *Item {
	item, err := sku.FromString(itemSku)
	if err != nil {
		return nil
	}

	return s.ItemByDef(item.Defindex)
}

func (s *Schema) UnusualEffects() []struct {
	Name string
	ID   int
} {
	return s.unusualEffectsCache
}

func (s *Schema) StrangeParts() map[string]string {
	if s.strangePartsCache != nil {
		return s.strangePartsCache
	}

	return s.buildStrangePartsCache()
}

func (s *Schema) SpellNameFromSKU(spell sku.Spell) string {
	idKey := fmt.Sprintf("%d-%d", spell.Attribute, spell.Value)

	name, ok := s.spellsByID[idKey]
	if !ok {
		return fmt.Sprintf("Unknown Spell (%d-%d)", spell.Attribute, spell.Value)
	}

	name = strings.TrimPrefix(name, "Halloween: ")
	if idx := strings.Index(name, " ("); idx != -1 {
		name = name[:idx]
	}

	return name
}

func (s *Schema) SpellIDByName(name string) (sku.Spell, bool) {
	return IdentifySpell(name)
}

func (s *Schema) WearByName(name string) int {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "(") {
		name = "(" + name + ")"
	}

	return wears[name]
}

func (s *Schema) GetRecipe(defindex int) *RecipeDefinition {
	if s == nil || s.recipes == nil {
		return nil
	}

	return s.recipes[defindex]
}

func (s *Schema) GetAllRecipes() []*RecipeDefinition {
	if s == nil {
		return nil
	}

	recipes := make([]*RecipeDefinition, 0, len(s.recipes))
	for _, r := range s.recipes {
		recipes = append(recipes, r)
	}

	return recipes
}

func (s *Schema) GetSupportedWeaponsForPaintkit(paintkitID int) []WeaponOption {
	var options []WeaponOption

	addOption := func(defindex uint32, name string, skinMap map[int]int) {
		if skinMap[paintkitID] != 0 {
			options = append(options, WeaponOption{
				Defindex: defindex,
				Name:     stringpool.Intern(name),
			})
		}
	}

	addOption(22, "Pistol", pistolSkins)
	addOption(18, "Rocket Launcher", rocketLauncherSkins)
	addOption(29, "Medi Gun", medicgunSkins)
	addOption(24, "Revolver", revolverSkins)
	addOption(20, "Stickybomb Launcher", stickybombSkins)
	addOption(14, "Sniper Rifle", sniperRifleSkins)
	addOption(21, "Flame Thrower", flameThrowerSkins)
	addOption(15, "Minigun", minigunSkins)
	addOption(13, "Scattergun", scattergunSkins)
	addOption(12, "Shotgun", shotgunSkins)
	addOption(16, "SMG", smgSkins)
	addOption(7, "Wrench", wrenchSkins)
	addOption(19, "Grenade Launcher", grenadeLauncherSkins)
	addOption(4, "Knife", knifeSkins)

	return options
}

// ============================================================
// SECTION 5: NORMALIZATION & METADATA
// ============================================================

func (s *Schema) NormalizeDefindex(defindex int) int {
	return NormalizeDefindex(defindex)
}

func (s *Schema) IsAustraliumDefindex(defindex int) bool {
	return IsAustraliumDefindex(defindex)
}

func (s *Schema) IsNativeFestive(defindex int) bool {
	return IsNativeFestive(defindex)
}

func (s *Schema) IsPromoItem(it *Item) bool {
	return strings.HasPrefix(it.Name, "Promo ") && it.CraftClass == ""
}

func (s *Schema) NormalizeItem(item *sku.Item) {
	if item == nil {
		return
	}

	item.Defindex = NormalizeDefindex(item.Defindex)

	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return
	}

	if schemaItem.ItemClass != "" && strings.Contains(schemaItem.Name, strings.ToUpper(schemaItem.ItemClass)) {
		for _, it := range s.itemList {
			if it.ItemClass != "" && it.ItemClass == schemaItem.ItemClass &&
				strings.HasPrefix(it.Name, "Upgradeable ") {
				item.Defindex = it.Defindex
				break
			}
		}
	}

	isPromo := s.IsPromoItem(schemaItem)
	if isPromo && item.Quality != QualityGenuine {
		for _, it := range s.itemList {
			if !s.IsPromoItem(it) && it.ItemName == schemaItem.ItemName {
				item.Defindex = it.Defindex
				break
			}
		}
	} else if !isPromo && item.Quality == QualityGenuine {
		for _, it := range s.itemList {
			if s.IsPromoItem(it) && it.ItemName == schemaItem.ItemName {
				item.Defindex = it.Defindex
				break
			}
		}
	}

	if item.Crateseries == 0 && schemaItem.ItemClass == "supply_crate" {
		if series, ok := s.crateSeriesList[item.Defindex]; ok {
			item.Crateseries = series
		}
	}

	if item.Effect != 0 {
		if item.Paintkit != 0 || item.Quality == QualityDecorated {
			if item.Quality == QualityStrange || item.Quality2 == QualityStrange {
				item.Quality2 = QualityStrange
			}

			item.Quality = QualityDecorated
		} else if item.Quality == QualityStrange || item.Quality2 == QualityStrange {
			item.Quality = QualityUnusual
			item.Quality2 = QualityStrange
		}
	}

	if item.Quality == QualityStrange {
		item.Quality2 = 0
	}
}

func (s *Schema) ToJSON() map[string]any {
	if s == nil {
		return nil
	}

	rawSchema := map[string]any{
		"items":        s.itemList,
		"qualities":    s.qualByName,
		"qualityNames": s.qualByID,
		"paintkits":    s.paintKitByID,
		"attribute_controlled_attached_particles": s.unusualEffectsCache,
	}

	if s.Raw != nil && len(s.Raw.Schema.OriginNames) > 0 {
		rawSchema["originNames"] = s.Raw.Schema.OriginNames
	}

	return map[string]any{
		"version": s.Version,
		"time":    s.Time.Unix(),
		"schema":  rawSchema,
	}
}

// ============================================================
// SECTION 6: SKU & ECON ITEM CONVERSION ENGINE
// ============================================================

func (s *Schema) ItemName(item *sku.Item, proper, usePipeForSkin, scmFormat bool) string {
	if item == nil {
		return ""
	}

	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return fmt.Sprintf("Item #%d", item.Defindex)
	}

	buf := nameBufferPool.Get().(*bytes.Buffer)

	buf.Reset()
	defer nameBufferPool.Put(buf)

	hasWritten := false
	appendWord := func(word string) {
		if word == "" {
			return
		}

		if hasWritten {
			buf.WriteByte(' ')
		}

		buf.WriteString(word)

		hasWritten = true
	}

	if !scmFormat && !item.Tradable {
		appendWord("Non-Tradable")
	}

	if !scmFormat && !item.Craftable {
		appendWord("Non-Craftable")
	}

	if item.Quality2 != 0 {
		qName := s.QualityByID(item.Quality2)
		if qName != "" {
			if !scmFormat && (item.Wear != 0 || item.Paintkit != 0) {
				qName += "(e)"
			}

			appendWord(qName)
		}
	}

	addPrimaryQuality := false
	switch {
	case item.Quality == QualityUnique && item.Quality2 != Quality2None,
		item.Quality != QualityUnique && item.Quality != QualityDecorated && item.Quality != QualityUnusual,
		item.Quality == QualityUnusual && item.Effect == 0,
		item.Quality == QualityUnusual && scmFormat,
		scmFormat && item.Effect != 0,
		schemaItem.ItemQuality == QualityUnusual:
		addPrimaryQuality = true
	}

	if addPrimaryQuality {
		qID := item.Quality
		if scmFormat && item.Effect != 0 {
			qID = QualityUnusual
		}

		if qName := s.QualityByID(qID); qName != "" {
			appendWord(qName)
		}
	}

	if !scmFormat && item.Effect != 0 {
		if effName := s.EffectByID(item.Effect); effName != "" {
			appendWord(effName)
		}
	}

	if item.Festivized {
		appendWord("Festivized")
	}

	if item.Killstreak > 0 {
		switch item.Killstreak {
		case 1:
			appendWord("Killstreak")
		case 2:
			appendWord("Specialized Killstreak")
		case 3:
			appendWord("Professional Killstreak")
		}
	}

	if item.Target != 0 {
		if targetItem := s.ItemByDef(item.Target); targetItem != nil {
			appendWord(targetItem.ItemName)
		}
	}

	if item.OutputQuality != 0 && item.OutputQuality != 6 {
		if oqName := s.QualityByID(item.OutputQuality); oqName != "" {
			appendWord(oqName)
		}
	}

	if item.Output != 0 {
		if outItem := s.ItemByDef(item.Output); outItem != nil {
			appendWord(outItem.ItemName)
		}
	}

	if item.Australium {
		appendWord("Australium")
	}

	if item.Paintkit != 0 {
		if skinName := s.SkinByID(item.Paintkit); skinName != "" {
			if usePipeForSkin {
				appendWord(skinName + " |")
			} else {
				appendWord(skinName)
			}
		}
	}

	baseName := ""
	if info, ok := retiredKeys[item.Defindex]; ok {
		baseName = info.Name
	} else if schemaItem.ItemName != "" {
		baseName = schemaItem.ItemName
	} else {
		baseName = schemaItem.Name
	}

	if proper && !hasWritten && schemaItem.ProperName {
		baseName = "The " + baseName
	}

	appendWord(baseName)

	if item.Wear >= 1 && item.Wear <= 5 {
		appendWord("(" + wearNamesStatic[item.Wear-1] + ")")
	}

	for _, spell := range item.Spells {
		appendWord("(Spell: " + s.SpellNameFromSKU(spell) + ")")
	}

	for _, partID := range item.Parts {
		partName := "Unknown Part"
		if name, ok := s.killEaterTypesByID[partID]; ok {
			partName = name
		}

		val := 0
		if item.PartValues != nil {
			val = item.PartValues[partID]
		}

		appendWord(fmt.Sprintf("(%s: %d)", partName, val))
	}

	crateSeries := item.Crateseries
	if crateSeries == 0 && item.Target != 0 {
		if series, ok := strangifierChemistrySetSeries[item.Target]; ok {
			crateSeries = series
		}
	}

	if crateSeries != 0 {
		if scmFormat {
			appendWord(fmt.Sprintf("Series %%23%d", crateSeries))
		} else {
			appendWord(fmt.Sprintf("#%d", crateSeries))
		}
	} else if item.Craftnumber != 0 {
		appendWord(fmt.Sprintf("#%d", item.Craftnumber))
	}

	if !scmFormat && item.Paint != 0 {
		if paintName := s.PaintNameByDecimal(item.Paint); paintName != "" {
			appendWord(fmt.Sprintf("(Paint: %s)", paintName))
		}
	}

	return buf.String()
}

func (s *Schema) SkuFromName(name string) string {
	item := s.ItemFromName(name)

	return sku.FromObject(item)
}

func (s *Schema) SKUFromItem(item *sku.Item) string {
	if item == nil {
		return ""
	}

	s.NormalizeItem(item)

	return sku.FromObject(item)
}

func (s *Schema) ItemFromEconItem(item *trading.Item) *sku.Item {
	if item == nil || s == nil {
		return nil
	}

	defindex := int(item.ClassID)
	nameToParse := item.MarketHashName

	if nameToParse == "" {
		nameToParse = item.MarketName
	}

	var skuItem *sku.Item
	if nameToParse != "" {
		skuItem = s.ItemFromName(nameToParse)
	}

	if skuItem == nil {
		defaultQuality := QualityUnique
		if defindex == 0 {
			defaultQuality = QualityNormal
		}

		skuItem = &sku.Item{
			Defindex:  defindex,
			Quality:   defaultQuality,
			Craftable: true,
			Tradable:  item.Tradable,
		}
	} else if skuItem.Defindex == 0 && defindex > 0 {
		skuItem.Defindex = defindex
	}

	s.applyEconWearAndSkins(skuItem, item)
	s.applyEconDescriptions(skuItem, item.Descriptions)

	if !skuItem.Festivized && (strings.Contains(nameToParse, "Festivized") || s.IsNativeFestive(skuItem.Defindex)) {
		skuItem.Festivized = true
	}

	if !skuItem.Australium && strings.Contains(nameToParse, "Australium") {
		skuItem.Australium = true
	}

	if skuItem.Quality != 11 && strings.HasPrefix(item.MarketHashName, "Strange ") {
		skuItem.Quality2 = 11
	}

	s.NormalizeItem(skuItem)

	return skuItem
}

func (s *Schema) applyEconWearAndSkins(skuItem *sku.Item, item *trading.Item) {
	for _, tag := range item.Tags {
		if tag.Category == "Exterior" {
			if wearID := s.WearByName(tag.LocalizedName); wearID != 0 {
				skuItem.Wear = wearID
			}
		}
	}

	if skuItem.Quality == QualityDecorated || item.ClassID == 205 {
		lowerName := strings.ToLower(item.MarketHashName)

		for pkName, pkID := range s.paintKitByName {
			if strings.Contains(lowerName, pkName) {
				skuItem.Paintkit = pkID
				break
			}
		}
	}

	skuItem.Tradable = item.Tradable
}

func (s *Schema) applyEconDescriptions(skuItem *sku.Item, descriptions []trading.Description) {
	for _, desc := range descriptions {
		val := strings.TrimSpace(desc.Value)
		if val == "" {
			continue
		}

		if wearName, ok := strings.CutPrefix(val, "Exterior: "); ok {
			if wearID := s.WearByName(wearName); wearID != 0 {
				skuItem.Wear = wearID
			}

			continue
		}

		if strings.Contains(val, "( Not Usable in Crafting )") {
			skuItem.Craftable = false
			continue
		}

		isUnusual := skuItem.Quality == QualityUnusual || skuItem.Quality2 == QualityUnusual ||
			skuItem.Quality == QualityDecorated

		if isUnusual && skuItem.Effect == 0 {
			if after, ok := strings.CutPrefix(val, "★ Unusual Effect: "); ok {
				if id := s.EffectIDByName(after); id != 0 {
					skuItem.Effect = id
				}
			}
		}

		if strings.Contains(val, "Killstreak Active") {
			switch {
			case strings.Contains(val, "Professional"):
				skuItem.Killstreak = 3
			case strings.Contains(val, "Specialized"):
				skuItem.Killstreak = 2
			case strings.Contains(val, "Killstreak"):
				skuItem.Killstreak = 1
			}
		}

		if paintName, ok := strings.CutPrefix(val, "Paint Color: "); ok {
			if paintID := s.PaintDecimalByName(paintName); paintID != 0 {
				skuItem.Paint = paintID
			}
		}

		if strings.Contains(val, "Crate Series #") {
			parts := strings.Split(val, "#")
			if len(parts) == 2 {
				if series, err := strconv.Atoi(parts[1]); err == nil {
					skuItem.Crateseries = series
				}
			}
		}

		if strings.Contains(val, "Festivized") {
			skuItem.Festivized = true
		}

		if bytesconv.EqualFoldASCII(desc.Color, "756b5e") {
			s.parseEconStrangePart(skuItem, val)
		}

		if bytesconv.EqualFoldASCII(desc.Color, "7ea9d1") {
			if spell, ok := s.SpellIDByName(strings.TrimSpace(val)); ok {
				skuItem.Spells = append(skuItem.Spells, spell)
			}
		}
	}
}

func (s *Schema) parseEconStrangePart(skuItem *sku.Item, val string) {
	clean := strings.Trim(val, "()")
	before, after, ok := strings.Cut(clean, ":")

	if !ok {
		return
	}

	partName := strings.TrimSpace(before)

	for name, suffix := range s.StrangeParts() {
		if strings.Contains(partName, name) || strings.EqualFold(partName, name) {
			if partID, err := strconv.Atoi(strings.TrimPrefix(suffix, "sp")); err == nil {
				skuItem.Parts = append(skuItem.Parts, partID)

				valStr := strings.TrimSpace(after)
				valStr = strings.ReplaceAll(valStr, ",", "")

				if valInt, err := strconv.Atoi(valStr); err == nil {
					if skuItem.PartValues == nil {
						skuItem.PartValues = make(map[int]int)
					}

					skuItem.PartValues[partID] = valInt
				}
			}

			break
		}
	}
}

func (s *Schema) SKUFromEconItem(item *trading.Item) string {
	skuItem := s.ItemFromEconItem(item)
	if skuItem == nil {
		return "unknown"
	}

	return sku.FromObject(skuItem)
}

func isSpecialName(name string) bool {
	return strings.Contains(name, "chemistry set") ||
		strings.Contains(name, "strangifier") ||
		strings.Contains(name, "unusualifier") ||
		strings.Contains(name, "kit") ||
		strings.Contains(name, "crate") ||
		strings.Contains(name, "war paint") ||
		strings.Contains(name, "strange part")
}

func (s *Schema) ItemFromName(name string) *sku.Item {
	item := &sku.Item{Craftable: true, Tradable: true}

	if isExcludedPattern(name) {
		return item
	}

	originalName := name
	name = strings.ToLower(name)

	if !isSpecialName(name) {
		if schemaItem := s.ItemByNameWithThe(name); schemaItem != nil {
			item.Defindex = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}

			return item
		}
	}

	debugLog("GetItemObjectFromName start:", originalName)

	if isStrangePartPrefix(name) {
		if schemaItem := s.ItemByName(originalName); schemaItem != nil {
			item.Defindex = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}
		}

		return item
	}

	for _, w := range staticWearsTable {
		if idx := strings.Index(name, w.str); idx != -1 {
			name = strings.TrimSpace(name[:idx] + name[idx+len(w.str):])
			item.Wear = w.val

			break
		}
	}

	isExplicitElevatedStrange := false

	if idx := strings.Index(name, "strange(e)"); idx != -1 {
		item.Quality2 = QualityStrange
		isExplicitElevatedStrange = true
		name = strings.TrimSpace(name[:idx] + name[idx+10:])
	}

	hasStrangePrefix := false

	if strings.Contains(name, "strange") && !strings.Contains(name, "strangifier") {
		hasStrangePrefix = true
		name = strings.TrimSpace(strings.ReplaceAll(name, "strange", ""))
	}

	name = s.parseCraftAndTradeRestrictions(name, item)

	if strings.Contains(name, "unusualifier") {
		return s.parseUnusualifier(name, item)
	}

	kitFabricatorDetected := strings.Contains(name, "kit fabricator")

	for _, ks := range staticKillstreaksTable {
		if idx := strings.Index(name, ks.phrase); idx != -1 {
			name = strings.TrimSpace(name[:idx] + name[idx+len(ks.phrase):])
			item.Killstreak = ks.value

			break
		}
	}

	if idx := strings.Index(name, "australium"); idx != -1 && !strings.Contains(name, "australium gold") {
		name = strings.TrimSpace(name[:idx] + name[idx+10:])
		item.Australium = true
	}

	if idx := strings.Index(name, "festivized"); idx != -1 && !strings.Contains(name, "festivized formation") {
		name = strings.TrimSpace(name[:idx] + name[idx+10:])
		item.Festivized = true
	}

	name = s.parseQualityFromName(name, item)
	name = s.parseEffectFromName(name, item)

	if item.Wear != 0 {
		if resItem, done := s.parsePaintkitAndSkins(name, item, isExplicitElevatedStrange); done {
			return resItem
		}
	}

	if strings.Contains(name, "(paint: ") {
		name = s.parsePaintColorInName(name, item)
	}

	if kitFabricatorDetected && item.Killstreak > 1 {
		if resItem, done := s.parseKitFabricator(name, item); done {
			return resItem
		}
	}

	if strings.Contains(name, "chemistry set") &&
		(!strings.Contains(name, "strangifier chemistry set") || strings.Contains(name, "collector's")) {
		return s.parseChemistrySet(name, item)
	}

	if strings.Contains(name, "strangifier chemistry set") {
		return s.parseStrangifierChemistrySet(name, item)
	}

	if strings.Contains(name, "strangifier") && !strings.Contains(name, "strangifier chemistry set") {
		name = strings.TrimSpace(strings.ReplaceAll(name, "strangifier", ""))
		item.Defindex = 6522

		if schemaItem := s.ItemByName(name); schemaItem != nil {
			item.Target = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}
		} else {
			return item
		}
	}

	if !kitFabricatorDetected && strings.Contains(name, "kit") && item.Killstreak > 0 {
		if resItem, done := s.parseKillstreakKit(name, item); done {
			return resItem
		}
	}

	if item.Defindex != 0 {
		return item
	}

	if item.Paintkit != 0 && strings.Contains(name, "war paint") {
		return s.parseWarPaint(item)
	}

	return s.parseCratesAndFinalItem(name, item, hasStrangePrefix)
}

func isStrangePartPrefix(name string) bool {
	return strings.HasPrefix(name, "strange part:") ||
		strings.HasPrefix(name, "strange cosmetic part:") ||
		strings.HasPrefix(name, "strange filter:") ||
		name == "strange count transfer tool" ||
		name == "strange bacon grease"
}

func (s *Schema) parseCraftAndTradeRestrictions(name string, item *sku.Item) string {
	if strings.Contains(name, "craft") {
		if idx := strings.Index(name, "uncraftable"); idx != -1 {
			name = strings.TrimSpace(name[:idx] + name[idx+11:])
			item.Craftable = false
		} else if idx := strings.Index(name, "non-craftable"); idx != -1 {
			name = strings.TrimSpace(name[:idx] + name[idx+13:])
			item.Craftable = false
		}
	}

	if strings.Contains(name, "trad") {
		for _, sub := range []string{"untradeable", "untradable", "non-tradeable", "non-tradable"} {
			if idx := strings.Index(name, sub); idx != -1 {
				name = strings.TrimSpace(name[:idx] + name[idx+len(sub):])
				item.Tradable = false

				break
			}
		}
	}

	return name
}

func (s *Schema) parseUnusualifier(name string, item *sku.Item) *sku.Item {
	name = strings.ReplaceAll(name, "unusual ", "")
	name = strings.ReplaceAll(name, " unusualifier", "")
	name = strings.ReplaceAll(name, "unusualifier", "")
	name = strings.TrimSpace(name)

	item.Defindex = 9258
	item.Quality = QualityUnusual

	if schemaItem := s.ItemByName(name); schemaItem != nil {
		item.Target = schemaItem.Defindex
	}

	return item
}

func (s *Schema) parseQualityFromName(name string, item *sku.Item) string {
	exception := []string{
		"haunted ghosts", "haunted phantasm jr", "haunted phantasm",
		"haunted metal scrap", "haunted hat", "unusual cap",
		"vintage tyrolean", "vintage merryweather", "haunted kraken",
		"haunted forever!", "haunted cremation", "haunted wick",
		"haunted mist",
	}

	qualitySearch := name
	for _, ex := range exception {
		if idx := strings.Index(name, ex); idx != -1 {
			qualitySearch = strings.TrimSpace(name[:idx] + name[idx+len(ex):])

			break
		}
	}

	if slices.Contains(exception, qualitySearch) {
		return name
	}

	for qName, qID := range s.qualByName {
		if qID == QualityDecorated {
			continue
		}

		if qID == QualityCollectors && strings.Contains(qualitySearch, "collector's") &&
			strings.Contains(qualitySearch, "chemistry set") {
			continue
		}

		if qID == QualityCommunity && strings.HasPrefix(qualitySearch, "community sparkle") {
			continue
		}

		if strings.HasPrefix(qualitySearch, qName) {
			if item.Quality != 0 && item.Quality != qID {
				if item.Quality2 == Quality2None {
					item.Quality2 = item.Quality
				}

				item.Quality = qID
			} else {
				item.Quality = qID
			}

			return strings.TrimSpace(strings.Replace(name, qName, "", 1))
		}
	}

	return name
}

func (s *Schema) parseEffectFromName(name string, item *sku.Item) string {
	excludeAtomic := strings.Contains(name, "bonk! atomic punch") || strings.Contains(name, "atomic accolade")

	for effName, effID := range s.effByName {
		if effName == "" || !strings.Contains(name, effName) {
			continue
		}

		if effName == "stardust" && strings.Contains(name, "starduster") &&
			!strings.Contains(strings.ReplaceAll(name, "stardust", ""), "starduster") {
			continue
		}

		if effName == "showstopper" && !strings.Contains(name, "taunt: ") && !strings.Contains(name, "shred alert") {
			continue
		}

		if effName == "smoking" && (name == "smoking jacket" || strings.Contains(name, "smoking skid lid")) &&
			!strings.HasPrefix(name, "smoking smoking") {
			continue
		}

		if (effName == "haunted ghosts" || effName == "pumpkin patch" || effName == "stardust") && item.Wear != 0 {
			continue
		}

		if effName == "atomic" && (strings.Contains(name, "subatomic") || excludeAtomic) {
			continue
		}

		if effName == "spellbound" && (strings.Contains(name, "taunt:") || strings.Contains(name, "shred alert")) {
			continue
		}

		if effName == "accursed" && strings.Contains(name, "accursed apparition") ||
			effName == "haunted" && strings.Contains(name, "haunted kraken") ||
			effName == "frostbite" && strings.Contains(name, "frostbite bonnet") ||
			effName == "sizzling" && strings.HasPrefix(name, "sizzling aroma") {
			continue
		}

		if effName == "hot" {
			if item.Wear == 0 ||
				(!strings.Contains(name, "hot ") && (strings.Contains(name, "shotgun") || strings.Contains(name, "shot ") || strings.Contains(name, "plaid potshotter"))) ||
				!strings.HasPrefix(name, "hot ") {
				continue
			}
		}

		if effName == "cool" && item.Wear == 0 {
			continue
		}

		name = strings.TrimSpace(strings.ReplaceAll(name, effName, ""))
		item.Effect = effID

		if effID == 4 {
			if item.Quality == 0 {
				item.Quality = QualityUnusual
			}
		} else if item.Quality != QualityUnusual {
			if item.Quality2 == Quality2None {
				item.Quality2 = item.Quality
			}

			item.Quality = QualityUnusual
		}

		break
	}

	return name
}

func (s *Schema) parsePaintkitAndSkins(name string, item *sku.Item, isExplicitElevatedStrange bool) (*sku.Item, bool) {
	for pkName, pkID := range s.paintKitByName {
		if strings.Contains(name, pkName) {
			if strings.Contains(name, "mk.ii") && !strings.Contains(pkName, "mk.ii") ||
				strings.Contains(name, "(green)") && !strings.Contains(pkName, "(green)") ||
				strings.Contains(name, "chilly") && !strings.Contains(pkName, "chilly") {
				continue
			}

			name = strings.ReplaceAll(name, pkName, "")
			name = strings.ReplaceAll(name, " | ", "")
			name = strings.TrimSpace(name)
			item.Paintkit = pkID

			if item.Effect != 0 {
				if item.Quality == QualityUnusual && item.Quality2 == QualityStrange {
					if !isExplicitElevatedStrange {
						item.Quality = QualityStrange
						item.Quality2 = Quality2None
					} else {
						item.Quality = QualityDecorated
					}
				} else if item.Quality == QualityUnusual && item.Quality2 == Quality2None {
					item.Quality = QualityDecorated
				}
			}

			if item.Quality == 0 {
				item.Quality = QualityDecorated
			}

			break
		}
	}

	if !strings.Contains(name, "war paint") {
		oldDefindex := item.Defindex

		switch {
		case strings.Contains(name, "pistol"):
			if def, ok := pistolSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15013
			}

		case strings.Contains(name, "rocket launcher"):
			if def, ok := rocketLauncherSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15014
			}

		case strings.Contains(name, "medi gun"):
			if def, ok := medicgunSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15010
			}

		case strings.Contains(name, "revolver"):
			if def, ok := revolverSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15011
			}

		case strings.Contains(name, "stickybomb launcher"):
			if def, ok := stickybombSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15012
			}

		case strings.Contains(name, "sniper rifle"):
			if def, ok := sniperRifleSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15007
			}

		case strings.Contains(name, "flame thrower"):
			if def, ok := flameThrowerSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15005
			}

		case strings.Contains(name, "minigun"):
			if def, ok := minigunSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15004
			}

		case strings.Contains(name, "scattergun"):
			if def, ok := scattergunSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15002
			}

		case strings.Contains(name, "shotgun"):
			if def, ok := shotgunSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15003
			}

		case strings.Contains(name, "smg"):
			if def, ok := smgSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15001
			}

		case strings.Contains(name, "grenade launcher"):
			if def, ok := grenadeLauncherSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15077
			}

		case strings.Contains(name, "wrench"):
			if def, ok := wrenchSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15074
			}

		case strings.Contains(name, "knife"):
			if def, ok := knifeSkins[item.Paintkit]; ok && def != 0 {
				item.Defindex = def
			} else {
				item.Defindex = 15080
			}
		}

		if oldDefindex != item.Defindex {
			return item, true
		}
	}

	return nil, false
}

func (s *Schema) parsePaintColorInName(name string, item *sku.Item) string {
	name = strings.ReplaceAll(name, "(paint: ", "")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.TrimSpace(name)

	for pName, pVal := range s.paintByName {
		if strings.Contains(name, pName) {
			name = strings.TrimSpace(strings.ReplaceAll(name, pName, ""))
			item.Paint = pVal

			break
		}
	}

	return name
}

func (s *Schema) parseKitFabricator(name string, item *sku.Item) (*sku.Item, bool) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "kit fabricator", ""))

	if item.Killstreak > 2 {
		item.Defindex = 20003
	} else {
		item.Defindex = 20002
	}

	if name != "" {
		schemaItem := s.ItemByName(name)
		if schemaItem != nil {
			item.Target = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}
		} else {
			return item, true
		}
	}

	if item.Quality == 0 {
		item.Quality = QualityUnique
	}

	if item.Killstreak > 2 {
		item.Output = 6526
	} else {
		item.Output = 6523
	}

	item.OutputQuality = QualityUnique
	item.Killstreak = 0

	return nil, false
}

func (s *Schema) parseChemistrySet(name string, item *sku.Item) *sku.Item {
	name = strings.ReplaceAll(name, "collector's ", "")
	name = strings.ReplaceAll(name, "chemistry set", "")
	name = strings.TrimSpace(name)

	if strings.Contains(name, "festive") && !strings.Contains(name, "a rather festive tree") {
		item.Defindex = 20007
	} else {
		item.Defindex = 20006
	}

	item.Quality = QualityUnique

	if name != "" {
		if schemaItem := s.ItemByName(name); schemaItem != nil {
			item.Output = schemaItem.Defindex
			item.OutputQuality = QualityCollectors
		}
	}

	return item
}

func (s *Schema) parseStrangifierChemistrySet(name string, item *sku.Item) *sku.Item {
	name = strings.TrimSpace(strings.ReplaceAll(name, "strangifier chemistry set", ""))

	item.Defindex = 20000
	item.Quality = QualityUnique
	item.Output = 6522
	item.OutputQuality = QualityUnique

	if name != "" {
		if schemaItem := s.ItemByName(name); schemaItem != nil {
			item.Target = schemaItem.Defindex
			if series, ok := strangifierChemistrySetSeries[item.Target]; ok {
				item.Crateseries = series
			}
		}
	}

	return item
}

func (s *Schema) parseKillstreakKit(name string, item *sku.Item) (*sku.Item, bool) {
	kitType := item.Killstreak
	item.Killstreak = 0
	name = strings.TrimSpace(strings.ReplaceAll(name, "kit", ""))

	switch kitType {
	case 1:
		item.Defindex = 6527
	case 2:
		item.Defindex = 6523
	case 3:
		item.Defindex = 6526
	}

	if name != "" {
		schemaItem := s.ItemByName(name)
		if schemaItem != nil {
			item.Target = schemaItem.Defindex
		} else {
			return item, true
		}
	}

	if item.Quality == 0 {
		item.Quality = QualityUnique
	}

	return nil, false
}

func (s *Schema) parseWarPaint(item *sku.Item) *sku.Item {
	searchName := fmt.Sprintf("Paintkit %d", item.Paintkit)
	if item.Quality == 0 {
		item.Quality = QualityDecorated
	}

	for _, it := range s.itemList {
		if it.Name == searchName {
			item.Defindex = it.Defindex

			break
		}
	}

	return item
}

func (s *Schema) parseCratesAndFinalItem(name string, item *sku.Item, hasStrangePrefix bool) *sku.Item {
	name = strings.ReplaceAll(name, " series ", " ")
	name = strings.ReplaceAll(name, " series#", " #")

	var number int
	if idx := strings.IndexByte(name, '#'); idx != -1 {
		number, _ = strconv.Atoi(strings.TrimSpace(name[idx+1:]))
		name = strings.TrimSpace(name[:idx])
	}

	if strings.Contains(name, "salvaged mann co. supply crate") && !strings.Contains(name, "key") {
		item.Crateseries = number
		item.Defindex = 5068
		item.Quality = QualityUnique

		return item
	}

	if strings.Contains(name, "select reserve mann co. supply crate") && !strings.Contains(name, "key") {
		item.Defindex = 5660
		item.Crateseries = 60
		item.Quality = QualityUnique

		return item
	}

	if strings.Contains(name, "mann co. supply crate") && !strings.Contains(name, "key") {
		crateseries := number
		switch crateseries {
		case 1, 3, 7, 12, 13, 18, 19, 23, 26, 31, 34, 39, 43, 47, 54, 57, 75:
			item.Defindex = 5022
		case 2, 4, 8, 11, 14, 17, 20, 24, 27, 32, 37, 42, 44, 49, 56, 71, 76:
			item.Defindex = 5041
		case 5, 9, 10, 15, 16, 21, 25, 28, 29, 33, 38, 41, 45, 55, 59, 77:
			item.Defindex = 5045
		}

		item.Crateseries = crateseries
		item.Quality = QualityUnique

		return item
	}

	if strings.Contains(name, "mann co. supply munition") {
		crateseries := number
		if def, ok := munitionCrate[crateseries]; ok {
			item.Defindex = def
		}

		item.Crateseries = crateseries
		item.Quality = QualityUnique

		return item
	}

	for _, keyName := range retiredKeysNames {
		if name == keyName {
			for _, info := range retiredKeys {
				if strings.ToLower(info.Name) == keyName {
					item.Defindex = info.Defindex
					if item.Quality == 0 {
						item.Quality = QualityUnique
					}

					return item
				}
			}
		}
	}

	schemaItem := s.ItemByNameWithThe(name)
	if schemaItem == nil {
		return item
	}

	item.Defindex = schemaItem.Defindex
	if item.Quality == 0 {
		item.Quality = schemaItem.ItemQuality
	}

	if item.Quality == QualityGenuine {
		if newDef, ok := exclusiveGenuine[item.Defindex]; ok {
			item.Defindex = newDef
		}
	}

	if hasStrangePrefix {
		isElevatedCapable := item.Quality == QualityUnusual ||
			item.Quality == QualityVintage ||
			item.Quality == QualityGenuine ||
			item.Quality == QualityHaunted ||
			item.Quality == QualityCollectors ||
			item.Quality == QualityDecorated

		if isElevatedCapable {
			item.Quality2 = QualityStrange
		} else {
			item.Quality = QualityStrange
		}
	}

	if schemaItem.ItemClass == "supply_crate" {
		if series, ok := s.crateSeriesList[item.Defindex]; ok {
			item.Crateseries = series
		} else if number != 0 {
			item.Crateseries = number
		}
	} else if number != 0 {
		item.Craftnumber = number
	}

	return item
}

// ============================================================
// SECTION 7: VALIDATION ENGINE (FLAT GUARD CLAUSES)
// ============================================================

func (s *Schema) CheckExistence(item *sku.Item) bool {
	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return false
	}

	if !s.validateBaseQuality(item, schemaItem) {
		return false
	}

	if !s.validateElevatedQuality(item) {
		return false
	}

	if !s.validateGenuineMapping(item) {
		return false
	}

	if !s.validateRetiredKey(item) {
		return false
	}

	if !s.validateCrateSeries(item, schemaItem) {
		return false
	}

	return true
}

func (s *Schema) validateBaseQuality(item *sku.Item, schemaItem *Item) bool {
	if schemaItem.ItemQuality == 0 || schemaItem.ItemQuality == QualityVintage ||
		schemaItem.ItemQuality == QualityUnusual || schemaItem.ItemQuality == QualityStrange {
		if item.Quality != schemaItem.ItemQuality {
			return false
		}
	}

	if item.Quality == schemaItem.ItemQuality {
		return true
	}

	switch schemaItem.ItemQuality {
	case QualityUnusual:
		return item.Quality == 11
	case QualityUnique:
		return item.Quality == 1 || item.Quality == 3 || item.Quality == 11
	case QualityStrange:
		return item.Quality == 5
	}

	return false
}

func (s *Schema) validateElevatedQuality(item *sku.Item) bool {
	if item.Quality2 == 0 {
		return true
	}

	isElevatedCapable := item.Quality == QualityUnusual ||
		item.Quality == QualityVintage ||
		item.Quality == QualityGenuine ||
		item.Quality == QualityHaunted ||
		item.Quality == QualityCollectors ||
		item.Quality == QualityDecorated

	return !isElevatedCapable
}

func (s *Schema) validateGenuineMapping(item *sku.Item) bool {
	if item.Quality != QualityGenuine {
		_, ok := exclusiveGenuineReversed[item.Defindex]

		return !ok
	}

	_, ok := exclusiveGenuine[item.Defindex]

	return !ok
}

func (s *Schema) validateRetiredKey(item *sku.Item) bool {
	if _, ok := retiredKeys[item.Defindex]; !ok {
		return true
	}

	switch item.Defindex {
	case 5713, 5716, 5717, 5762:
		return !item.Craftable
	default:
		if !item.Craftable && item.Defindex != 5791 && item.Defindex != 5792 {
			return false
		}
	}

	return true
}

func (s *Schema) validateCrateSeries(item *sku.Item, schemaItem *Item) bool {
	hasExtraAttr := item.Quality != QualityUnique ||
		item.Killstreak != 0 || item.Australium || item.Effect != 0 ||
		item.Festivized || item.Paintkit != 0 || item.Wear != 0 ||
		item.Quality2 != 0 || item.Craftnumber != 0 || item.Target != 0 ||
		item.Output != 0 || item.OutputQuality != 0 || item.Paint != 0

	if schemaItem.ItemClass == "supply_crate" && item.Crateseries == 0 {
		if item.Defindex != 5739 && item.Defindex != 5760 &&
			item.Defindex != 5737 && item.Defindex != 5738 {
			return false
		}

		return !hasExtraAttr
	}

	if item.Crateseries != 0 {
		if hasExtraAttr || schemaItem.ItemClass != "supply_crate" {
			return false
		}

		if list, ok := validSingleSeries[item.Defindex]; ok {
			return slices.Contains(list, item.Crateseries)
		}

		if munition, ok := munitionCrate[item.Crateseries]; ok {
			return item.Defindex == munition
		}

		val, ok := s.crateSeriesList[item.Defindex]

		return ok && val == item.Crateseries
	}

	return true
}

// parseRecipeBlock parses a single VDF recipe text block into a RecipeDefinition.
func parseRecipeBlock(defindex int, block string) *RecipeDefinition {
	r := &RecipeDefinition{DefIndex: defindex}
	lines := strings.Split(block, "\n")

	startIdx := 0
	for i, l := range lines {
		if strings.TrimSpace(l) == "{" {
			startIdx = i + 1
			break
		}
	}

	state := &recipeParseState{
		r:            r,
		sectionStack: []string{"root"},
	}

	for _, line := range lines[startIdx:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if len(state.sectionStack) == 0 {
			return nil
		}

		switch trimmed {
		case "{":
			state.handleOpenBrace()
		case "}":
			state.handleCloseBrace()
		default:
			key, value := parseRecipeVDFLine(trimmed)
			if key != "" {
				state.handleKeyValue(key, value)
			}
		}
	}

	return r
}

type recipeParseState struct {
	r             *RecipeDefinition
	sectionStack  []string
	pendingKey    string
	pendingInput  *RecipeInputItem
	pendingOutput *RecipeOutputItem
	condField     string
	condValue     string
}

func (s *recipeParseState) handleOpenBrace() {
	parent := s.sectionStack[len(s.sectionStack)-1]

	switch parent {
	case "root":
		switch s.pendingKey {
		case "input_items":
			s.sectionStack = append(s.sectionStack, "input_items")
		case "output_items":
			s.sectionStack = append(s.sectionStack, "output_items")
		case "tool":
			s.sectionStack = append(s.sectionStack, "tool")
		default:
			s.sectionStack = append(s.sectionStack, "skip")
		}

	case "input_items":
		s.pendingInput = &RecipeInputItem{Count: 1, Slot: -1}
		if c, err := strconv.Atoi(s.pendingKey); err == nil && c > 0 {
			s.pendingInput.Count = c
		}

		s.sectionStack = append(s.sectionStack, "input_item")

	case "output_items":
		s.pendingOutput = &RecipeOutputItem{Count: 1}
		s.sectionStack = append(s.sectionStack, "output_item")

	case "input_item", "output_item":
		if s.pendingKey == "conditions" {
			s.sectionStack = append(s.sectionStack, "conditions")
		} else {
			s.sectionStack = append(s.sectionStack, "skip")
		}

	case "conditions":
		s.condField = ""
		s.condValue = ""
		s.sectionStack = append(s.sectionStack, "condition")

	case "tool", "tool_usage", "tool_components":
		s.sectionStack = append(s.sectionStack, "tool_"+s.pendingKey)

	case "tool_input":
		s.pendingInput = &RecipeInputItem{Count: 1, Slot: -1}
		s.sectionStack = append(s.sectionStack, "dynamic_input")

	case "dynamic_input":
		if s.pendingKey == "counts" {
			s.sectionStack = append(s.sectionStack, "counts")
		} else {
			s.sectionStack = append(s.sectionStack, "skip")
		}

	default:
		s.sectionStack = append(s.sectionStack, "skip")
	}

	s.pendingKey = ""
}

func (s *recipeParseState) handleCloseBrace() {
	top := s.sectionStack[len(s.sectionStack)-1]

	switch top {
	case "input_item":
		if s.pendingInput != nil {
			s.r.InputItems = append(s.r.InputItems, *s.pendingInput)
			s.pendingInput = nil
		}

	case "output_item":
		if s.pendingOutput != nil {
			s.r.OutputItems = append(s.r.OutputItems, *s.pendingOutput)
			s.pendingOutput = nil
		}

	case "condition":
		s.applyCondition()

	case "dynamic_input":
		if s.pendingInput != nil {
			s.r.InputItems = append(s.r.InputItems, *s.pendingInput)
			s.pendingInput = nil
		}
	}

	s.sectionStack = s.sectionStack[:len(s.sectionStack)-1]
	s.pendingKey = ""
}

func (s *recipeParseState) applyCondition() {
	if s.condField == "" {
		return
	}

	if s.pendingInput != nil {
		switch s.condField {
		case "defindex":
			s.pendingInput.DefIndex, _ = strconv.Atoi(s.condValue)
		case "name":
			s.pendingInput.Name = stringpool.Intern(s.condValue)
		}
	}

	if s.pendingOutput != nil {
		switch s.condField {
		case "defindex":
			s.pendingOutput.DefIndex, _ = strconv.Atoi(s.condValue)
		case "name":
			s.pendingOutput.Name = stringpool.Intern(s.condValue)
		}
	}
}

func (s *recipeParseState) handleKeyValue(key, value string) {
	top := s.sectionStack[len(s.sectionStack)-1]

	switch top {
	case "root":
		s.pendingKey = key
		switch key {
		case "name":
			s.r.Name = stringpool.Intern(value)
		case "disabled":
			s.r.Disabled = value == "1"
		case "premium_only":
			s.r.PremiumAccountOnly = value == "1"
		case "all_same_class":
			s.r.RequiresAllSameClass = value == "1"
		case "all_same_slot":
			s.r.RequiresAllSameSlot = value == "1"
		case "category":
			s.r.Category = parseRecipeCategory(value)
		}

	case "input_item", "output_item":
		s.pendingKey = key

	case "condition":
		switch key {
		case "field":
			s.condField = value
		case "value":
			s.condValue = value
		}

	case "dynamic_input":
		s.pendingKey = key
		if s.pendingInput != nil {
			switch key {
			case "lootlist_name":
				s.pendingInput.LootlistName = stringpool.Intern(value)
			case "quality":
				s.pendingInput.Quality = stringpool.Intern(value)
			}
		}

	case "counts":
		if s.pendingInput != nil {
			if c, err := strconv.Atoi(value); err == nil {
				s.pendingInput.Count = c
			}
		}
	}
}

func parseRecipeVDFLine(line string) (string, string) {
	if !strings.HasPrefix(line, "\"") {
		return "", ""
	}

	endQuote := strings.Index(line[1:], "\"")
	if endQuote < 0 {
		return "", ""
	}

	key := line[1 : endQuote+1]

	rest := line[endQuote+2:]
	rest = strings.TrimLeft(rest, " \t")

	if len(rest) == 0 || !strings.HasPrefix(rest, "\"") {
		return key, ""
	}

	rest = rest[1:]

	endQuote2 := strings.Index(rest, "\"")
	if endQuote2 < 0 {
		return key, ""
	}

	return key, rest[:endQuote2]
}

func parseRecipeCategory(s string) RecipeCategory {
	switch s {
	case "crafting":
		return RecipeCategoryCraftingItems
	case "commonitem":
		return RecipeCategoryCommonItems
	case "rareitem":
		return RecipeCategoryRareItems
	case "special":
		return RecipeCategorySpecial
	default:
		return RecipeCategoryCraftingItems
	}
}
