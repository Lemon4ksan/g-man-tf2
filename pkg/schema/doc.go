// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package schema provides a manager for the Team Fortress 2 item schema.

The package handles item definitions, SKU translations, and item name parsing. It is designed
to maintain functional parity with standard Node.js TF2 schema implementations.

Key Types:
  - [Manager] coordinates background schema updates and caching.
  - [Schema] provides O(1) in-memory indices for item lookups and SKU conversions.
  - [Item] represents an individual item definition containing attributes and capabilities.

Basic Example:

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/schema"
		"github.com/lemon4ksan/g-man/pkg/steam"
	)

	func main() {
		ctx := context.Background()
		client := steam.NewClient(...)

		// Retrieve the schema manager module
		m := schema.From(client)
		if m == nil {
			return
		}

		s := m.Get()
		if s == nil {
			fmt.Println("Schema is not ready yet")
			return
		}

		// Look up an item by its definition index
		item := s.ItemByDef(5021)
		if item != nil {
			fmt.Printf("Item name: %s\n", item.ItemName)
		}
	}
*/
package schema
