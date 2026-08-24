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

	pb "github.com/lemon4ksan/g-man-tf2/protobuf/tf2"
)

func (t *TF2) RemoveItemName(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 0

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

func (t *TF2) RemoveItemDescription(ctx context.Context, itemID uint64) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = 1

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemName), data)
}

func (t *TF2) RemoveItemPaint(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemPaint), req)
}

func (t *TF2) RemoveMakersMark(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveMakersMark), req)
}

func (t *TF2) ResetStrangeScores(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCResetStrangeScores{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCResetStrangeScores), req)
}

func (t *TF2) RemoveKillstreak(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), req)
}

func (t *TF2) RemoveFestivizer(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveFestivizer), req)
}

func (t *TF2) RemoveGiftedBy(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCRemoveGiftedBy), req)
}

func (t *TF2) RemoveItemAttribute(ctx context.Context, itemID uint64, attributeID uint32) error {
	req := &pb.CMsgGCRemoveCustomizationAttributeSimple{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, attributeID, req)
}

func (t *TF2) AcknowledgeItem(ctx context.Context, itemID uint64) error {
	return t.SetItemPosition(ctx, itemID, 1)
}

func (t *TF2) NameItem(ctx context.Context, toolID, itemID uint64, name string) error {
	return t.nameOrDescribeItem(ctx, toolID, itemID, name, false)
}

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

func (t *TF2) SetItemStyle(ctx context.Context, itemID uint64, style uint8) error {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	data[8] = style

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemStyle), data)
}

func (t *TF2) SetItemPosition(ctx context.Context, itemID uint64, position uint32) error {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint64(data[0:8], itemID)
	binary.LittleEndian.PutUint32(data[8:12], position)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition), data)
}

func (t *TF2) DeleteItem(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDelete), buf.Bytes())
}

func (t *TF2) SetUnusualEffectOffset(ctx context.Context, itemID uint64, offset float32) error {
	req := &pb.CMsgSetItemEffectVerticalOffset{
		ItemId: proto.Uint64(itemID),
		Offset: proto.Float32(offset),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSetItemEffectVerticalOffset), req)
}

func (t *TF2) TransferStrangeCount(ctx context.Context, toolID, srcID, destID uint64) error {
	req := &pb.CMsgApplyStrangeCountTransfer{
		ToolItemId:     proto.Uint64(toolID),
		ItemSrcItemId:  proto.Uint64(srcID),
		ItemDestItemId: proto.Uint64(destID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangeCountTransfer), req)
}

func (t *TF2) ShuffleCrate(ctx context.Context, itemID uint64, userCode string) error {
	req := &pb.CMsgGCShuffleCrateContents{
		CrateItemId:    proto.Uint64(itemID),
		UserCodeString: proto.String(userCode),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCShuffleCrateContents), req)
}

func (t *TF2) ApplyAutograph(ctx context.Context, toolID, itemID uint64) error {
	req := &pb.CMsgApplyAutograph{
		AutographItemId: proto.Uint64(toolID),
		ItemItemId:      proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyAutograph), req)
}

func (t *TF2) RequestMarketData(ctx context.Context, currency uint32) error {
	req := &pb.CMsgGCClientMarketDataRequest{
		UserCurrency: proto.Uint32(currency),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCClientRequestMarketData), req)
}

func (t *TF2) ReportPlayer(ctx context.Context, accountID uint32, reason *pb.CMsgGC_ReportPlayer_EReason) error {
	req := &pb.CMsgGC_ReportPlayer{
		AccountIdTarget: proto.Uint32(accountID),
		Reason:          reason,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGC_ReportPlayer), req)
}

func (t *TF2) RequestFriends(ctx context.Context, accountIDs []uint32) error {
	req := &pb.CMsgTFRequestTF2Friends{
		AccountIds: accountIDs,
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCRequestTF2Friends), req)
}

func (t *TF2) UseItem(ctx context.Context, itemID uint64) error {
	req := &pb.CMsgUseItem{
		ItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUseItemRequest), req)
}

func (t *TF2) ApplyStrangePart(ctx context.Context, itemID, partID uint64) error {
	req := &pb.CMsgApplyStrangePart{
		ItemItemId:        proto.Uint64(itemID),
		StrangePartItemId: proto.Uint64(partID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangePart), req)
}

func (t *TF2) ApplyStrangifier(ctx context.Context, itemID, toolID uint64) error {
	req := &pb.CMsgApplyToolToItem{
		ToolItemId:    proto.Uint64(toolID),
		SubjectItemId: proto.Uint64(itemID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCApplyXifier), req)
}

func (t *TF2) SortBackpack(ctx context.Context, sortType uint32) error {
	req := &pb.CMsgSortItems{
		SortType: proto.Uint32(sortType),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCSortItems), req)
}

func (t *TF2) EquipItem(ctx context.Context, itemID uint64, classID, slot uint32) error {
	req := &pb.CMsgAdjustItemEquippedState{
		ItemId:   proto.Uint64(itemID),
		NewClass: proto.Uint32(classID),
		NewSlot:  proto.Uint32(slot),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCAdjustItemEquippedState), req)
}

func (t *TF2) UnlockCrate(ctx context.Context, keyID, crateID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, keyID)
	_ = binary.Write(buf, binary.LittleEndian, crateID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnlockCrate), buf.Bytes())
}

func (t *TF2) WrapItem(ctx context.Context, wrapID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, wrapID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCGiftWrapItem), buf.Bytes())
}

func (t *TF2) DeliverGift(ctx context.Context, giftID, targetSteamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, giftID)
	_ = binary.Write(buf, binary.LittleEndian, targetSteamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCDeliverGift), buf.Bytes())
}

func (t *TF2) UnwrapGiftRequest(ctx context.Context, itemID uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data[0:8], itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), data)
}

func (t *TF2) InviteToTrade(ctx context.Context, steamID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, steamID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest), buf.Bytes())
}

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

func (t *TF2) CancelTradeRequest(ctx context.Context) error {
	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCTrading_CancelSession), nil)
}

func (t *TF2) ApplyPaint(ctx context.Context, toolID, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, toolID)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCPaintItem), buf.Bytes())
}

func (t *TF2) UnwrapGift(ctx context.Context, itemID uint64) error {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, itemID)

	return t.gc.SendRaw(ctx, AppID, uint32(pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest), buf.Bytes())
}

type ItemPos struct {
	ID       uint64
	Position uint32
}

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

func (t *TF2) ConsumePaintkit(ctx context.Context, warpaintID uint64, weaponDefIndex uint32) error {
	req := &pb.CMsgConsumePaintkit{
		SourceId:       proto.Uint64(warpaintID),
		TargetDefindex: proto.Uint32(weaponDefIndex),
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
		AccountIdFriend: proto.Uint32(helperAccountID),
	}

	return t.gc.Send(ctx, AppID, uint32(pb.ETFGCMsg_k_EMsgGCFreeTrial_ChooseMostHelpfulFriend), req)
}
