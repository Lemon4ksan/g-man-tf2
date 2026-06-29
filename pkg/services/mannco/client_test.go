// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"encoding/json"
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (*Client, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	restClient := aoni.NewClient(stub)
	client := NewClient(restClient)

	return client, stub
}

func setJSONResponse(stub *mock.HTTPStub, path string, statusCode int, obj any) {
	stub.SetJSONResponse(path, statusCode, obj)
	stub.SetJSONResponse("/"+path, statusCode, obj)
	stub.SetJSONResponse("https://api.mannco.store/"+path, statusCode, obj)
	stub.SetJSONResponse("https://api.mannco.store//"+path, statusCode, obj)
}

func TestClient_Login_And_getClient(t *testing.T) {
	t.Parallel()
	client, stub := setupTestClient(t)

	stub.SetRawResponse(
		"https://api.mannco.store/user/login",
		200,
		[]byte(`{"success": true, "content": {"jwt": "my-jwt-token"}}`),
	)

	ctx := t.Context()
	err := client.Login(ctx, "my-api-key")
	require.NoError(t, err)
}

func TestBaseResponse(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		respJSON := `{"success": true, "err": false, "content": {"status": "ok"}}`

		var (
			b      BaseResponse
			target map[string]string
		)

		b.SetData(&target)
		err := json.Unmarshal([]byte(respJSON), &b)
		require.NoError(t, err)
		assert.True(t, b.IsSuccess())
		assert.Equal(t, "ok", target["status"])
	})

	t.Run("error", func(t *testing.T) {
		respJSON := `{"success": false, "err": true, "content": "some error message"}`

		var b BaseResponse

		err := json.Unmarshal([]byte(respJSON), &b)
		require.NoError(t, err)
		assert.False(t, b.IsSuccess())
		assert.Error(t, b.Error())
		assert.Contains(t, b.Error().Error(), "some error message")
	})
}

func TestUnmarshalJSON_UserAllBuyOrdersItem_AllNonNil(t *testing.T) {
	t.Parallel()

	inputJSON := `{
		"steamid": "76561198000000000; ",
		"type_steam": "type_steam_val; ",
		"class": "class_val; ",
		"hero": "hero_val; ",
		"weapon": "weapon_val; ",
		"exterior": "exterior_val; ",
		"description": "description_val; "
	}`

	var u UserAllBuyOrdersItem

	err := json.Unmarshal([]byte(inputJSON), &u)
	require.NoError(t, err)
	assert.Equal(t, "76561198000000000", u.SteamID)
	assert.Equal(t, "type_steam_val", *u.TypeSteam)
	assert.Equal(t, "class_val", *u.Class)
	assert.Equal(t, "hero_val", *u.Hero)
	assert.Equal(t, "weapon_val", *u.Weapon)
	assert.Equal(t, "exterior_val", *u.Exterior)
	assert.Equal(t, "description_val", *u.Description)
}

func TestUnmarshalJSON_ItemInfo_AllNonNil(t *testing.T) {
	t.Parallel()

	inputJSON := `{
		"type_steam": "type_steam_val; ",
		"class": "class_val; ",
		"hero": "hero_val; ",
		"description": "description_val; "
	}`

	var i ItemInfo

	err := json.Unmarshal([]byte(inputJSON), &i)
	require.NoError(t, err)
	assert.Equal(t, "type_steam_val", *i.TypeSteam)
	assert.Equal(t, "class_val", *i.Class)
	assert.Equal(t, "hero_val", *i.Hero)
	assert.Equal(t, "description_val", *i.Description)
}

func TestUnmarshalJSON_BackpackItemDetails_AllNonNil(t *testing.T) {
	t.Parallel()

	inputJSON := `{
		"parts": "parts_val; ",
		"paint": "paint_val; "
	}`

	var b BackpackItemDetails

	err := json.Unmarshal([]byte(inputJSON), &b)
	require.NoError(t, err)
	assert.Equal(t, "parts_val", *b.Parts)
	assert.Equal(t, "paint_val", *b.Paint)
}

func TestUnmarshalJSON_Listing_AllNonNil(t *testing.T) {
	t.Parallel()

	inputJSON := `{
		"getImage": "getImage_val; "
	}`

	var l Listing

	err := json.Unmarshal([]byte(inputJSON), &l)
	require.NoError(t, err)
	assert.Equal(t, "getImage_val", *l.GetImage)
}

func TestUnmarshalJSON_OfferItem_AllNonNil(t *testing.T) {
	t.Parallel()

	inputJSON := `{
		"html": "html_val; "
	}`

	var o OfferItem

	err := json.Unmarshal([]byte(inputJSON), &o)
	require.NoError(t, err)
	assert.Equal(t, "html_val", *o.HTML)
}

func TestCraftableUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Craftable
	}{
		{
			name:     "craftable integer 1",
			input:    `1`,
			expected: 1,
		},
		{
			name:     "craftable integer 0",
			input:    `0`,
			expected: 0,
		},
		{
			name:     "craftable empty string",
			input:    `""`,
			expected: 0,
		},
		{
			name:     "craftable invalid type float",
			input:    `1.5`,
			expected: 1,
		},
		{
			name:     "craftable non-empty string",
			input:    `"hello"`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Craftable
			if err := json.Unmarshal([]byte(tt.input), &c); err != nil {
				t.Fatalf("unexpected error unmarshaling %q: %v", tt.input, err)
			}

			if c != tt.expected {
				t.Errorf("expected %v, got %v for input %q", tt.expected, c, tt.input)
			}
		})
	}
}

func TestBackpackItemDetailsTrimString(t *testing.T) {
	inputJSON := `{
		"assetId": "12345; ",
		"user": "76561198000000000; ",
		"sheen": "Team Shine;",
		"name": "Strange Killstreak Rust Sniper Rifle (Field-Tested) ;",
		"craftable": ""
	}`

	var details BackpackItemDetails
	if err := json.Unmarshal([]byte(inputJSON), &details); err != nil {
		t.Fatalf("unexpected error unmarshaling details: %v", err)
	}

	if details.AssetID != "12345" {
		t.Errorf("expected details.AssetID to be '12345', got %q", details.AssetID)
	}

	if details.User != "76561198000000000" {
		t.Errorf("expected details.User to be '76561198000000000', got %q", details.User)
	}

	if details.Sheen != "Team Shine" {
		t.Errorf("expected details.Sheen to be 'Team Shine', got %q", details.Sheen)
	}

	if details.Name != "Strange Killstreak Rust Sniper Rifle (Field-Tested)" {
		t.Errorf(
			"expected details.Name to be 'Strange Killstreak Rust Sniper Rifle (Field-Tested)', got %q",
			details.Name,
		)
	}

	if details.Craftable != 0 {
		t.Errorf("expected details.Craftable to be 0, got %d", details.Craftable)
	}
}

func TestItemInfoTrimString(t *testing.T) {
	inputJSON := `{
		"name": "Souvenir M4A1-S | Imminent Danger (Field-Tested) ;",
		"quality": "Souvenir;",
		"url": "730-souvenir-m4a1-s-imminent-danger-field-tested ; "
	}`

	var info ItemInfo
	if err := json.Unmarshal([]byte(inputJSON), &info); err != nil {
		t.Fatalf("unexpected error unmarshaling item info: %v", err)
	}

	if info.Name != "Souvenir M4A1-S | Imminent Danger (Field-Tested)" {
		t.Errorf("expected info.Name to be 'Souvenir M4A1-S | Imminent Danger (Field-Tested)', got %q", info.Name)
	}

	if info.Quality != "Souvenir" {
		t.Errorf("expected info.Quality to be 'Souvenir', got %q", info.Quality)
	}

	if info.URL != "730-souvenir-m4a1-s-imminent-danger-field-tested" {
		t.Errorf("expected info.URL to be '730-souvenir-m4a1-s-imminent-danger-field-tested', got %q", info.URL)
	}
}

func TestGetReceivedOffersResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"offers": [
			{
				"id": 12345,
				"from": "76561198000000000; ",
				"to": "76561198111111111; ",
				"status": 0,
				"time": 1706832000,
				"price": 150000,
				"read": 0,
				"createdAt": 1706745600,
				"backpackid": 98765,
				"user": {
					"username": "BuyerName; ",
					"avatar": "https://avatars.steamstatic.com/...; "
				},
				"item": {
					"name": "Team Captain ;",
					"effect": "Burning Flames;",
					"url": "unusual_team_captain",
					"game": 440,
					"quality": "Unusual",
					"image": "https://...",
					"type": "Hat",
					"craftable": 1,
					"assetId": "987654321; "
				}
			}
		]
	}`

	var resp GetReceivedOffersResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling GetReceivedOffersResponse: %v", err)
	}

	if len(resp.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(resp.Offers))
	}

	o := resp.Offers[0]
	if o.ID != 12345 {
		t.Errorf("expected offer ID 12345, got %d", o.ID)
	}

	if o.From != "76561198000000000" {
		t.Errorf("expected From '76561198000000000', got %q", o.From)
	}

	if o.To != "76561198111111111" {
		t.Errorf("expected To '76561198111111111', got %q", o.To)
	}

	if o.User.Username != "BuyerName" {
		t.Errorf("expected User.Username 'BuyerName', got %q", o.User.Username)
	}

	if o.Item.Name != "Team Captain" {
		t.Errorf("expected Item.Name 'Team Captain', got %q", o.Item.Name)
	}

	if o.Item.AssetID != "987654321" {
		t.Errorf("expected Item.AssetID '987654321', got %q", o.Item.AssetID)
	}
}

func TestGetMyOffersResponseUnmarshal(t *testing.T) {
	inputJSON := `[
		{
			"id": 12345,
			"from": "76561198000000000",
			"to": "76561198111111111",
			"status": 0,
			"time": 1706832000,
			"price": 150000,
			"read": 0,
			"createdAt": 1706745600,
			"backpackid": 98765,
			"user": {
				"username": "SellerName",
				"avatar": "https://avatars.steamstatic.com/..."
			},
			"item": {
				"name": "Team Captain",
				"effect": "Burning Flames",
				"url": "unusual_team_captain",
				"game": 440,
				"quality": "Unusual",
				"image": "https://...",
				"type": "Hat",
				"craftable": 1,
				"assetId": "987654321"
			}
		}
	]`

	var offers []Offer
	if err := json.Unmarshal([]byte(inputJSON), &offers); err != nil {
		t.Fatalf("unexpected error unmarshaling MyOffers: %v", err)
	}

	if len(offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(offers))
	}

	o := offers[0]
	if o.ID != 12345 {
		t.Errorf("expected offer ID 12345, got %d", o.ID)
	}
}

func TestUserBuyOrderResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"informations": {
			"id": 98765,
			"steamid": "76561198000000000; ",
			"itemid": 12345,
			"price": 1500,
			"amount": 3,
			"timestamp": "1706745600; "
		}
	}`

	var resp UserBuyOrderResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling UserBuyOrderResponse: %v", err)
	}

	ubo := resp.Informations
	if ubo.ID != 98765 {
		t.Errorf("expected ID 98765, got %d", ubo.ID)
	}

	if ubo.SteamID != "76561198000000000" {
		t.Errorf("expected SteamID '76561198000000000', got %q", ubo.SteamID)
	}

	if ubo.Timestamp != "1706745600" {
		t.Errorf("expected Timestamp '1706745600', got %q", ubo.Timestamp)
	}
}

func TestUserAllBuyOrdersResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"values": [
			{
				"id": 98765,
				"steamid": "76561198000000000; ",
				"itemid": 12345,
				"price": 15000,
				"amount": 3,
				"name": "Burning Flames Team Captain ; ",
				"effect": "Burning Flames;",
				"url": "unusual_team_captain",
				"game": 440,
				"quality": "Unusual",
				"image": "https://...",
				"type": "Hat",
				"craftable": 1,
				"SKU": "123-strange ;"
			}
		],
		"count": {
			"nb": 15
		}
	}`

	var resp UserAllBuyOrdersResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling UserAllBuyOrdersResponse: %v", err)
	}

	if resp.Count.Nb != 15 {
		t.Errorf("expected count 15, got %d", resp.Count.Nb)
	}

	if len(resp.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(resp.Values))
	}

	v := resp.Values[0]
	if v.SteamID != "76561198000000000" {
		t.Errorf("expected SteamID '76561198000000000', got %q", v.SteamID)
	}

	if v.Name != "Burning Flames Team Captain" {
		t.Errorf("expected Name 'Burning Flames Team Captain', got %q", v.Name)
	}

	if v.SKU != "123-strange" {
		t.Errorf("expected SKU '123-strange', got %q", v.SKU)
	}
}

func TestPaymentResponseUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PaymentResponse
	}{
		{
			name:  "redirect url response",
			input: `{"url": "https://payment-provider.example/checkout/123"}`,
			expected: PaymentResponse{
				URL: "https://payment-provider.example/checkout/123",
			},
		},
		{
			name:  "success message response",
			input: `{"message": "Payment processed successfully"}`,
			expected: PaymentResponse{
				Message: "Payment processed successfully",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp PaymentResponse
			if err := json.Unmarshal([]byte(tt.input), &resp); err != nil {
				t.Fatalf("unexpected error unmarshaling payment response: %v", err)
			}

			if resp.URL != tt.expected.URL {
				t.Errorf("expected URL %q, got %q", tt.expected.URL, resp.URL)
			}

			if resp.Message != tt.expected.Message {
				t.Errorf("expected Message %q, got %q", tt.expected.Message, resp.Message)
			}
		})
	}
}

func TestGetInventoryResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"items": [
			{
				"ids": "987654321,987654322 ;",
				"count": 2,
				"item_id": 5678,
				"assetId": "987654321; ",
				"bot": "76561198000000001; ",
				"game": 440,
				"state": 1,
				"price": 15000,
				"name": "Unusual Burning Flames Team Captain ;",
				"effect": "Burning Flames;",
				"url": "unusual_team_captain",
				"quality": "Unusual",
				"image": "https://...",
				"type": "Hat",
				"craftable": 1
			}
		],
		"count": 25
	}`

	var resp GetInventoryResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling GetInventoryResponse: %v", err)
	}

	if resp.Count != 25 {
		t.Errorf("expected count 25, got %d", resp.Count)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	item := resp.Items[0]
	if item.IDs != "987654321,987654322" {
		t.Errorf("expected IDs '987654321,987654322', got %q", item.IDs)
	}

	if item.AssetID != "987654321" {
		t.Errorf("expected AssetID '987654321', got %q", item.AssetID)
	}

	if item.Bot != "76561198000000001" {
		t.Errorf("expected Bot '76561198000000001', got %q", item.Bot)
	}

	if item.Name != "Unusual Burning Flames Team Captain" {
		t.Errorf("expected Name 'Unusual Burning Flames Team Captain', got %q", item.Name)
	}

	if item.Effect != "Burning Flames" {
		t.Errorf("expected Effect 'Burning Flames', got %q", item.Effect)
	}
}

func TestWithdrawResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"message": "Items withdrawal processed",
		"updated": 3,
		"locked": 0
	}`

	var resp WithdrawResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling WithdrawResponse: %v", err)
	}

	if resp.Message != "Items withdrawal processed" {
		t.Errorf("expected Message 'Items withdrawal processed', got %q", resp.Message)
	}

	if resp.Updated != 3 {
		t.Errorf("expected Updated 3, got %d", resp.Updated)
	}

	if resp.Locked != 0 {
		t.Errorf("expected Locked 0, got %d", resp.Locked)
	}
}

func TestGetCartResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"cart": [
			{
				"cartId": 42,
				"assetId": "987654321,987654322 ;",
				"count": 2,
				"item_id": 5678,
				"price": 15000,
				"name": "Unusual Burning Flames Team Captain ;",
				"image": "https://... ;",
				"effect": "Burning Flames;",
				"rarity": "Unusual;",
				"color": "#8650AC",
				"url": "unusual_team_captain",
				"quality": "Unusual",
				"type_steam": "Hat",
				"class": "hat",
				"craftable": 1,
				"slot": "Misc",
				"game": 440,
				"sheen": "",
				"killstreaker": "",
				"spell": "",
				"parts": "",
				"paint": "",
				"level": null,
				"festivized": 0,
				"inspect": ""
			}
		]
	}`

	var resp GetCartResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling GetCartResponse: %v", err)
	}

	if len(resp.Cart) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(resp.Cart))
	}

	item := resp.Cart[0]
	if item.CartID != 42 {
		t.Errorf("expected CartID 42, got %d", item.CartID)
	}

	if item.AssetID != "987654321,987654322" {
		t.Errorf("expected AssetID '987654321,987654322', got %q", item.AssetID)
	}

	if item.Name != "Unusual Burning Flames Team Captain" {
		t.Errorf("expected Name 'Unusual Burning Flames Team Captain', got %q", item.Name)
	}

	if item.Effect != "Burning Flames" {
		t.Errorf("expected Effect 'Burning Flames', got %q", item.Effect)
	}

	if item.Rarity != "Unusual" {
		t.Errorf("expected Rarity 'Unusual', got %q", item.Rarity)
	}
}

func TestUpdateCartResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"integrity": {
			"valid": false,
			"invalidItems": [
				{
					"cartId": 42,
					"assetId": "987654321; ",
					"reason": "price_changed; ",
					"currentPrice": 16000,
					"expectedPrice": 15000
				},
				{
					"cartId": 43,
					"assetId": "987654322; ",
					"reason": "unavailable; "
				}
			]
		},
		"replaced": [
			{
				"cartId": 42,
				"oldAssetId": "987654321; ",
				"newAssetId": "987654999; ",
				"reason": "price_changed; "
			}
		],
		"removed": [
			{
				"cartId": 43,
				"assetId": "987654322; ",
				"reason": "unavailable; "
			}
		],
		"cart": [
			{
				"cartId": 42,
				"assetId": "987654999; ",
				"count": 1,
				"item_id": 5678,
				"price": 15000,
				"name": "Unusual Burning Flames Team Captain"
			}
		]
	}`

	var resp UpdateCartResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling UpdateCartResponse: %v", err)
	}

	if resp.Integrity.Valid {
		t.Errorf("expected integrity.valid to be false, got true")
	}

	if len(resp.Integrity.InvalidItems) != 2 {
		t.Fatalf("expected 2 invalid items, got %d", len(resp.Integrity.InvalidItems))
	}

	if resp.Integrity.InvalidItems[0].AssetID != "987654321" {
		t.Errorf("expected AssetID '987654321', got %q", resp.Integrity.InvalidItems[0].AssetID)
	}

	if resp.Integrity.InvalidItems[0].Reason != "price_changed" {
		t.Errorf("expected Reason 'price_changed', got %q", resp.Integrity.InvalidItems[0].Reason)
	}

	if len(resp.Replaced) != 1 {
		t.Fatalf("expected 1 replaced item, got %d", len(resp.Replaced))
	}

	if resp.Replaced[0].OldAssetID != "987654321" {
		t.Errorf("expected oldAssetId '987654321', got %q", resp.Replaced[0].OldAssetID)
	}

	if resp.Replaced[0].NewAssetID != "987654999" {
		t.Errorf("expected newAssetId '987654999', got %q", resp.Replaced[0].NewAssetID)
	}

	if len(resp.Removed) != 1 {
		t.Fatalf("expected 1 removed item, got %d", len(resp.Removed))
	}

	if resp.Removed[0].AssetID != "987654322" {
		t.Errorf("expected assetId '987654322', got %q", resp.Removed[0].AssetID)
	}

	if len(resp.Cart) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(resp.Cart))
	}
}

func TestGetDepositInfoResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"informations": [
			{
				"assetid": "123456789;987654321 ;",
				"count": 2,
				"market_hash_name": "Unusual Burning Flames Team Captain ;",
				"item_id": 5678,
				"url": "unusual-burning-flames-team-captain ; ",
				"depositkey": {
					"123456789": "a1b2c3d4e5f6...",
					"987654321": "f6e5d4c3b2a1..."
				},
				"nb_high_stock": 45,
				"high_stock_limit": 200
			}
		]
	}`

	var resp GetDepositInfoResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling GetDepositInfoResponse: %v", err)
	}

	if len(resp.Informations) != 1 {
		t.Fatalf("expected 1 deposit info item, got %d", len(resp.Informations))
	}

	info := resp.Informations[0]
	if info.AssetID != "123456789;987654321" {
		t.Errorf("expected AssetID '123456789;987654321', got %q", info.AssetID)
	}

	if info.MarketHashName != "Unusual Burning Flames Team Captain" {
		t.Errorf("expected MarketHashName 'Unusual Burning Flames Team Captain', got %q", info.MarketHashName)
	}

	if info.URL != "unusual-burning-flames-team-captain" {
		t.Errorf("expected URL 'unusual-burning-flames-team-captain', got %q", info.URL)
	}

	if info.DepositKey["123456789"] != "a1b2c3d4e5f6..." {
		t.Errorf("expected depositkey of '123456789' to match, got %q", info.DepositKey["123456789"])
	}
}

func TestTradeStatusResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"trade": {
			"id": 123456,
			"items_received": "123456789,987654321 ;",
			"items_send": ";",
			"status": 3,
			"user": "76561198000000000; ",
			"bot": "76561198111111111; ",
			"code": "...",
			"lasterror": "; ",
			"offerid": "5678901234; ",
			"timestamp": "1706745600000; ",
			"game": 440
		}
	}`

	var resp TradeStatusResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling TradeStatusResponse: %v", err)
	}

	trade := resp.Trade
	if trade.ID != 123456 {
		t.Errorf("expected ID 123456, got %d", trade.ID)
	}

	if trade.ItemsReceived != "123456789,987654321" {
		t.Errorf("expected ItemsReceived '123456789,987654321', got %q", trade.ItemsReceived)
	}

	if trade.ItemsSend != "" {
		t.Errorf("expected ItemsSend empty string, got %q", trade.ItemsSend)
	}

	if trade.Status != 3 {
		t.Errorf("expected Status 3, got %d", trade.Status)
	}

	if trade.User != "76561198000000000" {
		t.Errorf("expected User '76561198000000000', got %q", trade.User)
	}
}

func TestGetTradesResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"trades": [
			{
				"id": 123456,
				"items_received": "123456789,987654321 ;",
				"status": 0,
				"items_send": ";",
				"user": "76561198000000000; ",
				"bot": "76561198111111111; ",
				"code": "ABCDEFGH; ",
				"lasterror": "; ",
				"offerid": "5678901234; ",
				"timestamp": "1706745600000; "
			}
		]
	}`

	var resp GetTradesResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling GetTradesResponse: %v", err)
	}

	if len(resp.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(resp.Trades))
	}

	trade := resp.Trades[0]
	if trade.ID != 123456 {
		t.Errorf("expected ID 123456, got %d", trade.ID)
	}

	if trade.Code != "ABCDEFGH" {
		t.Errorf("expected Code 'ABCDEFGH', got %q", trade.Code)
	}
}

func TestResendTradeResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"message": "Trade resent successfully ;",
		"trade_id": 123456,
		"code": "ABCDEFGH; "
	}`

	var resp ResendTradeResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling ResendTradeResponse: %v", err)
	}

	if resp.Message != "Trade resent successfully" {
		t.Errorf("expected Message 'Trade resent successfully', got %q", resp.Message)
	}

	if resp.TradeID != 123456 {
		t.Errorf("expected TradeID 123456, got %d", resp.TradeID)
	}

	if resp.Code != "ABCDEFGH" {
		t.Errorf("expected Code 'ABCDEFGH', got %q", resp.Code)
	}
}

func TestUserInfoResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"informations": {
			"steamId": "76561198000000000; ",
			"balance": 250000,
			"tradeurl": "https://steamcommunity.com/...; ",
			"name": "G-man; ",
			"image": "https://...; ",
			"2fa": "true; ",
			"shorturl": "gman; "
		}
	}`

	var resp UserInfoResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling UserInfoResponse: %v", err)
	}

	ui := resp.Informations
	if ui.SteamID != "76561198000000000" {
		t.Errorf("expected SteamID '76561198000000000', got %q", ui.SteamID)
	}

	if ui.Balance != 250000 {
		t.Errorf("expected Balance 250000, got %d", ui.Balance)
	}

	if ui.Name != "G-man" {
		t.Errorf("expected Name 'G-man', got %q", ui.Name)
	}

	if ui.TwoFA != "true" {
		t.Errorf("expected TwoFA 'true', got %q", ui.TwoFA)
	}
}

func TestPublicStoreProfileUnmarshal(t *testing.T) {
	inputJSON := `{
		"steamId": "76561198000000000; ",
		"image": "https://...; ",
		"name": "Bot Shop; "
	}`

	var resp PublicStoreProfile
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling PublicStoreProfile: %v", err)
	}

	if resp.SteamID != "76561198000000000" {
		t.Errorf("expected SteamID '76561198000000000', got %q", resp.SteamID)
	}

	if resp.Name != "Bot Shop" {
		t.Errorf("expected Name 'Bot Shop', got %q", resp.Name)
	}
}

func TestIPSessionListResponseUnmarshal(t *testing.T) {
	inputJSON := `{
		"values": [
			{
				"ip": "127.0.0.1; ",
				"location": "Localhost; ",
				"isp": "Local ISP; ",
				"current": 1,
				"timestamp": 1706745600
			}
		],
		"count": 1
	}`

	var resp IPSessionListResponse
	if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
		t.Fatalf("unexpected error unmarshaling IPSessionListResponse: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("expected Count 1, got %d", resp.Count)
	}

	if len(resp.Values) != 1 {
		t.Fatalf("expected 1 session value, got %d", len(resp.Values))
	}

	s := resp.Values[0]
	if s.IP != "127.0.0.1" {
		t.Errorf("expected IP '127.0.0.1', got %q", s.IP)
	}

	if s.Location != "Localhost" {
		t.Errorf("expected Location 'Localhost', got %q", s.Location)
	}

	if s.ISP != "Local ISP" {
		t.Errorf("expected ISP 'Local ISP', got %q", s.ISP)
	}
}

func TestClient_AllAPI(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("buy_orders", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "item/buyorder", 200, map[string]any{
			"success": true,
			"content": map[string]any{},
		})

		_, err := client.CreateBuyOrder(ctx, 123, 100, 5)
		require.NoError(t, err)

		setJSONResponse(stub, "item/buyorder/update", 200, map[string]any{
			"success": true,
			"content": "Updated",
		})

		_, err = client.UpdateBuyOrder(ctx, 123, 110, 10)
		require.NoError(t, err)

		setJSONResponse(stub, "item/buyorder/remove", 200, map[string]any{
			"success": true,
			"content": map[string]any{},
		})

		_, err = client.RemoveBuyOrder(ctx, 123)
		require.NoError(t, err)

		setJSONResponse(stub, "user/buyorder/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{
					"id":        98765,
					"steamid":   "76561198000000000; ",
					"itemid":    12345,
					"price":     1500,
					"amount":    1,
					"timestamp": "1706745600; ",
				},
			},
		})

		_, err = client.GetUserBuyOrdersForItem(ctx, "5021")
		require.NoError(t, err)

		setJSONResponse(stub, "user/getBuyorder", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetUserBuyOrders(ctx, GetUserBuyOrdersQuery{})
		require.NoError(t, err)
	})

	t.Run("cart", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "cart/get", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"cart": []any{},
			},
		})

		_, err := client.GetCart(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "cart/add", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"cart": []any{},
			},
		})

		_, err = client.AddToCart(ctx, "123")
		require.NoError(t, err)

		setJSONResponse(stub, "cart/bulk", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"cart": []any{},
			},
		})

		_, err = client.BulkAddToCart(ctx, 123, 2, "7656119")
		require.NoError(t, err)

		setJSONResponse(stub, "cart/remove", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"cart": []any{},
			},
		})

		_, err = client.RemoveFromCart(ctx, 42)
		require.NoError(t, err)

		setJSONResponse(stub, "cart/update", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"integrity": map[string]any{
					"valid":        true,
					"invalidItems": []any{},
				},
				"replaced": []any{},
				"removed":  []any{},
				"cart":     []any{},
			},
		})

		_, err = client.UpdateCart(ctx)
		require.NoError(t, err)
	})

	t.Run("deposit", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "deposit/440", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": []any{},
			},
		})

		_, err := client.GetDepositInfo(ctx, 440)
		require.NoError(t, err)

		setJSONResponse(stub, "deposit/trade", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"id": 123,
			},
		})

		_, err = client.CreateDepositTrade(ctx, CreateDepositTradeReq{})
		require.NoError(t, err)

		setJSONResponse(stub, "deposit/instantSell/440", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": []any{},
			},
		})

		_, err = client.GetInstantSellInfo(ctx, 440)
		require.NoError(t, err)

		setJSONResponse(stub, "deposit/trade/instant", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"id": 123,
			},
		})

		_, err = client.CreateInstantSellTrade(ctx, CreateInstantSellTradeReq{})
		require.NoError(t, err)

		setJSONResponse(stub, "deposit/tradeStatus/123", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"trade": map[string]any{
					"id": 123,
				},
			},
		})

		_, err = client.GetDepositTradeStatus(ctx, 123)
		require.NoError(t, err)
	})

	t.Run("inventory", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "inventory/onSale", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"items": []any{},
			},
		})

		_, err := client.GetItemsOnSale(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "inventory/onInventory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"items": []any{},
			},
		})

		_, err = client.GetItemsInInventory(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "inventory/price", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.SetItemPrice(ctx, []string{"123"}, 500)
		require.NoError(t, err)

		setJSONResponse(stub, "inventory/withdraw", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "Items withdrawal processed",
				"updated": 1,
			},
		})

		_, err = client.WithdrawItems(ctx, []string{"123"})
		require.NoError(t, err)
	})

	t.Run("items", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "item/details/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{
					"id": 5021,
				},
			},
		})

		_, err := client.GetItemDetails(ctx, "5021")
		require.NoError(t, err)

		setJSONResponse(stub, "item/salesGraph/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetItemSalesGraph(ctx, "5021", "1M")
		require.NoError(t, err)

		// Test GetListingCount without userID
		setJSONResponse(stub, "item/listing/count/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"count": 10,
			},
		})

		_, err = client.GetListingCount(ctx, "5021", "")
		require.NoError(t, err)

		// Test GetListingCount with userID
		setJSONResponse(stub, "item/listing/count/5021/7656119", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"count": 10,
			},
		})

		_, err = client.GetListingCount(ctx, "5021", "7656119")
		require.NoError(t, err)

		// Test GetItemListings without userID
		setJSONResponse(stub, "item/listing/5021", 200, map[string]any{
			"success": true,
			"content": []any{},
		})

		_, err = client.GetItemListings(ctx, "5021", "", ListingsReq{})
		require.NoError(t, err)

		// Test GetItemListings with userID
		setJSONResponse(stub, "item/listing/5021/7656119", 200, map[string]any{
			"success": true,
			"content": []any{},
		})

		_, err = client.GetItemListings(ctx, "5021", "7656119", ListingsReq{})
		require.NoError(t, err)

		setJSONResponse(stub, "item/buyorderList/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{},
			},
		})

		_, err = client.GetBuyOrderList(ctx, "5021")
		require.NoError(t, err)

		setJSONResponse(stub, "item/pricing/5021", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"item_id": 5021,
			},
		})

		_, err = client.GetItemPricing(ctx, "5021")
		require.NoError(t, err)

		setJSONResponse(stub, "item/pricing/bulk", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"items": []any{},
			},
		})

		_, err = client.GetBulkPricing(ctx, []string{"5021"})
		require.NoError(t, err)

		setJSONResponse(stub, "item/details/fromid/123", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{
					"id": 123,
				},
			},
		})

		_, err = client.GetBackpackDetailsTF2(ctx, "123")
		require.NoError(t, err)

		setJSONResponse(stub, "item/cs/details/fromid/123", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{
					"id": 123,
				},
			},
		})

		_, err = client.GetBackpackDetailsCS2(ctx, "123")
		require.NoError(t, err)
	})

	t.Run("offers", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "offers/received", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"offers": []any{},
			},
		})

		_, err := client.GetReceivedOffers(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "offers/my", 200, map[string]any{
			"success": true,
			"content": []any{},
		})

		_, err = client.GetMyOffers(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "offers/create", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.CreateOffer(ctx, 12345, 500)
		require.NoError(t, err)

		setJSONResponse(stub, "offers/accept", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.AcceptOffer(ctx, 12345)
		require.NoError(t, err)

		setJSONResponse(stub, "offers/decline", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.DeclineOffer(ctx, 12345)
		require.NoError(t, err)

		setJSONResponse(stub, "offers/remove", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.RemoveOffer(ctx, 12345)
		require.NoError(t, err)
	})

	t.Run("payment", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "payment/mannco", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err := client.InitiatePayment(ctx, "mannco", PaymentReq{})
		require.NoError(t, err)
	})

	t.Run("trades", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "trades/active", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"trades": []any{},
			},
		})

		_, err := client.GetActiveTrades(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "trades/all", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"trades": []any{},
			},
		})

		_, err = client.GetAllTrades(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "trade/resend", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"message": "ok",
			},
		})

		_, err = client.ResendTrade(ctx, 123)
		require.NoError(t, err)
	})

	t.Run("user", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		setJSONResponse(stub, "user/disconnect", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"disconnect": true,
			},
		})

		_, err := client.Disconnect(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "user/infos", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"informations": map[string]any{
					"steamId": "123",
				},
			},
		})

		_, err = client.GetUserInfo(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "user/balance", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"balance": 1000,
			},
		})

		_, err = client.GetBalance(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "user/notifications", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"alertCount": 0,
			},
		})

		_, err = client.GetNotifications(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "user/ipList", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetIPSessionList(ctx, IPSessionListQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/store/my-shop", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"steamId": "123",
			},
		})

		_, err = client.GetPublicStoreProfile(ctx, "my-shop")
		require.NoError(t, err)

		setJSONResponse(stub, "user/getSalesInfos", 200, map[string]any{
			"success": true,
			"content": map[string]any{},
		})

		_, err = client.GetSalesInfos(ctx)
		require.NoError(t, err)

		setJSONResponse(stub, "user/getSalesChartInfos", 200, map[string]any{
			"success": true,
			"content": map[string]any{},
		})

		_, err = client.GetSalesChartInfos(ctx, SalesChartQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getBalanceHistory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetBalanceHistory(ctx, BalanceHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getPurchaseHistory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetPurchaseHistory(ctx, PurchaseHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getSalesHistory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetSalesHistory(ctx, SalesHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getSalesHistory/123", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetSalesHistoryForUser(ctx, "123", SalesHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getCashoutHistory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetCashoutHistory(ctx, CashoutHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getTransactionHistory", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"values": []any{},
			},
		})

		_, err = client.GetTransactionHistory(ctx, TransactionHistoryQuery{})
		require.NoError(t, err)

		setJSONResponse(stub, "user/getTransactionDetails", 200, map[string]any{
			"success": true,
			"content": map[string]any{
				"transaction": map[string]any{},
			},
		})

		_, err = client.GetTransactionDetails(ctx, "123")
		require.NoError(t, err)
	})
}
