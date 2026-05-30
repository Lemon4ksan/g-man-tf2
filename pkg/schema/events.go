// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
)

// ReadyEvent is emitted when the schema has loaded and is ready for use.
type ReadyEvent struct {
	bus.BaseEvent
}

// UpdatedEvent is emitted when the schema has successfully updated.
type UpdatedEvent struct {
	bus.BaseEvent
	// Timestamp represents the time when the schema was updated.
	Timestamp time.Time
}

// UpdateFailedEvent is emitted when a background schema update fails.
type UpdateFailedEvent struct {
	bus.BaseEvent
	// Error contains the failure reason.
	Error error
}

// UpdateRequestedEvent is emitted when a schema update is requested.
type UpdateRequestedEvent struct {
	bus.BaseEvent
	// Version represents the requested schema version.
	Version uint32
	// ItemsGameURL represents the URL of the items_game.txt file.
	ItemsGameURL string
}
