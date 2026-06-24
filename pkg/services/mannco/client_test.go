// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"encoding/json"
	"testing"
)

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
			expected: 1, // truncated to int
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
			"price": 15000,
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
