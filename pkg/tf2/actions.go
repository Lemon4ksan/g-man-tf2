// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
)

// RemoveItemName requests the Game Coordinator to strip a custom name from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveItemName(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 0 // m_bDescription = false (we want to remove the Name)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemDescription requests the Game Coordinator to strip a custom description from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveItemDescription(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 1 // m_bDescription = true (we want to remove the Description)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemPaint requests the Game Coordinator to strip custom paint from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveItemPaint(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemPaint), req)
}

// RemoveMakersMark requests the Game Coordinator to strip the custom creator tag from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveMakersMark(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveMakersMark), req)
}

// ResetStrangeScores requests the Game Coordinator to reset the strange counters on an item back to zero.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) ResetStrangeScores(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCResetStrangeScores{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCResetStrangeScores), req)
}

// RemoveKillstreak requests the Game Coordinator to strip a killstreak kit from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveKillstreak(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), req)
}

// RemoveFestivizer requests the Game Coordinator to strip a festivizer from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveFestivizer(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveFestivizer), req)
}

// RemoveGiftedBy requests the Game Coordinator to strip the custom gifter tag from an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveGiftedBy(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveGiftedBy), req)
}

// RemoveItemAttribute requests the Game Coordinator to strip a specific customization attribute from an item.
// This is used for removing spells or strange parts.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) RemoveItemAttribute(ctx context.Context, itemID uint64, attributeID uint32) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, attributeID, req)
}

// AcknowledgeItem acknowledges a newly acquired or traded item to the Game Coordinator.
// It moves the item from position 0 to slot 1 to complete the in-game notification process.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) AcknowledgeItem(ctx context.Context, itemID uint64) error {
	return t.SetItemPosition(ctx, itemID, 1)
}

// NameItem applies a custom name tag to an item.
// Returns an error if the toolID or itemID is 0, if the name is empty or too long, or if the context is cancelled.
func (t *TF2) NameItem(ctx context.Context, toolID, itemID uint64, name string) error {
	return t.nameOrDescribeItem(ctx, toolID, itemID, name, false)
}

// DescribeItem applies a custom description tag to an item.
// Returns an error if the toolID or itemID is 0, if the description is empty or too long, or if the context is cancelled.
func (t *TF2) DescribeItem(ctx context.Context, toolID, itemID uint64, description string) error {
	return t.nameOrDescribeItem(ctx, toolID, itemID, description, true)
}

func (t *TF2) nameOrDescribeItem(ctx context.Context, toolID, itemID uint64, text string, isDescription bool) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, toolID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	if isDescription {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	buf.WriteString(text)
	buf.WriteByte(0) // Null terminator

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCNameItem), buf.Bytes())
}

// AcknowledgeAll acknowledges all unacknowledged items in the backpack by moving them to the first available slots.
// Unacknowledged items are identified by having a position of 0 or a new item bit mask.
// Returns an error if the connection is down or if the context is cancelled.
func (t *TF2) AcknowledgeAll(ctx context.Context) error {
	items := t.cache.GetItems()

	var toMove []ItemPos

	nextSlot := uint32(1)

	for _, it := range items {
		isNew := (it.Inventory >> 30) & 1
		if it.Position() == 0 || isNew == 1 {
			toMove = append(toMove, ItemPos{ID: it.ID, Position: nextSlot})
			nextSlot++
		}
	}

	if len(toMove) == 0 {
		return nil
	}

	return t.MoveItems(ctx, toMove)
}

// SetItemStyle updates the cosmetic style index of the specified item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) SetItemStyle(ctx context.Context, itemID uint64, style uint8) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = style

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemStyle), data)
}

// SetItemPosition moves an item to the specified page/slot position index in the backpack.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) SetItemPosition(ctx context.Context, itemID uint64, position uint32) error {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	binary.LittleEndian.PutUint32(data[8:12], position)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition), data)
}

// DeleteItem permanently deletes an item from the inventory state.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) DeleteItem(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDelete), buf.Bytes())
}

// SetUnusualEffectOffset adjusts the vertical placement offset of an Unusual effect attached to an item.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) SetUnusualEffectOffset(ctx context.Context, itemID uint64, offset float32) error {
	req := &pb.CMsgSetItemEffectVerticalOffset{
		ItemId: proto.Uint64(itemID),
		Offset: proto.Float32(offset),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemEffectVerticalOffset), req)
}

// TransferStrangeCount transfers stats from a source strange item to a destination strange item using a tool.
// Returns an error if any ID is 0, if the items are incompatible, or if the context is cancelled.
func (t *TF2) TransferStrangeCount(ctx context.Context, toolID, srcID, destID uint64) error {
	req := &pb.CMsgApplyStrangeCountTransfer{
		ToolItemId:     proto.Uint64(toolID),
		ItemSrcItemId:  proto.Uint64(srcID),
		ItemDestItemId: proto.Uint64(destID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangeCountTransfer), req)
}

// ShuffleCrate shuffles the contents of a crate using a shuffle tool and custom user code.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) ShuffleCrate(ctx context.Context, itemID uint64, userCode string) error {
	req := &pb.CMsgGCShuffleCrateContents{
		CrateItemId:    proto.Uint64(itemID),
		UserCodeString: proto.String(userCode),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCShuffleCrateContents), req)
}

// ApplyAutograph applies an autograph tool to an item.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) ApplyAutograph(ctx context.Context, toolID, itemID uint64) error {
	req := &pb.CMsgApplyAutograph{
		AutographItemId: proto.Uint64(toolID),
		ItemItemId:      proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyAutograph), req)
}

// RequestMarketData requests official Steam Market pricing details from the Game Coordinator.
// Returns an error if the connection is down, if the currency is unsupported, or if the context is cancelled.
func (t *TF2) RequestMarketData(ctx context.Context, currency uint32) error {
	req := &pb.CMsgGCClientMarketDataRequest{
		UserCurrency: proto.Uint32(currency),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCClientRequestMarketData), req)
}

// ReportPlayer submits a player report for a specified reason to the Game Coordinator.
// Returns an error if target accountID is 0, if the reason is nil, or if the context is cancelled.
func (t *TF2) ReportPlayer(ctx context.Context, accountID uint32, reason *pb.CMsgGC_ReportPlayer_EReason) error {
	req := &pb.CMsgGC_ReportPlayer{
		AccountIdTarget: proto.Uint32(accountID),
		Reason:          reason,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGC_ReportPlayer), req)
}

// RequestFriends requests TF2-specific profile data for a slice of Steam account IDs.
// Returns an error if the connection is down, if the slice is empty, or if the context is cancelled.
func (t *TF2) RequestFriends(ctx context.Context, accountIDs []uint32) error {
	req := &pb.CMsgTFRequestTF2Friends{
		AccountIds: accountIDs,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCRequestTF2Friends), req)
}

// UseItem triggers an interactive use action on an item (e.g., opening bags or applying badges).
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) UseItem(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgUseItem{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUseItemRequest), req)
}

// ApplyStrangePart applies a strange part modifier item to a strange target item.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) ApplyStrangePart(ctx context.Context, itemID, partID uint64) error {
	req := &pb.CMsgApplyStrangePart{
		ItemItemId:        proto.Uint64(itemID),
		StrangePartItemId: proto.Uint64(partID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangePart), req)
}

// ApplyStrangifier applies a strangifier or unusualifier tool to a cosmetic item.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) ApplyStrangifier(ctx context.Context, itemID, toolID uint64) error {
	req := &pb.CMsgApplyToolToItem{
		ToolItemId:    proto.Uint64(toolID),
		SubjectItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyXifier), req)
}

// SortBackpack requests the Game Coordinator to sort the backpack by a specific sort type ID.
// Returns an error if the connection is down or if the context is cancelled.
func (t *TF2) SortBackpack(ctx context.Context, sortType uint32) error {
	req := &pb.CMsgSortItems{
		SortType: proto.Uint32(sortType),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSortItems), req)
}

// EquipItem assigns an item to a loadout slot for a specific character class.
// Returns an error if any ID is 0, if classID or slot are invalid, or if the context is cancelled.
func (t *TF2) EquipItem(ctx context.Context, itemID uint64, classID, slot uint32) error {
	req := &pb.CMsgAdjustItemEquippedState{
		ItemId:   proto.Uint64(itemID),
		NewClass: proto.Uint32(classID),
		NewSlot:  proto.Uint32(slot),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCAdjustItemEquippedState), req)
}

// UnlockCrate consumes a key to unlock a specific supply crate.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) UnlockCrate(ctx context.Context, keyID, crateID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, keyID)
	_ = binary.Write(buf, binary.LittleEndian, crateID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnlockCrate), buf.Bytes())
}

// WrapItem applies a gift wrap tool to an item.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) WrapItem(ctx context.Context, wrapID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, wrapID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCGiftWrapItem), buf.Bytes())
}

// DeliverGift delivers a wrapped gift item to the specified target Steam account.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) DeliverGift(ctx context.Context, giftID, targetSteamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, giftID)
	_ = binary.Write(buf, binary.LittleEndian, targetSteamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDeliverGift), buf.Bytes())
}

// InviteToTrade sends an in-game live trade session request to another Steam user.
// Returns an error if the connection is down, if steamID is 0, or if the context is cancelled.
func (t *TF2) InviteToTrade(ctx context.Context, steamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, steamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest), buf.Bytes())
}

// RespondToTrade responds to an incoming live trade request invitation.
// Returns an error if the connection is down, if tradeID is 0, or if the context is cancelled.
func (t *TF2) RespondToTrade(ctx context.Context, tradeID uint32, accept bool) error {
	const (
		ResponseAccepted = 0
		ResponseDeclined = 1
	)

	resp := uint32(ResponseDeclined)
	if accept {
		resp = ResponseAccepted
	}

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, resp)
	_ = binary.Write(buf, binary.LittleEndian, tradeID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeResponse), buf.Bytes())
}

// ApplyPaint applies a paint can tool to a paintable cosmetic item.
// Returns an error if the connection is down, if any ID is 0, or if the context is cancelled.
func (t *TF2) ApplyPaint(ctx context.Context, toolID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, toolID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCPaintItem), buf.Bytes())
}

// UnwrapGift unpacks a delivered gift package item in the inventory.
// Returns an error if the connection is down, if itemID is 0, or if the context is cancelled.
func (t *TF2) UnwrapGift(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), buf.Bytes())
}

// CancelTradeRequest cancels any active trading session invitation.
// Returns an error if the connection is down or if the context is cancelled.
func (t *TF2) CancelTradeRequest(ctx context.Context) error {
	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_CancelSession), nil)
}

// ItemPos represents an item placement configuration in the backpack.
type ItemPos struct {
	// ID represents the unique asset identifier of the item.
	ID uint64
	// Position represents the destination page/slot index in the backpack.
	Position uint32
}

// MoveItems moves multiple items to designated positions in the backpack.
// Operations are grouped and sent in sequential batches of 50 moves to prevent rate limiting.
// Returns an error if any batch fails to send or if the context is cancelled.
func (t *TF2) MoveItems(ctx context.Context, items []ItemPos) error {
	const maxBatchSize = 50

	for i := 0; i < len(items); i += maxBatchSize {
		end := min(i+maxBatchSize, len(items))
		batch := items[i:end]
		req := &pb.CMsgSetItemPositions{}

		for _, item := range batch {
			req.ItemPositions = append(req.ItemPositions, &pb.CMsgSetItemPositions_ItemPosition{
				ItemId:   proto.Uint64(item.ID),
				Position: proto.Uint32(item.Position),
			})
		}

		err := t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemPositions), req)
		if err != nil {
			return fmt.Errorf("failed to send batch %d-%d: %w", i, end, err)
		}

		if end < len(items) {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return nil
}
