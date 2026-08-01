// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"time"

	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"

	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
)

// SafetyMiddleware checks bans and user trust levels on backpack.tf.
func SafetyMiddleware(bptfClient *Client, cache *generic.Cache[string, any], logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			steamID := ctx.Offer.OtherSteamID
			cacheKey := "bptf_user_" + steamID.String()

			var isBanned bool

			if cachedData, ok := cache.Get(cacheKey); ok {
				isBanned = cachedData.(bool)
			} else {
				resp, err := bptfClient.GetUsersInfo(ctx, []id.ID{steamID})
				if err != nil {
					logger.Warn("Reputation API error, skipping safety check", log.Err(err))
					return next(ctx)
				}

				u, ok := resp.Users[steamID]
				if !ok {
					return next(ctx)
				}

				if u.Bans != nil {
					isBanned = true
				}

				cache.Set(cacheKey, isBanned, 2*time.Hour)
			}

			if isBanned {
				ctx.Decline(tf2reason.DeclineBannedBptf)
				return nil
			}

			return next(ctx)
		}
	}
}

// ValueTierMiddleware determines the value of partner's inventory.
func ValueTierMiddleware(bptfClient *Client) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			val, err := bptfClient.GetInventoryValues(ctx, ctx.Offer.OtherSteamID)
			if err != nil {
				return next(ctx)
			}

			floatVal, _ := val.Value.(float64)
			ctx.Set("partner_inv_value", floatVal)

			if floatVal > 500 {
				ctx.Set("is_whale", true)
			}

			return next(ctx)
		}
	}
}
