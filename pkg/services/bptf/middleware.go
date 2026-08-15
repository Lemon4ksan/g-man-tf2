// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"time"

	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"

	tf2reason "github.com/lemon4ksan/g-man-tf2/pkg/reason"
)

// SafetyMiddleware checks bans and user trust levels on backpack.tf.
func SafetyMiddleware(bptfClient API, cache *generic.Cache[string, any], logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			steamID := ctx.Offer.OtherSteamID
			cacheKey := "bptf_user_" + steamID.String()

			var isBanned bool

			if cachedData, ok := cache.Get(cacheKey); ok {
				isBanned = cachedData.(bool)
			} else {
				resp, err := bptfClient.GetUsersInfoV1(ctx, steamID.String())
				if err != nil {
					logger.Warn("Reputation API error, skipping safety check", log.Err(err))
					return next(ctx)
				}

				if users, ok := resp["users"].(map[string]any); ok {
					if u, ok := users[steamID.String()].(map[string]any); ok {
						if bans, ok := u["bans"].(map[string]any); ok && len(bans) > 0 {
							isBanned = true
						}
					}
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
func ValueTierMiddleware(bptfClient API) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			val, err := bptfClient.GetInventoryBySteamidValues(ctx, ctx.Offer.OtherSteamID.String())
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
