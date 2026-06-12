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

// RemoveItemName requests the Game Coordinator to clear a custom name from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveItemName(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 0 // m_bDescription = false (we want to remove the Name)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemDescription requests the Game Coordinator to clear a custom description from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveItemDescription(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 1 // m_bDescription = true (we want to remove the Description)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemPaint requests the Game Coordinator to clear custom paint from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveItemPaint(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemPaint), req)
}

// RemoveMakersMark requests the Game Coordinator to clear the crafted-by signature from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveMakersMark(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveMakersMark), req)
}

// ResetStrangeScores requests the Game Coordinator to clear counters on a Strange item back to zero.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ResetStrangeScores(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCResetStrangeScores{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCResetStrangeScores), req)
}

// RemoveKillstreak requests the Game Coordinator to clear a killstreak kit modification from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveKillstreak(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), req)
}

// RemoveFestivizer requests the Game Coordinator to clear a festivizer modification from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveFestivizer(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveFestivizer), req)
}

// RemoveGiftedBy requests the Game Coordinator to clear the gifted-by signature from an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveGiftedBy(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveGiftedBy), req)
}

// RemoveItemAttribute requests the Game Coordinator to clear a specific modification attribute from an item.
// This utilizes a specific Game Coordinator message type override.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RemoveItemAttribute(ctx context.Context, itemID uint64, attributeID uint32) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, attributeID, req)
}

// AcknowledgeItem requests the Game Coordinator to flag a newly received item as acknowledged.
// It implements this by shifting the item to position 1.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) AcknowledgeItem(ctx context.Context, itemID uint64) error {
	return t.SetItemPosition(ctx, itemID, 1)
}

// NameItem requests the Game Coordinator to apply a custom name tag to an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) NameItem(ctx context.Context, toolID, itemID uint64, name string) error {
	return t.nameOrDescribeItem(ctx, toolID, itemID, name, false)
}

// DescribeItem requests the Game Coordinator to apply a custom description tag to an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
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

// AcknowledgeAll requests the Game Coordinator to flag all unacknowledged items as seen.
// It shifts newly acquired items from index 0 to the first available slots.
// Returns an error if any of the position change packets fail to send.
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

// SetItemStyle requests the Game Coordinator to change the cosmetic style index of an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) SetItemStyle(ctx context.Context, itemID uint64, style uint8) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = style

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemStyle), data)
}

// SetItemPosition requests the Game Coordinator to shift an item to the specified backpack index.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) SetItemPosition(ctx context.Context, itemID uint64, position uint32) error {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	binary.LittleEndian.PutUint32(data[8:12], position)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition), data)
}

// DeleteItem requests the Game Coordinator to permanently delete the specified item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) DeleteItem(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDelete), buf.Bytes())
}

// SetUnusualEffectOffset requests the Game Coordinator to adjust the vertical alignment of an Unusual effect.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) SetUnusualEffectOffset(ctx context.Context, itemID uint64, offset float32) error {
	req := &pb.CMsgSetItemEffectVerticalOffset{
		ItemId: proto.Uint64(itemID),
		Offset: proto.Float32(offset),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemEffectVerticalOffset), req)
}

// TransferStrangeCount requests the Game Coordinator to shift stats between two compatible Strange items.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) TransferStrangeCount(ctx context.Context, toolID, srcID, destID uint64) error {
	req := &pb.CMsgApplyStrangeCountTransfer{
		ToolItemId:     proto.Uint64(toolID),
		ItemSrcItemId:  proto.Uint64(srcID),
		ItemDestItemId: proto.Uint64(destID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangeCountTransfer), req)
}

// ShuffleCrate requests the Game Coordinator to randomize the contents of the specified crate.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ShuffleCrate(ctx context.Context, itemID uint64, userCode string) error {
	req := &pb.CMsgGCShuffleCrateContents{
		CrateItemId:    proto.Uint64(itemID),
		UserCodeString: proto.String(userCode),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCShuffleCrateContents), req)
}

// ApplyAutograph requests the Game Coordinator to apply an autograph tool to the target item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ApplyAutograph(ctx context.Context, toolID, itemID uint64) error {
	req := &pb.CMsgApplyAutograph{
		AutographItemId: proto.Uint64(toolID),
		ItemItemId:      proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyAutograph), req)
}

// RequestMarketData requests the Game Coordinator to provide current Steam Market price lists.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RequestMarketData(ctx context.Context, currency uint32) error {
	req := &pb.CMsgGCClientMarketDataRequest{
		UserCurrency: proto.Uint32(currency),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCClientRequestMarketData), req)
}

// ReportPlayer requests the Game Coordinator to file a formal abuse or cheating report against a user.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ReportPlayer(ctx context.Context, accountID uint32, reason *pb.CMsgGC_ReportPlayer_EReason) error {
	req := &pb.CMsgGC_ReportPlayer{
		AccountIdTarget: proto.Uint32(accountID),
		Reason:          reason,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGC_ReportPlayer), req)
}

// RequestFriends requests the Game Coordinator to retrieve profile metadata for the specified account IDs.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) RequestFriends(ctx context.Context, accountIDs []uint32) error {
	req := &pb.CMsgTFRequestTF2Friends{
		AccountIds: accountIDs,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCRequestTF2Friends), req)
}

// UseItem requests the Game Coordinator to activate an item-specific action (e.g. consuming a tool).
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) UseItem(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgUseItem{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUseItemRequest), req)
}

// ApplyStrangePart requests the Game Coordinator to apply a strange part modifier to a Strange quality item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ApplyStrangePart(ctx context.Context, itemID, partID uint64) error {
	req := &pb.CMsgApplyStrangePart{
		ItemItemId:        proto.Uint64(itemID),
		StrangePartItemId: proto.Uint64(partID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangePart), req)
}

// ApplyStrangifier requests the Game Coordinator to apply a strangifier or unusualifier tool to an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ApplyStrangifier(ctx context.Context, itemID, toolID uint64) error {
	req := &pb.CMsgApplyToolToItem{
		ToolItemId:    proto.Uint64(toolID),
		SubjectItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyXifier), req)
}

// SortBackpack requests the Game Coordinator to trigger a native inventory sorting operation.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) SortBackpack(ctx context.Context, sortType uint32) error {
	req := &pb.CMsgSortItems{
		SortType: proto.Uint32(sortType),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSortItems), req)
}

// EquipItem requests the Game Coordinator to assign an item to the designated class loadout slot.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) EquipItem(ctx context.Context, itemID uint64, classID, slot uint32) error {
	req := &pb.CMsgAdjustItemEquippedState{
		ItemId:   proto.Uint64(itemID),
		NewClass: proto.Uint32(classID),
		NewSlot:  proto.Uint32(slot),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCAdjustItemEquippedState), req)
}

// UnlockCrate requests the Game Coordinator to consume a key and unlock the specified crate.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) UnlockCrate(ctx context.Context, keyID, crateID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, keyID)
	_ = binary.Write(buf, binary.LittleEndian, crateID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnlockCrate), buf.Bytes())
}

// WrapItem requests the Game Coordinator to apply a gift wrap modification to an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) WrapItem(ctx context.Context, wrapID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, wrapID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCGiftWrapItem), buf.Bytes())
}

// DeliverGift requests the Game Coordinator to send a packaged gift item to another player.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) DeliverGift(ctx context.Context, giftID, targetSteamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, giftID)
	_ = binary.Write(buf, binary.LittleEndian, targetSteamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDeliverGift), buf.Bytes())
}

// UnwrapGiftRequest requests the Game Coordinator to unwrap a received gift package.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) UnwrapGiftRequest(ctx context.Context, itemID uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data[0:8], itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), data)
}

// InviteToTrade requests the Game Coordinator to initiate a live trade invitation with another player.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) InviteToTrade(ctx context.Context, steamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, steamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest), buf.Bytes())
}

// RespondToTrade requests the Game Coordinator to accept or decline an active live trade invitation.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
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

// CancelTradeRequest requests the Game Coordinator to abort any active live trade invitation.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) CancelTradeRequest(ctx context.Context) error {
	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_CancelSession), nil)
}

// ApplyPaint requests the Game Coordinator to apply a paint can modification to an item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ApplyPaint(ctx context.Context, toolID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, toolID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCPaintItem), buf.Bytes())
}

// UnwrapGift requests the Game Coordinator to unwrap a packaged gift in the inventory.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) UnwrapGift(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), buf.Bytes())
}

// ItemPos represents an item identifier and its intended backpack position.
type ItemPos struct {
	// ID represents the unique asset identifier of the item.
	ID uint64
	// Position represents the intended backpack position index.
	Position uint32
}

// MoveItems sends batched backpack slot reassignment requests to the Game Coordinator.
// It splits the list into small chunks to avoid exceeding Game Coordinator limits.
// Returns an error if any of the batches fail to send or if the context is cancelled.
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

// FulfillDynamicRecipeComponent requests the Game Coordinator to feed an ingredient into a dynamic recipe (like a Fabricator or Chemistry Set).
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) FulfillDynamicRecipeComponent(ctx context.Context, toolID, subjectID, attributeIndex uint64) error {
	req := &pb.CMsgFulfillDynamicRecipeComponent{
		ToolItemId: proto.Uint64(toolID),
		ConsumptionComponents: []*pb.CMsgRecipeComponent{
			{
				SubjectItemId:  proto.Uint64(subjectID),
				AttributeIndex: proto.Uint64(attributeIndex),
			},
		},
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCFulfillDynamicRecipeComponent), req)
}

// ConsumePaintkit requests the Game Coordinator to apply a paint kit (Warpaint) and produce a decorated weapon item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (t *TF2) ConsumePaintkit(ctx context.Context, warpaintID uint64, weaponDefIndex uint32) error {
	req := &pb.CMsgConsumePaintkit{
		SourceId:       proto.Uint64(warpaintID),
		TargetDefindex: proto.Uint32(weaponDefIndex),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCConsumePaintKit), req)
}
