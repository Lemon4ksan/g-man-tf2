<div align="center">

# G-MAN TF2

### High-Performance Team Fortress 2 Domain & Economy Suite for Go

_"Professionals have standards."_

[![Go Version](https://img.shields.io/badge/go-1.27%2B-007d9c?logo=go&logoColor=white&style=flat-square)](https://go.dev/)
[![Go Reference](https://img.shields.io/badge/godoc-reference-007d9c?style=flat-square)](https://pkg.go.dev/github.com/lemon4ksan/g-man-tf2)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue?style=flat-square)](LICENSE)
[![Zero-Alloc SKU](https://img.shields.io/badge/memory-Zero--Alloc%20SKU-brightgreen?style=flat-square)](pkg/sku)
[![Linter](https://img.shields.io/badge/linter-golangci--lint-brightgreen?style=flat-square&logo=go)](https://github.com/golangci/golangci-lint)
[![Parity](https://img.shields.io/badge/parity-100%25%20Node.js%20Compliance-blueviolet?style=flat-square)](docs/PARITY.md)

**G-MAN TF2** is the production-grade Team Fortress 2 domain module and economy engine built for the [G-MAN](https://github.com/lemon4ksan/g-man) automation framework. It bridges Valve Game Coordinator (GC) protocols, live SOCache inventory synchronization, and lossless metal arithmetic into a decoupled, thread-safe Go architecture.

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md) • 📐 [Parity Specification](docs/PARITY.md)

</div>

```shell
go get github.com/lemon4ksan/g-man-tf2
```

## ⚡ Key Features

* **Game Coordinator & SOCache Engine (`pkg/tf2`):** Zero-allocation stream parsing of Valve GC Shared Object Cache updates (`CMsgSOCacheSubscribed`, `SO_UPDATE`, `SO_DESTROY`) with instant craft, smelt, and achievement dispatching.
* **Trie-Accelerated Schema Engine (`pkg/schema`):** Dual-index caching (Defindex + Normalized ASCII Trie) parsing official TF2 `items_game.txt` schemas with sub-millisecond lookups and ~10 MB heap footprint.
* **Lossless Currency & Scrap Arithmetic (`pkg/currency`):** Exact integer scrap arithmetic (`currency.Scrap`, `currency.Currency`) eliminating IEEE 754 floating-point rounding errors during multi-key trade valuations.
* **Automated Smelting & Change-Making (`pkg/crafting`):** Class-aware weapon pair combining (`CombineWeapons`), automated metal uncrafting/crafting, and instant change balancing during trade offer reviews.
* **Native Multi-Service Integrations (`pkg/services`):** High-throughput, typed clients built on [aoni](https://github.com/lemon4ksan/aoni) for backpack.tf, PriceDB (real-time WebSockets), Mannco.store, Crit.tf, Express-load, and Rep.tf.
* **Decoupled Onion Trade Middlewares (`pkg/trading`):** Composable trading pipeline components (`StockLimitMiddleware`, `PricerMiddleware`, `AutoCounterMiddleware`) for [g-man](https://github.com/lemon4ksan/g-man) offer evaluation.

## ⚔️ Go vs. Node.js: Why G-MAN TF2 Wins

Historically, Node.js libraries (`tf2autobot`, `tf2-schema`, `tf2-currencies`) have powered TF2 trade bots. In production at scale, JavaScript single-threading and V8 memory pressure create substantial operational bottlenecks:

| Dimension | 🤖 G-MAN TF2 (Go) | 📦 Node.js (`tf2autobot` / `tf2-schema`) | Why it matters |
| :--- | :--- | :--- | :--- |
| **Active Heap per Bot** | **~8 - 12 MB** | **~180 - 350 MB** | Run 20x to 40x more TF2 bots on a single entry-level VPS without OOM kills. |
| **Schema Initialization** | **<40 ms** (Trie + Flat Index) | **3.5 - 8.0 seconds** (V8 JSON tree) | Instant bot boot times and immediate recovery from network reconnects. |
| **Currency Math** | **Exact Integer Scrap (`int`)** | `float` + `bignumber.js` workarounds | Zero rounding bugs or floating point drift when balancing `0.11` scrap change. |
| **GC Latency** | **Sub-millisecond** | Up to 150ms V8 GC pauses | Eliminates offer processing lag during high-frequency trade spikes. |
| **Concurrency** | **Native CSP Goroutines** | Single-threaded Event Loop | Multiple accounts, price webhooks, and GC sessions run simultaneously without blocking. |

## 🚀 Quick Start

### 1. Initialize Steam Client with TF2 Plugins

```go
package main

import (
	"context"
	"os"

	"github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func main() {
	ctx := context.Background()
	store, _ := jsonfile.New("storage.json")
	logger := logkit.New(logkit.DefaultConfig(logkit.LevelInfo))

	// 1. Initialize core Steam Client with TF2 domain modules
	client, err := steam.NewClient(steam.DefaultConfig(),
		steam.WithLogger(logger),
		steam.WithStorage(store),
		gc.WithModule(),
		apps.WithModule(),
		tf2.WithModule(),
		schema.WithModule(schema.DefaultConfig()),
		backpack.WithModule(),
		web.WithModule(web.DefaultConfig()),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// 2. Access live backpack tracking synced over Game Coordinator
	bp := backpack.From(client)
	sub := client.Bus().Subscribe(&tf2.BackpackLoadedEvent{})
	go func() {
		for event := range sub.C() {
			if bpEvent, ok := event.(*tf2.BackpackLoadedEvent); ok {
				pure := bp.GetPureStock()
				logger.Info("TF2 Backpack synced!",
					logkit.Int("total_items", bpEvent.Count),
					logkit.Int("keys", pure.Keys),
					logkit.Float64("refined", pure.TotalRefined()),
				)
			}
		}
	}()

	if err := client.Run(); err != nil {
		panic(err)
	}

	// 3. Resolve CM and login
	dir := directory.New(client)
	server, _ := dir.GetOptimalCMServer(ctx)
	login := auth.NewLogOnDetails(os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS"))

	if err := client.ConnectAndLogin(ctx, server, login); err != nil {
		panic(err)
	}

	client.Wait()
}
```

### 2. Lossless Metal & Currency Equations

```go
package main

import (
	"fmt"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
)

func main() {
	// Parse float-based refined into exact integer scrap
	scrap := currency.ToScrap(45.33) // 408 Scrap

	// Convert back safely without precision loss
	ref := currency.ToRefined(scrap) // 45.33

	// Create multi-currency struct
	cur := currency.New(2, 45.33)
	fmt.Println(cur.String()) // "2 keys, 45.33 ref"
}
```

### 3. Register TF2 Onion-Trading Middlewares

```go
package main

import (
	"github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

func RegisterPipeline(
	tradeEngine *engine.Engine,
	bp *backpack.Backpack,
	priceMgr *pricedb.Manager,
	schemaMod *schema.Manager,
	logger logkit.Logger,
) {
	stockCfg := trading.StockConfig{
		MaxTotal:   3000,
		DefaultMax: 20,
		MaxPerSKU: map[string]int{
			"5021;6": 500, // Limit Mann Co. Supply Crate Keys to 500
		},
	}

	tradeEngine.Use(
		// 1. Enforce inventory maximums per SKU
		trading.StockLimitMiddleware(bp, stockCfg, logger),

		// 2. Validate trade prices against PriceDB real-time feeds
		trading.PricerMiddleware(priceMgr, schemaMod.Get, logger),
	)
}
```

## 📂 Package Architecture

```text
pkg/
├── tf2/              # TF2 Game Coordinator driver & live SOCache storage
├── backpack/         # In-memory inventory projections & item reservation locks
├── crafting/         # Automated weapon combining and metal smelting recipes
├── schema/           # items_game schema parser with Trie & Defindex indexing
├── sku/              # Zero-allocation canonical SKU parser & string formatter
├── currency/         # Exact integer scrap math & key-metal currency equations
├── services/         # Native HTTP/WebSocket clients powered by aoni
│   ├── pricedb/      # PriceDB real-time pricing client & WebSocket stream
│   ├── bptf/         # backpack.tf API client & listing manager
│   ├── crit/         # Crit.tf storefront listing synchronizer
│   ├── mannco/       # Mannco.store API & WebSocket market stream
│   ├── express/      # Express-load fast inventory service client
│   └── rep/          # Rep.tf trust, feedback, and scammer verification
├── trading/          # Onion-style trading middlewares for g-man trade engine
└── reason/           # Standardized TF2 trade rejection & review reason codes
```

## 📦 Ecosystem

* **[g-man](https://github.com/lemon4ksan/g-man)**: Core Steam client SDK and multi-game automation runtime.
* **[g-man-cli](https://github.com/lemon4ksan/g-man-cli)**: High-performance background daemon (`g-mand`) and TUI management CLI (`gmanctl`).
* **[aoni](https://github.com/lemon4ksan/aoni)**: High-performance network stack, HTTP/2, and WebSocket client engine.
* **[foundation](https://github.com/lemon4ksan/foundation)**: Zero-allocation concurrency, async logging (`logkit`), and data structures.

## ⚖️ Legal & License

**Disclaimer:** This software is **not** affiliated with, maintained by, or endorsed by **Valve Corporation** or any of its subsidiaries. Steam, Team Fortress 2, and all related Valve properties are registered trademarks of Valve Corporation.

This project is licensed under the **BSD 3-Clause License**. See [LICENSE](LICENSE) for full details.
