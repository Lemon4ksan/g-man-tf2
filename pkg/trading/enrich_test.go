// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"errors"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

type mockBasePartnerInventoryProvider struct {
	items []*trading.Item
	err   error
}

func (m *mockBasePartnerInventoryProvider) GetPartnerInventory(
	ctx context.Context,
	partnerID id.ID,
) ([]*trading.Item, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.items, nil
}

func TestEnrichItem_AllCases(t *testing.T) {
	t.Parallel()

	s := funcMockSchema()

	EnrichItem(nil, s)
	EnrichItem(&trading.Item{}, nil)

	itemUnmapped := &trading.Item{MarketHashName: "Non-existent item"}
	EnrichItem(itemUnmapped, s)
	assert.Equal(t, "0;0;untradable", itemUnmapped.SKU)

	item := &trading.Item{
		MarketHashName: "Mann Co. Supply Crate Key",
		Tradable:       true,
	}
	EnrichItem(item, s)
	assert.Equal(t, "5021;6", item.SKU)
}

func TestItemEnrichmentMiddleware(t *testing.T) {
	t.Parallel()

	s := funcMockSchema()

	t.Run("schema_nil", func(t *testing.T) {
		t.Parallel()

		mw := ItemEnrichmentMiddleware(func() *schema.Schema { return nil }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{MarketHashName: "Mann Co. Supply Crate Key"},
			},
		}
		ctx := engine.NewTradeContext(t.Context(), offer)
		err := handler(ctx)
		assert.NoError(t, err)
		assert.Empty(t, offer.ItemsToGive[0].SKU)
	})

	t.Run("schema_not_nil", func(t *testing.T) {
		t.Parallel()

		mw := ItemEnrichmentMiddleware(func() *schema.Schema { return s }, log.Discard)
		handler := mw(func(c *engine.TradeContext) error {
			return nil
		})

		offer := &trading.TradeOffer{
			ItemsToGive: []*trading.Item{
				{MarketHashName: "Mann Co. Supply Crate Key", Tradable: true},
			},
			ItemsToReceive: []*trading.Item{
				{MarketHashName: "Mann Co. Supply Crate Key", Tradable: true},
			},
		}
		ctx := engine.NewTradeContext(t.Context(), offer)
		err := handler(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "5021;6", offer.ItemsToGive[0].SKU)
		assert.Equal(t, "5021;6", offer.ItemsToReceive[0].SKU)
	})
}

func TestTF2PartnerInventoryProvider(t *testing.T) {
	t.Parallel()

	s := funcMockSchema()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		base := &mockBasePartnerInventoryProvider{
			items: []*trading.Item{
				{MarketHashName: "Mann Co. Supply Crate Key", Tradable: true},
			},
		}

		p := NewPartnerInventoryProvider(base, func() *schema.Schema { return s })
		items, err := p.GetPartnerInventory(t.Context(), id.ID(123))
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "5021;6", items[0].SKU)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		base := &mockBasePartnerInventoryProvider{
			err: errors.New("network error"),
		}

		p := NewPartnerInventoryProvider(base, func() *schema.Schema { return s })
		_, err := p.GetPartnerInventory(t.Context(), id.ID(123))
		assert.Error(t, err)
	})
}
