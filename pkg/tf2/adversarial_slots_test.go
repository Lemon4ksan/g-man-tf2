// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/lemon4ksan/g-man-tf2/protobuf/tf2"
)

func TestSlotAllocation_Adversarial(t *testing.T) {
	t.Parallel()

	t.Run("fragmented_slots_allocates_lowest_gap", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		ctx := t.Context()

		// Slots 1, 2, 4, 5 occupied; item 99 is unacknowledged (inventory 0)
		tf.cache.slots = 10
		tf.cache.items[1] = PackGCItem(&Item{ID: 1, Inventory: 1})
		tf.cache.items[2] = PackGCItem(&Item{ID: 2, Inventory: 2})
		tf.cache.items[4] = PackGCItem(&Item{ID: 4, Inventory: 4})
		tf.cache.items[5] = PackGCItem(&Item{ID: 5, Inventory: 5})
		tf.cache.items[99] = PackGCItem(&Item{ID: 99, Inventory: 0})

		err := tf.AcknowledgeItem(ctx, 99)
		require.NoError(t, err)

		// Must allocate slot 3 (the lowest free gap)
		expected := make([]byte, 12)
		binary.LittleEndian.PutUint64(expected[0:8], 99)
		binary.LittleEndian.PutUint32(expected[8:12], 3)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSetSingleItemPosition), mCoord.GetLastSendMsgType())
		assert.Equal(t, expected, mCoord.lastSendPayload)
	})

	t.Run("all_slots_occupied_returns_error", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		ctx := t.Context()

		// maxSlots = 5, all 5 occupied
		tf.cache.slots = 5
		for slot := uint32(1); slot <= 5; slot++ {
			tf.cache.items[uint64(slot)] = PackGCItem(&Item{ID: uint64(slot), Inventory: slot})
		}

		// Unacknowledged item 100
		tf.cache.items[100] = PackGCItem(&Item{ID: 100, Inventory: 0})

		err := tf.AcknowledgeItem(ctx, 100)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "backpack full, no unoccupied slot available for item 100 (max 5)")
	})

	t.Run("already_acknowledged_item_is_noop", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		ctx := t.Context()

		// Item 42 is in slot 5 and acknowledged (bit 30 is 0)
		tf.cache.slots = 10
		tf.cache.items[42] = PackGCItem(&Item{ID: 42, Inventory: 5})

		err := tf.AcknowledgeItem(ctx, 42)
		require.NoError(t, err)
		assert.Equal(
			t,
			uint32(0),
			mCoord.GetLastSendMsgType(),
			"no GC message should be sent for already acknowledged item",
		)
	})

	t.Run("acknowledge_all_with_fragmented_slots", func(t *testing.T) {
		t.Parallel()
		tf, _, mCoord := setupTF2(t)
		ctx := t.Context()

		// Slots 1, 2, 4, 5 occupied; items 10 and 20 are unplaced (inventory = 0)
		tf.cache.slots = 10
		tf.cache.items[1] = PackGCItem(&Item{ID: 1, Inventory: 1})
		tf.cache.items[2] = PackGCItem(&Item{ID: 2, Inventory: 2})
		tf.cache.items[4] = PackGCItem(&Item{ID: 4, Inventory: 4})
		tf.cache.items[5] = PackGCItem(&Item{ID: 5, Inventory: 5})
		tf.cache.items[10] = PackGCItem(&Item{ID: 10, Inventory: 0})
		tf.cache.items[20] = PackGCItem(&Item{ID: 20, Inventory: 0})

		err := tf.AcknowledgeAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint32(pb.EGCItemMsg_k_EMsgGCSetItemPositions), mCoord.GetLastSendMsgType())
	})

	t.Run("acknowledge_all_when_full_returns_error", func(t *testing.T) {
		t.Parallel()
		tf, _, _ := setupTF2(t)
		ctx := t.Context()

		// maxSlots = 3, 3 occupied, plus unplaced item
		tf.cache.slots = 3
		tf.cache.items[1] = PackGCItem(&Item{ID: 1, Inventory: 1})
		tf.cache.items[2] = PackGCItem(&Item{ID: 2, Inventory: 2})
		tf.cache.items[3] = PackGCItem(&Item{ID: 3, Inventory: 3})
		tf.cache.items[99] = PackGCItem(&Item{ID: 99, Inventory: 0})

		err := tf.AcknowledgeAll(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "backpack full, cannot acknowledge all items (max 3)")
	})
}
