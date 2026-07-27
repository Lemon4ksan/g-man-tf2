// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package schema maintains and indexes Team Fortress 2 item definitions, qualities,
// attributes, and particle effects.
//
// # Magic Defindexes & Steam WebAPI Constants
//
// Team Fortress 2 relies on fixed definition indexes (`Defindex`) for base currency and tools:
//
//   - Defindex 5021: Mann Co. Supply Crate Key
//   - Defindex 5002: Refined Metal
//   - Defindex 5001: Reclaimed Metal
//   - Defindex 5000: Scrap Metal
//
// # Description Color Codes & Attributes
//
// When parsing raw Steam Community inventory descriptions (`CEconItem`), dynamic attributes
// lacking explicit numeric WebAPI tags are identified via Valve's standard hex color strings:
//
//   - Color "7ea9d1": Halloween Spells (e.g., "Spell: Spectral Spectrum")
//   - Color "756b5e": Strange Parts (e.g., "Strange Part: Kills While Low Health")
//
// # Virtual Proxy Defindexes
//
// To allow unified SKU generation across WebAPI inventory feeds and internal Game Coordinator
// memory maps, non-numeric WebAPI attributes are assigned virtual proxy defindexes:
//
//   - DefPartsProxy (10000): Base offset for Strange Part slots
//   - DefSpellProxy (11000): Base offset for Halloween Spell slots
//
// # Defindex Normalization
//
// Legacy or retired item defindexes (such as seasonal event keys like "Eerie Key" #5628)
// automatically normalize to canonical defindex equivalents (e.g., #5021) via [NormalizeDefindex].
package schema
