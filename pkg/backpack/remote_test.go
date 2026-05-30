// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"testing"

	"github.com/lemon4ksan/g-man/test/requester"
	"github.com/stretchr/testify/assert"
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

func newMockPlayerItemsResponse(items []TF2Item) PlayerItemsResponse {
	resp := PlayerItemsResponse{}
	resp.Result.Status = 1
	resp.Result.NumBackpackSlots = 100
	resp.Result.Items = items

	return resp
}

func TestRemote_IsDuped_DupedAndCleanItems_ReturnsExpectedFlags(t *testing.T) {
	t.Parallel()

	mockAPI := requester.New()
	mockAPI.SetJSONResponse("IEconItems_440", "GetPlayerItems", newMockPlayerItemsResponse([]TF2Item{
		{ID: 100, OriginalID: 50},
		{ID: 200, OriginalID: 200},
	}))

	checker1 := &MockDupeChecker{
		Responses: map[uint64]HistoryStatus{
			200: {Recorded: true, IsDuped: false},
			50:  {Recorded: true, IsDuped: true},
		},
	}

	inv := NewRemote(7656119, mockAPI, WithDupeCheckers([]DupeChecker{checker1}))

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
					t.Errorf("IsDuped() = %v, want %v", got, tt.wantDuped)
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

	inv := NewRemote(7656119, nil, WithDupeCheckers([]DupeChecker{checker1, checker2}))

	got, _ := inv.IsDuped(t.Context(), 100)
	if got == nil || !*got {
		t.Error("expected IsDuped to be true from second checker")
	}
}

func TestRemote_GetItemsBySKU_ValidInventory_FiltersBySKU(t *testing.T) {
	t.Parallel()

	mockAPI := requester.New()
	mockAPI.SetJSONResponse("IEconItems_440", "GetPlayerItems", newMockPlayerItemsResponse([]TF2Item{
		{ID: 1, Defindex: 5021, Quality: 6, FlagCannotTrade: false},
		{ID: 2, Defindex: 1, Quality: 6, FlagCannotTrade: false},
	}))

	inv := NewRemote(7656119, mockAPI)

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

	inv := NewRemote(7656119, mockAPI)
	ok, err := inv.CanTradeWithoutHold(t.Context(), "token")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRemote_FindMetalInPartnerInventory_AvailableMetal_ReturnsRequiredChange(t *testing.T) {
	t.Parallel()

	mockAPI := requester.New()
	mockAPI.SetJSONResponse("IEconItems_440", "GetPlayerItems", newMockPlayerItemsResponse([]TF2Item{
		{ID: 10, Defindex: 5002, Quality: 6, FlagCannotTrade: false},
		{ID: 11, Defindex: 5002, Quality: 6, FlagCannotTrade: false},
		{ID: 12, Defindex: 5001, Quality: 6, FlagCannotTrade: false},
		{ID: 13, Defindex: 5000, Quality: 6, FlagCannotTrade: false},
	}))

	inv := NewRemote(7656119, mockAPI)

	items, err := inv.FindMetalInPartnerInventory(t.Context(), 21)
	assert.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestRemote_Fetch_WebAPIFailsNoCommunity_ReturnsError(t *testing.T) {
	t.Parallel()

	mockAPI := requester.New()
	mockAPI.ResponseErrs["IEconItems_440/GetPlayerItems"] = errors.New("rate limited")

	inv := NewRemote(7656119, mockAPI)
	err := inv.fetch(t.Context())
	assert.Error(t, err)
}

func boolPtr(b bool) *bool { return &b }
