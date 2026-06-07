// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"slices"
	"strings"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Filter represents a function used to screen and match items in the backpack.
type Filter func(item *tf2.Item, s *schema.Schema) bool

// LessFunc is a comparison function used to sort items within a section or page.
type LessFunc func(a, b *tf2.Item, s *schema.Schema) int

// SectionLayout defines a logical backpack division of items.
type SectionLayout struct {
	// Name is the descriptive name of the section (e.g. "Weapons").
	Name string
	// Filters contains the selection criteria. Items matching any filter are selected.
	Filters []Filter
	// OrderBy optionally defines how items should be sorted within this section.
	OrderBy LessFunc
	// StartPage is the 1-based start page for this section. If 0, it behaves continuously from the previous section.
	StartPage int
	// EndPage is the 1-based inclusive end page for this section. If 0, there is no upper limit.
	EndPage int
}

// Layout represents the configuration used to sort and arrange the backpack.
type Layout struct {
	// Sections defines the logical divisions of the inventory.
	Sections []SectionLayout
}

// And returns a [Filter] that requires all provided filters to match.
func And(filters ...Filter) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		for _, f := range filters {
			if !f(item, s) {
				return false
			}
		}

		return true
	}
}

// Or returns a [Filter] that matches if any of the provided filters match.
func Or(filters ...Filter) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		for _, f := range filters {
			if f(item, s) {
				return true
			}
		}

		return false
	}
}

// Not returns a [Filter] that negates the result of the specified filter.
func Not(f Filter) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return !f(item, s)
	}
}

// BySKU returns a filter that checks if the item matches the specified SKU.
func BySKU(targetSKU string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.GetSKU(s) == targetSKU
	}
}

// ByQuality returns a filter that checks if the item has the specified quality.
func ByQuality(q uint32) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.Quality == q
	}
}

// ByClass returns a filter that checks if the item is used by the specified class.
func ByClass(class string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		return slices.Contains(sch.UsedByClasses, class)
	}
}

// IsPure returns a filter that checks if the item is pure (reclaimed metal, refined metal, keys).
func IsPure() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		d := s.NormalizeDefindex(int(item.DefIndex))
		return d == schema.DefKey || d == schema.DefRefined || d == schema.DefReclaimed || d == schema.DefScrap
	}
}

// IsWeapon returns a [Filter] that matches weapons.
func IsWeapon() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		return sch != nil &&
			(sch.CraftClass == "weapon" || sch.ItemClass == "weapon" || strings.HasPrefix(sch.ItemClass, "tf_weapon_"))
	}
}

// IsCosmetic returns a [Filter] that matches cosmetics (hats and wearables, excluding taunts and action items).
func IsCosmetic() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		if sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:") {
			return false
		}

		if isActionItem(sch) {
			return false
		}

		return sch.CraftClass == "hat" || sch.ItemClass == "tf_wearable"
	}
}

// IsTaunt returns a [Filter] that matches action taunts.
func IsTaunt() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		return sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:")
	}
}

// IsCrate returns a [Filter] that matches crates and cases.
func IsCrate() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		return sch != nil && sch.ItemClass == "supply_crate"
	}
}

// IsTradable returns a [Filter] that matches tradable items.
func IsTradable() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.IsTradable
	}
}

// IsTool returns a [Filter] that matches tools.
func IsTool() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		return sch != nil && (sch.ItemClass == "tool" || sch.CraftClass == "tool")
	}
}

// IsAction returns a [Filter] that matches action items.
func IsAction() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		return isActionItem(sch)
	}
}

func isActionItem(sch *schema.Item) bool {
	if sch == nil {
		return false
	}

	if sch.ItemClass == "action" || sch.CraftClass == "action" {
		return true
	}

	nameLower := strings.ToLower(sch.ItemName)
	internalLower := strings.ToLower(sch.Name)

	if strings.Contains(nameLower, "noise maker") || strings.Contains(internalLower, "noise_maker") {
		return true
	}

	if nameLower == "secret saxton" || internalLower == "gift - 1 player" {
		return true
	}

	return false
}

// DefaultLayout returns the standard, optimal continuous hierarchical inventory layout.
func DefaultLayout() Layout {
	return Layout{
		Sections: []SectionLayout{
			{
				Name: "Currency",
				Filters: []Filter{
					And(IsTradable(), IsPure()),
				},
				OrderBy: CurrencySorter,
			},
			{
				Name: "Weapons",
				Filters: []Filter{
					And(IsTradable(), IsWeapon()),
				},
				OrderBy: WeaponsSorter,
			},
			{
				Name: "Cosmetics",
				Filters: []Filter{
					And(IsTradable(), IsCosmetic()),
				},
				OrderBy: CosmeticsSorter,
			},
			{
				Name: "Taunts",
				Filters: []Filter{
					And(IsTradable(), IsTaunt()),
				},
				OrderBy: DefindexSorter,
			},
			{
				Name: "Tools & Actions",
				Filters: []Filter{
					And(IsTradable(), Or(IsTool(), IsAction())),
				},
				OrderBy: DefindexSorter,
			},
			{
				Name: "Crates & Cases",
				Filters: []Filter{
					And(IsTradable(), IsCrate()),
				},
				OrderBy: DefindexSorter,
			},
			{
				Name: "Untradable Metal",
				Filters: []Filter{
					And(Not(IsTradable()), IsPure()),
				},
				OrderBy: CurrencySorter,
			},
			{
				Name: "Untradable Weapons",
				Filters: []Filter{
					And(Not(IsTradable()), IsWeapon()),
				},
				OrderBy: WeaponsSorter,
			},
			{
				Name: "Untradable Cosmetics",
				Filters: []Filter{
					And(Not(IsTradable()), IsCosmetic()),
				},
				OrderBy: CosmeticsSorter,
			},
			{
				Name: "Untradable Misc",
				Filters: []Filter{
					Not(IsTradable()),
				},
				OrderBy: DefindexSorter,
			},
		},
	}
}

// CurrencySorter sorts Keys first, then Ref, Rec, and Scrap.
func CurrencySorter(a, b *tf2.Item, s *schema.Schema) int {
	aPri, bPri := GetPurePriority(a.DefIndex, s), GetPurePriority(b.DefIndex, s)
	if aPri != bPri {
		return aPri - bPri
	}

	if a.DefIndex != b.DefIndex {
		return int(a.DefIndex) - int(b.DefIndex)
	}

	if a.ID < b.ID {
		return -1
	}

	return 1
}

// WeaponsSorter groups weapons by quality (Unique first, others second), then by class (Scout -> Spy -> Multiclass), slot (Primary -> Melee), and defindex.
func WeaponsSorter(a, b *tf2.Item, s *schema.Schema) int {
	aQualPri, bQualPri := GetQualityPriority(a.Quality), GetQualityPriority(b.Quality)
	if aQualPri != bQualPri {
		return aQualPri - bQualPri
	}

	aClassPri, bClassPri := GetClassPriority(a, s), GetClassPriority(b, s)
	if aClassPri != bClassPri {
		return aClassPri - bClassPri
	}

	aSlotPri, bSlotPri := GetSlotPriority(a, s), GetSlotPriority(b, s)
	if aSlotPri != bSlotPri {
		return aSlotPri - bSlotPri
	}

	if a.DefIndex != b.DefIndex {
		return int(a.DefIndex) - int(b.DefIndex)
	}

	if a.Quality != b.Quality {
		return int(a.Quality) - int(b.Quality)
	}

	if a.ID < b.ID {
		return -1
	}

	return 1
}

// CosmeticsSorter groups cosmetics by quality (Unique first, others second), then by class and defindex.
func CosmeticsSorter(a, b *tf2.Item, s *schema.Schema) int {
	aQualPri := GetQualityPriority(a.Quality)

	bQualPri := GetQualityPriority(b.Quality)
	if aQualPri != bQualPri {
		return aQualPri - bQualPri
	}

	aClassPri := GetClassPriority(a, s)

	bClassPri := GetClassPriority(b, s)
	if aClassPri != bClassPri {
		return aClassPri - bClassPri
	}

	if a.DefIndex != b.DefIndex {
		return int(a.DefIndex) - int(b.DefIndex)
	}

	if a.Quality != b.Quality {
		return int(a.Quality) - int(b.Quality)
	}

	if a.ID < b.ID {
		return -1
	}

	return 1
}

// DefindexSorter groups identical items side-by-side with Unique first.
func DefindexSorter(a, b *tf2.Item, s *schema.Schema) int {
	if a.DefIndex != b.DefIndex {
		return int(a.DefIndex) - int(b.DefIndex)
	}

	aQualPri := GetQualityPriority(a.Quality)

	bQualPri := GetQualityPriority(b.Quality)
	if aQualPri != bQualPri {
		return aQualPri - bQualPri
	}

	if a.Quality != b.Quality {
		return int(a.Quality) - int(b.Quality)
	}

	if a.ID < b.ID {
		return -1
	}

	return 1
}

// GetPurePriority maps DefIndexes to currency priorities.
func GetPurePriority(defIndex uint32, s *schema.Schema) int {
	norm := s.NormalizeDefindex(int(defIndex))
	switch norm {
	case schema.DefKey:
		return 1
	case schema.DefRefined:
		return 2
	case schema.DefReclaimed:
		return 3
	case schema.DefScrap:
		return 4
	default:
		return 5
	}
}

// GetClassPriority groups by TF2 classes (Scout -> Spy -> Multiclass -> All-Class).
func GetClassPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil || len(sch.UsedByClasses) == 0 {
		return 12 // Misc/All-Class
	}

	if len(sch.UsedByClasses) > 1 {
		return 10 // Multiclass
	}

	switch sch.UsedByClasses[0] {
	case "Scout":
		return 1
	case "Soldier":
		return 2
	case "Pyro":
		return 3
	case "Demoman":
		return 4
	case "Heavy":
		return 5
	case "Engineer":
		return 6
	case "Medic":
		return 7
	case "Sniper":
		return 8
	case "Spy":
		return 9
	default:
		return 11
	}
}

// GetSlotPriority resolves weapon slots using comprehensive prefix-matching.
func GetSlotPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return 5
	}

	isWeapon := sch.CraftClass == "weapon" || sch.ItemClass == "weapon" ||
		strings.HasPrefix(sch.ItemClass, "tf_weapon_")
	if !isWeapon {
		return 5
	}

	cls := sch.ItemClass
	def := item.DefIndex

	// 1. Primary Weapons
	if def == 9 || def == 141 || def == 527 || def == 588 || def == 997 || def == 1153 {
		return 1 // Primary Shotguns for Engineer
	}

	if strings.Contains(cls, "scattergun") ||
		strings.Contains(cls, "rocketlauncher") ||
		strings.Contains(cls, "flamethrower") ||
		strings.Contains(cls, "grenadelauncher") ||
		strings.Contains(cls, "minigun") ||
		strings.Contains(cls, "syringegun") ||
		strings.Contains(cls, "sniperrifle") ||
		strings.Contains(cls, "revolver") ||
		strings.Contains(cls, "crossbow") ||
		strings.Contains(cls, "compound_bow") ||
		strings.Contains(cls, "particle_cannon") ||
		strings.Contains(cls, "soda_popper") ||
		strings.Contains(cls, "handgun_scout_primary") ||
		def == 1178 { // Dragon's Fury
		return 1
	}

	// 2. Secondary Weapons
	if strings.Contains(cls, "pistol") ||
		strings.Contains(cls, "pipebomblauncher") ||
		strings.Contains(cls, "smg") ||
		strings.Contains(cls, "medigun") ||
		strings.Contains(cls, "buff_item") ||
		strings.Contains(cls, "parachute") ||
		strings.Contains(cls, "lunchbox") ||
		strings.Contains(cls, "jar") ||
		strings.Contains(cls, "laser_pointer") || // Wrangler
		strings.Contains(cls, "shotgun") || // pyro/soldier/heavy shotguns
		strings.Contains(cls, "handgun_scout_secondary") ||
		strings.Contains(cls, "raygun") ||
		def == 131 || def == 406 || def == 1101 { // Shields (Targe, Screen, Tide Turner)
		return 2
	}

	// 3. Melee Weapons
	if strings.Contains(cls, "bat") ||
		strings.Contains(cls, "shovel") ||
		strings.Contains(cls, "fireaxe") ||
		strings.Contains(cls, "club") ||
		strings.Contains(cls, "bonesaw") ||
		strings.Contains(cls, "fists") ||
		strings.Contains(cls, "wrench") ||
		strings.Contains(cls, "knife") ||
		strings.Contains(cls, "sword") ||
		strings.Contains(cls, "sledgehammer") ||
		strings.Contains(cls, "mechanical_arm") || // Gunslinger
		strings.Contains(cls, "stick") {
		return 3
	}

	// 4. PDA / Action / Builder
	if strings.Contains(cls, "pda") ||
		strings.Contains(cls, "builder") ||
		strings.Contains(cls, "spellbook") {
		return 4
	}

	return 5
}

// GetQualityPriority returns priority for Unique quality.
func GetQualityPriority(quality uint32) int {
	if quality == schema.QualityUnique {
		return 1
	}

	return 2
}
