// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

func TestSOCache_Lifecycle_GCEvents_UpdatesInternalState(t *testing.T) {
	t.Parallel()

	t.Run("create_and_get_item", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		itemBytes := createItemPayload(100, ItemScrap)

		msg := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: itemBytes,
		}
		payload, _ := proto.Marshal(msg)
		pkt := &protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create), Payload: payload}

		cache.handleSOUpdate(pkt)

		item, ok := cache.GetItem(100)
		require.True(t, ok)
		assert.Equal(t, uint64(100), item.ID)
		assert.Equal(t, uint32(ItemScrap), item.DefIndex)
	})

	t.Run("update", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		// Initial creation
		msgCreate := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(100, ItemScrap),
		}
		payloadCreate, _ := proto.Marshal(msgCreate)
		cache.handleSOUpdate(&protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create), Payload: payloadCreate})

		// Update item 100 to defindex Key
		itemBytes := createItemPayload(100, ItemKey)

		msgUpdate := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: itemBytes,
		}
		payloadUpdate, _ := proto.Marshal(msgUpdate)
		pkt := &protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Update), Payload: payloadUpdate}

		cache.handleSOUpdate(pkt)

		item, ok := cache.GetItem(100)
		require.True(t, ok)
		assert.Equal(t, uint32(ItemKey), item.DefIndex)
	})

	t.Run("update_multiple", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		msg := &pb.CMsgSOMultipleObjects{
			Objects: []*pb.CMsgSOMultipleObjects_SingleObject{
				{
					TypeId:     proto.Int32(SOTypeEconItem),
					ObjectData: createItemPayload(100, ItemScrap),
				},
				{
					TypeId:     proto.Int32(SOTypeEconItem),
					ObjectData: createItemPayload(200, ItemKey),
				},
			},
		}
		payload, _ := proto.Marshal(msg)
		pkt := &protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_UpdateMultiple), Payload: payload}

		cache.handleSOUpdate(pkt)

		assert.Equal(t, 2, len(cache.GetItems()))
		assert.Equal(t, 1, cache.GetMetalCount(ItemScrap))
	})

	t.Run("destroy", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		// Initial creation
		msgCreate1 := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(100, ItemScrap),
		}
		payloadCreate1, _ := proto.Marshal(msgCreate1)
		cache.handleSOUpdate(&protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create), Payload: payloadCreate1})

		msgCreate2 := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(200, ItemKey),
		}
		payloadCreate2, _ := proto.Marshal(msgCreate2)
		cache.handleSOUpdate(&protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create), Payload: payloadCreate2})

		msgDestroy := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(100, ItemScrap),
		}
		payloadDestroy, _ := proto.Marshal(msgDestroy)
		pkt := &protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Destroy), Payload: payloadDestroy}

		cache.handleSOUpdate(pkt)

		_, ok := cache.GetItem(100)
		assert.False(t, ok)
		assert.Equal(t, 1, len(cache.GetItems())) // item 200 should still exist
	})

	t.Run("cache_check_and_up_to_date", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		cache.handleSOCacheCheck(t.Context(), &protocol.GCPacket{})
		cache.handleUpToDate(&protocol.GCPacket{})
	})
}

func TestSOCache_Getters_ValidState_ReturnsCorrectValues(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	msg := &pb.CMsgSOCacheSubscribed{
		Objects: []*pb.CMsgSOCacheSubscribed_SubscribedType{
			{
				TypeId: proto.Int32(SOTypeEconItem),
				ObjectData: [][]byte{
					createItemPayload(1, ItemScrap),
					createItemPayload(2, ItemScrap),
					createItemPayload(3, ItemKey),
				},
			},
		},
	}
	payload, _ := proto.Marshal(msg)
	cache.handleSubscribed(&protocol.GCPacket{Payload: payload})

	t.Run("get_metal_count", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 2, cache.GetMetalCount(ItemScrap))
		assert.Equal(t, 0, cache.GetMetalCount(5001)) // Reclaimed
	})

	t.Run("find_craftable_items", func(t *testing.T) {
		t.Parallel()

		items := cache.FindCraftableItems(ItemScrap, 2)
		assert.Equal(t, 2, len(items))

		items = cache.FindCraftableItems(ItemScrap, 5)
		assert.Equal(t, 2, len(items)) // only 2 available
	})

	t.Run("find_weapons_by_class", func(t *testing.T) {
		t.Parallel()

		weapons := cache.FindWeaponsByClass("Scout")
		assert.Equal(t, 0, len(weapons))
	})
}

func TestSOCache_ExtraGetters_ValidState_ReturnsCorrectValues(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	msg := &pb.CMsgSOCacheSubscribed{
		Objects: []*pb.CMsgSOCacheSubscribed_SubscribedType{
			{
				TypeId: proto.Int32(SOTypeEconItem),
				ObjectData: [][]byte{
					createItemPayload(1, ItemScrap),
					createItemPayload(2, ItemKey),
				},
			},
		},
	}
	cache.handleSubscribed(createPacket(pb.ESOMsg_k_ESOMsg_CacheSubscribed, msg))

	t.Run("get_metal", func(t *testing.T) {
		t.Parallel()

		metal := cache.GetMetal(ItemScrap, 10)
		assert.Equal(t, 1, len(metal))

		if len(metal) > 0 {
			assert.Equal(t, uint64(1), metal[0])
		}
	})

	t.Run("get_max_slots", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 0, cache.GetMaxSlots())
	})

	t.Run("get_asset_ids_by_sku", func(t *testing.T) {
		t.Parallel()

		ids := cache.GetAssetIDsBySKU("5000;6", 10)
		assert.Equal(t, 0, len(ids))
	})

	t.Run("is_weapon", func(t *testing.T) {
		t.Parallel()

		item, _ := cache.GetItem(1)
		assert.False(t, item.IsWeapon(&schema.Schema{}))
	})
}

func createPacket(msgType pb.ESOMsg, msg proto.Message) *protocol.GCPacket {
	b, _ := proto.Marshal(msg)

	return &protocol.GCPacket{
		MsgType: uint32(msgType),
		Payload: b,
	}
}
