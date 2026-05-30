// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package tf2 implements the Team Fortress 2 Game Coordinator communication and inventory cache.

The package handles connection state handshakes, in-game actions, and real-time inventory updates.
It parses raw shared object notifications into structured, queryable data models.

Key Types:
  - [TF2] manages the Game Coordinator session connection state and action commands.
  - [SOCache] maintains the local inventory replica updated by shared object packets.
  - [Item] represents an inventory item parsed with all standard economic attributes.

Basic Example:

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
		"github.com/lemon4ksan/g-man/pkg/steam"
	)

	func main() {
		ctx := context.Background()
		client := steam.NewClient(...)

		// Get the registered TF2 module
		tf2Module := tf2.From(client)
		if tf2Module == nil {
			return
		}

		// Access the local synchronized inventory cache
		cache := tf2Module.Cache()
		fmt.Printf("Total backpack items: %d\n", len(cache.GetItems()))
	}
*/
package tf2
