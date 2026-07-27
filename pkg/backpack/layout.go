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

type Filter func(item *tf2.Item, s *schema.Schema) bool

type LessFunc func(a, b *tf2.Item, s *schema.Schema) int

type SectionLayout struct {
	Name      string
	Filters   []Filter
	OrderBy   LessFunc
	StartPage int
	EndPage   int
}

type Layout struct {
	Sections []SectionLayout
}

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

func Not(f Filter) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return !f(item, s)
	}
}

func BySKU(targetSKU string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.GetSKU(s) == targetSKU
	}
}

func ByQuality(q uint32) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.Quality == q
	}
}

func ByClass(class string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		return slices.Contains(sch.UsedByClasses, class)
	}
}

func IsPure() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		d := s.NormalizeDefindex(int(item.DefIndex))

		return d == schema.DefKey || d == schema.DefRefined || d == schema.DefReclaimed || d == schema.DefScrap
	}
}

func IsWeapon() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)

		return sch != nil &&
			(sch.CraftClass == "weapon" || sch.ItemClass == "weapon" || strings.HasPrefix(sch.ItemClass, "tf_weapon_"))
	}
}

func IsCosmetic() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		if sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:") ||
			isActionItem(sch) {
			return false
		}

		return sch.CraftClass == "hat" || sch.ItemClass == "tf_wearable"
	}
}

func IsTaunt() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		return sch.ItemClass == "tf_wearable_taunt" || strings.HasPrefix(strings.ToLower(sch.ItemName), "taunt:")
	}
}

func IsCrate() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)

		return sch != nil && sch.ItemClass == "supply_crate"
	}
}

func IsTradable() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.IsTradable
	}
}

func IsTool() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)

		return sch != nil && (sch.ItemClass == "tool" || sch.CraftClass == "tool")
	}
}

func IsAction() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return isActionItem(item.GetSchema(s))
	}
}

func isActionItem(sch *schema.Item) bool {
	if sch == nil {
		return false
	}

	if sch.ItemClass == "action" || sch.CraftClass == "action" || sch.ItemSlot == "action" {
		return true
	}

	nameLower := strings.ToLower(sch.ItemName)
	internalLower := strings.ToLower(sch.Name)

	return strings.Contains(nameLower, "noise maker") || strings.Contains(internalLower, "noise_maker") ||
		nameLower == "secret saxton" || internalLower == "gift - 1 player" || strings.Contains(nameLower, "gargoyle")
}

// DefaultLayout returns standard, optimal continuous inventory layout rules.
func DefaultLayout() Layout {
	return Layout{
		Sections: []SectionLayout{
			{
				Name:    "Currency",
				Filters: []Filter{And(IsTradable(), IsPure())},
				OrderBy: CurrencySorter,
			},
			{
				Name:    "Weapons",
				Filters: []Filter{And(IsTradable(), IsWeapon())},
				OrderBy: WeaponsSorter,
			},
			{
				Name:    "Cosmetics",
				Filters: []Filter{And(IsTradable(), IsCosmetic())},
				OrderBy: CosmeticsSorter,
			},
			{
				Name:    "Taunts",
				Filters: []Filter{And(IsTradable(), IsTaunt())},
				OrderBy: DefindexSorter,
			},
			{
				Name:    "Tools & Actions",
				Filters: []Filter{And(IsTradable(), Or(IsTool(), IsAction()))},
				OrderBy: DefindexSorter,
			},
			{
				Name:    "Crates & Cases",
				Filters: []Filter{And(IsTradable(), IsCrate())},
				OrderBy: DefindexSorter,
			},
			{
				Name:    "Untradable Metal",
				Filters: []Filter{And(Not(IsTradable()), IsPure())},
				OrderBy: CurrencySorter,
			},
			{
				Name:    "Untradable Weapons",
				Filters: []Filter{And(Not(IsTradable()), IsWeapon())},
				OrderBy: WeaponsSorter,
			},
			{
				Name:    "Untradable Cosmetics",
				Filters: []Filter{And(Not(IsTradable()), IsCosmetic())},
				OrderBy: CosmeticsSorter,
			},
			{
				Name:    "Untradable Tools & Actions",
				Filters: []Filter{And(Not(IsTradable()), Or(IsTool(), IsAction()))},
				OrderBy: DefindexSorter,
			},
			{
				Name:    "Untradable Misc",
				Filters: []Filter{Not(IsTradable())},
				OrderBy: DefindexSorter,
			},
		},
	}
}

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

func WeaponsSorter(a, b *tf2.Item, s *schema.Schema) int {
	if aQualPri, bQualPri := GetQualityPriority(a.Quality), GetQualityPriority(b.Quality); aQualPri != bQualPri {
		return aQualPri - bQualPri
	}

	if aClassPri, bClassPri := GetClassPriority(a, s), GetClassPriority(b, s); aClassPri != bClassPri {
		return aClassPri - bClassPri
	}

	if aSlotPri, bSlotPri := GetSlotPriority(a, s), GetSlotPriority(b, s); aSlotPri != bSlotPri {
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

func CosmeticsSorter(a, b *tf2.Item, s *schema.Schema) int {
	if aQualPri, bQualPri := GetQualityPriority(a.Quality), GetQualityPriority(b.Quality); aQualPri != bQualPri {
		return aQualPri - bQualPri
	}

	if aClassPri, bClassPri := GetClassPriority(a, s), GetClassPriority(b, s); aClassPri != bClassPri {
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

func DefindexSorter(a, b *tf2.Item, s *schema.Schema) int {
	if a.DefIndex != b.DefIndex {
		return int(a.DefIndex) - int(b.DefIndex)
	}

	if aQualPri, bQualPri := GetQualityPriority(a.Quality), GetQualityPriority(b.Quality); aQualPri != bQualPri {
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

func GetPurePriority(defIndex uint32, s *schema.Schema) int {
	switch s.NormalizeDefindex(int(defIndex)) {
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

func GetClassPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil || len(sch.UsedByClasses) == 0 {
		return 12
	}

	if len(sch.UsedByClasses) > 1 {
		return 10
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

func GetSlotPriority(item *tf2.Item, s *schema.Schema) int {
	sch := s.ItemByDef(int(item.DefIndex))
	if sch == nil {
		return 5
	}

	if sch.CraftClass != "weapon" && sch.ItemClass != "weapon" && !strings.HasPrefix(sch.ItemClass, "tf_weapon_") {
		return 5
	}

	cls := sch.ItemClass
	def := item.DefIndex

	switch {
	case def == 9 || def == 141 || def == 527 || def == 588 || def == 997 || def == 1153 ||
		strings.Contains(cls, "scattergun") || strings.Contains(cls, "rocketlauncher") ||
		strings.Contains(cls, "flamethrower") || strings.Contains(cls, "grenadelauncher") ||
		strings.Contains(cls, "minigun") || strings.Contains(cls, "syringegun") ||
		strings.Contains(cls, "sniperrifle") || strings.Contains(cls, "revolver") ||
		strings.Contains(cls, "crossbow") || strings.Contains(cls, "compound_bow") ||
		strings.Contains(cls, "particle_cannon") || strings.Contains(cls, "soda_popper") ||
		strings.Contains(cls, "handgun_scout_primary") || def == 1178:
		return 1

	case strings.Contains(cls, "pistol") || strings.Contains(cls, "pipebomblauncher") ||
		strings.Contains(cls, "smg") || strings.Contains(cls, "medigun") ||
		strings.Contains(cls, "buff_item") || strings.Contains(cls, "parachute") ||
		strings.Contains(cls, "lunchbox") || strings.Contains(cls, "jar") ||
		strings.Contains(cls, "laser_pointer") || strings.Contains(cls, "shotgun") ||
		strings.Contains(cls, "handgun_scout_secondary") || strings.Contains(cls, "raygun") ||
		def == 131 || def == 406 || def == 1101:
		return 2

	case strings.Contains(cls, "bat") || strings.Contains(cls, "shovel") ||
		strings.Contains(cls, "fireaxe") || strings.Contains(cls, "club") ||
		strings.Contains(cls, "bonesaw") || strings.Contains(cls, "fists") ||
		strings.Contains(cls, "wrench") || strings.Contains(cls, "knife") ||
		strings.Contains(cls, "sword") || strings.Contains(cls, "sledgehammer") ||
		strings.Contains(cls, "mechanical_arm") || strings.Contains(cls, "stick"):
		return 3

	case strings.Contains(cls, "pda") || strings.Contains(cls, "builder") || strings.Contains(cls, "spellbook"):
		return 4

	default:
		return 5
	}
}

func GetQualityPriority(quality uint32) int {
	if quality == schema.QualityUnique {
		return 1
	}

	return 2
}
