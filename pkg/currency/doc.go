// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package currency performs Team Fortress 2 currency parsing, comparison, and math.
//
// The package supports zero-allocation conversions between refined metal and atomic [Scrap] units
// (1 Scrap = 1/9 Refined) using the [Currency] representation.
//
// # Quick Start
//
// Parse a price string and convert it to its total value in scrap:
//
//	price, _ := currency.Parse("2 keys, 15.33 ref")
//	totalScrap, _ := price.ToValue(50.33) // Evaluates using key rate in refined
package currency
