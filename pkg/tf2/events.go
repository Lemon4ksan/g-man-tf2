// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/miyako/bus"
)

type ConnectedEvent struct {
	bus.BaseEvent
	Version uint32
}

type DisconnectedEvent struct {
	bus.BaseEvent
}

type BackpackLoadedEvent struct {
	bus.BaseEvent
	Count int
}

type ItemAcquiredEvent struct {
	bus.BaseEvent
	Item *Item
}

type ItemRemovedEvent struct {
	bus.BaseEvent
	ItemID uint64
}

type ItemUpdatedEvent struct {
	bus.BaseEvent
	Item *Item
}

type CraftResponseEvent struct {
	bus.BaseEvent
	BlueprintID  uint16
	CreatedItems []uint64
}

type TradeRequestEvent struct {
	bus.BaseEvent
	SteamID uint64
	TradeID uint32
}

type CraftingCompleteEvent struct {
	bus.BaseEvent
	RecipeID     int16
	ItemsCreated []uint64
}

type NotificationEvent struct {
	bus.BaseEvent
	TitleLocalizationKey string
	BodyLocalizationKey  string
	ReplacementStrings   map[string]string
}

type ItemBroadcastEvent struct {
	bus.BaseEvent
	UserName       string
	WasDestruction bool
	DefIndex       uint32
}

type BackpackSortFinishedEvent struct {
	bus.BaseEvent
}
