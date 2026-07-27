// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sku provides zero-allocation parsing, generation, and validation
// for Team Fortress 2 Stock Keeping Unit (SKU) strings.
//
// # SKU Specification & Attribute Ordering
//
// An SKU is a semicolon-delimited string representing an item's unique economic identity.
// The format adheres strictly to the backpack.tf and TF2Autobot standards:
//
//	Defindex;Quality[;Attributes...]
//
// Attributes are appended in a canonical order to ensure deterministic hashing and map lookups:
//
//  1. Effect          (;u<ID>)            Unusual particle effect ID (e.g., ;u13)
//  2. Australium      (;australium)       Flag for golden Australium variants
//  3. Craftable       (;uncraftable)      Flag present only if item CANNOT be crafted
//  4. Tradable        (;untradable)       Flag present only if item CANNOT be traded
//  5. Wear            (;w<1-5>)           Wear tier (1=FN, 2=MW, 3=FT, 4=WW, 5=BS)
//  6. Paintkit        (;pk<ID>)           War Paint / Skin texture definition ID
//  7. Quality2        (;strange)          Elevated Strange quality on non-Strange base items
//  8. Killstreak      (;kt-<1-3>)         Killstreak level (1=Basic, 2=Specialized, 3=Professional)
//  9. Target          (;td-<ID>)          Target defindex for Strangifiers/Kits/Unusualifiers
//  10. Festivized      (;festive)          Flag if Festivizer was applied
//  11. Craft Number    (;n<Number>)        Limited craft serial number (1-100)
//  12. Crate Series    (;c<Series>)        Supply crate / case series index
//  13. Output          (;od-<ID>)          Output item defindex for recipe inputs
//  14. Output Quality  (;oq-<ID>)          Output item quality for recipe inputs
//  15. Paint           (;p<Decimal>)       Applied paint color decimal representation
//  16. Spells          (;s-<Attr>-<Val>)   Halloween spell attribute and value pairs
//  17. Strange Parts   (;sp<ID>)           Applied Strange Part score tracker IDs
//  18. Seed            (;sd<Seed>)         Pattern seed for War Paints
//
// # Examples
//
//   - "5021;6"            -> Unique Mann Co. Supply Crate Key
//   - "5002;6"            -> Refined Metal
//   - "30000;5;u13;kt-3"  -> Unusual Hat with Effect 13 (Burning Flames) and Professional Killstreak
//
// # Performance & Memory Guarantees
//
// All parsing functions ([FromString], [ParseInto]) reuse thread-safe internal [sync.Pool]
// buffer instances to achieve zero heap allocations in hot trade execution paths.
package sku
