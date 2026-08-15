// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rep provides utilities for checking user bans against various ban lists.
package rep

import (
	"context"
	"fmt"

	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/g-man/pkg/steam/id"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
)

// BansManager handles checking users against various ban lists.
type BansManager struct {
	rest       request.Requester
	bptfClient bptf.API
	mptfAPIKey string
}

// NewBansManager creates a new bans manager.
func NewBansManager(r request.Requester, bptfClient bptf.API, mptfAPIKey string) *BansManager {
	return &BansManager{
		rest:       r,
		bptfClient: bptfClient,
		mptfAPIKey: mptfAPIKey,
	}
}

// BanResult contains the results of ban checks from different sources.
type BanResult struct {
	IsBanned bool
	Details  map[string]string // Source -> Status/Reason
}

// CheckBans checks a user against multiple ban lists (Backpack.tf, SteamRep, Marketplace.tf).
func (m *BansManager) CheckBans(ctx context.Context, steamID id.ID) (*BanResult, error) {
	result := &BanResult{
		Details: make(map[string]string),
	}

	// Check Backpack.tf (which also includes SteamRep info)
	userResp, err := m.bptfClient.GetUsersInfoV1(ctx, steamID.String())
	if err == nil {
		if users, ok := userResp["users"].(map[string]any); ok {
			if user, ok := users[steamID.String()].(map[string]any); ok {
				if bans, ok := user["bans"].(map[string]any); ok {
					if all, ok := bans["all"].(string); ok && all != "" {
						result.IsBanned = true
						result.Details["backpack.tf"] = "banned"
					} else if bptfBan, ok := bans["bptf"].(string); ok && bptfBan != "" {
						result.IsBanned = true
						result.Details["backpack.tf"] = "banned"
					}

					if sr, ok := bans["steamrep_scammer"].(float64); ok && sr == 1 {
						result.IsBanned = true
						result.Details["steamrep.com"] = "scammer"
					}
				}

				if trust, ok := user["trust"].(map[string]any); ok {
					pos, _ := trust["positive"].(float64)
					neg, _ := trust["negative"].(float64)
					if neg > 0 && neg > pos {
						result.Details["trust"] = fmt.Sprintf("negative (%d/%d)", int(neg), int(pos))
					}
				}
			}
		}
	}

	// Check Marketplace.tf (requires API key)
	if m.mptfAPIKey != "" {
		mptfBanned, err := m.checkMarketplaceTF(ctx, steamID)
		if err == nil && mptfBanned {
			result.IsBanned = true
			result.Details["marketplace.tf"] = "banned"
		}
	}

	return result, nil
}

func (m *BansManager) checkMarketplaceTF(ctx context.Context, steamID id.ID) (bool, error) {
	url := "https://marketplace.tf/api/Bans/GetUserBan/v2"

	req := struct {
		Key     string `url:"key"`
		SteamID string `url:"steamid"`
	}{
		Key:     m.mptfAPIKey,
		SteamID: steamID.String(),
	}

	type MPTFResponse struct {
		Status  string `json:"status"` // "success"
		Results []struct {
			SteamID string `json:"steamid"`
			Banned  bool   `json:"banned"`
		} `json:"results"`
	}

	resp, err := request.PostTo[MPTFResponse](ctx, m.rest, url, req)
	if err != nil {
		return false, err
	}

	if resp.Status != "success" {
		return false, fmt.Errorf("mptf returned error status: %s", resp.Status)
	}

	for _, res := range resp.Results {
		if res.SteamID == steamID.String() {
			return res.Banned, nil
		}
	}

	return false, nil
}
