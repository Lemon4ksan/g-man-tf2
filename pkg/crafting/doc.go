// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package crafting provides automatic crafting, metal condensing, and coin-change operations for Team Fortress 2.

The package resolves issues related to inventory slot optimization and automatic trade change balance.
It handles smelting duplicate weapons, combining scrap, and splitting higher-grade metal into change.

Key Types:
  - [Manager] coordinates standard crafting recipes (blueprints) with the Game Coordinator.
  - [Automator] manages background maintenance of low-grade metal reserves and duplicate weapon smelting.
  - [MetalManager] handles greedy currency selection and coin-change splitting for trade offers.

Basic Example:

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	)

	func runCraft(ctx context.Context, mgr *crafting.Manager) {
		// Automatically combine 3 scrap metal into 1 reclaimed metal
		createdIDs, err := mgr.CombineMetal(ctx, crafting.DefIndexScrap)
		if err != nil {
			fmt.Printf("Crafting failed: %v\n", err)
			return
		}
		fmt.Printf("Successfully created metal IDs: %v\n", createdIDs)
	}
*/
package crafting
