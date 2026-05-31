// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior/achievements"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/jobs"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/protobuf/custom"
	pb_steam "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

const (
	// AppID represents the Steam App ID for Team Fortress 2.
	AppID = 440
	// ModuleName is the unique registration name of the TF2 module.
	ModuleName string = "tf2"
)

// GetModule is a generic helper to retrieve a typed module from the init context.
func GetModule[T any](init module.InitContext, name string) (T, error) {
	var zero T

	mod := init.Module(name)
	if mod == nil {
		return zero, fmt.Errorf("module %q not registered", name)
	}

	typed, ok := mod.(T)
	if !ok {
		return zero, fmt.Errorf("module %q has invalid type %T (expected %T)", name, mod, zero)
	}

	return typed, nil
}

// WithModule returns an option that registers the [TF2] module with the client.
func WithModule() steam.Option {
	return func(c *steam.Client) {
		c.RegisterModule(New())
	}
}

// From returns the [TF2] module instance from the [steam.Client].
func From(c *steam.Client) *TF2 {
	return steam.GetModule[*TF2](c)
}

// AchievementConfig returns standard TF2 configuration options for the achievements manager.
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
			{2401, 2412},
			{2701, 2705},
			{2801, 2805},
		},
	}
}

// State represents the Game Coordinator session connection status.
type State int32

const (
	// Disconnected indicates the Game Coordinator session is not active.
	Disconnected State = iota
	// Connecting indicates a ClientHello handshake is in progress.
	Connecting
	// Connected indicates the Game Coordinator session is fully established.
	Connected
)

// CoordinatorProvider defines the interface for communicating with the Game Coordinator.
type CoordinatorProvider interface {
	Send(ctx context.Context, appID, msgType uint32, msg proto.Message) error
	SendRaw(ctx context.Context, appID, msgType uint32, payload []byte) error
	Call(ctx context.Context, appID, msgType uint32, msg proto.Message, cb jobs.Callback[*protocol.GCPacket]) error
	CallRaw(ctx context.Context, appID, msgType uint32, payload []byte, cb jobs.Callback[*protocol.GCPacket]) error
}

// AppsProvider defines the interface for starting and stopping games.
type AppsProvider interface {
	PlayGames(ctx context.Context, appIDs []uint32, forceKick bool) error
}

// SchemaProvider defines the interface for retrieving the active item schema.
type SchemaProvider interface {
	Get() *schema.Schema
}

// TF2 manages the session and coordinates commands with the Game Coordinator.
// It maintains the [SOCache] inventory view and publishes relevant lifecycle events.
type TF2 struct {
	module.Base

	steamID id.ID
	gc      CoordinatorProvider
	service service.Doer
	apps    AppsProvider

	state  atomic.Int32
	cache  *SOCache
	schema SchemaProvider
}

// New creates a new TF2 module.
func New() *TF2 {
	return &TF2{
		Base: module.New(ModuleName).WithDeps(gc.ModuleName, apps.ModuleName, schema.ModuleName),
	}
}

// Name returns the unique module identifier.
func (t *TF2) Name() string { return ModuleName }

// Init initializes the [TF2] module and registers internal handlers.
// Returns an error if any of the mandatory dependency modules are missing.
func (t *TF2) Init(init module.InitContext) error {
	if err := t.Base.Init(init); err != nil {
		return err
	}

	var err error

	t.gc, err = GetModule[CoordinatorProvider](init, gc.ModuleName)
	if err != nil {
		return err
	}

	t.service = init.Service()

	t.apps, err = GetModule[AppsProvider](init, apps.ModuleName)
	if err != nil {
		return err
	}

	t.schema, err = GetModule[SchemaProvider](init, schema.ModuleName)
	if err != nil {
		return err
	}

	sch := t.schema.Get()
	t.cache = NewSOCache(t.gc, WithBus(t.Bus), WithLogger(t.Logger), WithSchema(sch))

	sub := t.Bus.Subscribe(&gc.MessageEvent{}, &schema.ReadyEvent{}, &schema.UpdatedEvent{})
	t.Go(func(ctx context.Context) {
		t.messageLoop(ctx, sub)
	})

	return nil
}

// StartAuthed signals Steam to launch the TF2 client and starts the hello loop.
// Returns an error if the context is cancelled or the launch request fails.
func (t *TF2) StartAuthed(ctx context.Context, authCtx module.AuthContext) error {
	if authCtx != nil {
		t.steamID = authCtx.SteamID()
	}

	if err := t.apps.PlayGames(ctx, []uint32{AppID}, false); err != nil {
		return err
	}

	t.state.Store(int32(Connecting))
	t.Go(func(ctx context.Context) {
		t.helloLoop(ctx)
	})

	return nil
}

// Close terminates the session and cleans up active background loops.
func (t *TF2) Close() error {
	t.state.Store(int32(Disconnected))
	return t.Base.Close()
}

// Connected returns true if the module has established a session with the Game Coordinator.
func (t *TF2) Connected() bool {
	return t.state.Load() == int32(Connected)
}

// Cache returns the active [SOCache] inventory instance.
func (t *TF2) Cache() *SOCache {
	return t.cache
}

// AwardAchievement requests Steam to unlock the specified TF2 achievement.
// Returns an error if the GC is disconnected or the API request fails.
func (t *TF2) AwardAchievement(ctx context.Context, achievementID uint32) error {
	if t.state.Load() != int32(Connected) {
		return errors.New("tf2: GC is not connected")
	}

	req := &custom.CMsgClientStoreUserStats{
		GameId: proto.Uint64(AppID),
		Achievements: []*custom.CMsgClientStoreUserStats_Achievement{
			{
				AchievementId: proto.Uint32(achievementID),
				UnlockTime:    []uint32{0xFFFFFFFF},
			},
		},
	}

	_, err := service.LegacyProto[service.NoResponse](
		ctx,
		t.service,
		enums.EMsg_ClientStoreUserStats,
		protoadapt.MessageV2Of(req),
		service.WithRoutingAppID(AppID),
	)

	return err
}

// SetStat requests Steam to update the specified TF2 gameplay statistic.
// Returns an error if the GC is disconnected or the API request fails.
func (t *TF2) SetStat(ctx context.Context, statID, value uint32) error {
	if t.state.Load() != int32(Connected) {
		return errors.New("tf2: GC is not connected")
	}

	req := &custom.CMsgClientStoreUserStats{
		GameId: proto.Uint64(AppID),
		Stats: []*custom.CMsgClientStoreUserStats_Stat{
			{
				StatId:    proto.Uint32(statID),
				StatValue: proto.Uint32(value),
			},
		},
	}

	_, err := service.LegacyProto[service.NoResponse](
		ctx,
		t.service,
		enums.EMsg_ClientStoreUserStats,
		protoadapt.MessageV2Of(req),
		service.WithRoutingAppID(AppID),
	)

	return err
}

// GetCurrentAchievements retrieves a map of currently unlocked achievements.
// Returns an error if the GC is disconnected, or if the request fails.
func (t *TF2) GetCurrentAchievements(ctx context.Context) (map[uint32]bool, error) {
	if t.state.Load() != int32(Connected) {
		return nil, errors.New("tf2: GC is not connected")
	}

	t.Logger.DebugContext(ctx, "Querying achievements progress", log.Uint64("steam_idForUser", t.steamID.Uint64()))

	req := &pb_steam.CMsgClientGetUserStats{
		GameId: proto.Uint64(AppID),
	}

	resp, err := service.LegacyProto[pb_steam.CMsgClientGetUserStatsResponse](
		ctx,
		t.service,
		enums.EMsg_ClientGetUserStats,
		req,
		service.WithRoutingAppID(AppID),
	)
	if err != nil {
		return nil, err
	}

	baseIDs := map[uint32]uint32{
		266: 1001,
		267: 1033,
		268: 1101,
		269: 1133,
		313: 1201,
		314: 1233,
		333: 1301,
		348: 1333,
		359: 1401,
		360: 1433,
		386: 1501,
		405: 1533,
		408: 1601,
		684: 1633,
		687: 1701,
		748: 1733,
		757: 1801,
	}

	unlocked := make(map[uint32]bool)
	for _, block := range resp.GetAchievementBlocks() {
		statID := block.GetAchievementId()

		baseID, exists := baseIDs[statID]
		if !exists {
			continue
		}

		for idx, unlockTime := range block.GetUnlockTime() {
			if unlockTime > 0 {
				achievementID := baseID + uint32(idx)
				unlocked[achievementID] = true
			}
		}
	}

	return unlocked, nil
}

// PlayGames launches or stops TF2 and coordinates Game Coordinator states.
// Returns an error if the play request fails or the context is cancelled.
func (t *TF2) PlayGames(ctx context.Context, appIDs []uint32) error {
	err := t.apps.PlayGames(ctx, appIDs, false)
	if err == nil {
		hasTF2 := false
		for _, id := range appIDs {
			if id == AppID {
				hasTF2 = true
				break
			}
		}

		if !hasTF2 {
			oldState := t.state.Swap(int32(Disconnected))
			if oldState != int32(Disconnected) {
				t.Logger.Info("Game quit requested, disconnecting from TF2 GC")
				t.Bus.Publish(&DisconnectedEvent{})
			}
		} else {
			oldState := t.state.Swap(int32(Connecting))
			if oldState == int32(Disconnected) {
				t.Logger.Info("Game launch requested, connecting to TF2 GC")
				t.Go(func(ctx context.Context) {
					t.helloLoop(ctx)
				})
			}
		}
	}

	return err
}

// Craft sends a crafting request to the Game Coordinator and blocks waiting for response.
// It times out after 15 seconds if no response is received.
// Returns the asset IDs of the newly created items, or an error.
func (t *TF2) Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error) {
	if t.state.Load() != int32(Connected) {
		return nil, errors.New("tf2: GC is not connected")
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
				return nil, errors.New("craft: event bus subscription closed")
			}

			craftEv, ok := ev.(*CraftResponseEvent)
			if !ok {
				continue
			}

			return craftEv.CreatedItems, nil

		case <-timeout.C:
			return nil, errors.New("craft: timeout waiting for GC response")

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
			if t.state.Load() != int32(Connecting) {
				continue
			}

			t.sendHello(ctx)
		}
	}
}

func (t *TF2) sendHello(ctx context.Context) {
	msg := &pb.CMsgClientHello{
		Version: proto.Uint32(65580),
	}

	err := t.gc.Send(ctx, AppID, uint32(pb.EGCBaseClientMsg_k_EMsgGCClientHello), msg)
	if err != nil {
		t.Logger.ErrorContext(ctx, "Failed to send ClientHello to GC", log.Err(err))
	} else {
		t.Logger.DebugContext(ctx, "Sent ClientHello to TF2 GC")
	}
}

func (t *TF2) messageLoop(ctx context.Context, sub *bus.Subscription) {
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
			case *schema.ReadyEvent:
				t.Logger.Info("TF2 Schema is ready, updating SOCache schema")
				t.cache.UpdateSchema(t.schema.Get())
			case *schema.UpdatedEvent:
				t.Logger.Info("TF2 Schema is updated, updating SOCache schema")
				t.cache.UpdateSchema(t.schema.Get())
			}
		}
	}
}

func (t *TF2) routePacket(ctx context.Context, pkt *protocol.GCPacket) {
	switch pb.EGCBaseClientMsg(pkt.MsgType) {
	case pb.EGCBaseClientMsg_k_EMsgGCClientWelcome:
		t.handleWelcome(pkt)
	case pb.EGCBaseClientMsg_k_EMsgGCClientGoodbye:
		t.handleGoodbye(pkt)
	}

	switch pb.EGCItemMsg(pkt.MsgType) {
	case pb.EGCItemMsg_k_EMsgGCUpdateItemSchema:
		t.handleSchemaUpdate(pkt)
	case pb.EGCItemMsg_k_EMsgGCCraftResponse:
		t.handleCraftResponse(pkt)
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
}

func (t *TF2) handleWelcome(pkt *protocol.GCPacket) {
	msg := &pb.CMsgClientWelcome{}
	if err := proto.Unmarshal(pkt.Payload, msg); err != nil {
		t.Logger.Error("Failed to unmarshal Welcome", log.Err(err))
		return
	}

	if t.state.CompareAndSwap(int32(Connecting), int32(Connected)) {
		t.Logger.Info("Connected to TF2 Game Coordinator", log.Uint32("version", msg.GetVersion()))
		t.Bus.Publish(&ConnectedEvent{Version: msg.GetVersion()})
	}
}

func (t *TF2) handleGoodbye(_ *protocol.GCPacket) {
	t.Logger.Warn("Disconnected from TF2 Game Coordinator (Server Goodbye)")

	if t.state.CompareAndSwap(int32(Connected), int32(Connecting)) {
		t.Bus.Publish(&DisconnectedEvent{})
	}
}

func (t *TF2) handleSchemaUpdate(pkt *protocol.GCPacket) {
	msg := &pb.CMsgUpdateItemSchema{}
	if err := proto.Unmarshal(pkt.Payload, msg); err != nil {
		t.Logger.Error("Failed to unmarshal UpdateItemSchema", log.Err(err))
		return
	}

	t.Logger.Info("Received item schema update notification from GC",
		log.Uint32("version", msg.GetItemSchemaVersion()),
	)

	t.Bus.Publish(&schema.UpdateRequestedEvent{
		Version:      msg.GetItemSchemaVersion(),
		ItemsGameURL: msg.GetItemsGameUrl(),
	})
}

func (t *TF2) handleCraftResponse(pkt *protocol.GCPacket) {
	items := parseCraftResponse(pkt.Payload)
	if len(items) > 0 || len(pkt.Payload) >= 2 {
		blueprint := binary.LittleEndian.Uint16(pkt.Payload[0:])
		t.Bus.Publish(&CraftResponseEvent{
			BlueprintID:  blueprint,
			CreatedItems: items,
		})
	}
}

func parseCraftResponse(payload []byte) []uint64 {
	if len(payload) < 8 {
		return nil
	}

	count := int(binary.LittleEndian.Uint16(payload[6:]))
	items := make([]uint64, 0, count)

	for i := range count {
		offset := 8 + (i * 8)
		if len(payload) < offset+8 {
			break
		}

		items = append(items, binary.LittleEndian.Uint64(payload[offset:]))
	}

	return items
}
