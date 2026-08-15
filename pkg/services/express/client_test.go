// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package express_test

import (
	"sync"
	"testing"
	"time"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/express"
)

func setupTestClient(t *testing.T) (*express.Client, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	restClient := aoni.NewClient(stub)
	client := express.NewClient(restClient, "test-secret-token")

	return client, stub
}

func setJSONResponse(stub *mock.HTTPStub, path string, statusCode int, obj any) {
	stub.SetJSONResponse(path, statusCode, obj)
	stub.SetJSONResponse("/"+path, statusCode, obj)
	stub.SetJSONResponse("https://api.express-load.com/"+path, statusCode, obj)
	stub.SetJSONResponse("https://api.express-load.com//"+path, statusCode, obj)
}

func TestClient_Endpoints(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	testSteamID := id.Parse("76561198000000000")

	tests := []struct {
		name     string
		setup    func(stub *mock.HTTPStub)
		execute  func(client *express.Client) error
		validate func(t *testing.T)
	}{
		{
			name: "GetAccount success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(stub, "v2/account", 200, express.V2AccountResponse{
					Success: true,
					Data: &express.AccountData{
						CustomerID:     "cust-01900",
						BalanceCredits: 999,
						APIKey: &express.APIKeySummary{
							ID:                 "key-01900",
							Scopes:             []string{"steam.inventory.page.v1"},
							RateLimitPerMinute: 120,
						},
						Usage: &express.UsageSummary{
							Requests:       42,
							CreditsCharged: 40,
							Errors:         2,
						},
					},
					Meta: &express.ResponseMeta{RequestID: "req_01JEXAMPLE"},
				})
			},
			execute: func(client *express.Client) error {
				resp, err := client.GetAccountV2(ctx)
				if err != nil {
					return err
				}

				assert.True(t, resp.Success)
				assert.Equal(t, 999, resp.Data.BalanceCredits)
				assert.Equal(t, "cust-01900", resp.Data.CustomerID)
				assert.Equal(t, "req_01JEXAMPLE", resp.Meta.RequestID)

				return nil
			},
		},
		{
			name: "GetInventoryV2 success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(
					stub,
					"v2/steam/users/76561198000000000/inventory/440/2",
					200,
					express.V2InventoryResponse{
						Success: true,
						Data: &express.InventoryPage{
							TotalCount: 1,
							Assets: []express.SteamAsset{
								{
									Appid:      440,
									Contextid:  "2",
									Assetid:    "123456789",
									Classid:    "101",
									Instanceid: "0",
									Amount:     "1",
								},
							},
							Descriptions: []express.SteamDescription{
								{
									Appid:          440,
									Classid:        "101",
									Instanceid:     "0",
									MarketHashName: "Mann Co. Supply Crate Key",
									Tradable:       1,
								},
							},
							Pagination: &express.InventoryPagination{HasMore: false},
						},
						Meta: &express.BilledResponseMeta{
							RequestID:        "req_01JEXAMPLE",
							ProductCode:      "steam.inventory.page.v1",
							PriceVersion:     1,
							CreditsCharged:   1,
							CreditsRemaining: 998,
						},
					},
				)
			},
			execute: func(client *express.Client) error {
				resp, err := client.GetSteamUsersInventoryV2(
					ctx,
					testSteamID,
					440,
					2,
					express.GetSteamUsersInventoryV2Query{
						Language: "english",
					},
					express.WithIdempotencyKey("idemp-key-1"),
				)
				if err != nil {
					return err
				}

				assert.True(t, resp.Success)
				assert.Equal(t, 1, resp.Data.TotalCount)
				assert.Len(t, resp.Data.Assets, 1)
				assert.Equal(t, "123456789", resp.Data.Assets[0].Assetid)
				assert.Equal(t, "Mann Co. Supply Crate Key", resp.Data.Descriptions[0].MarketHashName)
				assert.Equal(t, 998, resp.Meta.CreditsRemaining)

				return nil
			},
		},
		{
			name: "GetMarketPrice success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(stub, "v2/steam/market/price", 200, express.V2MarketPriceResponse{
					Success: true,
					Data: &express.MarketPriceData{
						LowestPrice: strPtr("$2.26"),
						MedianPrice: strPtr("$2.27"),
						Volume:      strPtr("49,063"),
					},
					Meta: &express.BilledResponseMeta{
						CreditsCharged:   1,
						CreditsRemaining: 997,
					},
				})
			},
			execute: func(client *express.Client) error {
				resp, err := client.GetSteamMarketPriceV2(ctx, express.GetSteamMarketPriceV2Query{
					MarketHashName: "Mann Co. Supply Crate Key",
					AppID:          440,
					Currency:       1,
				})
				if err != nil {
					return err
				}

				assert.True(t, resp.Success)
				assert.Equal(t, "$2.26", resp.Data.LowestPrice)
				assert.Equal(t, "$2.27", resp.Data.MedianPrice)
				assert.Equal(t, "49,063", resp.Data.Volume)

				return nil
			},
		},
		{
			name: "GetStatus success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(stub, "v1/status", 200, express.PublicStatus{
					Status:    "operational",
					UpdatedAt: time.Now(),
					Services: []express.ServiceStatus{
						{Name: "Steam Inventory API", Status: "operational"},
					},
					History: []express.AvailabilityHour{
						{Timestamp: time.Now(), Availability: 1.0},
					},
				})
			},
			execute: func(client *express.Client) error {
				resp, err := client.GetStatus(ctx)
				if err != nil {
					return err
				}

				assert.Equal(t, "operational", resp.Status)
				assert.Len(t, resp.Services, 1)
				assert.Equal(t, "Steam Inventory API", resp.Services[0].Name)

				return nil
			},
		},
		{
			name: "CheckLive success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(stub, "health/live", 200, map[string]string{
					"status": "alive",
				})
			},
			execute: func(client *express.Client) error {
				resp, err := client.CheckLive(ctx)
				if err != nil {
					return err
				}

				assert.Equal(t, "alive", resp["status"])

				return nil
			},
		},
		{
			name: "CheckReady success",
			setup: func(stub *mock.HTTPStub) {
				setJSONResponse(stub, "health/ready", 200, map[string]string{
					"status": "ready",
				})
			},
			execute: func(client *express.Client) error {
				resp, err := client.CheckReady(ctx)
				if err != nil {
					return err
				}

				assert.Equal(t, "ready", resp["status"])

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, stub := setupTestClient(t)
			if tt.setup != nil {
				tt.setup(stub)
			}

			err := tt.execute(client)
			require.NoError(t, err)
		})
	}
}

func TestModels_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, data []byte)
	}{
		{
			name: "V2AccountResponse",
			input: `{
				"success": true,
				"data": {
					"customer_id": "01900000-0000-7000-8000-000000000001",
					"balance_credits": 999,
					"api_key": {
						"id": "01900000-0000-7000-8000-000000000002",
						"scopes": ["steam.inventory.page.v1"],
						"rate_limit_per_minute": 120
					},
					"usage": {
						"period_start": "2026-07-01",
						"period_end": "2026-07-19",
						"requests": 42,
						"credits_charged": 40,
						"errors": 2
					}
				},
				"meta": { "request_id": "req_01JEXAMPLE" }
			}`,
			validate: func(t *testing.T, data []byte) {
				var resp express.V2AccountResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, 999, resp.Data.BalanceCredits)
				assert.Equal(t, "01900000-0000-7000-8000-000000000001", resp.Data.CustomerID)
				assert.Equal(t, 120, resp.Data.APIKey.RateLimitPerMinute)
				assert.Equal(t, 42, resp.Data.Usage.Requests)
			},
		},
		{
			name: "V2InventoryResponse",
			input: `{
				"success": true,
				"data": {
					"assets": [
						{
							"appid": 440,
							"contextid": "2",
							"assetid": "123456789",
							"classid": "101",
							"instanceid": "0",
							"amount": "1"
						}
					],
					"descriptions": [
						{
							"appid": 440,
							"classid": "101",
							"instanceid": "0",
							"market_hash_name": "Mann Co. Supply Crate Key",
							"tradable": 1,
							"tags": [
								{
									"category": "Type",
									"internal_name": "Supply Crate Key",
									"localized_category_name": "Type",
									"localized_tag_name": "Supply Crate Key"
								}
							]
						}
					],
					"pagination": {
						"has_more": true,
						"next_cursor": "987654321"
					},
					"total_count": 100
				},
				"meta": {
					"request_id": "req_01JEXAMPLE",
					"product_code": "steam.inventory.page.v1",
					"price_version": 1,
					"credits_charged": 1,
					"credits_remaining": 998,
					"rate_limit": {
						"limit_per_minute": 120,
						"remaining": 119,
						"reset_seconds": 60
					}
				}
			}`,
			validate: func(t *testing.T, data []byte) {
				var resp express.V2InventoryResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, 100, resp.Data.TotalCount)
				assert.True(t, resp.Data.Pagination.HasMore)
				require.NotNil(t, resp.Data.Pagination.NextCursor)
				assert.Equal(t, "987654321", resp.Data.Pagination.NextCursor)
				assert.Len(t, resp.Data.Descriptions[0].Tags, 1)
				assert.Equal(t, "Type", resp.Data.Descriptions[0].Tags[0].Category)
			},
		},
		{
			name: "V2MarketPriceResponse",
			input: `{
				"success": true,
				"data": {
					"lowest_price": "$2.26",
					"median_price": "$2.27",
					"volume": "49,063"
				},
				"meta": {
					"request_id": "req_01JEXAMPLE",
					"product_code": "steam.market.price.v1",
					"price_version": 1,
					"credits_charged": 1,
					"credits_remaining": 997,
					"rate_limit": {
						"limit_per_minute": 120,
						"remaining": 118,
						"reset_seconds": 55
					}
				}
			}`,
			validate: func(t *testing.T, data []byte) {
				var resp express.V2MarketPriceResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				assert.True(t, resp.Success)
				require.NotNil(t, resp.Data.LowestPrice)
				assert.Equal(t, "$2.26", resp.Data.LowestPrice)
				require.NotNil(t, resp.Data.MedianPrice)
				assert.Equal(t, "$2.27", resp.Data.MedianPrice)
				require.NotNil(t, resp.Data.Volume)
				assert.Equal(t, "49,063", resp.Data.Volume)
			},
		},
		{
			name: "V2ErrorResponse",
			input: `{
				"success": false,
				"error": {
					"code": "invalid_api_key",
					"message": "Invalid API key",
					"request_id": "req_01JERR"
				}
			}`,
			validate: func(t *testing.T, data []byte) {
				var resp express.V2ErrorResponse

				err := json.Unmarshal(data, &resp)
				require.NoError(t, err)
				assert.False(t, resp.Success)
				assert.Equal(t, express.PublicAPIErrorCode("invalid_api_key"), resp.Error.Code)
				assert.Equal(t, express.PublicAPIErrorMessage("Invalid API key"), resp.Error.Message)
				assert.Equal(t, "req_01JERR", resp.Error.RequestID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.validate(t, []byte(tt.input))
		})
	}
}

func TestClient_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	client, stub := setupTestClient(t)

	setJSONResponse(stub, "v2/account", 200, express.V2AccountResponse{Success: true})
	setJSONResponse(stub, "v1/status", 200, express.PublicStatus{Status: "operational"})
	setJSONResponse(stub, "health/live", 200, map[string]string{"status": "alive"})
	setJSONResponse(stub, "health/ready", 200, map[string]string{"status": "ready"})

	ctx := t.Context()

	var wg sync.WaitGroup

	tasks := []func(){
		func() {
			resp, err := client.GetAccountV2(ctx)
			require.NoError(t, err)
			assert.True(t, resp.Success)
		},
		func() {
			resp, err := client.GetStatus(ctx)
			require.NoError(t, err)
			assert.Equal(t, "operational", resp.Status)
		},
		func() {
			resp, err := client.CheckLive(ctx)
			require.NoError(t, err)
			assert.Equal(t, "alive", resp["status"])
		},
		func() {
			resp, err := client.CheckReady(ctx)
			require.NoError(t, err)
			assert.Equal(t, "ready", resp["status"])
		},
	}

	for _, task := range tasks {
		wg.Go(task)
	}

	wg.Wait()
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
