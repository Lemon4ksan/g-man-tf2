// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"slices"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Filter represents a function used to screen and match items in the backpack.
type Filter func(item *tf2.Item, s *schema.Schema) bool

// PageLayout defines the matching rules and filters applied to a specific backpack page.
type PageLayout struct {
	// Filters contains the selection functions applied to items on this page.
	Filters []Filter
}

// Layout defines the complete page-by-page organization of the backpack.
type Layout struct {
	// Pages maps page numbers to their respective layout rules.
	Pages map[int]PageLayout
}

// BySKU returns a [Filter] that checks if an item matches the specified target SKU.
func BySKU(targetSKU string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.GetSKU(s) == targetSKU
	}
}

// ByQuality returns a [Filter] that checks if an item matches the specified numeric quality ID.
func ByQuality(q uint32) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		return item.Quality == q
	}
}

// ByClass returns a [Filter] that checks if an item can be used by the specified character class.
func ByClass(class string) Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		sch := item.GetSchema(s)
		if sch == nil {
			return false
		}

		return slices.Contains(sch.UsedByClasses, class)
	}
}

// IsPure returns a [Filter] that checks if an item is a pure currency type (keys or metal).
func IsPure() Filter {
	return func(item *tf2.Item, s *schema.Schema) bool {
		d := s.NormalizeDefindex(int(item.DefIndex))
		return d == schema.DefKey || d == schema.DefRefined || d == schema.DefReclaimed || d == schema.DefScrap
	}
}
