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

	pb "github.com/lemon4ksan/g-man-tf2/protobuf/tf2"
)

// RemoveItemName removes a custom name tag applied to an item.
func (t *TF2) RemoveItemName(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 0

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemDescription removes a custom description tag applied to an item.
func (t *TF2) RemoveItemDescription(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 1

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

// RemoveItemPaint strips custom paint color from an item.
func (t *TF2) RemoveItemPaint(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemPaint), req)
}

// RemoveMakersMark removes the crafter's name from a crafted item.
func (t *TF2) RemoveMakersMark(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveMakersMark), req)
}

// ResetStrangeScores resets all strange counters (kills, points) on a Strange item to zero.
func (t *TF2) ResetStrangeScores(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCResetStrangeScores{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCResetStrangeScores), req)
}

// RemoveKillstreak removes an applied Killstreak Kit from an item.
func (t *TF2) RemoveKillstreak(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), req)
}

// RemoveFestivizer removes Festivizer status and lights from an item.
func (t *TF2) RemoveFestivizer(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveFestivizer), req)
}

// RemoveGiftedBy removes a "Gifted by" inscription from a gifted item.
func (t *TF2) RemoveGiftedBy(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveGiftedBy), req)
}

// RemoveItemAttribute sends a raw GC message to strip a specific customization attribute ID.
func (t *TF2) RemoveItemAttribute(ctx context.Context, itemID uint64, attributeID uint32) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, attributeID, req)
}

// AcknowledgeItem places an unacknowledged item into the lowest unoccupied backpack slot.
// It scans existing items in the cache to avoid collisions with already occupied slots.
//
// Parity: matches @tf2autobot/tf2 inventory slot placement.
func (t *TF2) AcknowledgeItem(ctx context.Context, itemID uint64) error {
	if t.cache == nil {
		return t.SetItemPosition(ctx, itemID, 1)
	}

	if it, ok := t.cache.GetItem(itemID); ok {
		isNew := (it.Inventory >> 30) & 1
		if it.Position() > 0 && isNew == 0 {
			return nil
		}
	}

	occupied := make(map[uint32]bool)
	for _, it := range t.cache.GetItems() {
		if it.ID != itemID && it.Position() > 0 && (it.Inventory>>30)&1 == 0 {
			occupied[it.Position()] = true
		}
	}

	maxSlots := uint32(t.cache.GetMaxSlots())
	if maxSlots == 0 {
		maxSlots = 3000
	}

	slot := uint32(1)
	for slot <= maxSlots && occupied[slot] {
		slot++
	}

	if slot > maxSlots {
		return fmt.Errorf("tf2: backpack full, no unoccupied slot available for item %d (max %d)", itemID, maxSlots)
	}

	return t.SetItemPosition(ctx, itemID, slot)
}

// NameItem applies a Name Tag tool to give an item a custom display name.
func (t *TF2) NameItem(ctx context.Context, toolID, itemID uint64, name string) error {
	return t.nameOrDescribeItem(ctx, toolID, itemID, name, false)
}

// DescribeItem applies a Description Tag tool to give an item a custom description.
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
	buf.WriteByte(0)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCNameItem), buf.Bytes())
}

// AcknowledgeAll scans all items in the backpack for unplaced (position 0) or unacknowledged
// (bit 30 set) items, assigning each to the lowest available unoccupied slot without collisions.
//
// Parity: matches @tf2autobot/tf2 slot scan.
func (t *TF2) AcknowledgeAll(ctx context.Context) error {
	if t.cache == nil {
		return nil
	}

	items := t.cache.GetItems()
	occupied := make(map[uint32]bool)

	var unackItems []*Item

	for _, it := range items {
		isNew := (it.Inventory >> 30) & 1
		if it.Position() == 0 || isNew == 1 {
			unackItems = append(unackItems, it)
		} else {
			occupied[it.Position()] = true
		}
	}

	if len(unackItems) == 0 {
		return nil
	}

	maxSlots := uint32(t.cache.GetMaxSlots())
	if maxSlots == 0 {
		maxSlots = 3000
	}

	var toMove []ItemPos

	slot := uint32(1)

	for _, it := range unackItems {
		for slot <= maxSlots && occupied[slot] {
			slot++
		}

		if slot > maxSlots {
			return fmt.Errorf("tf2: backpack full, cannot acknowledge all items (max %d)", maxSlots)
		}

		occupied[slot] = true
		toMove = append(toMove, ItemPos{ID: it.ID, Position: slot})
		slot++
	}

	return t.MoveItems(ctx, toMove)
}

// SetItemStyle updates the active visual style index on a styled item.
func (t *TF2) SetItemStyle(ctx context.Context, itemID uint64, style uint8) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = style

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemStyle), data)
}

// SetItemPosition places an item into the specified 1-based backpack slot position.
func (t *TF2) SetItemPosition(ctx context.Context, itemID uint64, position uint32) error {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	binary.LittleEndian.PutUint32(data[8:12], position)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition), data)
}

// DeleteItem permanently deletes an item from the TF2 inventory via the GC.
func (t *TF2) DeleteItem(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDelete), buf.Bytes())
}

// SetUnusualEffectOffset adjusts the vertical height offset of an unusual particle effect on an item.
func (t *TF2) SetUnusualEffectOffset(ctx context.Context, itemID uint64, offset float32) error {
	req := &pb.CMsgSetItemEffectVerticalOffset{
		ItemId: new(itemID),
		Offset: new(offset),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemEffectVerticalOffset), req)
}

// TransferStrangeCount transfers accumulated strange scores between two strange items.
func (t *TF2) TransferStrangeCount(ctx context.Context, toolID, srcID, destID uint64) error {
	req := &pb.CMsgApplyStrangeCountTransfer{
		ToolItemId:     new(toolID),
		ItemSrcItemId:  new(srcID),
		ItemDestItemId: new(destID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangeCountTransfer), req)
}

// ShuffleCrate re-rolls the contents preview of a series crate.
func (t *TF2) ShuffleCrate(ctx context.Context, itemID uint64, userCode string) error {
	req := &pb.CMsgGCShuffleCrateContents{
		CrateItemId:    new(itemID),
		UserCodeString: new(userCode),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCShuffleCrateContents), req)
}

// ApplyAutograph applies an autograph tool to sign an item.
func (t *TF2) ApplyAutograph(ctx context.Context, toolID, itemID uint64) error {
	req := &pb.CMsgApplyAutograph{
		AutographItemId: new(toolID),
		ItemItemId:      new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyAutograph), req)
}

// RequestMarketData requests market data and pricing information from the GC.
func (t *TF2) RequestMarketData(ctx context.Context, currency uint32) error {
	req := &pb.CMsgGCClientMarketDataRequest{
		UserCurrency: new(currency),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCClientRequestMarketData), req)
}

// ReportPlayer submits an in-game player report to Valve's GC backend.
func (t *TF2) ReportPlayer(ctx context.Context, accountID uint32, reason *pb.CMsgGC_ReportPlayer_EReason) error {
	req := &pb.CMsgGC_ReportPlayer{
		AccountIdTarget: new(accountID),
		Reason:          reason,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGC_ReportPlayer), req)
}

// RequestFriends queries TF2 status for a list of Steam friend account IDs.
func (t *TF2) RequestFriends(ctx context.Context, accountIDs []uint32) error {
	req := &pb.CMsgTFRequestTF2Friends{
		AccountIds: accountIDs,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCRequestTF2Friends), req)
}

// UseItem triggers the default use action on a usable tool or consumable item.
func (t *TF2) UseItem(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgUseItem{
		ItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUseItemRequest), req)
}

// ApplyStrangePart attaches a strange part tool to a Strange item to record additional stats.
func (t *TF2) ApplyStrangePart(ctx context.Context, itemID, partID uint64) error {
	req := &pb.CMsgApplyStrangePart{
		ItemItemId:        new(itemID),
		StrangePartItemId: new(partID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangePart), req)
}

// ApplyStrangifier upgrades an eligible item to Strange quality using a Strangifier tool.
func (t *TF2) ApplyStrangifier(ctx context.Context, itemID, toolID uint64) error {
	req := &pb.CMsgApplyToolToItem{
		ToolItemId:    new(toolID),
		SubjectItemId: new(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyXifier), req)
}

// SortBackpack instructs the GC to sort the backpack by a specific sort type.
func (t *TF2) SortBackpack(ctx context.Context, sortType uint32) error {
	req := &pb.CMsgSortItems{
		SortType: new(sortType),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSortItems), req)
}

// EquipItem equips an item to a specific class and loadout slot.
func (t *TF2) EquipItem(ctx context.Context, itemID uint64, classID, slot uint32) error {
	req := &pb.CMsgAdjustItemEquippedState{
		ItemId:   new(itemID),
		NewClass: new(classID),
		NewSlot:  new(slot),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCAdjustItemEquippedState), req)
}

// UnlockCrate consumes a key to unlock a crate or case.
func (t *TF2) UnlockCrate(ctx context.Context, keyID, crateID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, keyID)
	_ = binary.Write(buf, binary.LittleEndian, crateID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnlockCrate), buf.Bytes())
}

// WrapItem wraps an item using a Gift Wrap tool.
func (t *TF2) WrapItem(ctx context.Context, wrapID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, wrapID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCGiftWrapItem), buf.Bytes())
}

// DeliverGift sends a wrapped gift to the target Steam user.
func (t *TF2) DeliverGift(ctx context.Context, giftID, targetSteamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, giftID)
	_ = binary.Write(buf, binary.LittleEndian, targetSteamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDeliverGift), buf.Bytes())
}

// UnwrapGiftRequest sends an unwrap request to open a received gift item.
func (t *TF2) UnwrapGiftRequest(ctx context.Context, itemID uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data[0:8], itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), data)
}

// InviteToTrade invites a partner to an in-game Steam trade session.
func (t *TF2) InviteToTrade(ctx context.Context, steamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, steamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest), buf.Bytes())
}

// RespondToTrade responds to an incoming in-game trade session invitation.
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

// CancelTradeRequest cancels an active in-game trade session.
func (t *TF2) CancelTradeRequest(ctx context.Context) error {
	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_CancelSession), nil)
}

// ApplyPaint applies a paint can tool to recolor an item.
func (t *TF2) ApplyPaint(ctx context.Context, toolID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, toolID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCPaintItem), buf.Bytes())
}

// UnwrapGift unpacks a gift in the player's inventory.
func (t *TF2) UnwrapGift(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), buf.Bytes())
}

// ItemPos maps an item asset ID to its target backpack slot position.
type ItemPos struct {
	ID       uint64
	Position uint32
}

// MoveItems sends batched position updates to the GC in chunks of 50 items.
func (t *TF2) MoveItems(ctx context.Context, items []ItemPos) error {
	const maxBatchSize = 50

	for i := 0; i < len(items); i += maxBatchSize {
		end := min(i+maxBatchSize, len(items))
		batch := items[i:end]
		req := &pb.CMsgSetItemPositions{}

		for _, item := range batch {
			req.ItemPositions = append(req.ItemPositions, &pb.CMsgSetItemPositions_ItemPosition{
				ItemId:   new(item.ID),
				Position: new(item.Position),
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

// FulfillDynamicRecipeComponent fulfills a component ingredient in a dynamic recipe (Chemistry Set / Fabricator).
func (t *TF2) FulfillDynamicRecipeComponent(ctx context.Context, toolID, subjectID, attributeIndex uint64) error {
	req := &pb.CMsgFulfillDynamicRecipeComponent{
		ToolItemId: new(toolID),
		ConsumptionComponents: []*pb.CMsgRecipeComponent{
			{
				SubjectItemId:  new(subjectID),
				AttributeIndex: new(attributeIndex),
			},
		},
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCFulfillDynamicRecipeComponent), req)
}

// ConsumePaintkit applies a War Paint texture to create a decorated weapon of the chosen defindex.
func (t *TF2) ConsumePaintkit(ctx context.Context, warpaintID uint64, weaponDefIndex uint32) error {
	req := &pb.CMsgConsumePaintkit{
		SourceId:       new(warpaintID),
		TargetDefindex: new(weaponDefIndex),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCConsumePaintKit), req)
}

// TradeUp submits items to a collection upgrade recipe (Trade-Up).
func (t *TF2) TradeUp(ctx context.Context, itemIDs []uint64) error {
	req := &pb.CMsgCraftCollectionUpgrade{
		ItemId: itemIDs,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCCraftCollectionUpgrade), req)
}

// SendProfessorSpeks thanks a helpful friend for a free trial account.
func (t *TF2) SendProfessorSpeks(ctx context.Context, helperAccountID uint32) error {
	req := &pb.CMsgTFFreeTrialChooseMostHelpfulFriend{
		AccountIdFriend: new(helperAccountID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCFreeTrial_ChooseMostHelpfulFriend), req)
}
