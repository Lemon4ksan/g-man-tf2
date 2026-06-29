// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewares(t *testing.T) {
	t.Parallel()

	t.Run("safety_middleware", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		resp := V1UserResponse{
			Users: map[id.ID]V1User{
				id.New(123): {
					Bans: &UserBans{
						BPTF: "banned",
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, resp)

		client := New(aoni.NewClient(stub), "key", "token")
		cache := generic.NewCache[string, any]()
		mw := SafetyMiddleware(client, cache, log.Discard)

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

		assert.False(t, handlerCalled)
		assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	})

	t.Run("safety_middleware_cached_user_no_api_calls", func(t *testing.T) {
		client := New(nil, "", "")
		cache := generic.NewCache[string, any]()

		steamID := id.New(123)

		cache.Set("bptf_user_123", V1User{
			Bans: &UserBans{BPTF: "banned"},
		}, 1*time.Hour)

		mw := SafetyMiddleware(client, cache, log.Discard)
		ctx := engine.NewTradeContext(t.Context(), &trading.TradeOffer{
			OtherSteamID: steamID,
		})

		handlerCalled := false
		next := func(tc *engine.TradeContext) error {
			handlerCalled = true
			return nil
		}

		err := mw(next)(ctx)
		require.NoError(t, err)
		assert.False(t, handlerCalled)
		assert.Equal(t, trading.ActionDecline, ctx.Verdict.Action)
	})

	t.Run("safety_middleware_api_error_skips_check", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/users/info/v1", 500, nil)

		client := New(aoni.NewClient(stub), "", "")
		cache := generic.NewCache[string, any]()
		mw := SafetyMiddleware(client, cache, log.Discard)

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
	})

	t.Run("safety_middleware_user_not_in_response_map", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		resp := V1UserResponse{
			Users: map[id.ID]V1User{
				id.New(999): {},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, resp)

		client := New(aoni.NewClient(stub), "", "")
		cache := generic.NewCache[string, any]()
		mw := SafetyMiddleware(client, cache, log.Discard)

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
	})

	t.Run("safety_middleware_clean_user", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		resp := V1UserResponse{
			Users: map[id.ID]V1User{
				id.New(123): {
					Bans: nil,
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, resp)

		client := New(aoni.NewClient(stub), "", "")
		cache := generic.NewCache[string, any]()
		mw := SafetyMiddleware(client, cache, log.Discard)

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
	})

	t.Run("value_tier_middleware", func(t *testing.T) {
		stub := mock.NewHTTPStub()

		resp := InventoryValues{
			Value: 600.0,
		}
		stub.SetJSONResponse("api/inventory/123/values", 200, resp)

		client := New(aoni.NewClient(stub), "key", "token")
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

		val, ok := ctx.Get("partner_inv_value")
		assert.True(t, ok)
		assert.Equal(t, 600.0, val)

		isWhale, ok := ctx.Get("is_whale")
		assert.True(t, ok)
		assert.True(t, isWhale.(bool))
	})

	t.Run("value_tier_middleware_api_error_skips_check", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/inventory/123/values", 500, nil)

		client := New(aoni.NewClient(stub), "", "")
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

		_, ok := ctx.Get("partner_inv_value")
		assert.False(t, ok)
	})

	t.Run("value_tier_middleware_low_value_partner", func(t *testing.T) {
		stub := mock.NewHTTPStub()
		resp := InventoryValues{
			Value: 300.0,
		}
		stub.SetJSONResponse("api/inventory/123/values", 200, resp)

		client := New(aoni.NewClient(stub), "", "")
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

		val, ok := ctx.Get("partner_inv_value")
		assert.True(t, ok)
		assert.Equal(t, 300.0, val)

		_, okWhale := ctx.Get("is_whale")
		assert.False(t, okWhale)
	})
}
