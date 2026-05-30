// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import "github.com/lemon4ksan/g-man/pkg/bus"

// FullEvent is published when the backpack storage reaches its maximum capacity.
type FullEvent struct {
	bus.BaseEvent
	// Count represents the current number of items in the backpack.
	Count int
	// Max represents the maximum available slots in the backpack.
	Max int
}
