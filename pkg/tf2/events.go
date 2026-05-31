// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/g-man/pkg/bus"
)

// ConnectedEvent is emitted when the client establishes a session with the Game Coordinator.
type ConnectedEvent struct {
	bus.BaseEvent
	// Version represents the welcome schema version returned by the Game Coordinator.
	Version uint32
}

// DisconnectedEvent is emitted when the Game Coordinator session is closed.
type DisconnectedEvent struct {
	bus.BaseEvent
}

// BackpackLoadedEvent is emitted when the initial inventory synchronization is completed.
type BackpackLoadedEvent struct {
	bus.BaseEvent
	// Count represents the total number of items loaded into the cache.
	Count int
}

// ItemAcquiredEvent is emitted when a new item is added to the local inventory.
type ItemAcquiredEvent struct {
	bus.BaseEvent
	// Item represents the details of the acquired item.
	Item *Item
}

// ItemRemovedEvent is emitted when an item is deleted or traded away.
type ItemRemovedEvent struct {
	bus.BaseEvent
	// ItemID represents the unique asset identifier of the removed item.
	ItemID uint64
}

// ItemUpdatedEvent is emitted when an existing item's metadata or position is modified.
type ItemUpdatedEvent struct {
	bus.BaseEvent
	// Item represents the updated state of the item.
	Item *Item
}

// CraftResponseEvent is emitted when a crafting request completes.
type CraftResponseEvent struct {
	bus.BaseEvent
	// BlueprintID represents the recipe index used for crafting.
	BlueprintID uint16
	// CreatedItems contains the asset IDs of the newly crafted items.
	CreatedItems []uint64
}

// TradeRequestEvent is emitted when an invitation to a live trade is received.
type TradeRequestEvent struct {
	bus.BaseEvent
	// SteamID represents the identifier of the inviting player.
	SteamID uint64
	// TradeID represents the temporary session ID of the trade.
	TradeID uint32
}

// CraftingCompleteEvent is emitted when an item crafting workflow completes.
type CraftingCompleteEvent struct {
	bus.BaseEvent
	// RecipeID represents the crafting recipe ID.
	RecipeID int16
	// ItemsCreated contains the asset IDs of the items created.
	ItemsCreated []uint64
}

// NotificationEvent is emitted when a display notification is sent by the Game Coordinator.
type NotificationEvent struct {
	bus.BaseEvent
	// TitleLocalizationKey represents the localization token for the title.
	TitleLocalizationKey string
	// BodyLocalizationKey represents the localization token for the body.
	BodyLocalizationKey string
	// ReplacementStrings contains the dynamic template parameters.
	ReplacementStrings map[string]string
}

// ItemBroadcastEvent is emitted for global server notifications.
type ItemBroadcastEvent struct {
	bus.BaseEvent
	// UserName represents the name of the player.
	UserName string
	// WasDestruction indicates whether the broadcast is for an item deletion.
	WasDestruction bool
	// DefIndex represents the item definition index.
	DefIndex uint32
}

// BackpackSortFinishedEvent is emitted when the backpack sorting request completes.
type BackpackSortFinishedEvent struct {
	bus.BaseEvent
}
