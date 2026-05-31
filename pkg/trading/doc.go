// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package trading defines the Team Fortress 2 automated trading, valuation, and security logic.

The package coordinates stock boundaries, price checks, escrow verification, and smart change calculations.
It extends the generic trading handler and translates business decisions into automated trade counter-offers.

Key Types:
  - [ConfigManager] handles strategy boundaries, trading limits, and dynamic hot-reloads.
  - [FIFOSubscriber] monitors completed trades to log purchase cost basis records.
  - [TF2TradeTester] provides an isolated mock context for verifying middleware chains.

Basic Example:

	package main

	import (
		"context"
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/trading"
	)

	func main() {
		// Load trade configuration boundaries
		cm, err := trading.NewConfigManager("config/trading.json")
		if err != nil {
			return
		}

		cfg := cm.GetConfig()
		fmt.Printf("Global inventory limit: %d slots\n", cfg.GlobalMaxStock)
	}
*/
package trading
