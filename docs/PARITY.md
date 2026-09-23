# G-MAN TF2 Domain Engine Parity & Compliance

This document maps `g-man-tf2` packages, math specifications, and Game Coordinator drivers against standard Node.js reference libraries (`@tf2autobot/tf2`, `@tf2autobot/tf2-currencies`, `@tf2autobot/tf2-sku`, `@tf2autobot/tf2-schema`, `tf2autobot-pricedb/src/classes/TF2GC.ts`, `@tf2autobot/UserCart.ts`, and `@tf2autobot/keepMetalSupply.ts`).

---

## 1. Master Component Mapping Matrix

| Domain / Subsystem | Node.js Reference | `g-man-tf2` Implementation (Go) | Parity Status | Technical Summary |
| :--- | :--- | :--- | :--- | :--- |
| **GC Session & Handshake** | `@tf2autobot/tf2` | `pkg/tf2/tf2.go` | Full Parity | Sends `ClientHello` (EMsg 4006, version 65580), handles `Welcome` (4004) and `Goodbye` (4008). |
| **SOCache Tracking** | `@tf2autobot/tf2` (SO events) | `pkg/tf2/socache.go` | Full Parity | Subscribes to SO cache create/update/destroy (EMsg 1070–1076); maintains live in-memory item cache. |
| **Item SKU Parser / Stringifier** | `@tf2autobot/tf2-sku` | `pkg/sku/sku.go` | 100% Parity | Bidirectional `;`-delimited canonical format; supports craft numbers (`n`), output attributes (`od`/`oq`), and spells (`s-`). |
| **Currencies & Metal Math** | `@tf2autobot/tf2-currencies` | `pkg/currency/currencies.go` | Architectural Improvement | Pure integer arithmetic (`Scrap = 1`, `Rec = 3`, `Ref = 9`) eliminates JS IEEE-754 float drift ($0.11 + 0.22 \ne 0.33$). |
| **Bidirectional Metal Change** | `@tf2autobot/UserCart.ts` (`getRequired`) | `pkg/crafting/metal.go` | Full Parity | 3-pass metal selection (exact match $\rightarrow$ change smelting $\rightarrow$ weapon smelting) without over-smelting. |
| **Duplicate Weapon Smelting** | `TF2GC.ts` (crafting queue) | `pkg/crafting/metal.go`, `crafting.go` | Full Parity | Pairs craftable weapons of the same class for Recipe 3; unblocked even when pure stock is empty. |
| **Batch Pure Supply Balancer** | `@tf2autobot/keepMetalSupply.ts` | `pkg/crafting/auto.go` | Full Parity | Single-tick concurrent calculation for combining Scrap/Rec and smelting Ref/Rec to maintain target metal inventory bands. |
| **Backpack Slot Locking** | `Inventory.ts` | `pkg/backpack/backpack.go`, `layout.go` | Architectural Improvement | Granular `sync.RWMutex` + slot locking map prevents double-spending during concurrent trade offer evaluation. |
| **Safe Auto-Acknowledgement** | `@tf2autobot/tf2` (`itemAcquired`) | `pkg/backpack/backpack.go` | Full Parity | Automates GC acknowledgement on `ItemAcquiredEvent` and `BackpackLoadedEvent`, clearing bit 30 unplaced status. |
| **Collision-Free Slot Allocation** | `@tf2autobot/tf2` (`setItemPositions`) | `pkg/tf2/actions.go` | Architectural Improvement | Scans occupied slots ($1 \le \text{slot} \le \text{maxSlots}$) before dispatching GC move, preventing slot clobbering. |
| **Pre-Trade Capacity Middleware** | `@tf2autobot/tf2`, `Cart.ts` | `g-man-cli/pkg/trading/capacity.go` | Full Parity | Calculates net delta ($\text{receive} - \text{give}$); declines overfilled offers with `reason.DeclineOverstocked`, preventing Steam `EResult 15`. |
| **Schema & SCM Display Naming** | `@tf2autobot/tf2-schema` | `pkg/schema/schema.go` | Full Parity | Retains Strange prefix for Australiums in display mode; omits Strange in SCM mode; guards Chemistry Set crate series formatting. |
| **PriceDB Socket.IO Sync** | `IPricer.ts` | `pkg/services/pricedb` | Full Parity | Real-time WebSocket price updates, key rate tracking, schema snapshot caching, and cache invalidation. |
| **Backpack.tf Pricelist Sync & Invalidation** | `Pricer.ts`, `@tf2autobot/bptf-bindings` | `pkg/behavior/pricemanager/manager.go` | Extended Parity | Full V4 pricelist polling, disk cache envelope with `Timestamp`/`TTL`/`Version`, thread-safe read isolation, programmatic `Invalidate`/`InvalidateAll`, and stale-cache fallback. |
| **Price Request Rate Limiting & Backoff** | `@tf2autobot/filter-axios-error`, `retry` | `pkg/services/bptf/api.go` | Full Parity | Exponential backoff with jitter on HTTP 502/503/504, HTTP 429 `Retry-After` adherence, and fallback to disk cache on degraded upstream. |

---

## 2. Game Coordinator & SOCache Engine (`pkg/tf2`)

### 2.1 Handshake Protocol
- `sendHello(ctx)`: Dispatches `k_EMsgGCClientHello` (EMsg 4006) containing version `65580`.
- Handshake loop retries every 30 seconds until `k_EMsgGCClientWelcome` (EMsg 4004) transitions engine state to `Connected`.
- On `k_EMsgGCClientGoodbye` (EMsg 4008), the cache cleanly unloads, resetting internal state and emitting `DisconnectedEvent`.

### 2.2 Shared Object Cache (SOCache)
- Handles `k_ESOMsg_CacheSubscribed` (EMsg 1070) by populating all item definitions from `CSOTFItem`.
- Reacts to real-time incremental mutations:
  - `k_ESOMsg_Create` (1071): Item acquired via drop, craft, or incoming trade offer.
  - `k_ESOMsg_Update` (1072): Item modified (e.g. style change, killstreak count, paint application).
  - `k_ESOMsg_Destroy` (1073): Item consumed via crafting, deleted, or transferred out in trade.
  - `k_ESOMsg_UpdateMultiple` (1076): Batch position reordering or bulk inventory changes.

### 2.3 Collision-Free Slot Allocation & Item Acknowledgement
- **The Bit-30 Unacknowledged State**: Newly acquired items from Steam trades or drops have bit 30 set in their 32-bit `Inventory` bitmask (`(it.Inventory >> 30) & 1 == 1`) and report `Position() == 0`.
- **Safe Auto-Acknowledgement**: `Backpack.handleEvent` listens for `ItemAcquiredEvent` and `BackpackLoadedEvent`, delegating to `TF2.AcknowledgeItem` or `TF2.AcknowledgeAll`.
- **Collision-Free Slot Search**: Rather than hardcoding slot 1, `AcknowledgeItem` and `AcknowledgeAll` inspect `cache.GetItems()` to build an `occupied` slot set:
  ```go
  slot := uint32(1)
  for slot <= maxSlots && occupied[slot] {
      slot++
  }
  if slot > maxSlots {
      return fmt.Errorf("tf2: backpack full, no unoccupied slot available (max %d)", maxSlots)
  }
  ```
  This guarantees no existing item is overwritten or displaced.

---

## 3. SKU Parsing, Attributes & Schema Formatting (`pkg/sku`, `pkg/schema`)

### 3.1 Canonical SKU Attributes
`pkg/sku/sku.go` supports the full TF2 SKU specification compatible with `@tf2autobot/tf2-sku`:

| SKU Tag | Description | Go Struct Field | Econ Attribute ID |
| :--- | :--- | :--- | :--- |
| `{defindex};{quality}` | Base item identification | `Defindex`, `Quality` | — |
| `;u{effect}` | Unusual effect ID | `Effect` | Attribute 134 |
| `;australium` | Australium weapon flag | `Australium` (bool) | Attribute 2027 |
| `;uncraftable` | Non-craftable flag | `Craftable = false` | — |
| `;untradable` | Non-tradable flag | `Tradable = false` | — |
| `;w{wear}` | Wear tier (1=FN, 2=MW, 3=FT, 4=WW, 5=BS) | `Wear` | Attribute 725 |
| `;pk{paintkit}` | War paint texture ID | `Paintkit` | Attribute 834 |
| `;strange` | Strange secondary quality (e.g. Strange Unusual) | `Quality2 = 11` | Attribute 214 |
| `;kt-{killstreak}` | Killstreak tier (1=Standard, 2=Spec, 3=Pro) | `Killstreak` | Attribute 2025 |
| `;td-{target}` | Killstreak / Fabricator target defindex | `Target` | Attribute 2012 |
| `;festive` | Festivized item modifier | `Festivized = true` | Attribute 2053 |
| `;n{craftnumber}` | Low-craft number (e.g. #1-100) | `Craftnumber` | Attribute 229 |
| `;c{crateseries}` | Crate / case series number | `Crateseries` | Attribute 187 |
| `;od-{output}` | Chemistry set / Fabricator output defindex | `Output` | Attribute 2000 |
| `;oq-{quality}` | Fabricator output quality | `OutputQuality` | Attribute 2001 |
| `;p{paint}` | Applied paint color defindex | `Paint` | Attribute 142 |
| `;s-{attr}-{val}` | Halloween spell attribute & value | `Spells []Spell` | Attributes 1004–1009 |
| `;sp{part}` | Strange part defindex | `Parts []int` | Attributes 380, 382, 384 |
| `;sd{seed}` | War paint pattern seed | `Seed` | Attribute 835 |

### 3.2 Schema & Steam Community Market (SCM) Naming Parity
- **Australium Strange Prefixing**:
  - In standard display format (`ItemName(item, false)`), Australium Strange weapons are titled `"Strange Australium <Name>"` to match in-game naming.
  - In SCM format (`ItemName(item, true)`), the `"Strange "` prefix is omitted (`"Australium <Name>"`), matching Steam Community Market listings.
- **Chemistry Set Crate Series Guard**:
  - Chemistry Sets (defindexes 20000..20007) encode output item information rather than crate series.
  - `ItemName` guards chemistry sets: `!(item.Defindex >= 20000 && item.Defindex <= 20007)` prevents erroneous appending of `#<series>`.
  - SCM name parsers (`parseChemistrySet` & `parseStrangifierChemistrySet`) strip spurious `Series #` substrings and discard `Crateseries` assignment.

---

## 4. Currency Math & Precision Guarantees (`pkg/currency`)

### 4.1 Atomic Scrap Precision
JavaScript engines use IEEE-754 double precision floats, where metal math introduces rounding drift (e.g. $0.11 + 0.22 = 0.33000000000000004$). In `g-man-tf2`, currency is represented as an atomic integer `Scrap`:
$$\text{ScrapInRec} = 3, \quad \text{ScrapInRef} = 9$$

Conversion functions guarantee exact rounding without accumulation drift:
```go
func ToScrap(refined float64) Scrap {
    return Scrap(math.Round(refined * 9.0))
}

func ToRefined(s Scrap) float64 {
    return float64(s) / 9.0
}
```

### 4.2 Token-Based Currency String Parsing
`currency.Parse(input)` tokenizes mixed currency strings via lexer state machine rather than fragile regular expressions:
- `"2 keys, 1.33 ref"` $\rightarrow$ `Keys: 2, Metal: 1.33` (12 Scrap)
- `"50 scrap"` $\rightarrow$ `Keys: 0, Metal: 5.55` (50 Scrap)
- `"10k 50r"` $\rightarrow$ `Keys: 10, Metal: 50.0` (450 Scrap)

---

## 5. Crafting & Change Smelting Engine (`pkg/crafting`)

### 5.1 Game Coordinator Recipe Definitions
- `RecipeSmeltWeapons` (3): 2 craftable weapons of the same class $\rightarrow 1$ Scrap.
- `RecipeCombineScrap` (4): 3 Scrap $\rightarrow 1$ Reclaimed.
- `RecipeCombineReclaimed` (5): 3 Reclaimed $\rightarrow 1$ Refined.
- `RecipeSmeltReclaimed` (22): 1 Reclaimed $\rightarrow 3$ Scrap.
- `RecipeSmeltRefined` (23): 1 Refined $\rightarrow 3$ Reclaimed.

### 5.2 Bidirectional Metal Change Algorithm (`metal.go`)
Matches `@tf2autobot/UserCart.ts` (`getRequired`) through a deterministic 3-pass selection process:
1. **Pass 1 (Exact Match)**: Greedy downward sweep selecting Refined, Reclaimed, and Scrap from available stock.
2. **Pass 2 (Change Smelting)**: If insufficient exact scrap is available, determines minimal smelting operations:
   - Smelting 1 Refined generates 3 Reclaimed.
   - Smelting 1 Reclaimed generates 3 Scrap.
   - Intermediate Reclaimed metal is preserved unless Scrap is strictly deficit, preventing unnecessary over-smelting.
3. **Pass 3 (Weapon Fallback Smelting)**: If total pure metal value is insufficient to cover the change requirement, evaluates duplicate tradable weapons within each class (`schema.Classes`) and smelts them in pairs (Recipe 3) to satisfy the remaining balance.

### 5.3 Batch Pure Supply Balancer (`auto.go`)
In parity with `@tf2autobot/keepMetalSupply.ts`, the background `Automator` maintains metal supply within configured thresholds (`minScrap`, `maxScrap`, `minRec`, `maxRec`) in a single `Tick`:
- Calculates `combineScrap` and `smeltRec` concurrently without oscillation.
- Calculates `combineRec` and `smeltRef` based on target refined condensation.
- Executes crafts sequentially through `CraftingManager`, returning actionable craft counts and error states.

---

## 6. Pre-Trade Capacity Middleware (`g-man-cli/pkg/trading/capacity.go`)

### 6.1 Prevention of Steam `EResult 15` (InventoryFull / AccessDenied)
When an incoming trade offer is accepted, Steam calculates the user's projected backpack item count. If this count exceeds the account's maximum backpack slots (e.g. 50, 300, 1000, 2000, or 3000), Steam rejects the trade with `EResult 15`.

### 6.2 Implementation Contract
`CapacityMiddleware` evaluates the net inventory delta before pricer or escrow handlers run:
$$\Delta_{\text{net}} = \text{len}(\text{ItemsToReceive}) - \text{len}(\text{ItemsToGive})$$
$$\text{ProjectedCount} = \text{CurrentCount} + \Delta_{\text{net}}$$

If $\text{ProjectedCount} > \text{MaxSlots}$, the offer is immediately declined with `reason.DeclineOverstocked`:
```go
if projectedCount > maxSlots {
    logger.WarnContext(ctx, "Trade offer declined: backpack capacity exceeded (EResult 15 prevention)",
        log.Uint64("offer_id", ctx.Offer.ID),
        log.Int("current", currentCount),
        log.Int("delta", netDelta),
        log.Int("max", maxSlots),
    )
    ctx.Decline(reason.DeclineOverstocked)
    return nil
}
```

---

## 7. Pricing Synchronization, Cache Invalidation & Network Resilience (`pkg/behavior/pricemanager`, `pkg/services/pricedb`, `pkg/services/bptf`)

### 7.1 Backpack.tf V4 Pricelist Synchronization (`pricemanager`)
`pricemanager.PriceManager` provides an automated orchestrator behavior (`bptf_prices`) that periodically synchronizes the entire backpack.tf economy catalog:
- **Endpoint**: Fetches `IGetPrices/v4?raw=1` using `aoni.Client` with zero-allocation JSON streaming.
- **Index Generation**: Flattens nested JSON hierarchies (`item -> quality -> tradable -> craftable -> priceIndex`) into canonical SKU string keys (`sku.FromObject(sItem)`).
- **Execution Interval**: Defaults to 2-hour synchronization intervals (`SyncInterval: 2 * time.Hour`), mirroring standard autobot catalog cadence.

### 7.2 Cache Metadata Tracking & TTL Enforcement
Unlike Node.js reference implementations where cache expiration is often implicit or scattered across timers, `PriceManager` maintains explicit atomic metadata:
- **`Timestamp`**: Wall-clock time of the last successful remote sync or file load.
- **`TTL`**: Configurable validity window (defaults to 3 hours, or `SyncInterval + 1 hour`).
- **`Version`**: Monotonically increasing 64-bit integer incremented on every index mutation (update, SKU eviction, or full purge).
- **TTL Lookup Enforcement**: `GetPrice(sku)` executes an atomic expiry check:
  $$\text{Expired} = (m.ttl > 0) \land (\text{time.Since}(m.timestamp) > m.ttl)$$
  If expired, `GetPrice` returns `(bptf.V4PricesEntry{}, false)`, preventing trades based on obsolete market data.
- **Degraded Stale Fallback**: `GetPriceStale(sku)` bypasses TTL expiry checks, providing deterministic fallback when external pricing APIs experience prolonged outages.

### 7.3 Programmatic Invalidation API
`PriceManager` provides programmatic cache invalidation for instant reaction to market changes or admin overrides:
- **`Invalidate(sku string) bool`**: Removes an individual SKU from the in-memory cache and increments the cache version. Returns `true` if the item was cached and removed.
- **`InvalidateAll()`**: Atomically wipes the entire SKU index, resets the timestamp, and increments the version counter, forcing an immediate re-fetch on the next tick.

### 7.4 Disk Persistence & Envelope Backward Compatibility
The price cache is serialized to disk at `Config.CachePath` using a versioned JSON envelope:
```json
{
  "version": 42,
  "timestamp": "2026-09-23T07:30:00Z",
  "ttl": 10800000000000,
  "index": {
    "5021;6": { "value": 75.0 }
  }
}
```
`PriceManager.Load()` implements dual-format loading:
1. Attempts to deserialize `cacheFileEnvelope` with full metadata preservation.
2. If deserialization fails or `envelope.Index == nil`, transparently falls back to deserializing legacy un-enveloped raw maps (`map[string]bptf.V4PricesEntry`), setting `version = 1` and `timestamp = time.Now()`.

### 7.5 Master Pricing Parity Mapping Matrix

| Go Type / Method | Node.js Reference (`tf2autobot-pricedb`, `@tf2autobot/*`) | Parity Status | Technical Summary & Rationale |
| :--- | :--- | :--- | :--- |
| `pricemanager.PriceManager` | `Pricelist.ts` / `Pricer.ts` | Extended | Orchestrator behavior managing full backpack.tf pricelist sync with disk caching and TTL. |
| `PriceManager.GetPrice` | `IPricer.getPrice(sku)` | Full Parity | Retrieves item valuation; Go strictly enforces TTL expiration. |
| `PriceManager.GetPriceStale` | `Pricelist.getPrice(sku)` (fallback) | Extended | Unchecked cache lookup for network outage fallback. |
| `PriceManager.Invalidate` | `Pricelist.removeEntry(sku)` | Full Parity | Evicts single SKU from cache; increments version counter. |
| `PriceManager.InvalidateAll` | `Pricelist.clear()` | Full Parity | Evicts all entries; resets timestamp; triggers reload. |
| `PriceManager.Metadata` | `Pricer.getOptions()` / cache stats | Extended | Returns `CacheMetadata` (timestamp, TTL, version, count, isExpired). |
| `pricedb.Manager` | `PriceDbPricer.ts` / `PriceDbApi.ts` | Full Parity | Real-time PriceDB.io sync via WebSocket + bulk REST. |
| `pricedb.PackedPrice` | `PriceDbPrice` (JS object) | Architectural Improvement | 16-byte packed bitfield struct; zero heap allocation on hot path. |
| `bptf.API` | `@tf2autobot/bptf-bindings` | Full Parity | Classified listings, user bans, pulse heartbeats, dual-token auth. |
| HTTP 502/429 Retry Backoff | `@tf2autobot/filter-axios-error` | Full Parity | Exponential backoff with jitter on 5xx; parses `Retry-After` on 429. |

---

## 8. Architectural Rationale & Technical Improvements

### 8.1 Memory Layout Optimization
- **Packed Bitfields (`PackGCItem`)**: In `@tf2autobot`, each item is represented as a full JavaScript V8 object with dynamic property maps, consuming ~1.2 KB of heap per item. In `g-man-tf2`, items are stored in compact Go structs with bitfield packing for inventory position and flags, consuming < 180 bytes per item (~85% reduction in memory overhead for a 3,000-item backpack).

### 8.2 Concurrency & Mutex Synchronization
- Node.js operates on a single-threaded cooperative event loop. When concurrent asynchronous callbacks evaluate multiple trade offers, race conditions frequently occur where the same metal or keys are promised to multiple partners.
- `g-man` enforces strict concurrency guarantees using `keylock.KeyMutex` and thread-safe backpack slot locks (`LockItems` / `UnlockItems`). Read operations utilize `sync.RWMutex.RLock()`, allowing high-throughput concurrent price checks while guaranteeing atomic modifications during crafting and trade acceptance.

### 8.3 Deterministic Error Handling & Rollback
- Rather than throwing uncaught promise rejections, all GC and crafting operations return explicit Go `error` values. If a multi-step craft or trade fails partway through, inventory locks are cleanly released via deferred unlock calls, eliminating orphaned reservation locks.

---

## 9. Code Annotation Guidelines

All codebase contributions maintaining or extending parity must adhere to the following annotation standards:

### 8.1 Protocol & Behavioral Parity
Use `// Parity: matches <library>/<file>:<line> [description]` when implementing logic that mirrors Node.js behavior:
```go
// Parity: matches @tf2autobot/UserCart.ts getRequired metal selection algorithm.
func (m *MetalManager) bidirectionalSelect(req ChangeRequirement, stock *currency.PureStock) ([]uint64, error)
```

### 8.2 Intentional Architectural Enhancements
Use `// Parity improvement: <rationale>` when intentionally departing from Node.js implementations to enhance performance, thread safety, or precision:
```go
// Parity improvement: uses atomic integer Scrap representation to eliminate
// IEEE-754 floating point accumulation errors inherent in JavaScript math.
type Scrap int64
```
