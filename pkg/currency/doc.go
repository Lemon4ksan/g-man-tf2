// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package currency provides high-performance structures and utilities for Team Fortress 2 currency math.

The package models the primary trading commodities (Keys and Metal) and performs zero-allocation
and overflow-safe conversions between refined metal values and absolute Scrap units.

Key Types:
  - [Scrap] represents the base atomic unit of TF2 currency (1 Scrap = 1/9 Refined).
  - [Currency] represents a combined balance of keys and metal.
  - [ValueDiff] calculates trade value disparities and missing currencies for counter-offers.
  - [PureStock] represents a physical inventory count of keys and metals.

Basic Example:

	package main

	import (
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	)

	func main() {
		// Parse standard price string into a Currency structure
		price, err := currency.Parse("2 keys, 15.33 ref")
		if err != nil {
			return
		}

		// Convert combined balance into base Scrap units using current key rate
		totalScrap, err := price.ToValue(50.33)
		if err != nil {
			return
		}

		fmt.Printf("Total value in scrap units: %d\n", totalScrap)
	}
*/
package currency
