// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"testing"

	communitymock "github.com/lemon4ksan/g-man/test/community"
	"github.com/lemon4ksan/g-man/test/requester"
	"github.com/stretchr/testify/assert"

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

func newMockSchema() *schema.Schema {
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

func TestRemote_IsDuped_DupedAndCleanItems_ReturnsExpectedFlags(t *testing.T) {
	t.Parallel()

	communityMock := communitymock.New()
	s := newMockSchema()

	mockResp := mockInventoryResponse{
		Success:    1,
		TotalCount: 2,
		Assets: []mockAsset{
			{AssetID: "100", ClassID: "100", InstanceID: "0", Amount: "1"},
			{AssetID: "200", ClassID: "200", InstanceID: "0", Amount: "1"},
		},
		Descriptions: []mockDescription{
			{
				ClassID: "100", InstanceID: "0", Tradable: 1,
				Name: "Item 100", MarketHashName: "Item 100",
				AppData: map[string]any{"def_index": "1", "quality": "6", "original_id": "50"},
			},
			{
				ClassID: "200", InstanceID: "0", Tradable: 1,
				Name: "Item 200", MarketHashName: "Item 200",
				AppData: map[string]any{"def_index": "1", "quality": "6", "original_id": "200"},
			},
		},
	}
	communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

	inv := NewRemote(7656119, nil, communityMock, s)

	checker1 := &MockDupeChecker{
		Responses: map[uint64]HistoryStatus{
			200: {Recorded: true, IsDuped: false},
			50:  {Recorded: true, IsDuped: true},
		},
	}

	inv.dupeCheckers = []DupeChecker{checker1}

	tests := []struct {
		name      string
		assetID   uint64
		wantDuped *bool
		wantErr   error
	}{
		{"clean_item", 200, boolPtr(false), nil},
		{"duped_via_original_id", 100, boolPtr(true), nil},
		{"item_not_in_inventory", 999, nil, ErrItemNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inv.IsDuped(t.Context(), tt.assetID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("IsDuped() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantDuped == nil {
				if got != nil {
					t.Errorf("expected nil result, got %v", *got)
				}
			} else {
				if got == nil || *got != *tt.wantDuped {
					t.Errorf("IsDuped() = %v, want %v", got, *tt.wantDuped)
				}
			}
		})
	}
}

func TestRemote_IsDuped_MultipleCheckers_VerifiesWithSubsequentCheckers(t *testing.T) {
	t.Parallel()

	checker1 := &MockDupeChecker{
		Responses: map[uint64]HistoryStatus{
			100: {Recorded: false},
		},
	}
	checker2 := &MockDupeChecker{
		Responses: map[uint64]HistoryStatus{
			100: {Recorded: true, IsDuped: true},
		},
	}

	s := newMockSchema()
	inv := NewRemote(7656119, nil, nil, s, WithDupeCheckers([]DupeChecker{checker1, checker2}))

	got, _ := inv.IsDuped(t.Context(), 100)
	if got == nil || !*got {
		t.Error("expected IsDuped to be true from second checker")
	}
}

func TestRemote_GetItemsBySKU_ValidInventory_FiltersBySKU(t *testing.T) {
	t.Parallel()

	communityMock := communitymock.New()
	s := newMockSchema()

	mockResp := mockInventoryResponse{
		Success:    1,
		TotalCount: 2,
		Assets: []mockAsset{
			{AssetID: "1", ClassID: "5021", InstanceID: "0", Amount: "1"},
			{AssetID: "2", ClassID: "1", InstanceID: "0", Amount: "1"},
		},
		Descriptions: []mockDescription{
			{
				ClassID: "5021", InstanceID: "0", Tradable: 1,
				Name: "Key", MarketHashName: "Key",
				AppData: map[string]any{"def_index": "5021", "quality": "6"},
			},
			{
				ClassID: "1", InstanceID: "0", Tradable: 1,
				Name: "Scattergun", MarketHashName: "Scattergun",
				AppData: map[string]any{"def_index": "1", "quality": "6"},
			},
		},
	}
	communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

	inv := NewRemote(7656119, nil, communityMock, s)

	items, err := inv.GetItemsBySKU(t.Context(), "5021;6")
	assert.NoError(t, err)

	if assert.Len(t, items, 1) {
		assert.Equal(t, uint64(1), items[0].ID)
	}
}

func TestRemote_CanTradeWithoutHold_ValidResponse_ReturnsHoldStatus(t *testing.T) {
	t.Parallel()

	mockAPI := requester.New()
	mockAPI.SetJSONResponse("IEconService", "GetTradeHoldDurations", map[string]any{
		"their_escrow": 0,
		"my_escrow":    0,
	})

	s := newMockSchema()
	inv := NewRemote(7656119, mockAPI, mockAPI, s)

	ok, err := inv.CanTradeWithoutHold(t.Context(), "token")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRemote_FindMetalInPartnerInventory_AvailableMetal_ReturnsRequiredChange(t *testing.T) {
	t.Parallel()

	communityMock := communitymock.New()
	s := newMockSchema()

	mockResp := mockInventoryResponse{
		Success:    1,
		TotalCount: 4,
		Assets: []mockAsset{
			{AssetID: "10", ClassID: "5002", InstanceID: "0", Amount: "1"},
			{AssetID: "11", ClassID: "5002", InstanceID: "0", Amount: "1"},
			{AssetID: "12", ClassID: "5001", InstanceID: "0", Amount: "1"},
			{AssetID: "13", ClassID: "5000", InstanceID: "0", Amount: "1"},
		},
		Descriptions: []mockDescription{
			{
				ClassID: "5002", InstanceID: "0", Tradable: 1,
				Name: "Refined Metal", MarketHashName: "Refined Metal",
				AppData: map[string]any{"def_index": "5002", "quality": "6"},
			},
			{
				ClassID: "5001", InstanceID: "0", Tradable: 1,
				Name: "Reclaimed Metal", MarketHashName: "Reclaimed Metal",
				AppData: map[string]any{"def_index": "5001", "quality": "6"},
			},
			{
				ClassID: "5000", InstanceID: "0", Tradable: 1,
				Name: "Scrap Metal", MarketHashName: "Scrap Metal",
				AppData: map[string]any{"def_index": "5000", "quality": "6"},
			},
		},
	}
	communityMock.SetJSONResponse("inventory/7656119/440/2", 200, mockResp)

	inv := NewRemote(7656119, nil, communityMock, s)

	items, err := inv.FindMetalInPartnerInventory(t.Context(), 21)
	assert.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestRemote_Fetch_NoCommunitySession_ReturnsError(t *testing.T) {
	t.Parallel()

	s := newMockSchema()
	inv := NewRemote(7656119, nil, nil, s)

	err := inv.fetch(t.Context())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no community web session")
}

func TestRemote_Fetch_NoSchema_ReturnsError(t *testing.T) {
	t.Parallel()

	communityMock := communitymock.New()
	communityMock.MockSessionID = "mock_session_12345"

	inv := NewRemote(7656119, nil, communityMock, nil)

	err := inv.fetch(t.Context())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no schema available")
}

func boolPtr(b bool) *bool { return &b }
