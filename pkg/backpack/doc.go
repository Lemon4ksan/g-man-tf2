// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package backpack manages local inventory caching, item locking, stock calculation,
// and structural layout sorting for Team Fortress 2.
//
// # Grid Coordinates & Positioning
//
// TF2 inventories are organized into pages with fixed grid dimensions:
//   - ItemsPerPage: 50 items per page
//   - SlotsPerRow:  10 items per row
//
// Position formula used by Game Coordinator:
//
//	Position = (Page - 1) * 50 + Slot
//
// # Concurrent Item Locking
//
// To prevent race conditions where two simultaneous trade offers attempt to give away the same
// underlying Steam asset ID (double-spending), Backpack encapsulates an atomic key-lock mechanism.
// Items reserved for pending sent trade offers are locked via [Backpack.LockItems] and excluded
// from selection algorithms until confirmed accepted or explicitly released by background stale-lock
// cleanup routines.
package backpack
