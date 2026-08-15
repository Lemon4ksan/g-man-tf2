// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

// V4PricesEntry is an alias for the generated V4 pricing schema entry.
type V4PricesEntry = V4pricesEntry

// V4PricesBaseItemDocExt represents the item details in backpack.tf V4 pricing schema.
type V4PricesBaseItemDocExt struct {
	Defindex []string                                                  `json:"defindex,omitempty"`
	Prices   map[string]map[string]map[string]map[string]V4PricesEntry `json:"prices,omitempty"`
}

// V4PricesResponseExt represents the full response from IGetPrices/v4.
type V4PricesResponseExt struct {
	CurrentTime      int                               `json:"current_time,omitempty"`
	Items            map[string]V4PricesBaseItemDocExt `json:"items,omitempty"`
	Message          string                            `json:"message,omitempty"`
	RawUsdValue      int                               `json:"raw_usd_value,omitempty"`
	Success          SuccessBoolProp                   `json:"success,omitempty"`
	UsdCurrency      string                            `json:"usd_currency,omitempty"`
	UsdCurrencyIndex int                               `json:"usd_currency_index,omitempty"`
}

// UserBans contains ban indicators from backpack.tf and SteamRep.
type UserBans struct {
	All             string `json:"all,omitempty"`
	BPTF            string `json:"bptf,omitempty"`
	SteamRepScammer int    `json:"steamrep_scammer,omitempty"`
}

// UserTrust contains trust ratings on backpack.tf.
type UserTrust struct {
	Positive int `json:"positive,omitempty"`
	Negative int `json:"negative,omitempty"`
}

// UserInfo represents detailed user information on backpack.tf.
type UserInfo struct {
	Name       string     `json:"name,omitempty"`
	Avatar     string     `json:"avatar,omitempty"`
	LastOnline int64      `json:"last_online,omitempty"`
	Bans       *UserBans  `json:"bans,omitempty"`
	Trust      *UserTrust `json:"trust,omitempty"`
}

// V1UserResponse represents the response wrapper for /users/info/v1.
type V1UserResponse struct {
	Users map[id.ID]UserInfo `json:"users,omitempty"`
}
