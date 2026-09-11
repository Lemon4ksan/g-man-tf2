// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import "strconv"

// ItemID represents a unique 64-bit Steam economy item/asset identifier.
type ItemID uint64

// Uint64 converts ItemID to primitive uint64.
func (id ItemID) Uint64() uint64 {
	return uint64(id)
}

// String returns the decimal string representation of the item identifier.
func (id ItemID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

// AssetID is a semantic alias for ItemID representing Steam economy assets.
type AssetID = ItemID

// Defindex represents a TF2 schema definition index (e.g. 5021 for Mann Co. Supply Crate Key).
type Defindex uint32

// Uint32 converts Defindex to primitive uint32.
func (d Defindex) Uint32() uint32 {
	return uint32(d)
}

// Int converts Defindex to int.
func (d Defindex) Int() int {
	return int(d)
}

// String returns the decimal representation of the definition index.
func (d Defindex) String() string {
	return strconv.FormatUint(uint64(d), 10)
}

// QualityID represents an item quality code (e.g. 6 for Unique, 11 for Strange, 5 for Unusual).
type QualityID uint32

// Uint32 returns the quality code as primitive uint32.
func (q QualityID) Uint32() uint32 {
	return uint32(q)
}
