// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func createItemPayload(id uint64, defIndex uint32) []byte {
	b, _ := proto.Marshal(&pb.CSOEconItem{
		Id:       proto.Uint64(id),
		DefIndex: proto.Uint32(defIndex),
	})

	return b
}

func createItemPayloadFull(id uint64, defIndex, inventory uint32) []byte {
	b, _ := proto.Marshal(&pb.CSOEconItem{
		Id:        proto.Uint64(id),
		DefIndex:  proto.Uint32(defIndex),
		Inventory: proto.Uint32(inventory),
	})

	return b
}

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

		msgCreate := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(100, ItemScrap),
		}
		payloadCreate, _ := proto.Marshal(msgCreate)
		cache.handleSOUpdate(&protocol.GCPacket{MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create), Payload: payloadCreate})

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
		assert.Equal(t, 1, len(cache.GetItems()))
	})

	t.Run("cache_check_and_up_to_date", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		msg := &pb.CMsgSOCacheSubscribedUpToDate{
			Version: proto.Uint64(456),
		}
		payload, _ := proto.Marshal(msg)
		pkt := &protocol.GCPacket{
			MsgType: uint32(pb.ESOMsg_k_ESOMsg_CacheSubscribedUpToDate),
			Payload: payload,
		}
		cache.handleUpToDate(pkt)
		assert.Equal(t, uint64(456), cache.version.Load())
	})
}

func TestSOCache_Getters_ValidState_ReturnsCorrectValues(t *testing.T) {
	t.Parallel()

	t.Run("get_metal_count", func(t *testing.T) {
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

		assert.Equal(t, 2, cache.GetMetalCount(ItemScrap))
		assert.Equal(t, 0, cache.GetMetalCount(5001))
	})

	t.Run("find_craftable_items", func(t *testing.T) {
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

		items := cache.FindCraftableItems(ItemScrap, 2)
		assert.Equal(t, 2, len(items))

		items = cache.FindCraftableItems(ItemScrap, 5)
		assert.Equal(t, 2, len(items))
	})

	t.Run("find_weapons_by_class", func(t *testing.T) {
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

		weapons := cache.FindWeaponsByClass("Scout")
		assert.Equal(t, 0, len(weapons))
	})
}

func TestSOCache_ExtraGetters_ValidState_ReturnsCorrectValues(t *testing.T) {
	t.Parallel()

	t.Run("get_metal", func(t *testing.T) {
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

		metal := cache.GetMetal(ItemScrap, 10)
		assert.Equal(t, 1, len(metal))

		if len(metal) > 0 {
			assert.Equal(t, uint64(1), metal[0])
		}
	})

	t.Run("get_max_slots", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()
		assert.Equal(t, 0, cache.GetMaxSlots())
	})

	t.Run("get_asset_ids_by_sku", func(t *testing.T) {
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

		ids := cache.GetAssetIDsBySKU("5000;6", 10)
		assert.Equal(t, 0, len(ids))
	})

	t.Run("is_weapon", func(t *testing.T) {
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

		item, _ := cache.GetItem(1)
		assert.False(t, item.IsWeapon(&schema.Schema{}))
	})
}

func TestSOCache_Item_ToEconItem_ConvertsCorrectly(t *testing.T) {
	t.Parallel()

	item := Item{
		ID:           42,
		Quantity:     3,
		CustomName:   "Special Item",
		IsTradable:   true,
		IsMarketable: true,
	}

	econ := item.ToEconItem()
	assert.Equal(t, uint32(AppID), econ.AppID)
	assert.Equal(t, uint64(42), econ.AssetID)
	assert.Equal(t, int64(3), econ.Amount)
	assert.Equal(t, "Special Item", econ.Name)
	assert.True(t, econ.Tradable)
	assert.True(t, econ.Marketable)
}

func TestSOCache_Item_ToSKUObject_ElevatedEffect(t *testing.T) {
	t.Parallel()

	t.Run("elevated_strange", func(t *testing.T) {
		t.Parallel()

		item := Item{
			DefIndex:       200,
			Quality:        QualityUnique,
			IsElevated:     true,
			IsTradable:     true,
			IsCraftable:    true,
			KillstreakTier: 2,
			Effect:         10,
			Spells: []sku.Spell{
				{Attribute: 1004, Value: 1},
			},
			Parts: []uint32{380},
		}

		skuObj := item.ToSKUObject()
		assert.Equal(t, 200, skuObj.Defindex)
		assert.Equal(t, int(QualityUnique), skuObj.Quality)
		assert.Equal(t, 11, skuObj.Quality2)
		assert.Equal(t, 10, skuObj.Effect)
		assert.Equal(t, 2, skuObj.Killstreak)
		assert.Len(t, skuObj.Spells, 1)
		assert.Equal(t, []int{380}, skuObj.Parts)
	})

	t.Run("strange_unusual", func(t *testing.T) {
		t.Parallel()

		item := Item{
			DefIndex:    200,
			Quality:     11,
			Effect:      12,
			IsTradable:  true,
			IsCraftable: true,
		}

		skuObj := item.ToSKUObject()
		assert.Equal(t, 5, skuObj.Quality)
		assert.Equal(t, 11, skuObj.Quality2)
	})
}

func TestSOCache_Item_Fix_NormalizesDefIndex(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{Defindex: 6527, ItemClass: "tool"},
		{Defindex: 6522, ItemClass: "tool"},
		{Defindex: 20000, ItemClass: "tool"},
		{Defindex: 5726, ItemClass: "tool"},
		{Defindex: 5745, ItemClass: "tool"},
		{Defindex: 5795, ItemClass: "tool"},
		{Defindex: 5661, ItemClass: "tool"},
		{Defindex: 20005, ItemClass: "tool"},
		{
			Defindex:  5022,
			ItemClass: "supply_crate",
			Attributes: []schema.ItemAttribute{
				{Name: "set supply crate series", Value: 85},
			},
		},
	}
	s := schema.New(raw)

	tests := []struct {
		name     string
		input    uint32
		expected uint32
	}{
		{"range 1", 5726, 6527},
		{"range 2", 5745, 6527},
		{"range 3", 5795, 6527},
		{"strangifier", 5661, 6522},
		{"paintkit range", 20005, 20000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &Item{DefIndex: tt.input}
			item.Fix(s)
			assert.Equal(t, tt.expected, item.DefIndex)
		})
	}

	t.Run("supply_crate_series", func(t *testing.T) {
		t.Parallel()

		item := &Item{
			DefIndex:    5022,
			CrateSeries: 0,
		}
		item.Fix(s)
		assert.Equal(t, uint32(85), item.CrateSeries)
	})
}

func TestSOCache_protoToItem_Attributes_ParsedCorrectly(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	nowUnix := uint32(time.Now().Unix())

	attrs := []*pb.CSOEconItemAttribute{
		{DefIndex: proto.Uint32(AttrCustomName), ValueBytes: []byte("My Named Weapon")},
		{DefIndex: proto.Uint32(AttrCustomDesc), ValueBytes: []byte("Custom Description")},
		{DefIndex: proto.Uint32(AttrMedalNumber), ValueBytes: uint32ToBytes(1234)},
		{DefIndex: proto.Uint32(AttrUnusualEffect), ValueBytes: float32ToBytes(15.0)},
		{DefIndex: proto.Uint32(AttrPaintPrimary), ValueBytes: float32ToBytes(456.0)},
		{DefIndex: proto.Uint32(AttrCannotTrade), ValueBytes: uint32ToBytes(1)},
		{DefIndex: proto.Uint32(AttrCannotCraft), ValueBytes: uint32ToBytes(1)},
		{DefIndex: proto.Uint32(AttrCrateSeries), ValueBytes: float32ToBytes(90.0)},
		{DefIndex: proto.Uint32(AttrAlwaysTradable), ValueBytes: uint32ToBytes(1)},
		{DefIndex: proto.Uint32(AttrTradableAfter), ValueBytes: uint32ToBytes(nowUnix + 3600)},
		{DefIndex: proto.Uint32(AttrKillEater), ValueBytes: uint32ToBytes(1)},
		{DefIndex: proto.Uint32(AttrCraftNumber), ValueBytes: uint32ToBytes(42)},
		{DefIndex: proto.Uint32(AttrStrangePart1), ValueBytes: float32ToBytes(101.0)},
		{DefIndex: proto.Uint32(AttrStrangePart1Val), ValueBytes: uint32ToBytes(42)},
		{DefIndex: proto.Uint32(AttrStrangePart2), ValueBytes: float32ToBytes(102.0)},
		{DefIndex: proto.Uint32(AttrStrangePart2Val), ValueBytes: uint32ToBytes(500)},
		{DefIndex: proto.Uint32(AttrStrangePart3), ValueBytes: float32ToBytes(103.0)},
		{DefIndex: proto.Uint32(AttrStrangePart3Val), ValueBytes: uint32ToBytes(1337)},
		{DefIndex: proto.Uint32(AttrCannotCraftVariant), ValueBytes: uint32ToBytes(1)},
		{DefIndex: proto.Uint32(AttrEOTLEarlySupporter), ValueBytes: float32ToBytes(1.0)},
		{DefIndex: proto.Uint32(AttrQuestLoanerIDLow), ValueBytes: uint32ToBytes(0xAA55AA55)},
		{DefIndex: proto.Uint32(AttrQuestLoanerIDHigh), ValueBytes: uint32ToBytes(0x55AA55AA)},
		{DefIndex: proto.Uint32(AttrWear), ValueBytes: float32ToBytes(0.123)},
		{DefIndex: proto.Uint32(AttrPaintkit), ValueBytes: uint32ToBytes(200)},
		{DefIndex: proto.Uint32(AttrSpell1), ValueBytes: float32ToBytes(5.0)},
		{DefIndex: proto.Uint32(AttrSpell2), ValueBytes: float32ToBytes(6.0)},
		{DefIndex: proto.Uint32(AttrSpell3), ValueBytes: float32ToBytes(7.0)},
		{DefIndex: proto.Uint32(AttrSpell4), ValueBytes: float32ToBytes(8.0)},
		{DefIndex: proto.Uint32(AttrSpell5), ValueBytes: float32ToBytes(9.0)},
		{DefIndex: proto.Uint32(AttrSpell6), ValueBytes: float32ToBytes(10.0)},
		{DefIndex: proto.Uint32(AttrTarget), ValueBytes: float32ToBytes(100.0)},
		{DefIndex: proto.Uint32(AttrKillstreaker), ValueBytes: float32ToBytes(2008.0)},
		{DefIndex: proto.Uint32(AttrSheen), ValueBytes: float32ToBytes(3.0)},
		{DefIndex: proto.Uint32(AttrKillstreakTier), ValueBytes: float32ToBytes(3.0)},
		{DefIndex: proto.Uint32(AttrSeries), ValueBytes: float32ToBytes(4.0)},
		{DefIndex: proto.Uint32(AttrTauntUnusualEffect), ValueBytes: float32ToBytes(25.0)},
		{DefIndex: proto.Uint32(AttrAustralium), ValueBytes: float32ToBytes(1.0)},
		{DefIndex: proto.Uint32(AttrFestivized), ValueBytes: float32ToBytes(1.0)},
	}

	p := &pb.CSOEconItem{
		Id:        proto.Uint64(123),
		Attribute: attrs,
	}

	item := cache.protoToItem(p)

	assert.Equal(t, "My Named Weapon", item.CustomName)
	assert.Equal(t, "Custom Description", item.CustomDesc)
	assert.Equal(t, uint32(1234), item.MedalNumber)
	assert.Equal(t, uint32(25), item.Effect)
	assert.Equal(t, uint32(456), item.Paint)
	assert.True(t, item.IsTradable)
	assert.False(t, item.IsCraftable)
	assert.Equal(t, uint32(90), item.CrateSeries)
	assert.True(t, item.IsElevated)
	assert.Equal(t, uint32(42), item.CraftNumber)
	assert.Equal(t, []uint32{101, 102, 103}, item.Parts)
	assert.Equal(t, map[uint32]uint32{101: 42, 102: 500, 103: 1337}, item.PartValues)
	assert.True(t, item.EarlySupporter)
	assert.Equal(t, uint64(0x55AA55AAAA55AA55), item.QuestID)
	assert.InDelta(t, float32(0.123), item.Wear, 0.0001)
	assert.Equal(t, uint32(200), item.Paintkit)
	assert.Len(t, item.Spells, 6)
	assert.Equal(t, int(AttrSpell1), item.Spells[0].Attribute)
	assert.Equal(t, uint32(100), item.Target)
	assert.Equal(t, uint32(2008), item.Killstreaker)
	assert.Equal(t, uint32(3), item.Sheen)
	assert.Equal(t, uint32(3), item.KillstreakTier)
	assert.Equal(t, uint32(4), item.Series)
	assert.True(t, item.Australium)
	assert.True(t, item.Festivized)
}

func TestSOCache_protoToItem_OriginsAndFlags(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	t.Run("achievement_untradable", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:     proto.Uint64(1),
			Origin: proto.Uint32(OriginAchievement),
		}
		item := cache.protoToItem(p)
		assert.False(t, item.IsTradable)
		assert.False(t, item.IsMarketable)
	})

	t.Run("loaner_bugged", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:     proto.Uint64(2),
			Origin: proto.Uint32(OriginLoaner),
			Flags:  proto.Uint32(0),
		}
		item := cache.protoToItem(p)
		assert.True(t, item.IsBuggedLoaner)
	})

	t.Run("store_promo_uncraftable", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:     proto.Uint64(3),
			Origin: proto.Uint32(OriginStorePromo),
		}
		item := cache.protoToItem(p)
		assert.False(t, item.IsCraftable)
	})

	t.Run("quality_selfmade_untradable", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:      proto.Uint64(4),
			Quality: proto.Uint32(QualitySelfMade),
		}
		item := cache.protoToItem(p)
		assert.False(t, item.IsTradable)
		assert.False(t, item.IsCraftable)
	})

	t.Run("purchase_old_uncraftable", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:     proto.Uint64(5),
			Origin: proto.Uint32(OriginPurchase),
			Flags:  proto.Uint32(0),
		}
		item := cache.protoToItem(p)
		assert.False(t, item.IsCraftable)
	})

	t.Run("preview_untradable_uncraftable", func(t *testing.T) {
		t.Parallel()

		p := &pb.CSOEconItem{
			Id:    proto.Uint64(6),
			Flags: proto.Uint32(uint32(EconItemFlagPreview)),
		}
		item := cache.protoToItem(p)
		assert.False(t, item.IsTradable)
		assert.False(t, item.IsCraftable)
	})
}

func TestSOCache_requestRefresh_SendsCorrectMessage(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	cache := tf.Cache()

	cache.requestRefresh(t.Context(), 9999, log.Discard)

	assert.Equal(t, uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionRefresh), mCoord.GetLastSendMsgType())

	var sent pb.CMsgSOCacheSubscriptionRefresh

	err := proto.Unmarshal(mCoord.lastSendPayload, &sent)
	require.NoError(t, err)
	assert.Equal(t, uint64(9999), sent.GetOwner())
}

func TestSOCache_handleSOCacheCheck_Desync_RequestsRefresh(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	cache := tf.Cache()

	cache.version.Store(100)

	msg := &pb.CMsgSOCacheSubscriptionCheck{
		Version: proto.Uint64(200),
		Owner:   proto.Uint64(12345),
	}
	payload, _ := proto.Marshal(msg)

	pkt := &protocol.GCPacket{
		MsgType: uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionCheck),
		Payload: payload,
	}

	cache.handleSOCacheCheck(t.Context(), pkt)

	assert.Equal(t, uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionRefresh), mCoord.GetLastSendMsgType())
}

func TestSOCache_processDestroy_WrongTypeID_DoesNothing(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	cache.items[100] = &Item{ID: 100, DefIndex: ItemScrap}

	cache.processDestroy(999, []byte("some junk"), nil)
	assert.Len(t, cache.items, 1)

	cache.processDestroy(SOTypeEconItem, []byte("invalid pb junk"), nil)
	assert.Len(t, cache.items, 1)

	destroyMsg := &pb.CSOEconItem{Id: proto.Uint64(100)}
	destroyBytes, _ := proto.Marshal(destroyMsg)
	cache.processDestroy(SOTypeEconItem, destroyBytes, nil)
	assert.Len(t, cache.items, 0)
}

func TestSOCache_protoToItem_CustomDecal(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	p := &pb.CSOEconItem{
		Id: proto.Uint64(123),
		Attribute: []*pb.CSOEconItemAttribute{
			{DefIndex: proto.Uint32(152), ValueBytes: uint32ToBytes(0x11223344)},
			{DefIndex: proto.Uint32(227), ValueBytes: uint32ToBytes(0x55667788)},
		},
	}

	item := cache.protoToItem(p)
	assert.True(t, item.HasCustomDecal)
	assert.Equal(t, uint64(0x5566778811223344), item.DecalUGCID)
}

func createPacket(msgType pb.ESOMsg, msg proto.Message) *protocol.GCPacket {
	b, _ := proto.Marshal(msg)

	return &protocol.GCPacket{
		MsgType: uint32(msgType),
		Payload: b,
	}
}

func float32ToBytes(f float32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(f))
	return b
}

func uint32ToBytes(u uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, u)
	return b
}

func TestSOCache_Item_Fix_StaticRestrictions(t *testing.T) {
	t.Parallel()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:  537,
			ItemName:  "Party Hat",
			ItemClass: "tf_wearable",
			Attributes: []schema.ItemAttribute{
				{Name: "cannot trade", Value: 1},
			},
		},
		{
			Defindex:  9999,
			ItemName:  "Uncraftable Item",
			ItemClass: "tool",
			Attributes: []schema.ItemAttribute{
				{Class: "cannot_craft", Value: 1},
			},
		},
	}
	s := schema.New(raw)

	t.Run("Item.Fix applies cannot trade and cannot craft", func(t *testing.T) {
		item1 := &Item{DefIndex: 537, IsTradable: true, IsCraftable: true}
		item1.Fix(s)
		assert.False(t, item1.IsTradable)
		assert.False(t, item1.IsMarketable)
		assert.True(t, item1.IsCraftable)

		item2 := &Item{DefIndex: 9999, IsTradable: true, IsCraftable: true}
		item2.Fix(s)
		assert.True(t, item2.IsTradable)
		assert.False(t, item2.IsCraftable)
	})

	t.Run("SOCache.UpdateSchema updates and fixes cached items", func(t *testing.T) {
		tf, _, _ := setupTF2(t)
		cache := tf.Cache()

		item := &Item{ID: 101, DefIndex: 537, IsTradable: true, IsCraftable: true}
		cache.items[101] = item

		assert.True(t, item.IsTradable)

		cache.UpdateSchema(s)

		assert.False(t, item.IsTradable)
		assert.False(t, item.IsMarketable)
	})
}

func TestSOCache_RecipeComponent_Getters(t *testing.T) {
	t.Parallel()

	comp := RecipeComponent{
		Flags:        0x01 | 0x02 | 0x04 | 0x08,
		NumRequired:  5,
		NumFulfilled: 3,
	}

	assert.True(t, comp.IsOutput())
	assert.True(t, comp.IsUntradable())
	assert.True(t, comp.HasDefIndex())
	assert.True(t, comp.HasQuality())
	assert.False(t, comp.IsComplete())

	comp.NumFulfilled = 5
	assert.True(t, comp.IsComplete())
}

func TestSOCache_Item_GetSKU(t *testing.T) {
	t.Parallel()

	item := &Item{SKU: "5021;6"}
	assert.Equal(t, "5021;6", item.GetSKU(&schema.Schema{}))

	itemEmpty := &Item{DefIndex: 5021, Quality: 6, IsTradable: true, IsCraftable: true}
	raw := &schema.Raw{}
	s := schema.New(raw)
	assert.Equal(t, "5021;6", itemEmpty.GetSKU(s))
}

func TestSOCache_NewSOCache_NilBus(t *testing.T) {
	t.Parallel()

	cache := NewSOCache(&mockCoordinator{})
	assert.NotNil(t, cache.bus)
}

func TestSOCache_UpdateSchema_Nil(t *testing.T) {
	t.Parallel()

	cache := NewSOCache(&mockCoordinator{})
	cache.UpdateSchema(nil)
}

func TestSOCache_GetItemByOriginalID(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	cache.items[100] = &Item{ID: 100, OriginalID: 555}

	item, ok := cache.GetItemByOriginalID(555)
	assert.True(t, ok)
	assert.Equal(t, uint64(100), item.ID)

	_, ok = cache.GetItemByOriginalID(999)
	assert.False(t, ok)
}

func TestSOCache_Getters_EdgeCases(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	cache.items[1] = &Item{ID: 1, DefIndex: 5000, Quality: 6, IsTradable: true, SKU: "5000;6"}
	cache.items[2] = &Item{ID: 2, DefIndex: 5000, Quality: 6, IsTradable: false, SKU: "5000;6"}

	res := cache.GetMetal(5000, 1)
	assert.Len(t, res, 1)
	assert.Equal(t, uint64(1), res[0])

	resSKU := cache.GetAssetIDsBySKU("5000;6", 10)
	assert.Len(t, resSKU, 1)
	assert.Equal(t, uint64(1), resSKU[0])
}

func TestSOCache_cleanGCString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", cleanGCString(nil))
	assert.Equal(t, "", cleanGCString([]byte{}))
	assert.Equal(t, "", cleanGCString([]byte{1, 2, 3}))
	assert.Equal(t, "Hello", cleanGCString([]byte{'H', 'e', 'l', 'l', 'o', 0}))
}

func TestSOCache_handleSOCacheCheck_Sync(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	cache := tf.Cache()

	cache.version.Store(100)

	msg := &pb.CMsgSOCacheSubscriptionCheck{
		Version: proto.Uint64(100),
		Owner:   proto.Uint64(12345),
	}
	payload, _ := proto.Marshal(msg)

	pkt := &protocol.GCPacket{
		MsgType: uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionCheck),
		Payload: payload,
	}

	cache.handleSOCacheCheck(t.Context(), pkt)

	assert.Equal(t, uint32(0), mCoord.GetLastSendMsgType())
}

func TestSOCache_GCEvents_UnmarshalErrors(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	cache.handleSubscribed(&protocol.GCPacket{Payload: []byte("invalid-payload")})

	cache.handleSOCacheCheck(t.Context(), &protocol.GCPacket{Payload: []byte("invalid-payload")})

	cache.handleSOUpdate(&protocol.GCPacket{
		MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create),
		Payload: []byte("invalid-payload"),
	})

	cache.handleSOUpdate(&protocol.GCPacket{
		MsgType: uint32(pb.ESOMsg_k_ESOMsg_Destroy),
		Payload: []byte("invalid-payload"),
	})

	cache.handleSOUpdate(&protocol.GCPacket{
		MsgType: uint32(pb.ESOMsg_k_ESOMsg_UpdateMultiple),
		Payload: []byte("invalid-payload"),
	})

	tf.handleWelcome(&protocol.GCPacket{Payload: []byte("invalid-payload")})

	tf.handleSchemaUpdate(&protocol.GCPacket{Payload: []byte("invalid-payload")})
}
