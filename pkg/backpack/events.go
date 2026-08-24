// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import "github.com/lemon4ksan/foundation/async/event"

// FullEvent is broadcast when the backpack inventory reaches or exceeds maximum slot capacity.
type FullEvent struct {
	event.BaseEvent
	Count int
	Max   int
}
