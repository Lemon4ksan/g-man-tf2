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
