// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

// IsJunk returns true if the given [trading.Item] is a low-value commodity (such as standard supply crates).
// Returns true if the item is nil or has an empty SKU.
func IsJunk(it *trading.Item) bool {
	if it == nil || it.SKU == "" {
		return true
	}

	if HasSpells(it) {
		return false
	}

	for _, attr := range it.Attributes {
		if attr.Defindex == schema.AttrCrateSeries {
			return true
		}
	}

	return false
}

// HasSpells checks whether the [trading.Item] contains any active Halloween spells in its description or attributes.
// Returns false if the item is nil.
func HasSpells(it *trading.Item) bool {
	if it == nil {
		return false
	}

	for _, attr := range it.Attributes {
		if attr.Defindex >= 1004 && attr.Defindex <= 1009 {
			return true
		}
	}

	for _, desc := range it.Descriptions {
		if _, ok := schema.IdentifySpell(desc.Value); ok {
			return true
		}
	}

	return false
}
