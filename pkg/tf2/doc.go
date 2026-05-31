// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package tf2 provides integration with the Team Fortress 2 Game Coordinator.

It handles sessions, manages the shared object cache, and exposes native actions.

Key Types:
  - [TF2] represents the central Game Coordinator client.
  - [SOCache] manages the real-time local inventory replica.
  - [Item] represents parsed item parameters and SKU metadata.

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

		// Retrieve the registered TF2 module
		t := tf2.From(client)
		if t == nil {
			return
		}

		// Retrieve items currently synchronized in the cache
		items := t.Cache().GetItems()
		fmt.Printf("Total items in cache: %d\n", len(items))
	}
*/
package tf2
