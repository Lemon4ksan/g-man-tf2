// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tf2 integrates with the Team Fortress 2 Game Coordinator.
package tf2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"time"

	"github.com/lemon4ksan/foundation/async/event"
	"github.com/lemon4ksan/foundation/async/fsm"
	"github.com/lemon4ksan/foundation/async/task"
	"github.com/lemon4ksan/g-man/pkg/behavior/achievements"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/protobuf/custom"
	pb_steam "github.com/lemon4ksan/g-man/protobuf/steam"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	pb "github.com/lemon4ksan/g-man-tf2/protobuf/tf2"
)

const (
	AppID      = 440
	ModuleName = "tf2"
)

var (
	ErrGCNotConnected = errors.New("tf2: GC is not connected")
	ErrCraftTimeout   = errors.New("craft: timeout waiting for GC response")
)

func WithModule() steam.Option {
	return steam.WithModule(New())
}

func From(c *steam.Client) *TF2 {
	return steam.GetModule[*TF2](c)
}

func AchievementConfig() achievements.Config {
	return achievements.Config{
		AppID:            AppID,
		TotalCount:       520,
		MinTargetPercent: 0.70,
		MaxTargetPercent: 0.82,
		UnlockChance:     0.40,
		BreakChance:      0.02,
		CheckInterval:    24 * time.Hour,
		InitialDelay:     5 * time.Second,
		AchievementPool: [][]uint32{
			{1001, 1041},
			{1101, 1142},
			{1201, 1240},
			{1301, 1340},
			{1401, 1440},
			{1501, 1540},
			{1601, 1640},
			{1701, 1740},
			{1801, 1840},
			{1901, 1921},
			{2201, 2212},
			{2301, 2352},
		},
	}
}

type State int32

const (
	Disconnected State = iota
	Connecting
	Connected
)

type Event int32

const (
	EventConnect Event = iota
	EventConnected
	EventServerGoodbye
	EventDisconnect
)

type CoordinatorProvider interface {
	Send(ctx context.Context, appID, msgType uint32, msg proto.Message) error
	SendRaw(ctx context.Context, appID, msgType uint32, payload []byte) error
	Call(ctx context.Context, appID, msgType uint32, msg proto.Message, cb task.Callback[*protocol.GCPacket]) error
	CallRaw(ctx context.Context, appID, msgType uint32, payload []byte, cb task.Callback[*protocol.GCPacket]) error
}

type AppsProvider interface {
	PlayGames(ctx context.Context, appIDs []uint32, forceKick bool) error
}

type SchemaProvider interface {
	Get() *schema.Schema
}

// TF2 coordinates sessions, achievements, and commands with the Game Coordinator.
type TF2 struct {
	module.Base

	steamID id.ID
	gc      CoordinatorProvider
	service service.Doer
	apps    AppsProvider

	fsm        *fsm.FSM[State, Event]
	cache      *SOCache
	schema     SchemaProvider
	keepActive bool
}

func New() *TF2 {
	mach := fsm.NewFSM[State, Event](Disconnected)
	mach.AddRules(
		fsm.TransitionRule[State, Event]{From: Disconnected, Event: EventConnect, To: Connecting},
		fsm.TransitionRule[State, Event]{From: Connecting, Event: EventConnected, To: Connected},
		fsm.TransitionRule[State, Event]{From: Connected, Event: EventServerGoodbye, To: Connecting},
		fsm.TransitionRule[State, Event]{From: Connecting, Event: EventDisconnect, To: Disconnected},
		fsm.TransitionRule[State, Event]{From: Connected, Event: EventDisconnect, To: Disconnected},
		fsm.TransitionRule[State, Event]{From: Disconnected, Event: EventDisconnect, To: Disconnected},
	)

	return &TF2{
		Base: module.New(ModuleName).WithDeps(gc.ModuleName, apps.ModuleName, schema.ModuleName),
		fsm:  mach,
	}
}

func (t *TF2) Name() string { return ModuleName }

func (t *TF2) Init(init module.InitContext) error {
	if err := t.Base.Init(init); err != nil {
		return err
	}

	var err error

	t.gc, err = module.Get[CoordinatorProvider](init, gc.ModuleName)
	if err != nil {
		return err
	}

	t.service = init.Service()

	t.apps, err = module.Get[AppsProvider](init, apps.ModuleName)
	if err != nil {
		return err
	}

	t.schema, err = module.Get[SchemaProvider](init, schema.ModuleName)
	if err != nil {
		return err
	}

	t.cache = NewSOCache(t.gc, WithBus(t.Bus), WithLogger(t.Logger), WithSchema(t.schema.Get()))

	return nil
}

func (t *TF2) Start(ctx context.Context) error {
	if err := t.Base.Start(ctx); err != nil {
		return err
	}

	sub := t.Bus.Subscribe(&gc.MessageEvent{}, &schema.ReadyEvent{}, &schema.UpdatedEvent{})
	t.Go(func(ctx context.Context) {
		t.messageLoop(ctx, sub)
	})

	return nil
}

func (t *TF2) StartAuthed(ctx context.Context, authCtx module.AuthContext) error {
	if authCtx != nil {
		t.steamID = authCtx.SteamID()
	}

	if err := t.apps.PlayGames(ctx, []uint32{AppID}, false); err != nil {
		return err
	}

	_ = t.fsm.Transition(ctx, EventConnect)
	t.Go(func(ctx context.Context) {
		t.helloLoop(ctx)
	})

	return nil
}

func (t *TF2) Close() error {
	_ = t.fsm.Transition(context.Background(), EventDisconnect)

	return t.Base.Close()
}

func (t *TF2) Connected() bool { return t.fsm.CurrentState() == Connected }

func (t *TF2) Cache() *SOCache { return t.cache }

// SetKeepActive forces TF2 to stay in the active games list even when other apps launch/stop.
func (t *TF2) SetKeepActive(val bool) {
	t.keepActive = val
}

// PlayGames launches or stops TF2 and manages GC connection state machine transitions.
// Satisfies achievements.Provider.
func (t *TF2) PlayGames(ctx context.Context, appIDs []uint32) error {
	if t.keepActive && !slices.Contains(appIDs, AppID) {
		appIDs = append(slices.Clone(appIDs), AppID)
	}

	err := t.apps.PlayGames(ctx, appIDs, false)
	if err != nil {
		return err
	}

	hasTF2 := slices.Contains(appIDs, AppID)
	if !hasTF2 {
		if t.fsm.CurrentState() != Disconnected {
			_ = t.fsm.Transition(ctx, EventDisconnect)
			t.Logger.Info("Game quit requested, disconnecting from TF2 GC")
			t.Bus.Publish(&DisconnectedEvent{})
		}

		return nil
	}

	if t.fsm.CurrentState() == Disconnected {
		_ = t.fsm.Transition(ctx, EventConnect)
		t.Logger.Info("Game launch requested, connecting to TF2 GC")
		t.Go(func(ctx context.Context) {
			t.helloLoop(ctx)
		})
	} else {
		_ = t.fsm.Transition(ctx, EventConnect)
	}

	return nil
}

// AwardAchievement unlocks a TF2 achievement by ID via Steam Store User Stats.
// Satisfies achievements.Provider.
func (t *TF2) AwardAchievement(ctx context.Context, achievementID uint32) error {
	if t.fsm.CurrentState() != Connected {
		return ErrGCNotConnected
	}

	req := &custom.CMsgClientStoreUserStats{
		GameId: proto.Uint64(AppID),
		Achievements: []*custom.CMsgClientStoreUserStats_Achievement{
			{
				AchievementId: new(achievementID),
				UnlockTime:    []uint32{0xFFFFFFFF},
			},
		},
	}

	_, err := service.LegacyProto[service.NoResponse](
		ctx, t.service, enums.EMsg_ClientStoreUserStats, protoadapt.MessageV2Of(req), service.WithRoutingAppID(AppID),
	)

	return err
}

// SetStat updates a TF2 statistic value.
func (t *TF2) SetStat(ctx context.Context, statID, value uint32) error {
	if t.fsm.CurrentState() != Connected {
		return ErrGCNotConnected
	}

	req := &custom.CMsgClientStoreUserStats{
		GameId: proto.Uint64(AppID),
		Stats: []*custom.CMsgClientStoreUserStats_Stat{
			{
				StatId:    new(statID),
				StatValue: new(value),
			},
		},
	}

	_, err := service.LegacyProto[service.NoResponse](
		ctx, t.service, enums.EMsg_ClientStoreUserStats, protoadapt.MessageV2Of(req), service.WithRoutingAppID(AppID),
	)

	return err
}

// GetCurrentAchievements retrieves a map of unlocked TF2 achievement IDs.
// Satisfies achievements.Provider.
func (t *TF2) GetCurrentAchievements(ctx context.Context) (map[uint32]bool, error) {
	if t.fsm.CurrentState() != Connected {
		return nil, ErrGCNotConnected
	}

	req := &pb_steam.CMsgClientGetUserStats{GameId: proto.Uint64(AppID)}

	resp, err := service.LegacyProto[pb_steam.CMsgClientGetUserStatsResponse](
		ctx, t.service, enums.EMsg_ClientGetUserStats, req, service.WithRoutingAppID(AppID),
	)
	if err != nil {
		return nil, err
	}

	baseIDs := map[uint32]uint32{
		266: 1001, 267: 1033, 268: 1101, 269: 1133, 313: 1201, 314: 1233,
	}

	unlocked := make(map[uint32]bool)

	for _, block := range resp.GetAchievementBlocks() {
		baseID, exists := baseIDs[block.GetAchievementId()]
		if !exists {
			continue
		}

		for idx, unlockTime := range block.GetUnlockTime() {
			if unlockTime > 0 {
				unlocked[baseID+uint32(idx)] = true
			}
		}
	}

	return unlocked, nil
}

func (t *TF2) Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error) {
	if t.fsm.CurrentState() != Connected {
		return nil, ErrGCNotConnected
	}

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, recipe)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(items)))

	for _, id := range items {
		_ = binary.Write(buf, binary.LittleEndian, id)
	}

	sub := t.Bus.Subscribe(&CraftResponseEvent{})
	defer sub.Unsubscribe()

	err := t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCCraft), buf.Bytes())
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(15 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case ev, ok := <-sub.C():
			if !ok {
				return nil, errors.New("craft: subscription closed")
			}

			if craftEv, ok := ev.(*CraftResponseEvent); ok {
				return craftEv.CreatedItems, nil
			}

		case <-timeout.C:
			return nil, ErrCraftTimeout

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (t *TF2) helloLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	t.sendHello(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.fsm.CurrentState() == Connecting {
				t.sendHello(ctx)
			}
		}
	}
}

func (t *TF2) sendHello(ctx context.Context) {
	msg := &pb.CMsgClientHello{Version: proto.Uint32(65580)}
	_ = t.gc.Send(ctx, AppID, uint32(pb.EGCBaseClientMsg_k_EMsgGCClientHello), msg)
}

func (t *TF2) messageLoop(ctx context.Context, sub *event.Subscription) {
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.C():
			if !ok {
				return
			}

			switch e := ev.(type) {
			case *gc.MessageEvent:
				if e.Packet.AppID == AppID {
					t.routePacket(ctx, e.Packet)
				}
			case *schema.ReadyEvent, *schema.UpdatedEvent:
				t.cache.UpdateSchema(t.schema.Get())
			}
		}
	}
}

func (t *TF2) routePacket(ctx context.Context, pkt *protocol.GCPacket) {
	switch pb.EGCBaseClientMsg(pkt.MsgType) {
	case pb.EGCBaseClientMsg_k_EMsgGCClientWelcome:
		if t.fsm.Transition(context.Background(), EventConnected) == nil {
			t.Bus.Publish(&ConnectedEvent{})
		}
	case pb.EGCBaseClientMsg_k_EMsgGCClientGoodbye:
		if t.fsm.Transition(context.Background(), EventServerGoodbye) == nil {
			t.cache.Unload()
			t.Bus.Publish(&DisconnectedEvent{})
		}
	}

	switch pb.ESOMsg(pkt.MsgType) {
	case pb.ESOMsg_k_ESOMsg_CacheSubscribed:
		t.cache.handleSubscribed(pkt)
	case pb.ESOMsg_k_ESOMsg_Create,
		pb.ESOMsg_k_ESOMsg_Update,
		pb.ESOMsg_k_ESOMsg_Destroy,
		pb.ESOMsg_k_ESOMsg_UpdateMultiple:
		t.cache.handleSOUpdate(pkt)
	case pb.ESOMsg_k_ESOMsg_CacheSubscriptionCheck:
		t.cache.handleSOCacheCheck(ctx, pkt)
	case pb.ESOMsg_k_ESOMsg_CacheSubscribedUpToDate:
		t.cache.handleUpToDate(pkt)
	}

	switch pb.EGCItemMsg(pkt.MsgType) {
	case pb.EGCItemMsg_k_EMsgGCCraftResponse:
		if len(pkt.Payload) >= 8 {
			blueprint := binary.LittleEndian.Uint16(pkt.Payload[0:2])
			idCount := binary.LittleEndian.Uint16(pkt.Payload[6:8])
			offset := 8

			var items []uint64
			for i := 0; i < int(idCount) && offset+8 <= len(pkt.Payload); i++ {
				items = append(items, binary.LittleEndian.Uint64(pkt.Payload[offset:offset+8]))
				offset += 8
			}

			t.Bus.Publish(&CraftResponseEvent{
				BlueprintID:  blueprint,
				CreatedItems: items,
			})
			t.Bus.Publish(&CraftingCompleteEvent{
				RecipeID:     int16(blueprint),
				ItemsCreated: items,
			})
		}

	case pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest:
		if len(pkt.Payload) >= 12 {
			tradeID := binary.LittleEndian.Uint32(pkt.Payload[0:4])
			steamID := binary.LittleEndian.Uint64(pkt.Payload[4:12])
			t.Bus.Publish(&TradeRequestEvent{
				TradeID: tradeID,
				SteamID: steamID,
			})
		}

	case pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeResponse:
		if len(pkt.Payload) >= 8 {
			response := binary.LittleEndian.Uint32(pkt.Payload[0:4])
			tradeID := binary.LittleEndian.Uint32(pkt.Payload[4:8])
			t.Bus.Publish(&TradeResponseEvent{
				Response: response,
				TradeID:  tradeID,
			})
		}

	case pb.EGCItemMsg_k_EMsgGCBackpackSortFinished:
		t.Bus.Publish(&BackpackSortFinishedEvent{})
	case pb.EGCItemMsg_k_EMsgGCClientDisplayNotification:
		var msg pb.CMsgGCClientDisplayNotification
		if proto.Unmarshal(pkt.Payload, &msg) == nil {
			replacements := make(map[string]string)
			keys := msg.GetBodySubstringKeys()

			values := msg.GetBodySubstringValues()
			for i := 0; i < len(keys) && i < len(values); i++ {
				replacements[keys[i]] = values[i]
			}

			t.Bus.Publish(&NotificationEvent{
				TitleLocalizationKey: msg.GetNotificationTitleLocalizationKey(),
				BodyLocalizationKey:  msg.GetNotificationBodyLocalizationKey(),
				ReplacementStrings:   replacements,
			})
		}

	case pb.EGCItemMsg_k_EMsgGCTFSpecificItemBroadcast:
		var msg pb.CMsgGCTFSpecificItemBroadcast
		if proto.Unmarshal(pkt.Payload, &msg) == nil {
			t.Bus.Publish(&ItemBroadcastEvent{
				UserName:       msg.GetUserName(),
				WasDestruction: msg.GetWasDestruction(),
				DefIndex:       msg.GetItemDefIndex(),
			})
		}
	}
}
