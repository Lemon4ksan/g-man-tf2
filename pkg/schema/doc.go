// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package schema manages and resolves Team Fortress 2 item schemas and SKU conversions.
//
// The package uses [Manager] to coordinate background schema updates and caching, serving
// in-memory [Schema] lookups with O(1) time complexity.
//
// # Quick Start
//
// Retrieve the schema and look up an item by definition index:
//
//	s := schema.From(client).Get()
//	item := s.ItemByDef(5021) // Look up Mann Co. Supply Crate Key
package schema
