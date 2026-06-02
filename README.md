<div align="center">

# 🎒 G-MAN TF2

### The Ultimate Team Fortress 2 Domain Module & Economy Engine for G-MAN

[![Go Reference](https://img.shields.io/badge/go-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lemon4ksan/g-man-tf2)
[![Go Report Card](https://goreportcard.com/badge/github.com/lemon4ksan/g-man-tf2?style=flat-square)](https://goreportcard.com/report/github.com/lemon4ksan/g-man-tf2)
[![License](https://img.shields.io/github/license/lemon4ksan/g-man-tf2?style=flat-square)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/lemon4ksan/g-man-tf2?style=flat-square)](https://github.com/lemon4ksan/g-man-tf2/stargazers)

> _"Professionals have standards"_

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md)

</div>

**G-MAN TF2** is the official, production-grade Team Fortress 2 domain module and economy engine designed for the [G-MAN](https://github.com/lemon4ksan/g-man) automation framework. It bridges Valve's Game Coordinator (GC), real-time inventory caching, and complex TF2 trading math into a high-performance, decoupled Go package.

```shell
go get github.com/lemon4ksan/g-man-tf2@latest
```

## 📂 Project Directory Structure

```text
pkg/
├── tf2/              # Central TF2 GC Session Driver & SOCache Cache
│   ├── tf2.go        # Module implementation & options (RegisterModule)
│   ├── socache.go    # Live GC Shared Object parser & inventory keeper
│   └── actions.go    # Low-level GC commands (Crafting, Achievement Unlocking)
├── backpack/         # Unified in-memory inventory views & slot lock management
├── crafting/         # Automated crafting & weapon smelting engine recipes
├── schema/           # High-fidelity TF2 schema manager & items_game parser
├── sku/              # Standardized item SKU parsers (quality, effect, paint, etc.)
├── currency/         # Float-safe metal arithmetic & Key-to-Scrap equations
├── services/         # Third-party platform services integrations
│   ├── pricedb/      # Local pricing store adapters and PriceDB Socket.IO connection sync
│   ├── bptf/         # backpack.tf integrations (listing management, snap scraper)
│   ├── crit/         # Crit.tf storefront listing synchronizer
│   └── rep/          # Trust, feedback, and user reputation lookup utilities
├── behavior/         # High-level behavior loops
├── trading/          # Onion-style trading middlewares (pricer, limits, counters)
├── reason/           # TF2-specific trade rejection reasons
```

## 🚀 Quick Start

### 1. Install Dependencies
You need both the core G-MAN runtime client and the TF2 domain package:

```shell
go get github.com/lemon4ksan/g-man@latest
go get github.com/lemon4ksan/g-man-tf2@latest
```

### 2. Initialize the Orchestrator
Launch the client, register the TF2 schema and backpack managers, and load active trading middlewares:

```go
package main

import (
	"context"
	"os"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	
	// G-MAN TF2 Imports
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func main() {
	ctx := context.Background()
	store, _ := jsonfile.New("storage.json")
	logger := log.New(log.DefaultConfig(log.LevelInfo))

	// 1. Initialize Steam Client with modular G-MAN TF2 plugins
	client, err := steam.NewClient(steam.Config{Storage: store},
		steam.WithLogger(logger),
		tf2.WithModule(),
		schema.WithModule(schema.DefaultConfig()),
		backpack.WithModule(),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// 2. Fetch registered module references
	tf2Mod := tf2.From(client)
	bpMod := backpack.From(client)

	// 3. Listen for inventory updates synced via GC SOCache
	sub := client.Bus().Subscribe(&tf2.BackpackLoadedEvent{})
	go func() {
		for event := range sub.C() {
			if bpEvent, ok := event.(*tf2.BackpackLoadedEvent); ok {
				logger.Info("TF2 Inventory synchronized via SOCache!", 
					log.Int("items_count", bpEvent.Count),
				)
				
				pure := bpMod.GetPureStock()
				logger.Info("Current balances",
					log.Int("keys", pure.Keys),
					log.Float64("refined", pure.TotalRefined()),
				)
			}
		}
	}()

	// 4. Discover optimal connection server and login
	dir := directory.New(client.Service())
	server, _ := dir.GetOptimalCMServer(ctx)
	login := auth.NewLogOnDetails(os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS"))

	if err := client.Run(); err != nil {
		panic(err)
	}

	if err := client.ConnectAndLogin(ctx, server, login); err != nil {
		panic(err)
	}

	client.Wait()
}
```

### 3. Register TF2 Onion-Trading Middlewares
Add decoupled processing steps to build your custom business rule checks inside G-MAN's Trade Offer Engine:

```go
package main

import (
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

func RegisterPipeline(
	tradeEngine *engine.Engine,
	bp *backpack.Backpack,
	priceMgr *pricedb.Manager,
	logger log.Logger,
) {
	stockCfg := trading.StockConfig{
		MaxTotal:   3000,
		DefaultMax: 20,
		MaxPerSKU: map[string]int{
			"5021;6": 500, // Limit Mann Co. Supply Crate Keys to 500
		},
	}

	tradeEngine.Use(
		// 1. Stock checking middleware
		trading.StockLimitMiddleware(bp, stockCfg, logger),
		
		// 2. Price DB validation middleware
		trading.PricerMiddleware(priceMgr, logger),
	)
}
```

## ⚡ Memory & Performance Efficiency

G-MAN TF2 inherits G-MAN’s core focus on low-footprint systems, making it highly suitable for running dozens of concurrent accounts on a single cheap VPS:
* **Fidelity Schema Engine:** Prunes excess game tracker structures (especially in `LiteMode`), indexing item defindexes and schema attributes in a mere **~10 MB** of active heap memory.
* **SOCache Storage:** Employs zero-allocation pointer mappings to reflect inventories, keeping physical memory footprint at **~25 MB RSS** overall under production workloads.

## 🤝 Contributing

We welcome contributions to G-MAN TF2! If you're interested in refining metal combining formulas, improving the dynamic schema deserializer, or enhancing reputation lookup APIs:

1. Review [CONTRIBUTING.md](CONTRIBUTING.md) for conventions.
2. Verify changes with unit tests: `go test -race ./...`.
3. Open a Pull Request detailing the changes and your design logic.

## ☕ Support the Development

Testing Game Coordinator states, live trade offers, and smelting workflows requires active capital to cover Steam Market transaction fees, in-game item acquisitions, and test-transaction fees. If G-man helped you automate your trading workflows or optimized your server resources, feel free to show some support:

<div align="center">

[![Trade Offer](https://img.shields.io/badge/Steam-Trade_Offer-blue?style=for-the-badge&logo=steam)](https://steamcommunity.com/tradeoffer/new/?partner=1141078357&token=HjsTJQFX)

> _"Yeah, money well spent!"_

</div>

## ⚖️ Legal & License

**Disclaimer:** This software is **not** affiliated with, maintained by, or endorsed by **Valve Corporation** or any of its subsidiaries. Steam, Team Fortress 2, and all related Valve properties are registered trademarks of Valve Corporation. Use of this library is at your own risk.

This project is licensed under the **BSD 3-Clause License**. See [LICENSE](LICENSE) for full details.
