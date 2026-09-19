// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/foundation/async/event"
)

type ConnectedEvent struct {
	event.BaseEvent
	Version uint32
}

type DisconnectedEvent struct {
	event.BaseEvent
}

type BackpackLoadedEvent struct {
	event.BaseEvent
	Count int
}

type ItemAcquiredEvent struct {
	event.BaseEvent
	Item *Item
}

type ItemRemovedEvent struct {
	event.BaseEvent
	ItemID uint64
}

type ItemUpdatedEvent struct {
	event.BaseEvent
	Item *Item
}

type CraftResponseEvent struct {
	event.BaseEvent
	BlueprintID  uint16
	CreatedItems []uint64
}

type TradeRequestEvent struct {
	event.BaseEvent
	SteamID uint64
	TradeID uint32
}

type TradeResponseEvent struct {
	event.BaseEvent
	Response uint32
	TradeID  uint32
}

type CraftingCompleteEvent struct {
	event.BaseEvent
	RecipeID     int16
	ItemsCreated []uint64
}

type NotificationEvent struct {
	event.BaseEvent
	TitleLocalizationKey string
	BodyLocalizationKey  string
	ReplacementStrings   map[string]string
}

type ItemBroadcastEvent struct {
	event.BaseEvent
	UserName       string
	WasDestruction bool
	DefIndex       uint32
}

type BackpackSortFinishedEvent struct {
	event.BaseEvent
}
