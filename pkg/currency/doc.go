// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package currency handles Team Fortress 2 monetary calculations, parsing, and formatting.
//
// # Atomic Scrap Math
//
// Floating-point math (`float64`) inherently suffers from IEEE 754 precision loss during
// repetitive division and addition (e.g., `0.1 + 0.2 != 0.3`). In TF2 trading, key and metal
// calculations must be exact down to the individual Scrap Metal unit to prevent double-spending
// or loss of value during counter-offers.
//
// All currency operations within this package strictly use the [Scrap] type, an integer-backed
// atomic unit:
//
//	1 Refined Metal (Ref)   = 9 Scrap
//	1 Reclaimed Metal (Rec) = 3 Scrap
//	1 Scrap Metal (Scrap)   = 1 Scrap
package currency
