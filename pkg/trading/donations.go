// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

// IsJunk returns true if the item is considered "junk" in TF2.
// Junk items include crates/cases (unless they are rare/special)
// and items without a valid SKU.
func IsJunk(it *trading.Item) bool {
	if it.SKU == "" {
		return true
	}

	// If it has spells, it's definitely not junk
	if HasSpells(it) {
		return false
	}

	// Check for crates/cases
	for _, attr := range it.Attributes {
		if attr.Defindex == schema.AttrCrateSeries {
			return true
		}
	}

	return false
}

// HasSpells checks if an item has any Halloween spells attached.
func HasSpells(it *trading.Item) bool {
	// Check attributes for known Steam spell IDs (1004-1009)
	for _, attr := range it.Attributes {
		if attr.Defindex >= 1004 && attr.Defindex <= 1009 {
			return true
		}
	}

	// Fallback to description parsing (Steam API often lacks attribute data)
	for _, desc := range it.Descriptions {
		if _, ok := schema.IdentifySpell(desc.Value); ok {
			return true
		}
	}

	return false
}
