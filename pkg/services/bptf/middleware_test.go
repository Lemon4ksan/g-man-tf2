// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/test/mock"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewares(t *testing.T) {
	t.Parallel()

	t.Run("value_tier_middleware", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		resp := InventoryValues{
			Value: 600.0,
		}
		stub.SetJSONResponse("api/inventory/123/values", 200, resp)

		client := NewAPI(aoni.NewClient(stub))
		mw := ValueTierMiddleware(client)

		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			OtherSteamID: id.New(123),
		})

		handlerCalled := false
		next := func(tc *engine.TradeContext) error {
			handlerCalled = true
			return nil
		}

		err := mw(next)(ctx)
		require.NoError(t, err)

		assert.True(t, handlerCalled)

		val, ok := ctx.Get("partner_inv_value").Value()
		assert.True(t, ok)
		assert.Equal(t, 600.0, val)

		isWhale, ok := ctx.Get("is_whale").Value()
		assert.True(t, ok)
		assert.True(t, isWhale.(bool))
	})
}
