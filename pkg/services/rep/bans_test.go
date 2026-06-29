// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rep

import (
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
)

func setupBansManager(t *testing.T, apiKey string) (*BansManager, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	bptfClient := bptf.New(aoni.NewClient(stub), "mock-bptf-key", "mock-token")
	manager := NewBansManager(bptfClient, apiKey)

	return manager, stub
}

func TestBansManager_CheckBans(t *testing.T) {
	t.Parallel()

	cleanSteamID := id.New(76561198033830321)
	bannedSteamID := id.New(76561198000000001)
	scammerSteamID := id.New(76561198000000002)
	badTrustSteamID := id.New(76561198000000003)
	mptfBannedSteamID := id.New(76561198000000004)

	t.Run("clean_user", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "mock-mptf-key")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				cleanSteamID: {
					Name: "Clean User",
					Trust: bptf.UserTrust{
						Positive: 10,
						Negative: 0,
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		respMptf := map[string]any{
			"status": "success",
			"results": []any{
				map[string]any{
					"steamid": cleanSteamID.String(),
					"banned":  false,
				},
			},
		}
		stub.SetJSONResponse("api/Bans/GetUserBan/v2", 200, respMptf)

		res, err := manager.CheckBans(t.Context(), cleanSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.Len(t, res.Details, 0)
	})

	t.Run("banned_bptf", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				bannedSteamID: {
					Name: "Banned User",
					Bans: &bptf.UserBans{
						All:  "banned everywhere",
						BPTF: "banned on bptf",
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		res, err := manager.CheckBans(t.Context(), bannedSteamID)
		require.NoError(t, err)
		assert.True(t, res.IsBanned)
		assert.Equal(t, "banned", res.Details["backpack.tf"])
	})

	t.Run("steamrep_scammer", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				scammerSteamID: {
					Name: "SteamRep Scammer",
					Bans: &bptf.UserBans{
						SteamRepScammer: 1,
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		res, err := manager.CheckBans(t.Context(), scammerSteamID)
		require.NoError(t, err)
		assert.True(t, res.IsBanned)
		assert.Equal(t, "scammer", res.Details["steamrep.com"])
	})

	t.Run("negative_trust", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				badTrustSteamID: {
					Name: "Negative Trust User",
					Trust: bptf.UserTrust{
						Positive: 1,
						Negative: 5,
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		res, err := manager.CheckBans(t.Context(), badTrustSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.Contains(t, res.Details["trust"], "negative")
	})

	t.Run("marketplace_banned", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "mock-mptf-key")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				mptfBannedSteamID: {
					Name: "Marketplace Banned User",
					Trust: bptf.UserTrust{
						Positive: 2,
						Negative: 0,
					},
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		respMptf := map[string]any{
			"status": "success",
			"results": []any{
				map[string]any{
					"steamid": mptfBannedSteamID.String(),
					"banned":  true,
				},
			},
		}
		stub.SetJSONResponse("api/Bans/GetUserBan/v2", 200, respMptf)

		res, err := manager.CheckBans(t.Context(), mptfBannedSteamID)
		require.NoError(t, err)
		assert.True(t, res.IsBanned)
		assert.Equal(t, "banned", res.Details["marketplace.tf"])
	})

	t.Run("marketplace_no_key", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				mptfBannedSteamID: {
					Name: "Marketplace Banned User",
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		res, err := manager.CheckBans(t.Context(), mptfBannedSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.NotContains(t, res.Details, "marketplace.tf")
	})

	t.Run("bptf_api_error_swallowed", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "")

		stub.SetJSONResponse("api/users/info/v1", 500, nil)

		res, err := manager.CheckBans(t.Context(), cleanSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.Empty(t, res.Details)
	})

	t.Run("mptf_api_error_swallowed", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "mock-mptf-key")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				cleanSteamID: {
					Name: "Clean User",
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		stub.SetJSONResponse("api/Bans/GetUserBan/v2", 500, nil)

		res, err := manager.CheckBans(t.Context(), cleanSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.Empty(t, res.Details)
	})

	t.Run("mptf_api_status_failed_swallowed", func(t *testing.T) {
		t.Parallel()

		manager, stub := setupBansManager(t, "mock-mptf-key")

		respBptf := bptf.V1UserResponse{
			Users: map[id.ID]bptf.V1User{
				cleanSteamID: {
					Name: "Clean User",
				},
			},
		}
		stub.SetJSONResponse("api/users/info/v1", 200, respBptf)

		respMptf := map[string]any{
			"status": "error",
		}
		stub.SetJSONResponse("api/Bans/GetUserBan/v2", 200, respMptf)

		res, err := manager.CheckBans(t.Context(), cleanSteamID)
		require.NoError(t, err)
		assert.False(t, res.IsBanned)
		assert.Empty(t, res.Details)
	})
}
