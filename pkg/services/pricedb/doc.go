// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricedb implements a client for the PriceDB.io API to track TF2 item prices.
//
// The package combines [Client] for REST API queries (bulk fetches, price history) and
// [SocketManager] for real-time price updates via WebSockets.
//
// # Quick Start
//
// Subscribe to real-time price updates:
//
//	sm := pricedb.NewSocketManager("ws://ws.pricedb.io/", logger)
//	sm.OnPrice(func(p *pricedb.Price) {
//		// Update local valuation cache
//	})
//	go sm.Run(ctx)
package pricedb
