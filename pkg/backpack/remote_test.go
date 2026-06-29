// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/steam/transport"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

type MockDupeChecker struct {
	Responses map[uint64]HistoryStatus
	Err       error
}

func (m *MockDupeChecker) CheckHistory(ctx context.Context, id uint64) (HistoryStatus, error) {
	if m.Err != nil {
		return HistoryStatus{}, m.Err
	}

	return m.Responses[id], nil
}

func mockSchema() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:    5021,
			ItemQuality: 6,
			Name:        "Mann Co. Supply Crate Key",
		},
		{
			Defindex:    5002,
			ItemQuality: 6,
			Name:        "Reclaimed Metal",
		},
		{
			Defindex:    5001,
			ItemQuality: 6,
			Name:        "Reclaimed Metal",
		},
		{
			Defindex:    5000,
			ItemQuality: 6,
			Name:        "Scrap Metal",
		},
		{
			Defindex:    1,
			ItemQuality: 6,
			Name:        "Scattergun",
		},
	}

	return schema.New(raw)
}

type mockInventoryResponse struct {
	Success      int               `json:"success"`
	Assets       []mockAsset       `json:"assets"`
	Descriptions []mockDescription `json:"descriptions"`
	TotalCount   int               `json:"total_inventory_count"`
}

type mockAsset struct {
	AssetID    string `json:"assetid"`
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	Amount     string `json:"amount"`
}

type mockDescription struct {
	ClassID        string         `json:"classid"`
	InstanceID     string         `json:"instanceid"`
	Tradable       int            `json:"tradable"`
	Name           string         `json:"name"`
	MarketHashName string         `json:"market_hash_name"`
	AppData        map[string]any `json:"app_data,omitempty"`
}

func TestRemote_Fetch(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("success_first_then_cached", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"

		mockResp := mockInventoryResponse{
			Success:    1,
			TotalCount: 1,
			Assets: []mockAsset{
				{AssetID: "100", ClassID: "13", InstanceID: "0", Amount: "1"},
			},
			Descriptions: []mockDescription{
				{
					ClassID: "13", InstanceID: "0", Tradable: 1,
					Name: "Paint Can", MarketHashName: "Paint Can",
					AppData: map[string]any{"def_index": "13", "quality": "6"},
				},
			},
		}
		communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

		inv := NewRemote(7656119, nil, communityMock, s)

		items, err := inv.GetItems(t.Context())
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, 13, items[0].Defindex)

		communityMock.ClearCalls()

		itemsCached, err := inv.GetItems(t.Context())
		require.NoError(t, err)
		require.Len(t, itemsCached, 1)
		assert.Equal(t, 13, itemsCached[0].Defindex)
		assert.Equal(t, 0, communityMock.CallsCount())
	})

	t.Run("no_community_session_error", func(t *testing.T) {
		inv := NewRemote(7656119, nil, nil, s)
		err := inv.fetch(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no community web session")
	})

	t.Run("no_schema_error", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"
		inv := NewRemote(7656119, nil, communityMock, nil)
		err := inv.fetch(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no schema available")
	})
}

func TestRemote_IsDuped(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("clean_then_duped_scenarios", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"

		mockResp := mockInventoryResponse{
			Success:    1,
			TotalCount: 2,
			Assets: []mockAsset{
				{AssetID: "100", ClassID: "131", InstanceID: "0", Amount: "1"},
				{AssetID: "200", ClassID: "132", InstanceID: "0", Amount: "1"},
			},
			Descriptions: []mockDescription{
				{
					ClassID: "131", InstanceID: "0", Tradable: 1,
					Name: "Paint Can 1", MarketHashName: "Paint Can 1",
					AppData: map[string]any{"def_index": "13", "quality": "6", "original_id": "50"},
				},
				{
					ClassID: "132", InstanceID: "0", Tradable: 1,
					Name: "Paint Can 2", MarketHashName: "Paint Can 2",
					AppData: map[string]any{"def_index": "13", "quality": "6", "original_id": "200"},
				},
			},
		}
		communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

		inv := NewRemote(7656119, nil, communityMock, s)

		checkerFail := &MockDupeChecker{
			Err: errors.New("temporary failure"),
		}
		checker1 := &MockDupeChecker{
			Responses: map[uint64]HistoryStatus{
				200: {Recorded: true, IsDuped: false},
				50:  {Recorded: true, IsDuped: true},
			},
		}

		inv.dupeCheckers = []DupeChecker{checkerFail, checker1}

		got, err := inv.IsDuped(t.Context(), 200)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.False(t, *got)

		got, err = inv.IsDuped(t.Context(), 100)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, *got)

		got, err = inv.IsDuped(t.Context(), 999)
		assert.Error(t, err)
		assert.Nil(t, got)
		assert.True(t, errors.Is(err, ErrItemNotFound))
	})
}

func TestRemote_FindMetalInPartnerInventory(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("change_possible", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"

		mockResp := mockInventoryResponse{
			Success:    1,
			TotalCount: 4,
			Assets: []mockAsset{
				{AssetID: "10", ClassID: "131", InstanceID: "0", Amount: "1"},
				{AssetID: "11", ClassID: "131", InstanceID: "0", Amount: "1"},
				{AssetID: "12", ClassID: "132", InstanceID: "0", Amount: "1"},
				{AssetID: "13", ClassID: "133", InstanceID: "0", Amount: "1"},
			},
			Descriptions: []mockDescription{
				{
					ClassID: "131", InstanceID: "0", Tradable: 1,
					Name: "Refined Metal", MarketHashName: "Refined Metal",
					AppData: map[string]any{"def_index": "5002", "quality": "6"},
				},
				{
					ClassID: "132", InstanceID: "0", Tradable: 1,
					Name: "Reclaimed Metal", MarketHashName: "Reclaimed Metal",
					AppData: map[string]any{"def_index": "5001", "quality": "6"},
				},
				{
					ClassID: "133", InstanceID: "0", Tradable: 1,
					Name: "Scrap Metal", MarketHashName: "Scrap Metal",
					AppData: map[string]any{"def_index": "5000", "quality": "6"},
				},
			},
		}
		communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

		inv := NewRemote(7656119, nil, communityMock, s)

		items, err := inv.FindMetalInPartnerInventory(t.Context(), 21)
		require.NoError(t, err)
		assert.NotEmpty(t, items)
	})

	t.Run("change_impossible_error", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"

		mockResp := mockInventoryResponse{
			Success:    1,
			TotalCount: 1,
			Assets: []mockAsset{
				{AssetID: "10", ClassID: "133", InstanceID: "0", Amount: "1"},
			},
			Descriptions: []mockDescription{
				{
					ClassID: "133", InstanceID: "0", Tradable: 1,
					Name: "Scrap Metal", MarketHashName: "Scrap Metal",
					AppData: map[string]any{"def_index": "5000", "quality": "6"},
				},
			},
		}
		communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

		inv := NewRemote(7656119, nil, communityMock, s)

		items, err := inv.FindMetalInPartnerInventory(t.Context(), 100)
		assert.Error(t, err)
		assert.Nil(t, items)
		assert.Contains(t, err.Error(), "partner is missing")
	})
}

func TestRemote_EnrichCommunityItems(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("missing_def_index_trigger_enrichment", func(t *testing.T) {
		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"

		mockResp := mockInventoryResponse{
			Success:    1,
			TotalCount: 52,
		}

		for i := range 52 {
			idStr := strconv.Itoa(i + 1)
			mockResp.Assets = append(mockResp.Assets, mockAsset{
				AssetID:    idStr,
				ClassID:    idStr,
				InstanceID: "0",
				Amount:     "1",
			})
			mockResp.Descriptions = append(mockResp.Descriptions, mockDescription{
				ClassID:    idStr,
				InstanceID: "0",
				Tradable:   1,
				Name:       "Item " + idStr,
			})
		}

		communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

		mockAPI := mock.NewServiceMock()

		apiResult := make(map[string]json.RawMessage)
		for i := range 52 {
			idStr := strconv.Itoa(i + 1)
			rawPayload, _ := json.Marshal(map[string]any{
				"classid":    idStr,
				"instanceid": "0",
				"app_data": map[string]any{
					"def_index": "13",
					"quality":   "6",
				},
			})
			apiResult[idStr] = rawPayload
		}

		mockAPI.OnDo = func(req *transport.Request) (*transport.Response, error) {
			return mock.JSONResponse(map[string]any{
				"result": apiResult,
			})
		}

		inv := NewRemote(7656119, mockAPI, communityMock, s)

		items, err := inv.GetItems(t.Context())
		require.NoError(t, err)
		require.Len(t, items, 52)

		for _, item := range items {
			assert.Equal(t, 13, item.Defindex)
		}
	})
}

func TestRemote_WithModifiers(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	dc := []DupeChecker{&MockDupeChecker{}}

	inv := NewRemote(7656119, nil, nil, nil, WithLogger(logger), WithDupeCheckers(dc))
	assert.Equal(t, logger, inv.logger)
	assert.Equal(t, dc, inv.dupeCheckers)
}

func TestRemote_CanTradeWithoutHold(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	t.Run("success_no_hold", func(t *testing.T) {
		mockAPI := mock.NewServiceMock()
		mockAPI.SetJSONResponse("IEconService", "GetTradeHoldDurations", map[string]any{
			"their_escrow": 0,
			"my_escrow":    0,
		})

		inv := NewRemote(7656119, mockAPI, nil, s)
		ok, err := inv.CanTradeWithoutHold(t.Context(), "token_123")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("success_with_hold", func(t *testing.T) {
		mockAPI := mock.NewServiceMock()
		mockAPI.SetJSONResponse("IEconService", "GetTradeHoldDurations", map[string]any{
			"their_escrow": 15,
			"my_escrow":    0,
		})

		inv := NewRemote(7656119, mockAPI, nil, s)
		ok, err := inv.CanTradeWithoutHold(t.Context(), "token_123")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("failure_error", func(t *testing.T) {
		mockAPI := mock.NewServiceMock()
		mockAPI.SetErrorResponse("IEconService", "GetTradeHoldDurations", errors.New("api error"))

		inv := NewRemote(7656119, mockAPI, nil, s)
		_, err := inv.CanTradeWithoutHold(t.Context(), "token_123")
		assert.Error(t, err)
	})
}

func TestRemote_EnrichCommunityItems_NilClient(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	inv := NewRemote(7656119, nil, nil, s)

	err := inv.enrichCommunityItems(
		t.Context(),
		[]inventory.CEconItem{{Description: inventory.Description{ClassID: "1"}}},
	)
	assert.NoError(t, err)
}

func TestRemote_EnrichCommunityItems_APIError(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	mockAPI := mock.NewServiceMock()
	mockAPI.SetErrorResponse("ISteamEconomy", "GetAssetClassInfo", errors.New("webapi down"))

	inv := NewRemote(7656119, mockAPI, nil, s)

	err := inv.enrichCommunityItems(t.Context(), []inventory.CEconItem{{
		Description: inventory.Description{
			ClassID:    "1",
			InstanceID: "0",
			Name:       "Some Item",
		},
	}})
	assert.Error(t, err)
}

func TestRemote_EnrichCommunityItems_NoMissingKeys(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	mockAPI := mock.NewServiceMock()
	inv := NewRemote(7656119, mockAPI, nil, s)

	err := inv.enrichCommunityItems(
		t.Context(),
		[]inventory.CEconItem{
			{
				Description: inventory.Description{
					ClassID:    "1",
					InstanceID: "0",
					AppData:    map[string]any{"def_index": "13"},
				},
			},
		},
	)
	assert.NoError(t, err)
}

func TestRemote_GetItemsBySKU_FetchError(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	communityMock := mock.NewHTTPStub()
	communityMock.MockSessionID = "mock_session"
	communityMock.ResponseErrs["inventory/7656119/440/2"] = errors.New("rate limited")

	inv := NewRemote(7656119, nil, communityMock, s)
	_, err := inv.GetItemsBySKU(t.Context(), "13;6")
	assert.Error(t, err)
}

func TestRemote_GetItems_FetchError(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	communityMock := mock.NewHTTPStub()
	communityMock.MockSessionID = "mock_session"
	communityMock.ResponseErrs["inventory/7656119/440/2"] = errors.New("rate limited")

	inv := NewRemote(7656119, nil, communityMock, s)
	_, err := inv.GetItems(t.Context())
	assert.Error(t, err)
}

func TestRemote_IsDuped_FetchError(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	communityMock := mock.NewHTTPStub()
	communityMock.MockSessionID = "mock_session"
	communityMock.ResponseErrs["inventory/7656119/440/2"] = errors.New("rate limited")

	inv := NewRemote(7656119, nil, communityMock, s)
	_, err := inv.IsDuped(t.Context(), 100)
	assert.Error(t, err)
}
