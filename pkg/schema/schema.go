// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/vdf"
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

var debugLog = func(v ...any) {
	if os.Getenv("DEBUG_SCHEMA") == "true" {
		log.Println(v...)
	}
}

// Raw represents the raw schema and VDF configuration payload returned by APIs.
type Raw struct {
	// Schema contains the parsed details from the schema overview.
	Schema struct {
		// Items contains the list of individual item definitions.
		Items []*Item `json:"items"`
		// Attributes contains the list of attribute schemas.
		Attributes []*AttributeSchema `json:"attributes"`
		// Qualities maps quality names to their numeric IDs.
		Qualities map[string]int `json:"qualities"`
		// QualityNames maps internal quality keys to display names.
		QualityNames map[string]string `json:"qualityNames"`
		// OriginNames contains translation strings for item origins.
		OriginNames []*OriginName `json:"originNames"`
		// ItemSets contains the lists of defined item sets.
		ItemSets []*ItemSet `json:"item_sets"`
		// AttributeControlledAttachedParticles contains unusual particle details.
		AttributeControlledAttachedParticles []*ParticleEffect `json:"attribute_controlled_attached_particles"`
		// ItemLevels contains rank thresholds for items.
		ItemLevels []*ItemLevel `json:"item_levels"`
		// KillEaterScoreTypes contains tracked statistic counters.
		KillEaterScoreTypes []*KillEaterScoreType `json:"kill_eater_score_types"`
		// StringLookups contains lookup tables for strings.
		StringLookups []*StringLookup `json:"string_lookups"`
		// PaintKits maps paintkit IDs to their localized names.
		PaintKits map[string]string `json:"paintkits"`
	} `json:"schema"`

	// ItemsGame contains raw parsed fields from the items_game.txt file.
	ItemsGame map[string]any `json:"items_game"`
}

// Item represents a single TF2 item definition in the schema.
type Item struct {
	// Defindex represents the unique item definition index.
	Defindex int `json:"defindex"`
	// Name represents the unique internal string identifier.
	Name string `json:"name"`
	// ItemName represents the localized display name.
	ItemName string `json:"item_name"`
	// ItemClass represents the internal item class name.
	ItemClass string `json:"item_class"`
	// ItemQuality represents the default quality ID of the item.
	ItemQuality int `json:"item_quality"`
	// ProperName indicates whether "The" should prepend the item name.
	ProperName bool `json:"proper_name"`
	// CraftClass represents the craft class name (e.g. "weapon", "hat").
	CraftClass string `json:"craft_class"`
	// Capabilities defines the customization actions permitted on this item.
	Capabilities *Capabilities `json:"capabilities"`
	// UsedByClasses lists character classes that can equip this item.
	UsedByClasses []string `json:"used_by_classes"`
	// Attributes contains static attributes defined on this item.
	Attributes []ItemAttribute `json:"attributes"`
	// ImageURL represents the URL of the small (128x128) backpack icon.
	ImageURL string `json:"image_url"`
	// ImageURLLarge represents the URL of the large (512x512) backpack image.
	ImageURLLarge string `json:"image_image_url_large"`
	// Flags represents item flags bitmask (trade/craft restrictions).
	Flags int `json:"flags,omitempty"`
	// Origin represents the item origin/provenance ID.
	Origin int `json:"origin,omitempty"`
	// LoadoutSlot represents the default loadout slot position.
	LoadoutSlot int `json:"loadoutslot,omitempty"`
	// ItemClass specifies the item class for equipping.
	ItemSlot string `json:"item_slot,omitempty"`
	// StyleCount represents the number of available styles.
	StyleCount int `json:"styles,omitempty"`
	// UsedByClassesRaw is the raw class usability bitmask (if present).
	UsedByClassesRaw int `json:"used_by_classes_mask,omitempty"`
}

// IsTradableByFlags checks if the item is tradable based on its flags bitmask.
// Returns true if the CannotTrade flag is NOT set.
func (it *Item) IsTradableByFlags() bool {
	return it.Flags&FlagCannotTrade == 0
}

// IsCraftableByFlags checks if the item is craftable based on its flags bitmask.
// Returns true if the CannotBeUsedInCrafting flag is NOT set.
func (it *Item) IsCraftableByFlags() bool {
	return it.Flags&FlagCannotBeUsedInCrafting == 0
}

// HasFlag checks if a specific flag bit is set in the item's flags.
func (it *Item) HasFlag(flag int) bool {
	return it.Flags&flag != 0
}

// GetLoadoutSlot returns the loadout slot position for this item.
// Returns LoadoutInvalid if not set.
func (it *Item) GetLoadoutSlot() int {
	if it.LoadoutSlot != 0 {
		return it.LoadoutSlot
	}

	return LoadoutInvalid
}

// IsWeapon checks if the item is a weapon based on its craft class.
func (it *Item) IsWeapon() bool {
	return it.CraftClass == "weapon"
}

// IsCosmetic checks if the item is a cosmetic based on its loadout slot.
func (it *Item) IsCosmetic() bool {
	return it.LoadoutSlot == LoadoutHead || it.LoadoutSlot == LoadoutMisc || it.LoadoutSlot == LoadoutMisc2
}

// IsTaunt checks if the item is a taunt based on its loadout slot.
func (it *Item) IsTaunt() bool {
	return it.LoadoutSlot >= LoadoutTaunt && it.LoadoutSlot <= LoadoutTaunt8
}

// IsTool checks if the item is a tool/consumable.
func (it *Item) IsTool() bool {
	return it.ItemClass == "tool"
}

// Capabilities defines customization options and trade/craft permissions for an item.
type Capabilities struct {
	// Paintable indicates whether paint can be applied to the item.
	Paintable bool `json:"paintable"`
	// Nameable indicates whether a name tag can be applied to the item.
	Nameable bool `json:"nameable"`
	// CanCraft indicates whether purchased copies of the item remain craftable.
	CanCraft bool `json:"can_craft_if_purchased"`
	// Decodable indicates whether the item can be unlocked with a key (crate).
	Decodable bool `json:"decodable"`
	// CanCustomizeTexture indicates whether the item's texture can be customized (War Paints).
	CanCustomizeTexture bool `json:"can_customize_texture"`
	// Usable indicates whether the item can be used/consumed.
	Usable bool `json:"usable"`
	// CanGiftWrap indicates whether the item can be gift wrapped.
	CanGiftWrap bool `json:"can_gift_wrap"`
	// CanCollect indicates whether the item can be collected into a craft set.
	CanCollect bool `json:"can_collect"`
	// CanCraftCount indicates whether the item tracks craft count.
	CanCraftCount bool `json:"can_craft_count"`
	// CanCraftMark indicates whether a crafted-by mark can be applied.
	CanCraftMark bool `json:"can_craft_mark"`
	// CanBeRestored indicates whether the item can be restored from Trade Up.
	CanBeRestored bool `json:"can_be_restored"`
	// CanUseStrangeParts indicates whether strange parts can be applied.
	CanUseStrangeParts bool `json:"can_use_strange_parts"`
	// CanStrangify indicates whether a strangifier can be applied.
	CanStrangify bool `json:"can_strangify"`
	// CanKillstreakify indicates whether a killstreak kit can be applied.
	CanKillstreakify bool `json:"can_killstreakify"`
	// CanConsume indicates whether the item is consumable.
	CanConsume bool `json:"can_consume"`
	// PaintableTeamColors indicates whether team-specific paint can be applied.
	PaintableTeamColors bool `json:"paintable_team_colors"`
}

// HasCapability checks if the item has a specific capability flag.
// Returns false if Capabilities is nil.
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
	case "can_use_strange_parts":
		return c.CanUseStrangeParts
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

// CanApplyTool checks if a specific tool type can be applied to this item.
func (c *Capabilities) CanApplyTool(toolType string) bool {
	if c == nil {
		return false
	}

	switch toolType {
	case "paint":
		return c.Paintable || c.PaintableTeamColors
	case "nametag":
		return c.Nameable
	case "desctag":
		return c.Nameable
	case "strangifier":
		return c.CanStrangify
	case "strange-part":
		return c.CanUseStrangeParts
	case "killstreak":
		return c.CanKillstreakify
	case "gift-wrap":
		return c.CanGiftWrap
	default:
		return false
	}
}

// ItemAttribute represents a static attribute or modifier applied to an item.
type ItemAttribute struct {
	// Name represents the attribute name.
	Name string `json:"name"`
	// Class represents the internal attribute class.
	Class string `json:"class"`
	// Value represents the floating-point interpretation of the value.
	Value float64 `json:"value"`
	// ValueString represents the string value if the attribute value is a string.
	ValueString string `json:"value_string,omitempty"`
}

// UnmarshalJSON custom unmarshaler to handle dynamic "value" types without allocations.
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

// AttributeSchema defines the structure and parsing rules for a specific attribute ID.
type AttributeSchema struct {
	// Defindex represents the attribute definition index.
	Defindex int `json:"defindex"`
	// Name represents the unique internal name of the attribute.
	Name string `json:"name"`
	// AttributeClass represents the internal class of the attribute.
	AttributeClass string `json:"attribute_class"`
	// Description represents the localized description template string.
	Description string `json:"description_string"`
	// DescriptionFmt represents the format type of the value substitution.
	DescriptionFmt string `json:"description_format"`
	// EffectType represents the type of effect (e.g. positive, negative).
	EffectType string `json:"effect_type"`
	// Hidden indicates whether the attribute is visible in the client UI.
	Hidden bool `json:"hidden"`
	// StoredAsInteger indicates whether the float value represents an integer.
	StoredAsInteger bool `json:"stored_as_integer"`
}

// ParticleEffect represents an Unusual or Killstreak particle effect.
type ParticleEffect struct {
	// ID represents the unique effect ID.
	ID int `json:"id"`
	// System represents the particle system name used by the game engine.
	System string `json:"system"`
	// AttachToRootbone indicates whether the effect is attached to the player's root bone.
	AttachToRootbone bool `json:"attach_to_rootbone"`
	// Name represents the localized name of the effect.
	Name string `json:"name"`
}

// KillEaterScoreType defines a tracked statistic category (e.g. Strange Part counters).
type KillEaterScoreType struct {
	// Type represents the numeric tracking event ID.
	Type int `json:"type"`
	// TypeName represents the localized display name of the counter.
	TypeName string `json:"type_name"`
	// LevelData represents the level threshold configuration.
	LevelData string `json:"level_data"`
}

// ItemSet represents a collection of items that grant bonuses when equipped together.
type ItemSet struct {
	// ItemSet represents the internal item set identifier.
	ItemSet string `json:"item_set"`
	// Name represents the localized display name of the set.
	Name string `json:"name"`
	// Items contains the list of internal item names belonging to the set.
	Items []string `json:"items"`
	// Attributes contains the set bonus attributes applied when equipped.
	Attributes []ItemAttribute `json:"attributes"`
}

// RecipeCategory represents the category of a crafting recipe
type RecipeCategory int

// Possible recipe categories
const (
	RecipeCategoryCraftingItems RecipeCategory = 0
	RecipeCategoryCommonItems   RecipeCategory = 1
	RecipeCategoryRareItems     RecipeCategory = 2
	RecipeCategorySpecial       RecipeCategory = 3
)

// RecipeDefinition represents a crafting recipe from the TF2 schema.
type RecipeDefinition struct {
	// DefIndex is the unique recipe definition index.
	DefIndex int `json:"defindex"`
	// Name is the internal recipe name.
	Name string `json:"name"`
	// Disabled indicates if the recipe is currently disabled.
	Disabled bool `json:"disabled"`
	// RequiresAllSameClass requires all input items to be the same class.
	RequiresAllSameClass bool `json:"require_all_same_class"`
	// RequiresAllSameSlot requires all input items to be the same slot.
	RequiresAllSameSlot bool `json:"require_all_same_slot"`
	// PremiumAccountOnly requires a premium account to use.
	PremiumAccountOnly bool `json:"premium_account_only"`
	// Category is the recipe category.
	Category RecipeCategory `json:"category"`
	// InputItems lists the input item criteria (defindexes and counts).
	InputItems []RecipeInputItem `json:"input_items"`
	// OutputItems lists the output item criteria.
	OutputItems []RecipeOutputItem `json:"output_items"`
}

// RecipeInputItem represents an input ingredient for a crafting recipe.
type RecipeInputItem struct {
	// DefIndex is the item definition index required (-1 for name-based lookup).
	DefIndex int `json:"defindex"`
	// Name is the item name for name-based conditions (e.g. "The Sandvich").
	Name string `json:"name,omitempty"`
	// Count is the number of this item required.
	Count int `json:"count"`
	// Slot is the loadout slot filter (-1 for any).
	Slot int `json:"slot"`
	// Class is the class filter (empty for any).
	Class string `json:"class"`
	// LootlistName is the lootlist for dynamic recipes (e.g. "all_particle_hats").
	LootlistName string `json:"lootlist_name,omitempty"`
	// Quality is the quality filter for dynamic recipes.
	Quality string `json:"quality,omitempty"`
}

// RecipeOutputItem represents an output result from a crafting recipe.
type RecipeOutputItem struct {
	// DefIndex is the item definition index of the output (-1 for name-based lookup).
	DefIndex int `json:"defindex"`
	// Name is the item name for name-based conditions.
	Name string `json:"name,omitempty"`
	// Count is the number of items produced.
	Count int `json:"count"`
	// LootlistName is the lootlist for dynamic recipe outputs.
	LootlistName string `json:"lootlist_name,omitempty"`
}

// GetRecipe returns the recipe definition for the given defindex, or nil if not found.
func (s *Schema) GetRecipe(defindex int) *RecipeDefinition {
	if s == nil || s.recipes == nil {
		return nil
	}

	r, ok := s.recipes[defindex]
	if !ok {
		return nil
	}

	return r
}

// GetAllRecipes returns all recipe definitions.
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

// OriginName maps a numeric origin ID to its localized display name.
type OriginName struct {
	// Origin represents the item origin ID.
	Origin int `json:"origin"`
	// Name represents the display name of the origin.
	Name string `json:"name"`
}

// ItemLevel defines name progression and thresholds for ranked items (e.g. Strange weapons).
type ItemLevel struct {
	// Name represents the level progression template name.
	Name string `json:"name"`
	// Levels contains the list of rank thresholds and their display names.
	Levels []struct {
		// Level represents the target level index.
		Level int `json:"level"`
		// RequiredScore represents the count required to reach this rank.
		RequiredScore int `json:"required_score"`
		// Name represents the display name of the rank.
		Name string `json:"name"`
	} `json:"levels"`
}

// StringLookup represents a static lookup table used to map indexes to strings.
type StringLookup struct {
	// TableName represents the lookup table name.
	TableName string `json:"table_name"`
	// Strings contains the mapped index-string pairs.
	Strings []struct {
		// Index represents the key index.
		Index int `json:"index"`
		// String represents the corresponding value string.
		String string `json:"string"`
	} `json:"strings"`
}

// Schema represents the indexed TF2 item schema.
// It provides O(1) lookups for item definitions, qualities, effects, and SKU translation operations.
// Use [New] to build indices from a [Raw] schema payload.
type Schema struct {
	// Version represents the schema version identifier.
	Version string
	// Raw represents the raw unindexed schema payload.
	Raw *Raw
	// Time represents the timestamp when the schema was indexed.
	Time time.Time

	itemsByDef  map[int]*Item
	itemsByName map[string]*Item

	attrsByDef map[int]*AttributeSchema

	qualByID   map[int]string
	qualByName map[string]int

	effByID   map[int]string
	effByName map[string]int

	paintKitByID   map[int]string
	paintKitByName map[string]int

	paintByDecimal map[int]string
	paintByName    map[string]int

	crateSeriesList map[int]int

	itemsByNameStripped map[string]*Item

	spellsByName map[string]sku.Spell
	spellsByID   map[string]string

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

// New constructs a [Schema] instance and indexes the [Raw] payload for O(1) lookups.
func New(raw *Raw) *Schema {
	if raw != nil && len(raw.Schema.KillEaterScoreTypes) == 0 {
		raw.Schema.KillEaterScoreTypes = make([]*KillEaterScoreType, 0, len(StrangePartsMap))
		for typeID, typeName := range StrangePartsMap {
			raw.Schema.KillEaterScoreTypes = append(raw.Schema.KillEaterScoreTypes, &KillEaterScoreType{
				Type:     typeID,
				TypeName: typeName,
			})
		}
	}

	s := &Schema{
		Raw:            raw,
		itemsByDef:     make(map[int]*Item),
		itemsByName:    make(map[string]*Item),
		attrsByDef:     make(map[int]*AttributeSchema),
		qualByID:       make(map[int]string),
		qualByName:     make(map[string]int),
		effByID:        make(map[int]string),
		effByName:      make(map[string]int),
		paintKitByID:   make(map[int]string),
		paintKitByName: make(map[string]int),
		paintByDecimal: make(map[int]string),
		paintByName:    make(map[string]int),
		spellsByName:   make(map[string]sku.Spell),
		spellsByID:     make(map[string]string),
	}
	s.buildIndices()
	s.buildSpellIndices()

	return s
}

func (s *Schema) buildIndices() {
	s.itemsByNameStripped = make(map[string]*Item)

	for _, item := range s.Raw.Schema.Items {
		lowName := strings.ToLower(item.ItemName)
		s.itemsByDef[item.Defindex] = item

		if item.ItemQuality == 0 || (item.ItemName == "Name Tag" && item.Defindex == 2093) {
			continue
		}

		if _, exists := s.itemsByName[lowName]; !exists {
			s.itemsByName[lowName] = item
		}

		stripped := strings.TrimPrefix(lowName, "the ")
		if _, exists := s.itemsByNameStripped[stripped]; !exists {
			s.itemsByNameStripped[stripped] = item
		}
	}

	for _, attr := range s.Raw.Schema.Attributes {
		s.attrsByDef[attr.Defindex] = attr
	}

	for qType, id := range s.Raw.Schema.Qualities {
		if name, ok := s.Raw.Schema.QualityNames[qType]; ok {
			s.qualByID[id] = name
			s.qualByName[strings.ToLower(name)] = id
		}
	}

	if len(s.qualByName) == 0 {
		fallbackQualities := map[int]string{
			0:  "Normal",
			1:  "Genuine",
			3:  "Vintage",
			5:  "Unusual",
			6:  "Unique",
			7:  "Community",
			8:  "Valve",
			9:  "Self-Made",
			10: "Customized",
			11: "Strange",
			12: "Completed",
			13: "Haunted",
			14: "Collector's",
			15: "Decorated Weapon",
		}
		for id, name := range fallbackQualities {
			s.qualByID[id] = name
			s.qualByName[strings.ToLower(name)] = id
		}
	}

	seenEffects := make(map[string]bool)

	for _, eff := range s.Raw.Schema.AttributeControlledAttachedParticles {
		if eff.Name == "" {
			continue
		}

		if !seenEffects[eff.Name] {
			s.effByID[eff.ID] = eff.Name
			s.effByName[strings.ToLower(eff.Name)] = eff.ID
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

	for idStr, name := range s.Raw.Schema.PaintKits {
		if id, err := strconv.Atoi(idStr); err == nil {
			s.paintKitByID[id] = name
			s.paintKitByName[strings.ToLower(name)] = id
		}
	}

	for _, it := range s.Raw.Schema.Items {
		if strings.Contains(it.Name, "Paint Can") && it.Name != "Paint Can" && it.Attributes != nil {
			if len(it.Attributes) > 0 {
				decimal := int(it.Attributes[0].Value)

				s.paintByDecimal[decimal] = it.ItemName
				s.paintByName[strings.ToLower(it.ItemName)] = decimal
			}
		}
	}

	s.paintByDecimal[5801378] = "Legacy Paint"
	s.paintByName["legacy paint"] = 5801378

	s.crateSeriesList = s.buildCrateSeriesList()
	s.buildSpellIndices()

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

	s.buildRecipes()
	s.Raw.ItemsGame = nil
}

func (s *Schema) buildSpellIndices() {
	s.spellsByName = make(map[string]sku.Spell)
	s.spellsByID = make(map[string]string)

	for name, spell := range SpellDefinitions {
		lowerName := strings.ToLower(name)
		s.spellsByName[lowerName] = spell

		idKey := fmt.Sprintf("%d-%d", spell.Attribute, spell.Value)
		s.spellsByID[idKey] = name

		if spellObj, ok := IdentifySpell(lowerName); ok {
			s.spellsByName[lowerName] = spellObj
		}
	}
}

func (s *Schema) buildRecipes() {
	if s.Raw.ItemsGame == nil {
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

		parser := vdf.NewParser(strings.NewReader(blockStr))

		parsed, err := parser.Parse()
		if err != nil {
			debugLog("failed to parse recipe", defindex, ":", err)
			continue
		}

		recipeData := findFirstMap(parsed)
		if recipeData == nil {
			continue
		}

		recipe := &RecipeDefinition{
			DefIndex: defindex,
		}

		if v, ok := recipeData["name"].(string); ok {
			recipe.Name = v
		}

		if v, ok := recipeData["disabled"].(string); ok {
			recipe.Disabled = v == "1"
		}

		if v, ok := recipeData["premium_only"].(string); ok {
			recipe.PremiumAccountOnly = v == "1"
		}

		if v, ok := recipeData["all_same_class"].(string); ok {
			recipe.RequiresAllSameClass = v == "1"
		}

		if v, ok := recipeData["all_same_slot"].(string); ok {
			recipe.RequiresAllSameSlot = v == "1"
		}

		if v, ok := recipeData["category"].(string); ok {
			recipe.Category = parseRecipeCategory(v)
		}

		// Parse standard input_items
		if inputItems, ok := recipeData["input_items"].(map[string]any); ok {
			recipe.InputItems = parseRecipeInputItems(inputItems)
		}

		// Parse standard output_items
		if outputItems, ok := recipeData["output_items"].(map[string]any); ok {
			recipe.OutputItems = parseRecipeOutputItems(outputItems)
		}

		// Parse dynamic recipe (tool/usage/components)
		if tool, ok := recipeData["tool"].(map[string]any); ok {
			recipe.InputItems = parseDynamicRecipeInputs(tool)
		}

		s.recipes[defindex] = recipe
	}
}

func findFirstMap(m map[string]any) map[string]any {
	for _, v := range m {
		if sub, ok := v.(map[string]any); ok {
			return sub
		}
	}

	return nil
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

func parseRecipeInputItems(inputItems map[string]any) []RecipeInputItem {
	var result []RecipeInputItem

	for countStr, itemData := range inputItems {
		itemMap, ok := itemData.(map[string]any)
		if !ok {
			continue
		}

		count, _ := strconv.Atoi(countStr)
		if count <= 0 {
			count = 1
		}

		input := RecipeInputItem{
			Count: count,
			Slot:  -1,
		}

		if conditions, ok := itemMap["conditions"].(map[string]any); ok {
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]any)
				if !ok {
					continue
				}

				field, _ := condMap["field"].(string)
				value, _ := condMap["value"].(string)

				switch field {
				case "defindex":
					if di, err := strconv.Atoi(value); err == nil {
						input.DefIndex = di
					}
				case "name":
					input.Name = value
				}
			}
		}

		result = append(result, input)
	}

	return result
}

func parseRecipeOutputItems(outputItems map[string]any) []RecipeOutputItem {
	var result []RecipeOutputItem

	for _, itemData := range outputItems {
		itemMap, ok := itemData.(map[string]any)
		if !ok {
			continue
		}

		output := RecipeOutputItem{Count: 1}

		if conditions, ok := itemMap["conditions"].(map[string]any); ok {
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]any)
				if !ok {
					continue
				}

				field, _ := condMap["field"].(string)
				value, _ := condMap["value"].(string)

				switch field {
				case "defindex":
					if di, err := strconv.Atoi(value); err == nil {
						output.DefIndex = di
					}
				case "name":
					output.Name = value
				}
			}
		}

		result = append(result, output)
	}

	return result
}

func parseDynamicRecipeInputs(tool map[string]any) []RecipeInputItem {
	usage, ok := tool["usage"].(map[string]any)
	if !ok {
		return nil
	}

	components, ok := usage["components"].(map[string]any)
	if !ok {
		return nil
	}

	input, ok := components["input"].(map[string]any)
	if !ok {
		return nil
	}

	var result []RecipeInputItem

	for _, inputData := range input {
		inputMap, ok := inputData.(map[string]any)
		if !ok {
			continue
		}

		ri := RecipeInputItem{Count: 1}

		if v, ok := inputMap["lootlist_name"].(string); ok {
			ri.LootlistName = v
		}

		if v, ok := inputMap["quality"].(string); ok {
			ri.Quality = v
		}

		if counts, ok := inputMap["counts"].(map[string]any); ok {
			for _, countVal := range counts {
				if c, ok := countVal.(string); ok {
					if n, err := strconv.Atoi(c); err == nil {
						ri.Count = n
					}
				}

				break
			}
		}

		result = append(result, ri)
	}

	return result
}

func (s *Schema) buildCrateSeriesList() map[int]int {
	series := make(map[int]int)

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
				if err != nil {
					continue
				}

				if _, ok := series[defindex]; ok {
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

// ItemByDef returns the [Item] matching the specified defindex.
func (s *Schema) ItemByDef(def int) *Item {
	return s.itemsByDef[def]
}

// ItemByName returns the [Item] matching the specified internal name.
func (s *Schema) ItemByName(name string) *Item {
	return s.itemsByName[strings.ToLower(name)]
}

// AttributeByDef returns the [AttributeSchema] matching the specified defindex.
func (s *Schema) AttributeByDef(def int) *AttributeSchema {
	return s.attrsByDef[def]
}

// QualityByID returns the quality name matching the specified ID.
func (s *Schema) QualityByID(id int) string {
	return s.qualByID[id]
}

// QualityIDByName returns the quality ID matching the specified name.
func (s *Schema) QualityIDByName(name string) int {
	return s.qualByName[strings.ToLower(name)]
}

// EffectByID returns the particle effect name matching the specified ID.
func (s *Schema) EffectByID(id int) string {
	return s.effByID[id]
}

// EffectIDByName returns the particle effect ID matching the specified name.
func (s *Schema) EffectIDByName(name string) int {
	return s.effByName[strings.ToLower(name)]
}

// SkinByID returns the localized paint kit (skin) name matching the specified ID.
func (s *Schema) SkinByID(id int) string {
	return s.paintKitByID[id]
}

// SkinIDByName returns the paint kit ID matching the specified localized name.
func (s *Schema) SkinIDByName(name string) int {
	return s.paintKitByName[strings.ToLower(name)]
}

// PaintNameByDecimal returns the paint color name matching the decimal value.
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

// PaintDecimalByName returns the paint color decimal value matching the name.
func (s *Schema) PaintDecimalByName(name string) int {
	return s.paintByName[strings.ToLower(name)]
}

// ItemByNameWithThe searches for an item, ignoring the "The " prefix in the name.
func (s *Schema) ItemByNameWithThe(name string) *Item {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "the ")
	name = strings.TrimSpace(name)

	return s.itemsByNameStripped[name]
}

// ItemBySKU returns the [Item] definition matching the provided SKU string.
func (s *Schema) ItemBySKU(itemSku string) *Item {
	item, err := sku.FromString(itemSku)
	if err != nil {
		return nil
	}

	return s.ItemByDef(item.Defindex)
}

// UnusualEffects returns a list of all indexed unusual particle effects.
func (s *Schema) UnusualEffects() []struct {
	Name string
	ID   int
} {
	return s.unusualEffectsCache
}

// Paints returns a map of all paint color names to their decimal values.
func (s *Schema) Paints() map[string]int {
	return s.paintByName
}

// PaintableItemDefindexes returns a list of defindexes for all paintable items.
func (s *Schema) PaintableItemDefindexes() []int {
	return s.paintableItemDefindexesCache
}

// StrangeParts returns a map of strange part names to their SKU suffixes.
func (s *Schema) StrangeParts() map[string]string {
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

	for _, p := range s.Raw.Schema.KillEaterScoreTypes {
		if partsToExclude[p.TypeName] || p.Type == 0 || p.Type == 97 {
			continue
		}

		m[p.TypeName] = fmt.Sprintf("sp%d", p.Type)
	}

	return m
}

// SpellNameFromSKU returns the display name of the specified [sku.Spell].
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

// SpellIDByName returns the [sku.Spell] attributes matching the specified spell name.
func (s *Schema) SpellIDByName(name string) (sku.Spell, bool) {
	return IdentifySpell(name)
}

// CraftableWeaponsSchema returns all craftable weapon definitions in the schema.
func (s *Schema) CraftableWeaponsSchema() []*Item {
	return s.craftableWeapons
}

// WeaponsForCraftingByClass returns weapon SKUs usable by the specified character class.
func (s *Schema) WeaponsForCraftingByClass(class string) []string {
	return s.weaponsForCraftingByClass[class]
}

// CraftableWeaponsForTrading returns SKUs of all craftable unique weapons.
func (s *Schema) CraftableWeaponsForTrading() []string {
	return s.craftableWeaponsForTrading
}

// UncraftableWeaponsForTrading returns SKUs of all uncraftable unique weapons.
func (s *Schema) UncraftableWeaponsForTrading() []string {
	return s.uncraftableWeaponsForTrading
}

// CrateSeriesList returns a map of crate defindexes to their default series numbers.
func (s *Schema) CrateSeriesList() map[int]int {
	return s.crateSeriesList
}

// NormalizeDefindex converts retired or legacy defindexes to their canonical IDs.
func (s *Schema) NormalizeDefindex(defindex int) int {
	return NormalizeDefindex(defindex)
}

// IsAustraliumDefindex returns true if the defindex is eligible for an Australium variant.
func (s *Schema) IsAustraliumDefindex(defindex int) bool {
	return IsAustraliumDefindex(defindex)
}

// IsNativeFestive returns true if the defindex belongs to an older native Festive item.
func (s *Schema) IsNativeFestive(defindex int) bool {
	return IsNativeFestive(defindex)
}

// Qualities returns a map of all quality names to their numeric IDs.
func (s *Schema) Qualities() map[string]int {
	return s.qualByName
}

// WearByName returns the wear level ID matching the specified string (e.g., "Factory New").
func (s *Schema) WearByName(name string) int {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "(") {
		name = "(" + name + ")"
	}

	return wears[name]
}

// ParticleEffects returns a map of particle effect names to their numeric IDs.
func (s *Schema) ParticleEffects() map[string]int {
	return s.effByName
}

// PaintKitsByName returns a map of paint kit (skin) names to their numeric IDs.
func (s *Schema) PaintKitsByName() map[string]int {
	return s.paintKitByName
}

// PaintKits returns a map of paint kit names to their numeric IDs.
func (s *Schema) PaintKits() map[string]int {
	return s.paintKitByName
}

// QualityName returns the quality name for the given quality ID.
// Returns an empty string if the quality ID is not found.
func (s *Schema) QualityName(qualityID int) string {
	if s == nil {
		return ""
	}

	return s.qualByID[qualityID]
}

// QualityID returns the numeric quality ID for the given quality name (case-insensitive).
// Returns -1 if the quality name is not found.
func (s *Schema) QualityID(name string) int {
	if s == nil {
		return -1
	}

	if id, ok := s.qualByName[strings.ToLower(name)]; ok {
		return id
	}

	return -1
}

// IsPaintKitWeapon checks if the item is eligible for War Paint / PaintKit application.
// Returns true if the item has the can_customize_texture capability.
func (it *Item) IsPaintKitWeapon() bool {
	return it.Capabilities != nil && it.Capabilities.CanCustomizeTexture
}

// ValidatePaintKit checks if a specific paintkit can be applied to this weapon item.
// Returns true if the weapon supports the given paintkit ID.
// This is a basic validation — full validation requires the paintkit's supported weapon list.
func (it *Item) ValidatePaintKit(paintkitID int) bool {
	if !it.IsPaintKitWeapon() {
		return false
	}

	if paintkitID <= 0 {
		return false
	}

	return true
}

var validSingleSeries = map[int][]int{
	5022: {1, 3, 7, 12, 13, 18, 19, 23, 26, 31, 34, 39, 43, 47, 54, 57, 75},
	5041: {2, 4, 8, 11, 14, 17, 20, 24, 27, 32, 37, 42, 44, 49, 56, 71, 76},
	5045: {5, 9, 10, 15, 16, 21, 25, 28, 29, 33, 38, 41, 45, 55, 59, 77},
	5068: {30, 40, 50},
}

// CheckExistence verifies whether the specified [sku.Item] possesses valid quality and attribute configurations.
func (s *Schema) CheckExistence(item *sku.Item) bool {
	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return false
	}

	if schemaItem.ItemQuality == 0 || schemaItem.ItemQuality == QualityVintage ||
		schemaItem.ItemQuality == QualityUnusual || schemaItem.ItemQuality == QualityStrange {
		if item.Quality != schemaItem.ItemQuality {
			return false
		}
	}

	qualityValid := item.Quality == schemaItem.ItemQuality
	if !qualityValid {
		switch schemaItem.ItemQuality {
		case QualityUnusual:
			qualityValid = item.Quality == 11
		case QualityUnique:
			qualityValid = item.Quality == 1 || item.Quality == 3 || item.Quality == 11
		case QualityStrange:
			qualityValid = item.Quality == 5
		}
	}

	if !qualityValid {
		return false
	}

	if item.Quality2 != 0 {
		isElevatedCapable := item.Quality == QualityUnusual ||
			item.Quality == QualityVintage ||
			item.Quality == QualityGenuine ||
			item.Quality == QualityHaunted ||
			item.Quality == QualityCollectors ||
			item.Quality == QualityDecorated

		if isElevatedCapable {
			return false
		}
	}

	if item.Quality != QualityGenuine {
		if _, ok := exclusiveGenuineReversed[item.Defindex]; ok {
			return false
		}
	} else {
		if _, ok := exclusiveGenuine[item.Defindex]; ok {
			return false
		}
	}

	if _, ok := retiredKeys[item.Defindex]; ok {
		switch item.Defindex {
		case 5713, 5716, 5717, 5762:
			if item.Craftable {
				return false
			}
		default:
			if !item.Craftable && item.Defindex != 5791 && item.Defindex != 5792 {
				return false
			}
		}
	}

	hasExtraAttr := item.Quality != QualityUnique ||
		item.Killstreak != 0 ||
		item.Australium ||
		item.Effect != 0 ||
		item.Festivized ||
		item.Paintkit != 0 ||
		item.Wear != 0 ||
		item.Quality2 != 0 ||
		item.Craftnumber != 0 ||
		item.Target != 0 ||
		item.Output != 0 ||
		item.OutputQuality != 0 ||
		item.Paint != 0

	if schemaItem.ItemClass == "supply_crate" && item.Crateseries == 0 {
		if item.Defindex != 5739 && item.Defindex != 5760 &&
			item.Defindex != 5737 && item.Defindex != 5738 {
			return false
		}

		if hasExtraAttr {
			return false
		}
	}

	if item.Crateseries != 0 {
		if hasExtraAttr {
			return false
		}

		if schemaItem.ItemClass != "supply_crate" {
			return false
		}

		if list, ok := validSingleSeries[item.Defindex]; ok {
			if !slices.Contains(list, item.Crateseries) {
				return false
			}
		} else if munition, ok := munitionCrate[item.Crateseries]; ok {
			if item.Defindex != munition {
				return false
			}
		} else {
			if val, ok := s.crateSeriesList[item.Defindex]; !ok || val != item.Crateseries {
				return false
			}
		}
	}

	return true
}

// ItemName constructs the localized display name for the specified [sku.Item].
func (s *Schema) ItemName(item *sku.Item, proper, usePipeForSkin, scmFormat bool) string {
	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return ""
	}

	var parts []string

	if !scmFormat && !item.Tradable {
		parts = append(parts, "Non-Tradable")
	}

	if !scmFormat && !item.Craftable {
		parts = append(parts, "Non-Craftable")
	}

	if item.Quality2 != 0 {
		qName := s.QualityByID(item.Quality2)
		if qName != "" {
			if !scmFormat && (item.Wear != 0 || item.Paintkit != 0) {
				qName += "(e)"
			}

			parts = append(parts, qName)
		}
	}

	addPrimaryQuality := false
	switch {
	case item.Quality == QualityUnique && item.Quality2 != Quality2None,
		item.Quality != QualityUnique && item.Quality != QualityDecorated && item.Quality != QualityUnusual,
		item.Quality == QualityUnusual && item.Effect == 0,
		item.Quality == QualityUnusual && scmFormat,
		schemaItem.ItemQuality == QualityUnusual:
		addPrimaryQuality = true
	}

	if addPrimaryQuality {
		qName := s.QualityByID(item.Quality)
		if qName != "" {
			parts = append(parts, qName)
		}
	}

	if !scmFormat && item.Effect != 0 {
		effName := s.EffectByID(item.Effect)
		if effName != "" {
			parts = append(parts, effName)
		}
	}

	if item.Festivized {
		parts = append(parts, "Festivized")
	}

	if item.Killstreak > 0 {
		switch item.Killstreak {
		case 1:
			parts = append(parts, "Killstreak")
		case 2:
			parts = append(parts, "Specialized Killstreak")
		case 3:
			parts = append(parts, "Professional Killstreak")
		}
	}

	if item.Target != 0 {
		targetItem := s.ItemByDef(item.Target)
		if targetItem != nil {
			parts = append(parts, targetItem.ItemName)
		}
	}

	if item.OutputQuality != 0 && item.OutputQuality != 6 {
		oqName := s.QualityByID(item.OutputQuality)
		if oqName != "" {
			parts = append([]string{oqName}, parts...)
		}
	}

	if item.Output != 0 {
		outItem := s.ItemByDef(item.Output)
		if outItem != nil {
			parts = append(parts, outItem.ItemName)
		}
	}

	if item.Australium {
		parts = append(parts, "Australium")
	}

	if item.Paintkit != 0 {
		skinName := s.SkinByID(item.Paintkit)
		if skinName != "" {
			if usePipeForSkin {
				parts = append(parts, skinName+" |")
			} else {
				parts = append(parts, skinName)
			}
		}
	}

	baseName := ""
	if info, ok := retiredKeys[item.Defindex]; ok {
		baseName = info.Name
	} else {
		baseName = schemaItem.ItemName
	}

	if proper && len(parts) == 0 && schemaItem.ProperName {
		baseName = "The " + baseName
	}

	parts = append(parts, baseName)

	if item.Wear != 0 {
		wears := []string{"Factory New", "Minimal Wear", "Field-Tested", "Well-Worn", "Battle Scarred"}
		if item.Wear >= 1 && item.Wear <= 5 {
			parts = append(parts, "("+wears[item.Wear-1]+")")
		}
	}

	for _, spell := range item.Spells {
		parts = append(parts, "(Spell: "+s.SpellNameFromSKU(spell)+")")
	}

	for _, partID := range item.Parts {
		partName := "Unknown Part"
		for _, p := range s.Raw.Schema.KillEaterScoreTypes {
			if p.Type == partID {
				partName = p.TypeName
				break
			}
		}

		val := 0
		if item.PartValues != nil {
			val = item.PartValues[partID]
		}

		parts = append(parts, "("+partName+": "+strconv.Itoa(val)+")")
	}

	if item.Crateseries != 0 {
		if scmFormat {
			hasSeriesAttr := false

			if schemaItem.Attributes != nil {
				for _, attr := range schemaItem.Attributes {
					if attr.Class == "supply_crate_series" {
						hasSeriesAttr = true
						break
					}
				}
			}

			if hasSeriesAttr {
				parts = append(parts, fmt.Sprintf("Series %%23%d", item.Crateseries))
			}
		} else {
			parts = append(parts, fmt.Sprintf("#%d", item.Crateseries))
		}
	} else if item.Craftnumber != 0 {
		parts = append(parts, fmt.Sprintf("#%d", item.Craftnumber))
	}

	if !scmFormat && item.Paint != 0 {
		paintName := s.PaintNameByDecimal(item.Paint)
		if paintName != "" {
			parts = append(parts, fmt.Sprintf("(Paint: %s)", paintName))
		}
	}

	if scmFormat && schemaItem.ItemName == "Chemistry Set" && item.Output == 6522 {
		if item.Target != 0 {
			if series, ok := strangifierChemistrySetSeries[item.Target]; ok {
				parts = append(parts, fmt.Sprintf("Series %%23%d", series))
			}
		}
	}

	if scmFormat && item.Wear != 0 && item.Effect != 0 && item.Quality == QualityDecorated {
		parts = append([]string{"Unusual"}, parts...)
	}

	return strings.Join(parts, " ")
}

// ItemFromName parses a localized display name string into a structured [sku.Item].
func (s *Schema) ItemFromName(name string) *sku.Item {
	item := &sku.Item{
		Craftable: true,
		Tradable:  true,
	}
	originalName := name
	name = strings.ToLower(name)

	debugLog("GetItemObjectFromName start:", originalName)

	if strings.Contains(name, "strange part:") ||
		strings.Contains(name, "strange cosmetic part:") ||
		strings.Contains(name, "strange filter:") ||
		strings.Contains(name, "strange count transfer tool") ||
		strings.Contains(name, "strange bacon grease") {
		schemaItem := s.ItemByName(originalName)
		if schemaItem != nil {
			item.Defindex = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}
		}

		debugLog("return early (strange part)", item)

		return item
	}

	for w, val := range wears {
		if strings.Contains(name, w) {
			debugLog("wear before", name, item)
			name = strings.ReplaceAll(name, w, "")
			name = strings.TrimSpace(name)
			item.Wear = val
			debugLog("wear after", name, item)

			break
		}
	}

	isExplicitElevatedStrange := false

	if strings.Contains(name, "strange(e)") {
		debugLog("strange(e) before", name, item)
		item.Quality2 = QualityStrange
		isExplicitElevatedStrange = true
		name = strings.ReplaceAll(name, "strange(e)", "")
		name = strings.TrimSpace(name)
		debugLog("strange(e) after", name, item)
	}

	hasStrangePrefix := false

	if strings.Contains(name, "strange") && !strings.Contains(name, "strangifier") {
		debugLog("strange before", name, item)

		hasStrangePrefix = true
		name = strings.ReplaceAll(name, "strange", "")
		name = strings.TrimSpace(name)
		debugLog("strange after", name, item)
	}

	if strings.Contains(name, "craft") {
		name = strings.ReplaceAll(name, "uncraftable", "non-craftable")
		if strings.Contains(name, "non-craftable") {
			debugLog("non-craftable before", name, item)
			name = strings.ReplaceAll(name, "non-craftable", "")
			name = strings.TrimSpace(name)
			item.Craftable = false
			debugLog("non-craftable after", name, item)
		}
	}

	if strings.Contains(name, "trad") {
		name = strings.ReplaceAll(name, "untradeable", "non-tradable")
		name = strings.ReplaceAll(name, "untradable", "non-tradable")

		name = strings.ReplaceAll(name, "non-tradeable", "non-tradable")
		if strings.Contains(name, "non-tradable") {
			debugLog("non-tradable before", name, item)
			name = strings.ReplaceAll(name, "non-tradable", "")
			name = strings.TrimSpace(name)
			item.Tradable = false
			debugLog("non-tradable after", name, item)
		}
	}

	if strings.Contains(name, "unusualifier") {
		debugLog("unusualifier before", name, item)
		name = strings.ReplaceAll(name, "unusual ", "")
		name = strings.ReplaceAll(name, " unusualifier", "")
		name = strings.ReplaceAll(name, "unusualifier", "")
		name = strings.TrimSpace(name)
		item.Defindex = 9258
		item.Quality = QualityUnusual

		schemaItem := s.ItemByName(name)
		if schemaItem != nil {
			item.Target = schemaItem.Defindex
		}

		debugLog("unusualifier after", name, item)

		return item
	}

	kitFabricatorDetected := strings.Contains(name, "kit fabricator")

	killstreaks := []struct {
		phrase string
		value  int
	}{
		{"professional killstreak", 3},
		{"specialized killstreak", 2},
		{"killstreak", 1},
	}
	for _, ks := range killstreaks {
		if strings.Contains(name, ks.phrase) {
			debugLog("killstreak before", name, item)
			name = strings.Replace(name, ks.phrase, "", 1)
			name = strings.TrimSpace(name)
			item.Killstreak = ks.value
			debugLog("killstreak after", name, item)

			break
		}
	}

	if strings.Contains(name, "australium") && !strings.Contains(name, "australium gold") {
		debugLog("australium before", name, item)
		name = strings.ReplaceAll(name, "australium", "")
		name = strings.TrimSpace(name)
		item.Australium = true
		debugLog("australium after", name, item)
	}

	if strings.Contains(name, "festivized") && !strings.Contains(name, "festivized formation") {
		debugLog("festivized before", name, item)
		name = strings.ReplaceAll(name, "festivized", "")
		name = strings.TrimSpace(name)
		item.Festivized = true
		debugLog("festivized after", name, item)
	}

	exception := []string{
		"haunted ghosts", "haunted phantasm jr", "haunted phantasm",
		"haunted metal scrap", "haunted hat", "unusual cap",
		"vintage tyrolean", "vintage merryweather", "haunted kraken",
		"haunted forever!", "haunted cremation", "haunted wick",
	}

	qualitySearch := name
	for _, ex := range exception {
		if strings.Contains(name, ex) {
			qualitySearch = strings.ReplaceAll(name, ex, "")
			qualitySearch = strings.TrimSpace(qualitySearch)

			break
		}
	}

	if !slices.Contains(exception, qualitySearch) {
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
				debugLog("quality before", name, item)

				if item.Quality != 0 && item.Quality != qID {
					if item.Quality2 == Quality2None {
						item.Quality2 = item.Quality
					}

					item.Quality = qID
				} else {
					item.Quality = qID
				}

				name = strings.Replace(name, qName, "", 1)
				name = strings.TrimSpace(name)

				debugLog("quality after", name, item)

				break
			}
		}
	}

	excludeAtomic := strings.Contains(name, "bonk! atomic punch") || strings.Contains(name, "atomic accolade")

	for effName, effID := range s.effByName {
		if effName == "" {
			continue
		}

		if strings.Contains(name, effName) {
			if effName == "stardust" && strings.Contains(name, "starduster") {
				sub := strings.ReplaceAll(name, "stardust", "")
				if !strings.Contains(sub, "starduster") {
					continue
				}
			}

			if effName == "showstopper" && !strings.Contains(name, "taunt: ") &&
				!strings.Contains(name, "shred alert") {
				continue
			}

			if effName == "smoking" && (name == "smoking jacket" || strings.Contains(name, "smoking skid lid")) {
				if !strings.HasPrefix(name, "smoking smoking") {
					continue
				}
			}

			if effName == "haunted ghosts" && strings.Contains(name, "haunted ghosts") && item.Wear != 0 {
				continue
			}

			if effName == "pumpkin patch" && strings.Contains(name, "pumpkin patch") && item.Wear != 0 {
				continue
			}

			if effName == "stardust" && strings.Contains(name, "stardust") && item.Wear != 0 {
				continue
			}

			if effName == "atomic" && (strings.Contains(name, "subatomic") || excludeAtomic) {
				continue
			}

			if effName == "spellbound" && (strings.Contains(name, "taunt:") || strings.Contains(name, "shred alert")) {
				continue
			}

			if effName == "accursed" && strings.Contains(name, "accursed apparition") {
				continue
			}

			if effName == "haunted" && strings.Contains(name, "haunted kraken") {
				continue
			}

			if effName == "frostbite" && strings.Contains(name, "frostbite bonnet") {
				continue
			}

			if effName == "hot" {
				if item.Wear == 0 {
					continue
				}

				if !strings.Contains(name, "hot ") && (strings.Contains(name, "shotgun") ||
					strings.Contains(name, "shot ") || strings.Contains(name, "plaid potshotter")) {
					continue
				}

				if !strings.HasPrefix(name, "hot ") {
					continue
				}
			}

			if effName == "cool" && item.Wear == 0 {
				continue
			}

			debugLog("effect before", name, item)
			name = strings.ReplaceAll(name, effName, "")
			name = strings.TrimSpace(name)

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

			debugLog("effect after", name, item)

			break
		}
	}

	if item.Wear != 0 {
		for pkName, pkID := range s.paintKitByName {
			if strings.Contains(name, pkName) {
				if strings.Contains(name, "mk.ii") && !strings.Contains(pkName, "mk.ii") {
					continue
				}

				if strings.Contains(name, "(green)") && !strings.Contains(pkName, "(green)") {
					continue
				}

				if strings.Contains(name, "chilly") && !strings.Contains(pkName, "chilly") {
					continue
				}

				debugLog("paintkit before", name, item)
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

				debugLog("paintkit after", name, item)

				break
			}
		}

		if !strings.Contains(name, "war paint") {
			oldDefindex := item.Defindex
			switch {
			case strings.Contains(name, "pistol") && pistolSkins[item.Paintkit] != 0:
				item.Defindex = pistolSkins[item.Paintkit]
			case strings.Contains(name, "rocket launcher") && rocketLauncherSkins[item.Paintkit] != 0:
				item.Defindex = rocketLauncherSkins[item.Paintkit]
			case strings.Contains(name, "medi gun") && medicgunSkins[item.Paintkit] != 0:
				item.Defindex = medicgunSkins[item.Paintkit]
			case strings.Contains(name, "revolver") && revolverSkins[item.Paintkit] != 0:
				item.Defindex = revolverSkins[item.Paintkit]
			case strings.Contains(name, "stickybomb launcher") && stickybombSkins[item.Paintkit] != 0:
				item.Defindex = stickybombSkins[item.Paintkit]
			case strings.Contains(name, "sniper rifle") && sniperRifleSkins[item.Paintkit] != 0:
				item.Defindex = sniperRifleSkins[item.Paintkit]
			case strings.Contains(name, "flame thrower") && flameThrowerSkins[item.Paintkit] != 0:
				item.Defindex = flameThrowerSkins[item.Paintkit]
			case strings.Contains(name, "minigun") && minigunSkins[item.Paintkit] != 0:
				item.Defindex = minigunSkins[item.Paintkit]
			case strings.Contains(name, "scattergun") && scattergunSkins[item.Paintkit] != 0:
				item.Defindex = scattergunSkins[item.Paintkit]
			case strings.Contains(name, "shotgun") && shotgunSkins[item.Paintkit] != 0:
				item.Defindex = shotgunSkins[item.Paintkit]
			case strings.Contains(name, "smg") && smgSkins[item.Paintkit] != 0:
				item.Defindex = smgSkins[item.Paintkit]
			case strings.Contains(name, "grenade launcher") && grenadeLauncherSkins[item.Paintkit] != 0:
				item.Defindex = grenadeLauncherSkins[item.Paintkit]
			case strings.Contains(name, "wrench") && wrenchSkins[item.Paintkit] != 0:
				item.Defindex = wrenchSkins[item.Paintkit]
			case strings.Contains(name, "knife") && knifeSkins[item.Paintkit] != 0:
				item.Defindex = knifeSkins[item.Paintkit]
			}

			if oldDefindex != item.Defindex {
				debugLog("return after skin mapping", name, item)
				return item
			}
		}
	}

	if strings.Contains(name, "(paint: ") {
		debugLog("paint before loop", name, item)
		name = strings.ReplaceAll(name, "(paint: ", "")
		name = strings.ReplaceAll(name, ")", "")

		name = strings.TrimSpace(name)
		for pName, pVal := range s.paintByName {
			if strings.Contains(name, pName) {
				debugLog("paint in loop before", name, item)
				name = strings.ReplaceAll(name, pName, "")
				name = strings.TrimSpace(name)
				item.Paint = pVal
				debugLog("paint after", name, item)

				break
			}
		}
	}

	if kitFabricatorDetected && item.Killstreak > 1 {
		debugLog("kit fabricator before", name, item)
		name = strings.ReplaceAll(name, "kit fabricator", "")
		name = strings.TrimSpace(name)

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
				debugLog("return kit fabricator (no target)", name, item)
				return item
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
		debugLog("kit fabricator after", name, item)
	}

	if strings.Contains(name, "chemistry set") &&
		(!strings.Contains(name, "strangifier chemistry set") || strings.Contains(name, "collector's")) {
		debugLog("collector's chemistry set before", name, item)
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
			schemaItem := s.ItemByName(name)
			if schemaItem != nil {
				item.Output = schemaItem.Defindex
				item.OutputQuality = QualityCollectors
			}
		}

		debugLog("collector's chemistry set after", name, item)

		return item
	}

	if strings.Contains(name, "strangifier chemistry set") {
		debugLog("strangifier chemistry set before", name, item)
		name = strings.ReplaceAll(name, "strangifier chemistry set", "")
		name = strings.TrimSpace(name)

		item.Defindex = 20000
		item.Quality = QualityUnique
		item.Output = 6522
		item.OutputQuality = QualityUnique

		if name != "" {
			schemaItem := s.ItemByName(name)
			if schemaItem != nil {
				item.Target = schemaItem.Defindex
			}
		}

		debugLog("strangifier chemistry set after", name, item)

		return item
	}

	if strings.Contains(name, "strangifier") && !strings.Contains(name, "strangifier chemistry set") {
		debugLog("strangifier before", name, item)
		name = strings.ReplaceAll(name, "strangifier", "")
		name = strings.TrimSpace(name)
		item.Defindex = 6522

		schemaItem := s.ItemByName(name)
		if schemaItem != nil {
			item.Target = schemaItem.Defindex
			if item.Quality == 0 {
				item.Quality = schemaItem.ItemQuality
			}
		} else {
			debugLog("return strangifier (no target)", name, item)
			return item
		}

		debugLog("strangifier after", name, item)
	}

	if !kitFabricatorDetected && strings.Contains(name, "kit") && item.Killstreak > 0 {
		debugLog("kit before", name, item)
		kitType := item.Killstreak
		item.Killstreak = 0

		name = strings.ReplaceAll(name, "kit", "")
		name = strings.TrimSpace(name)

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
				debugLog("return kit (no target)", name, item)
				return item
			}
		}

		if item.Quality == 0 {
			item.Quality = QualityUnique
		}

		debugLog("kit after", name, item)
	}

	if item.Defindex != 0 {
		debugLog("return after defindex set", name, item)
		return item
	}

	if item.Paintkit != 0 && strings.Contains(name, "war paint") {
		debugLog("war paint before", name, item)

		searchName := fmt.Sprintf("Paintkit %d", item.Paintkit)
		if item.Quality == 0 {
			item.Quality = QualityDecorated
		}

		for _, it := range s.Raw.Schema.Items {
			if it.Name == searchName {
				item.Defindex = it.Defindex
				break
			}
		}

		debugLog("war paint after", name, item)

		return item
	}

	name = strings.ReplaceAll(name, " series ", " ")
	name = strings.ReplaceAll(name, " series#", " #")

	var number int

	if strings.Contains(name, "#") {
		debugLog("with # before", name, item)
		parts := strings.SplitN(name, "#", 2)
		name = strings.TrimSpace(parts[0])
		number, _ = strconv.Atoi(strings.TrimSpace(parts[1]))

		debugLog("with # after", name, item)
	}

	if strings.Contains(name, "salvaged mann co. supply crate") && !strings.Contains(name, "key") {
		debugLog("salvaged crate", name, item)
		item.Crateseries = number
		item.Defindex = 5068
		item.Quality = QualityUnique
		debugLog("return salvaged crate", name, item)

		return item
	}

	if strings.Contains(name, "select reserve mann co. supply crate") && !strings.Contains(name, "key") {
		item.Defindex = 5660
		item.Crateseries = 60
		item.Quality = QualityUnique

		return item
	}

	if strings.Contains(name, "mann co. supply crate") && !strings.Contains(name, "key") {
		debugLog("mann co crate", name, item)

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
		debugLog("return mann co crate", name, item)

		return item
	}

	if strings.Contains(name, "mann co. supply munition") {
		debugLog("munition crate", name, item)

		crateseries := number
		if def, ok := munitionCrate[crateseries]; ok {
			item.Defindex = def
		}

		item.Crateseries = crateseries
		item.Quality = QualityUnique
		debugLog("return munition crate", name, item)

		return item
	}

	for _, keyName := range retiredKeysNames {
		if strings.ToLower(name) == keyName {
			for _, info := range retiredKeys {
				if strings.ToLower(info.Name) == keyName {
					item.Defindex = info.Defindex
					if item.Quality == 0 {
						item.Quality = QualityUnique
					}

					debugLog("return retired key", name, item)

					return item
				}
			}
		}
	}

	schemaItem := s.ItemByNameWithThe(name)
	if schemaItem == nil {
		debugLog("return no schema item", name, item)
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
		debugLog("supply_crate before", name, item)

		if series, ok := s.crateSeriesList[item.Defindex]; ok {
			item.Crateseries = series
		} else if number != 0 {
			item.Crateseries = number
		}

		debugLog("supply_crate after", name, item)
	} else if number != 0 {
		debugLog("craftnumber before", name, item)
		item.Craftnumber = number
		debugLog("craftnumber after", name, item)
	}

	debugLog("final return", name, item)

	return item
}

// SkuFromName parses a localized name and returns its standardized SKU string.
func (s *Schema) SkuFromName(name string) string {
	item := s.ItemFromName(name)
	return sku.FromObject(item)
}

// SKUFromItem normalizes the [sku.Item] and returns its standardized SKU string.
func (s *Schema) SKUFromItem(item *sku.Item) string {
	if item == nil {
		return ""
	}

	s.NormalizeItem(item)

	return sku.FromObject(item)
}

// ItemFromEconItem converts a generic [trading.Item] into a structured [sku.Item] with all attributes parsed.
func (s *Schema) ItemFromEconItem(item *trading.Item) *sku.Item {
	if item == nil {
		return nil
	}

	nameToParse := item.MarketHashName
	if nameToParse == "" {
		nameToParse = item.MarketName
	}

	skuItem := s.ItemFromName(nameToParse)
	if skuItem == nil {
		return nil
	}

	for _, tag := range item.Tags {
		if tag.Category == "Exterior" {
			if wearID := s.WearByName(tag.LocalizedName); wearID != 0 {
				skuItem.Wear = wearID
			}
		}
	}

	if skuItem.Quality == QualityDecorated {
		lowerName := strings.ToLower(item.MarketHashName)
		for pkName, pkID := range s.paintKitByName {
			if strings.Contains(lowerName, pkName) {
				skuItem.Paintkit = pkID
				break
			}
		}
	}

	skuItem.Tradable = item.Tradable

	for _, desc := range item.Descriptions {
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
			break
		}
	}

	for _, d := range item.Descriptions {
		val := d.Value

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

		if d.Color == "756b5e" {
			clean := strings.Trim(val, "()")
			if before, after, ok := strings.Cut(clean, ":"); ok {
				partName := strings.TrimSpace(before)
				for name, suffix := range s.StrangeParts() {
					if strings.Contains(partName, name) {
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
		}

		if strings.ToLower(d.Color) == "7ea9d1" {
			spellName := strings.TrimSpace(val)
			if spell, ok := s.SpellIDByName(spellName); ok {
				skuItem.Spells = append(skuItem.Spells, spell)
			}
		}
	}

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

// SKUFromEconItem converts a generic [trading.Item] into a standardized TF2 SKU string.
func (s *Schema) SKUFromEconItem(item *trading.Item) string {
	skuItem := s.ItemFromEconItem(item)
	if skuItem == nil {
		return "unknown"
	}

	return sku.FromObject(skuItem)
}

// IsPromoItem returns true if the specified [Item] is a promotional item.
func (s *Schema) IsPromoItem(it *Item) bool {
	return strings.HasPrefix(it.Name, "Promo ") && it.CraftClass == ""
}

// NormalizeItem adjusts the [sku.Item] defindex and quality parameters to follow trading standards.
func (s *Schema) NormalizeItem(item *sku.Item) {
	item.Defindex = NormalizeDefindex(item.Defindex)

	schemaItem := s.ItemByDef(item.Defindex)
	if schemaItem == nil {
		return
	}

	if strings.Contains(schemaItem.Name, strings.ToUpper(schemaItem.ItemClass)) {
		for _, it := range s.Raw.Schema.Items {
			if it.ItemClass == schemaItem.ItemClass && strings.HasPrefix(it.Name, "Upgradeable ") {
				item.Defindex = it.Defindex
				break
			}
		}
	}

	isPromo := s.IsPromoItem(schemaItem)
	if isPromo && item.Quality != QualityGenuine {
		for _, it := range s.Raw.Schema.Items {
			if !s.IsPromoItem(it) && it.ItemName == schemaItem.ItemName {
				item.Defindex = it.Defindex
				break
			}
		}
	} else if !isPromo && item.Quality == QualityGenuine {
		for _, it := range s.Raw.Schema.Items {
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

// ToJSON serializes the [Schema] metadata and raw data to a generic JSON-friendly map.
func (s *Schema) ToJSON() map[string]any {
	return map[string]any{
		"version": s.Version,
		"time":    s.Time.Unix(),
		"raw":     s.Raw,
	}
}

// WeaponOption represents a base weapon definition name and defindex.
type WeaponOption struct {
	Defindex uint32
	Name     string
}

// GetSupportedWeaponsForPaintkit returns the list of base weapons (name and defindex) that can have the specified paintkit applied.
func (s *Schema) GetSupportedWeaponsForPaintkit(paintkitID int) []WeaponOption {
	var options []WeaponOption

	// Helper to add if mapped
	addOption := func(defindex uint32, name string, skinMap map[int]int) {
		if skinMap[paintkitID] != 0 {
			options = append(options, WeaponOption{
				Defindex: defindex,
				Name:     name,
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
