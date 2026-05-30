// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
)

func TestTF2_Actions_ValidInputs_SendsCorrectGCPackets(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	ctx := t.Context()

	tests := []struct {
		name         string
		action       func() error
		expectedMsg  pb.EGCItemMsg
		expectedBody func() []byte
	}{
		{
			name:        "remove_item_name",
			action:      func() error { return tf.RemoveItemName(ctx, 123) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCRemoveItemName,
			expectedBody: func() []byte {
				b := make([]byte, 9)
				binary.LittleEndian.PutUint64(b[0:8], 123)
				b[8] = 0 // Name

				return b
			},
		},
		{
			name:        "remove_item_description",
			action:      func() error { return tf.RemoveItemDescription(ctx, 123) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCRemoveItemName,
			expectedBody: func() []byte {
				b := make([]byte, 9)
				binary.LittleEndian.PutUint64(b[0:8], 123)
				b[8] = 1 // Description

				return b
			},
		},
		{
			name: "name_item",
			action: func() error {
				return tf.NameItem(ctx, 100, 200, "Cool Weapon")
			},
			expectedMsg: pb.EGCItemMsg_k_EMsgGCNameItem,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100)) // ToolID
				_ = binary.Write(buf, binary.LittleEndian, uint64(200)) // ItemID
				buf.WriteByte(0)                                        // IsDescription = false
				buf.WriteString("Cool Weapon")
				buf.WriteByte(0) // Null terminator

				return buf.Bytes()
			},
		},
		{
			name: "describe_item",
			action: func() error {
				return tf.DescribeItem(ctx, 100, 200, "Epic Description")
			},
			expectedMsg: pb.EGCItemMsg_k_EMsgGCNameItem,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100)) // ToolID
				_ = binary.Write(buf, binary.LittleEndian, uint64(200)) // ItemID
				buf.WriteByte(1)                                        // IsDescription = true
				buf.WriteString("Epic Description")
				buf.WriteByte(0) // Null terminator

				return buf.Bytes()
			},
		},
		{
			name:        "acknowledge_item",
			action:      func() error { return tf.AcknowledgeItem(ctx, 123) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition,
			expectedBody: func() []byte {
				b := make([]byte, 12)
				binary.LittleEndian.PutUint64(b[0:8], 123)
				binary.LittleEndian.PutUint32(b[8:12], 1) // Default pos for ack

				return b
			},
		},
		{
			name:        "set_item_style",
			action:      func() error { return tf.SetItemStyle(ctx, 123, 1) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCSetItemStyle,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(123))
				_ = binary.Write(buf, binary.LittleEndian, uint8(1)) // SDK: uint8 m_iStyle

				return buf.Bytes()
			},
		},
		{
			name:        "set_item_position",
			action:      func() error { return tf.SetItemPosition(ctx, 123, 42) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(123))
				_ = binary.Write(buf, binary.LittleEndian, uint32(42))

				return buf.Bytes()
			},
		},
		{
			name:        "delete_item",
			action:      func() error { return tf.DeleteItem(ctx, 123) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCDelete,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(123))
				return buf.Bytes()
			},
		},
		{
			name:        "unlock_crate",
			action:      func() error { return tf.UnlockCrate(ctx, 100, 200) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCUnlockCrate,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100))
				_ = binary.Write(buf, binary.LittleEndian, uint64(200))

				return buf.Bytes()
			},
		},
		{
			name:        "wrap_item",
			action:      func() error { return tf.WrapItem(ctx, 100, 200) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCGiftWrapItem,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100))
				_ = binary.Write(buf, binary.LittleEndian, uint64(200))

				return buf.Bytes()
			},
		},
		{
			name:        "deliver_gift",
			action:      func() error { return tf.DeliverGift(ctx, 100, 76561198000000000) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCDeliverGift,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100))
				_ = binary.Write(buf, binary.LittleEndian, uint64(76561198000000000))

				return buf.Bytes()
			},
		},
		{
			name:        "invite_to_trade",
			action:      func() error { return tf.InviteToTrade(ctx, 76561198000000000) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeRequest,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint32(0))
				_ = binary.Write(buf, binary.LittleEndian, uint64(76561198000000000))

				return buf.Bytes()
			},
		},
		{
			name:        "respond_to_trade_accept",
			action:      func() error { return tf.RespondToTrade(ctx, 42, true) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeResponse,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // Accept
				_ = binary.Write(buf, binary.LittleEndian, uint32(42))

				return buf.Bytes()
			},
		},
		{
			name:        "apply_paint",
			action:      func() error { return tf.ApplyPaint(ctx, 100, 200) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCPaintItem,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint64(100))
				_ = binary.Write(buf, binary.LittleEndian, uint64(200))

				return buf.Bytes()
			},
		},
		{
			name:        "unwrap_gift",
			action:      func() error { return tf.UnwrapGift(ctx, 123) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCUnwrapGiftRequest,
			expectedBody: func() []byte {
				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, 123)
				return b
			},
		},
		{
			name:        "respond_to_trade_decline",
			action:      func() error { return tf.RespondToTrade(ctx, 42, false) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCTrading_InitiateTradeResponse,
			expectedBody: func() []byte {
				buf := new(bytes.Buffer)
				_ = binary.Write(buf, binary.LittleEndian, uint32(1)) // Decline
				_ = binary.Write(buf, binary.LittleEndian, uint32(42))

				return buf.Bytes()
			},
		},
		{
			name:        "cancel_trade_request",
			action:      func() error { return tf.CancelTradeRequest(ctx) },
			expectedMsg: pb.EGCItemMsg_k_EMsgGCTrading_CancelSession,
			expectedBody: func() []byte {
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			require.NoError(t, err)

			assert.Equal(t, uint32(tt.expectedMsg), mCoord.GetLastSendMsgType())

			if tt.expectedBody != nil {
				assert.Equal(t, tt.expectedBody(), mCoord.lastSendPayload)
			}
		})
	}
}

func TestTF2_ProtoActions_ValidInputs_SendsCorrectGCPackets(t *testing.T) {
	t.Parallel()

	tf, _, mCoord := setupTF2(t)
	ctx := t.Context()

	t.Run("use_item", func(t *testing.T) {
		err := tf.UseItem(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCUseItemRequest), mCoord.GetLastSendMsgType())
	})

	t.Run("sort_backpack", func(t *testing.T) {
		err := tf.SortBackpack(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSortItems), mCoord.GetLastSendMsgType())
	})

	t.Run("equip_item", func(t *testing.T) {
		err := tf.EquipItem(ctx, 123, 2, 3)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCAdjustItemEquippedState), mCoord.GetLastSendMsgType())
	})

	t.Run("move_items", func(t *testing.T) {
		items := []ItemPos{{ID: 1, Position: 2}, {ID: 3, Position: 4}}
		err := tf.MoveItems(ctx, items)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSetItemPositions), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_item_paint", func(t *testing.T) {
		err := tf.RemoveItemPaint(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveItemPaint), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_makers_mark", func(t *testing.T) {
		err := tf.RemoveMakersMark(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveMakersMark), mCoord.GetLastSendMsgType())
	})

	t.Run("reset_strange_scores", func(t *testing.T) {
		err := tf.ResetStrangeScores(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCResetStrangeScores), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_killstreak", func(t *testing.T) {
		err := tf.RemoveKillstreak(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveKillStreak), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_festivizer", func(t *testing.T) {
		err := tf.RemoveFestivizer(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveFestivizer), mCoord.GetLastSendMsgType())
	})

	t.Run("remove_gifted_by", func(t *testing.T) {
		err := tf.RemoveGiftedBy(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCRemoveGiftedBy), mCoord.GetLastSendMsgType())
	})

	t.Run("shuffle_crate", func(t *testing.T) {
		err := tf.ShuffleCrate(ctx, 123, "CODE")
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCShuffleCrateContents), mCoord.GetLastSendMsgType())
	})

	t.Run("apply_autograph", func(t *testing.T) {
		err := tf.ApplyAutograph(ctx, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCApplyAutograph), mCoord.GetLastSendMsgType())
	})

	t.Run("request_market_data", func(t *testing.T) {
		err := tf.RequestMarketData(ctx, 1) // USD
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCClientRequestMarketData), mCoord.GetLastSendMsgType())
	})

	t.Run("request_friends", func(t *testing.T) {
		err := tf.RequestFriends(ctx, []uint32{1, 2, 3})
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.ETFGCMsg_k_EMsgGCRequestTF2Friends), mCoord.GetLastSendMsgType())
	})

	t.Run("apply_strange_part", func(t *testing.T) {
		err := tf.ApplyStrangePart(ctx, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCApplyStrangePart), mCoord.GetLastSendMsgType())
	})

	t.Run("apply_strangifier", func(t *testing.T) {
		err := tf.ApplyStrangifier(ctx, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCApplyXifier), mCoord.GetLastSendMsgType())
	})
}
