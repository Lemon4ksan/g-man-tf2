// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package backpack manages the Team Fortress 2 inventory state for both local and remote profiles.

The package addresses local inventory layout sorting, trade locking, and remote audit tasks.
It provides duplicate item history checking and anti-scam validation for trading partners.

Key Types:
  - [Backpack] represents the local bot inventory, synchronized with the Game Coordinator.
  - [Remote] represents an external player inventory, loaded lazily for scouting.
  - [DupeChecker] defines an interface to verify item histories and detect duplicates.

Basic Example:

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
		"github.com/lemon4ksan/g-man/pkg/steam"
	)

	func main() {
		ctx := context.Background()
		client := steam.NewClient(...)

		// Get the registered local backpack module
		bp := backpack.From(client)
		if bp == nil {
			return
		}

		// Retrieve stock counts or lock items during trades
		count := bp.GetStock("5021;6")
		fmt.Printf("Current keys stock: %d\n", count)
	}
*/
package backpack
