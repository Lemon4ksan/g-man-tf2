# G-MAN TF2 Domain Engine Parity & Compliance

This document maps `g-man-tf2` packages, math specifications, and Game Coordinator drivers against standard Node.js libraries (`@tf2autobot/tf2`, `@tf2autobot/tf2-currencies`, `@tf2autobot/tf2-sku`, `@tf2autobot/tf2-schema`, `tf2autobot-pricedb/src/classes/TF2GC.ts`).

## 1. Component Mapping Matrix

| Domain | Node.js Reference | `g-man-tf2` Implementation (Go) | Parity Status |
| :--- | :--- | :--- | :--- |
| **GC Session & Handshake** | `@tf2autobot/tf2` | `pkg/tf2/tf2.go` | Full parity (`ClientHello` v65580, `Welcome`, `Goodbye`) |
| **SOCache Item Tracking** | `@tf2autobot/tf2` (SO events) | `pkg/tf2/socache.go` | Full parity (Live SOCache, attributes, killstreaks, paint) |
| **Item SKU Parser/Stringifier** | `@tf2autobot/tf2-sku` | `pkg/sku/sku.go` | 100% attribute & format parity (Canonical `;` delimited) |
| **Currencies & Metal Math** | `@tf2autobot/tf2-currencies` | `pkg/currency/currencies.go`, `parser.go` | Extended (Atomic integer `Scrap` avoids float rounding) |
| **Crafting & Change Smelting** | `TF2GC.ts` (crafting queue) | `pkg/crafting/crafting.go`, `metal.go`, `auto.go` | Full parity (Recipes 3, 4, 5, 22, 23, duplicate smelting) |
| **Backpack & Slot Locking** | `Inventory.ts` | `pkg/backpack/backpack.go`, `layout.go` | Extended (Thread-safe slot locks, dirty tracker) |
| **PriceDB Socket.IO Sync** | `IPricer.ts` | `pkg/services/pricedb` (`socket.go`, `manager.go`) | Full parity (Real-time price feed, key rate tracking) |

## 2. Game Coordinator & SOCache Engine (`pkg/tf2`)

### 2.1 Handshake Protocol
- `sendHello(ctx)`: Sends `k_EMsgGCClientHello` (EMsg 4006) with version `65580`.
- Handshake loop retries every 30 seconds until `k_EMsgGCClientWelcome` (EMsg 4004) transitions state to `Connected`.
- On `k_EMsgGCClientGoodbye` (EMsg 4008), the cache unloads, emitting `DisconnectedEvent`.

### 2.2 Shared Object Cache (SOCache)
- Handles `k_ESOMsg_CacheSubscribed` (EMsg 1070) by populating all item definitions from `CSOTFItem`.
- Subscribes to incremental mutations:
  - `k_ESOMsg_Create` (1071): Item dropped/acquired.
  - `k_ESOMsg_Update` (1072): Item modified (e.g. style change, killstreak count, paint).
  - `k_ESOMsg_Destroy` (1073): Item crafted, deleted, or traded away.
  - `k_ESOMsg_UpdateMultiple` (1076): Batch position reordering or bulk changes.

## 3. SKU Parsing & Formatting (`pkg/sku`)

`pkg/sku/sku.go` supports the full TF2 SKU specification compatible with `@tf2autobot/tf2-sku`:

| SKU Tag | Meaning | Go Struct Field |
| :--- | :--- | :--- |
| `{defindex};{quality}` | Base item identification | `Defindex`, `Quality` |
| `;u{effect}` | Unusual effect ID | `Effect` |
| `;australium` | Australium weapon flag | `Australium` (bool) |
| `;uncraftable` | Non-craftable modifier | `Craftable = false` |
| `;untradable` | Non-tradable modifier | `Tradable = false` |
| `;w{wear}` | Wear tier (1=Factory New, 5=Battle-Scarred) | `Wear` |
| `;pk{paintkit}` | War paint texture ID | `Paintkit` |
| `;strange` | Strange quality secondary attribute | `Quality2 = 11` |
| `;kt-{killstreak}` | Killstreak tier (1=Standard, 2=Spec, 3=Pro) | `Killstreak` |
| `;td-{target}` | Killstreak / fabricator target defindex | `Target` |
| `;festive` | Festivized item flag | `Festivized = true` |
| `;n{craftnumber}` | Low-craft number (e.g. #1-100) | `Craftnumber` |
| `;c{crateseries}` | Crate / case series number | `Crateseries` |
| `;od-{output}` | Chemistry set / fabricator output defindex | `Output` |
| `;oq-{quality}` | Fabricator output quality | `OutputQuality` |
| `;p{paint}` | Applied paint color defindex | `Paint` |
| `;s-{attr}-{val}` | Halloween spell attribute and value | `Spells []Spell` |
| `;sp{part}` | Strange part defindex | `Parts []int` |
| `;sd{seed}` | War paint pattern seed | `Seed` |

## 4. Currency Math & Precision Guarantees (`pkg/currency`)

### 4.1 Atomic Scrap Unit
To eliminate JavaScript floating point accumulation errors ($0.11 + 0.22 \ne 0.33$), `currency.Scrap` uses integer arithmetic where:
$$\text{ScrapInRec} = 3, \quad \text{ScrapInRef} = 9$$

Conversion functions:
```go
func ToScrap(refined float64) Scrap {
    return Scrap(math.Round(refined * 9.0))
}

func ToRefined(s Scrap) float64 {
    return float64(s) / 9.0
}
```

### 4.2 String Parsing & Currency Inputs
`currency.Parse(input)` tokenizes mixed currency strings without relying on regex:
- `"2 keys, 1.33 ref"` $\rightarrow$ `Keys: 2, Metal: 1.33`
- `"50 scrap"` $\rightarrow$ `Metal: 5.55` (50 / 9 ref)
- `"10k 50r"` $\rightarrow$ `Keys: 10, Metal: 50.0`

## 5. Crafting & Change Smelting Engine (`pkg/crafting`)

### 5.1 Recipe IDs
- `RecipeSmeltWeapons` (3): 2 craftable weapons of the same class $\rightarrow 1$ Scrap.
- `RecipeCombineScrap` (4): 3 Scrap $\rightarrow 1$ Reclaimed.
- `RecipeCombineReclaimed` (5): 3 Reclaimed $\rightarrow 1$ Refined.
- `RecipeSmeltReclaimed` (22): 1 Reclaimed $\rightarrow 3$ Scrap.
- `RecipeSmeltRefined` (23): 1 Refined $\rightarrow 3$ Reclaimed.

### 5.2 Automatic Metal & Weapon Balancing
- **Change Generation (`MakeChange`):** If a trade requires more Scrap metal than is currently available, `MetalManager` recursively breaks Refined $\rightarrow$ Reclaimed $\rightarrow$ Scrap.
- **Weapon Fallback Smelting (`SmeltDuplicates`):** If metal change is insufficient, `SmeltDuplicates` scans inventory for duplicate weapons within each class (`schema.Classes`), verifies they are tradable, and smelts them in pairs into scrap to satisfy the exact change amount.
- **Pure Liquidator Automator (`auto.go`):** Background worker that maintains configured thresholds (`minScrap`, `minRec`, `maxScrap`, `maxRec`) and condenses surplus metal into refined.
