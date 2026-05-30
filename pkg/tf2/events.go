// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"github.com/lemon4ksan/g-man/pkg/bus"
)

// ConnectedEvent is emitted when the connection handshake with the Game Coordinator completes.
type ConnectedEvent struct {
	bus.BaseEvent
	// Version represents the welcome protocol version returned by the Game Coordinator.
	Version uint32
}

// DisconnectedEvent is emitted when the connection with the Game Coordinator is lost.
type DisconnectedEvent struct {
	bus.BaseEvent
}

// BackpackLoadedEvent is emitted after the [SOCache] completes its initial full synchronization.
type BackpackLoadedEvent struct {
	bus.BaseEvent
	// Count represents the total number of items loaded into the cache.
	Count int
}

// ItemAcquiredEvent is emitted in real-time when a new item is added to the inventory cache.
type ItemAcquiredEvent struct {
	bus.BaseEvent
	// Item represents the details of the acquired item.
	Item *Item
}

// ItemRemovedEvent is emitted in real-time when an item is deleted or traded away from the cache.
type ItemRemovedEvent struct {
	bus.BaseEvent
	// ItemID represents the unique asset identifier of the removed item.
	ItemID uint64
}

// ItemUpdatedEvent is emitted in real-time when properties or positions of an item change in the cache.
type ItemUpdatedEvent struct {
	bus.BaseEvent
	// Item represents the updated details of the item.
	Item *Item
}

// CraftResponseEvent is emitted when a craft command is processed by the Game Coordinator.
type CraftResponseEvent struct {
	bus.BaseEvent
	// BlueprintID represents the recipe ID used for crafting.
	BlueprintID uint16
	// CreatedItems contains the asset IDs of newly created items.
	CreatedItems []uint64
}

// TradeRequestEvent is emitted when another player invites the bot to a live trade session.
type TradeRequestEvent struct {
	bus.BaseEvent
	// SteamID represents the 64-bit Steam ID of the initiating player.
	SteamID uint64
	// TradeID represents the temporary session identifier of the trade invitation.
	TradeID uint32
}

// CraftingCompleteEvent is emitted when a manual crafting recipe is successfully processed.
type CraftingCompleteEvent struct {
	bus.BaseEvent
	// RecipeID represents the blueprint ID used for the craft.
	RecipeID int16
	// ItemsCreated contains the asset IDs of the resulting items.
	ItemsCreated []uint64
}

// NotificationEvent is emitted when the Game Coordinator sends a client display notification.
type NotificationEvent struct {
	bus.BaseEvent
	// TitleLocalizationKey represents the localization string key for the title header.
	TitleLocalizationKey string
	// BodyLocalizationKey represents the localization string key for the main body text.
	BodyLocalizationKey string
	// ReplacementStrings contains the formatting variables used to fill the localization text.
	ReplacementStrings map[string]string
}

// ItemBroadcastEvent is emitted for global item events (such as Golden Wrench drops or destructions).
type ItemBroadcastEvent struct {
	bus.BaseEvent
	// UserName represents the name of the player associated with the event.
	UserName string
	// WasDestruction indicates whether the global event was a destruction of an item.
	WasDestruction bool
	// DefIndex represents the item definition index associated with the event.
	DefIndex uint32
}

// BackpackSortFinishedEvent is emitted when a sort request is completed by the Game Coordinator.
type BackpackSortFinishedEvent struct {
	bus.BaseEvent
}
