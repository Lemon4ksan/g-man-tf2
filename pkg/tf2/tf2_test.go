// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/jobs"
	bm "github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/test/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

const (
	ItemScrap = 5000
	ItemKey   = 5021
)

type mockCoordinator struct {
	bm.Base
	mu              sync.RWMutex
	lastSendMsgType uint32
	lastSendPayload []byte

	onCallRaw func(msgType uint32, payload []byte) (*protocol.GCPacket, error)
	onSendRaw func(msgType uint32, payload []byte) error
}

type mockSchemaProvider struct {
	schema *schema.Schema
}

func (m *mockSchemaProvider) Init(init bm.InitContext) error {
	return nil
}

func (m *mockSchemaProvider) Name() string {
	return "mockSchemaProvider"
}

func (m *mockSchemaProvider) Start(ctx context.Context) error {
	return nil
}

func (m *mockSchemaProvider) Get() *schema.Schema {
	return m.schema
}

type mockAppsProvider struct{}

func (m *mockAppsProvider) Init(init bm.InitContext) error {
	return nil
}

func (m *mockAppsProvider) Name() string {
	return "mockAppsProvider"
}

func (m *mockAppsProvider) Start(ctx context.Context) error {
	return nil
}

func (m *mockAppsProvider) PlayGames(ctx context.Context, appIDs []uint32, forceKick bool) error {
	return nil
}

func (m *mockCoordinator) Send(ctx context.Context, appID, msgType uint32, msg proto.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastSendMsgType = msgType
	m.lastSendPayload, _ = proto.Marshal(msg)

	return nil
}

func (m *mockCoordinator) SendRaw(ctx context.Context, appID, msgType uint32, payload []byte) error {
	m.mu.Lock()
	m.lastSendMsgType = msgType
	m.lastSendPayload = payload
	m.mu.Unlock()

	if m.onSendRaw != nil {
		return m.onSendRaw(msgType, payload)
	}

	return nil
}

func (m *mockCoordinator) GetLastSendMsgType() uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSendMsgType
}

func (m *mockCoordinator) Call(
	ctx context.Context,
	appID, msgType uint32,
	msg proto.Message,
	cb jobs.Callback[*protocol.GCPacket],
) error {
	return nil
}

func (m *mockCoordinator) CallRaw(
	ctx context.Context,
	appID, msgType uint32,
	payload []byte,
	cb jobs.Callback[*protocol.GCPacket],
) error {
	m.lastSendMsgType = msgType
	m.lastSendPayload = payload

	if m.onCallRaw != nil {
		resp, err := m.onCallRaw(msgType, payload)
		go cb(ctx, resp, err)

		return nil
	}

	return errors.New("onCallRaw not configured")
}

func setupTF2(t *testing.T) (*TF2, *module.InitContext, *mockCoordinator) {
	t.Helper()

	ictx := module.NewInitContext()

	mCoord := &mockCoordinator{}
	ictx.SetModule(gc.ModuleName, mCoord)
	ictx.SetModule(apps.ModuleName, &mockAppsProvider{})

	mSchema := &mockSchemaProvider{schema: &schema.Schema{}}
	ictx.SetModule(schema.ModuleName, mSchema)

	tf := New()
	if err := tf.Init(ictx); err != nil {
		t.Fatalf("failed to init TF2: %v", err)
	}

	return tf, ictx, mCoord
}

func TestTF2_SOCacheEvents_TriggerBusSignals_FiresCorrectEvents(t *testing.T) {
	t.Parallel()

	t.Run("initial_load_via_bus", func(t *testing.T) {
		t.Parallel()
		_, ictx, _ := setupTF2(t)
		subLoaded := ictx.Bus().Subscribe(&BackpackLoadedEvent{})

		msg := &pb.CMsgSOCacheSubscribed{
			Objects: []*pb.CMsgSOCacheSubscribed_SubscribedType{
				{
					TypeId: proto.Int32(SOTypeEconItem),
					ObjectData: [][]byte{
						createItemPayload(100, ItemKey),
						createItemPayload(200, ItemScrap),
					},
				},
			},
		}

		payload, _ := proto.Marshal(msg)
		ictx.Bus().Publish(&gc.MessageEvent{
			Packet: &protocol.GCPacket{
				AppID:   AppID,
				MsgType: uint32(pb.ESOMsg_k_ESOMsg_CacheSubscribed),
				Payload: payload,
			},
		})

		select {
		case ev := <-subLoaded.C():
			loadedEv := ev.(*BackpackLoadedEvent)
			assert.Equal(t, 2, loadedEv.Count)
		case <-time.After(1 * time.Second):
			t.Fatal("BackpackLoadedEvent not received")
		}
	})

	t.Run("item_acquired_via_bus", func(t *testing.T) {
		t.Parallel()
		_, ictx, _ := setupTF2(t)
		subAcquired := ictx.Bus().Subscribe(&ItemAcquiredEvent{})

		msg := &pb.CMsgSOSingleObject{
			TypeId:     proto.Int32(SOTypeEconItem),
			ObjectData: createItemPayload(300, ItemScrap),
		}

		payload, _ := proto.Marshal(msg)
		ictx.Bus().Publish(&gc.MessageEvent{
			Packet: &protocol.GCPacket{
				AppID:   AppID,
				MsgType: uint32(pb.ESOMsg_k_ESOMsg_Create),
				Payload: payload,
			},
		})

		select {
		case ev := <-subAcquired.C():
			acqEv := ev.(*ItemAcquiredEvent)
			assert.Equal(t, uint64(300), acqEv.Item.ID)
		case <-time.After(1 * time.Second):
			t.Fatal("ItemAcquiredEvent not received")
		}
	})
}

func TestTF2_Lifecycle_ConnectionHandshakes_EmitsExpectedEvents(t *testing.T) {
	t.Parallel()

	tf, ictx, mCoord := setupTF2(t)
	subConn := ictx.Bus().Subscribe(&ConnectedEvent{})
	subDisc := ictx.Bus().Subscribe(&DisconnectedEvent{})
	ctx := t.Context()

	err := tf.StartAuthed(ctx, nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return mCoord.GetLastSendMsgType() == uint32(pb.EGCBaseClientMsg_k_EMsgGCClientHello)
	}, 1*time.Second, 10*time.Millisecond)

	msg := &pb.CMsgClientWelcome{Version: proto.Uint32(1)}
	payload, _ := proto.Marshal(msg)

	ictx.Bus().Publish(&gc.MessageEvent{
		Packet: &protocol.GCPacket{
			AppID:   AppID,
			MsgType: uint32(pb.EGCBaseClientMsg_k_EMsgGCClientWelcome),
			Payload: payload,
		},
	})

	select {
	case <-subConn.C():
		assert.Equal(t, int32(Connected), tf.state.Load())
	case <-time.After(1 * time.Second):
		t.Fatal("GCConnectedEvent not received")
	}

	ictx.Bus().Publish(&gc.MessageEvent{
		Packet: &protocol.GCPacket{
			AppID:   AppID,
			MsgType: uint32(pb.EGCBaseClientMsg_k_EMsgGCClientGoodbye),
		},
	})

	select {
	case <-subDisc.C():
		assert.Equal(t, int32(Connecting), tf.state.Load())
	case <-time.After(1 * time.Second):
		t.Fatal("GCDisconnectedEvent not received")
	}

	err = tf.Close()
	require.NoError(t, err)
	assert.Equal(t, int32(Disconnected), tf.state.Load())
}

func TestTF2_AcknowledgeAll_UnacknowledgedItems_TriggersGCBatchMoves(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)

	msg := &pb.CMsgSOCacheSubscribed{
		Objects: []*pb.CMsgSOCacheSubscribed_SubscribedType{
			{
				TypeId: proto.Int32(SOTypeEconItem),
				ObjectData: [][]byte{
					createItemPayloadFull(1, ItemScrap, 1<<30),
					createItemPayloadFull(2, ItemScrap, 0),
					createItemPayloadFull(3, ItemScrap, 5),
				},
			},
		},
	}
	payload, _ := proto.Marshal(msg)
	tf.cache.handleSubscribed(&protocol.GCPacket{Payload: payload})

	err := tf.AcknowledgeAll(t.Context())
	require.NoError(t, err)

	assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSetItemPositions), mCoord.GetLastSendMsgType())
}

func TestTF2_AdvancedActions_ComplexCustomizations_DispatchesGCPackets(t *testing.T) {
	t.Parallel()

	t.Run("set_unusual_effect_offset", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		err := tf.SetUnusualEffectOffset(t.Context(), 123, 1.5)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSetItemEffectVerticalOffset), mCoord.GetLastSendMsgType())
	})

	t.Run("transfer_strange_count", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		err := tf.TransferStrangeCount(t.Context(), 1, 2, 3)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangeCountTransfer), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_killstreak", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		err := tf.RemoveKillstreak(t.Context(), 456)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), mCoord.GetLastSendMsgType())
	})

	t.Run("report_player", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		reason := pb.CMsgGC_ReportPlayer_kReason_CHEATING
		err := tf.ReportPlayer(t.Context(), 777, &reason)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.ETFGCMsg_k_EMsgGC_ReportPlayer), mCoord.GetLastSendMsgType())
	})
}

func TestTF2_SOCache_Metadata_RawPayloadUpdates_SavesValidMetadata(t *testing.T) {
	t.Parallel()

	t.Run("account_metadata_update", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)

		accMsg := &pb.CSOEconGameAccountClient{
			AdditionalBackpackSlots: proto.Uint32(100),
			TrialAccount:            proto.Bool(false),
			CompetitiveAccess:       proto.Bool(true),
			TradeBanExpiration:      proto.Uint32(123456),
		}
		data, _ := proto.Marshal(accMsg)

		tf.cache.processObject(SOTypeEconGameAccountClient, data, false, nil)

		assert.True(t, tf.cache.IsPremium())
		assert.Equal(t, 400, tf.cache.GetMaxSlots())
		assert.True(t, tf.cache.HasCompetitiveAccess())
		assert.Equal(t, uint32(123456), tf.cache.GetTradeBanExpiration())
	})

	t.Run("mmr_update", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)

		ratingMsg := &pb.CSOTFRatingData{
			RatingType:    proto.Int32(2),
			RatingPrimary: proto.Uint32(1500),
		}
		data, _ := proto.Marshal(ratingMsg)

		tf.cache.processObject(SOTypeTFRatingData, data, false, nil)

		assert.Equal(t, uint32(1500), tf.cache.GetMMR(2))
	})
}

func TestTF2_Crafting_BlueprintRecipe_ExecutesSynchronousCraft(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	tf.state.Store(int32(Connected))

	mCoord.onSendRaw = func(msgType uint32, p []byte) error {
		if msgType != uint32(pb.EGCItemMsg_k_EMsgGCCraft) {
			return errors.New("unexpected msg type")
		}

		go func() {
			tf.Bus.Publish(&CraftResponseEvent{
				BlueprintID:  uint16(65535),
				CreatedItems: []uint64{777},
			})
		}()

		return nil
	}

	t.Run("successful_craft_synchronous", func(t *testing.T) {
		t.Parallel()

		items := []uint64{100, 200, 300}

		result, err := tf.Craft(t.Context(), items, -1)
		if err != nil {
			t.Fatalf("Craft failed: %v", err)
		}

		if len(result) != 1 || result[0] != 777 {
			t.Errorf("expected new item [777], got %v", result)
		}

		sentBody := mCoord.lastSendPayload
		reader := bytes.NewReader(sentBody)

		var (
			recipe int16
			count  int16
		)

		_ = binary.Read(reader, binary.LittleEndian, &recipe)
		_ = binary.Read(reader, binary.LittleEndian, &count)

		if recipe != -1 || count != 3 {
			t.Errorf("invalid binary header sent to GC: recipe=%d, count=%d", recipe, count)
		}
	})
}

func TestTF2_CraftResponse_ValidPayload_PublishesCraftResponseEvent(t *testing.T) {
	t.Parallel()

	t.Run("handle_successful_response", func(t *testing.T) {
		t.Parallel()
		tf, ictx, _ := setupTF2(t)
		sub := ictx.Bus().Subscribe(&CraftResponseEvent{})

		resp := new(bytes.Buffer)
		_ = binary.Write(resp, binary.LittleEndian, int16(3))
		_ = binary.Write(resp, binary.LittleEndian, uint32(0))
		_ = binary.Write(resp, binary.LittleEndian, uint16(1))
		_ = binary.Write(resp, binary.LittleEndian, uint64(555))

		tf.handleCraftResponse(&protocol.GCPacket{Payload: resp.Bytes()})

		select {
		case ev := <-sub.C():
			craftEv := ev.(*CraftResponseEvent)
			assert.Equal(t, uint16(3), craftEv.BlueprintID)
			assert.Equal(t, []uint64{555}, craftEv.CreatedItems)
		case <-time.After(1 * time.Second):
			t.Fatal("CraftResponseEvent not received")
		}
	})

	t.Run("empty_response", func(t *testing.T) {
		t.Parallel()
		tf, ictx, _ := setupTF2(t)
		sub := ictx.Bus().Subscribe(&CraftResponseEvent{})

		tf.handleCraftResponse(&protocol.GCPacket{Payload: []byte{}})

		select {
		case <-sub.C():
			t.Error("Did not expect event for empty payload")
		default:
		}
	})
}

func TestTF2_ParseCraftResponse_EdgeCases_ReturnsSafely(t *testing.T) {
	t.Parallel()

	t.Run("short_payload", func(t *testing.T) {
		t.Parallel()

		res := parseCraftResponse([]byte{1, 2, 3})
		assert.Nil(t, res)
	})

	t.Run("incomplete_item_list", func(t *testing.T) {
		t.Parallel()

		resp := new(bytes.Buffer)
		_ = binary.Write(resp, binary.LittleEndian, int16(3))
		_ = binary.Write(resp, binary.LittleEndian, uint32(0))
		_ = binary.Write(resp, binary.LittleEndian, uint16(5))
		_ = binary.Write(resp, binary.LittleEndian, uint64(111))

		res := parseCraftResponse(resp.Bytes())
		assert.Equal(t, 1, len(res))
		assert.Equal(t, uint64(111), res[0])
	})
}

func TestTF2_HandleSchemaUpdate_SchemaPkt_EmitsSchemaUpdateEvent(t *testing.T) {
	t.Parallel()

	tf, ictx, _ := setupTF2(t)
	sub := ictx.Bus().Subscribe(&schema.UpdateRequestedEvent{})

	msg := &pb.CMsgUpdateItemSchema{
		ItemSchemaVersion: proto.Uint32(1234),
		ItemsGameUrl:      proto.String("http://example.com/items_game.txt"),
	}
	payload, _ := proto.Marshal(msg)

	tf.handleSchemaUpdate(&protocol.GCPacket{
		MsgType: uint32(pb.EGCItemMsg_k_EMsgGCUpdateItemSchema),
		Payload: payload,
	})

	select {
	case ev := <-sub.C():
		updateEv := ev.(*schema.UpdateRequestedEvent)
		assert.Equal(t, uint32(1234), updateEv.Version)
		assert.Equal(t, "http://example.com/items_game.txt", updateEv.ItemsGameURL)
	case <-time.After(1 * time.Second):
		t.Fatal("UpdateRequestedEvent not received")
	}
}

func TestTF2_SimpleGetters_ReturnsExpected(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	assert.Equal(t, "tf2", tf.Name())

	assert.False(t, tf.Connected())
	tf.state.Store(int32(Connected))
	assert.True(t, tf.Connected())

	cfg := AchievementConfig()
	assert.Equal(t, uint32(AppID), cfg.AppID)
	assert.Equal(t, 520, cfg.TotalCount)
}

func TestTF2_RemoveItemAttribute_SendsCorrectGCPacket(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)

	err := tf.RemoveItemAttribute(t.Context(), 12345, 999)
	require.NoError(t, err)

	assert.Equal(t, uint32(999), mCoord.GetLastSendMsgType())

	var sent pb.CMsgGCRemoveCustomizationAttributeSimple

	err = proto.Unmarshal(mCoord.lastSendPayload, &sent)
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), sent.GetItemId())
}

func TestTF2_MoveItems_BatchingAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("success_with_batching", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)

		items := make([]ItemPos, 0, 51)
		for i := range 51 {
			items = append(items, ItemPos{
				ID:       uint64(1000 + i),
				Position: uint32(i + 1),
			})
		}

		err := tf.MoveItems(t.Context(), items)
		require.NoError(t, err)
	})

	t.Run("error_path", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)

		tf.gc = &errorCoordinator{}

		err := tf.MoveItems(t.Context(), []ItemPos{{ID: 100, Position: 5}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send batch")
	})
}

func TestTF2_PlayGames_TransitionsStateAndPublishesEvents(t *testing.T) {
	t.Parallel()

	tf, ictx, _ := setupTF2(t)

	sub := ictx.Bus().Subscribe(&DisconnectedEvent{})

	tf.state.Store(int32(Connected))
	err := tf.PlayGames(t.Context(), []uint32{730})
	require.NoError(t, err)
	assert.False(t, tf.Connected())

	select {
	case <-sub.C():
	case <-time.After(2 * time.Second):
		t.Fatal("Expected DisconnectedEvent to be published")
	}

	err = tf.PlayGames(t.Context(), []uint32{AppID})
	require.NoError(t, err)
	assert.Equal(t, int32(Connecting), tf.state.Load())
}

func TestSOCache_FindWeaponsByClass_WithWeapon(t *testing.T) {
	t.Parallel()

	tf, _, _ := setupTF2(t)
	cache := tf.Cache()

	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      100,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout", "Soldier"},
		},
	}
	s := schema.New(raw)
	cache.schema = s

	cache.items[1] = &Item{
		ID:         1,
		DefIndex:   100,
		IsTradable: true,
	}

	weapons := cache.FindWeaponsByClass("Scout")
	require.Len(t, weapons, 1)
	assert.Equal(t, uint64(1), weapons[0].ID)

	weaponsNone := cache.FindWeaponsByClass("Medic")
	assert.Len(t, weaponsNone, 0)
}

type errorCoordinator struct {
	mockCoordinator
}

func (e *errorCoordinator) Send(ctx context.Context, appID, msgType uint32, msg proto.Message) error {
	return errors.New("gc send error")
}
