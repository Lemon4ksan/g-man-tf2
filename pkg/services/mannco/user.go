// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lemon4ksan/aoni"
)

// DisconnectResponse outlines session invalidation results.
type DisconnectResponse struct {
	Disconnect bool `json:"disconnect"` // True if session disconnected
}

// UserInfo represents authenticated user profile parameters.
type UserInfo struct {
	SteamID      string          `json:"steamId"`      // Steam ID
	Balance      int             `json:"balance"`      // Account balance in cents
	TradeURL     string          `json:"tradeurl"`     // Configured Steam Trade URL
	Name         string          `json:"name"`         // Profile name
	Image        string          `json:"image"`        // Avatar URL
	TwoFA        string          `json:"2fa"`          // 2FA status string ("true" or "false")
	ShortURL     string          `json:"shorturl"`     // Public store slug / short URL
	Notification json.RawMessage `json:"notification"` // Notifications configuration
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on UserInfo.
func (ui *UserInfo) UnmarshalJSON(data []byte) error {
	type Alias UserInfo

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ui = UserInfo(aux)
	ui.SteamID = strings.TrimRight(ui.SteamID, " ;")
	ui.TradeURL = strings.TrimRight(ui.TradeURL, " ;")
	ui.Name = strings.TrimRight(ui.Name, " ;")
	ui.Image = strings.TrimRight(ui.Image, " ;")
	ui.TwoFA = strings.TrimRight(ui.TwoFA, " ;")
	ui.ShortURL = strings.TrimRight(ui.ShortURL, " ;")

	return nil
}

// UserInfoResponse wraps user account details.
type UserInfoResponse struct {
	Informations UserInfo `json:"informations"` // User details
}

// UserBalanceResponse wraps account balance.
type UserBalanceResponse struct {
	Balance int `json:"balance"` // Balance in cents
}

// NotificationsResponse summarizes unread notifications count.
type NotificationsResponse struct {
	Alerts              []json.RawMessage `json:"alerts"`              // Unread alerts array
	AlertCount          int               `json:"alertCount"`          // Alerts count
	PaymentReviews      []json.RawMessage `json:"paymentReviews"`      // Payment review logs
	PaymentReviewsCount int               `json:"paymentReviewsCount"` // Payment reviews count
	Messages            []json.RawMessage `json:"messages"`            // Direct messages
	MessagesCount       int               `json:"messagesCount"`       // Messages count
	Offers              []json.RawMessage `json:"offers"`              // Trade offers
	OffersCount         int               `json:"offersCount"`         // Offers count
	Trades              []json.RawMessage `json:"trades"`              // Bot trades
	TradesCount         int               `json:"tradesCount"`         // Trades count
}

// SessionRow represents user login session details.
type SessionRow struct {
	IP        string `json:"ip"`        // Session login IP address
	Location  string `json:"location"`  // Geographic location lookup
	ISP       string `json:"isp"`       // Internet Service Provider name
	Current   int    `json:"current"`   // 1 if current active session, 0 otherwise
	Timestamp int64  `json:"timestamp"` // Login Unix timestamp
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on SessionRow.
func (s *SessionRow) UnmarshalJSON(data []byte) error {
	type Alias SessionRow

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*s = SessionRow(aux)
	s.IP = strings.TrimRight(s.IP, " ;")
	s.Location = strings.TrimRight(s.Location, " ;")
	s.ISP = strings.TrimRight(s.ISP, " ;")

	return nil
}

// IPSessionListResponse represents active and historical login sessions.
type IPSessionListResponse struct {
	Values []SessionRow `json:"values"` // Session rows
	Count  int          `json:"count"`  // Total count
}

// IPSessionListQuery holds filtering queries for GetIPSessionList.
type IPSessionListQuery struct {
	Page    int    `url:"page,omitempty"`    // Page index
	PerPage int    `url:"perPage,omitempty"` // Page size (max 50)
	Expire  string `url:"expire,omitempty"`  // Pass "true" to include expired sessions
}

// PublicStoreProfile contains public metadata for user store lookup.
type PublicStoreProfile struct {
	SteamID string `json:"steamId"` // Store owner's Steam ID
	Image   string `json:"image"`   // Avatar image URL
	Name    string `json:"name"`    // Store name
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on PublicStoreProfile.
func (p *PublicStoreProfile) UnmarshalJSON(data []byte) error {
	type Alias PublicStoreProfile

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = PublicStoreProfile(aux)
	p.SteamID = strings.TrimRight(p.SteamID, " ;")
	p.Image = strings.TrimRight(p.Image, " ;")
	p.Name = strings.TrimRight(p.Name, " ;")

	return nil
}

// SalesInfosResponse summarizes sale metrics.
type SalesInfosResponse map[string]any

// SalesChartQuery holds query parameters for GetSalesChartInfos.
type SalesChartQuery struct {
	Period    string `url:"period,omitempty"`     // 1M, 3M, 6M, 1Y, 5Y, ALL
	Chart     any    `url:"chart,omitempty"`      // Filter charts configuration
	ChartOnly any    `url:"chart_only,omitempty"` // Filter charts only configuration
}

// SalesChartResponse contains chart data points.
type SalesChartResponse map[string]any

// BalanceHistoryRow represents balance history entry.
type BalanceHistoryRow map[string]any

// BalanceHistoryResponse represents list of balance deposits, withdrawals, and updates.
type BalanceHistoryResponse struct {
	Values []BalanceHistoryRow `json:"values"`          // History entries
	Count  int                 `json:"count,omitempty"` // Total count (returned on first page only)
}

// BalanceHistoryQuery holds query parameters for GetBalanceHistory.
type BalanceHistoryQuery struct {
	Page  int `url:"page,omitempty"`  // Page index
	Limit int `url:"limit,omitempty"` // Page size (max 50)
}

// PurchaseHistoryRow represents purchase history entry.
type PurchaseHistoryRow map[string]any

// PurchaseHistoryResponse represents purchase checkouts logs.
type PurchaseHistoryResponse struct {
	Values []PurchaseHistoryRow `json:"values"`          // History entries
	Count  int                  `json:"count,omitempty"` // Total count (returned on first page only)
}

// PurchaseHistoryQuery holds query parameters for GetPurchaseHistory.
type PurchaseHistoryQuery struct {
	Page  int `url:"page,omitempty"`  // Page index
	Count int `url:"count,omitempty"` // Page size (max 50)
}

// SalesHistoryQuery holds search and paging parameters for GetSalesHistory.
type SalesHistoryQuery struct {
	Page    int    `url:"page,omitempty"`    // Page index
	PerPage int    `url:"perPage,omitempty"` // Page size (max 50)
	Range   string `url:"range,omitempty"`   // Range window: 1W (default), 1M, 1Y, or all
	Search  string `url:"search,omitempty"`  // Term filter (min 3 chars)
}

// SalesHistoryRow represents item sales checkout entry.
type SalesHistoryRow map[string]any

// SalesHistoryResponse represents listings sales transactions.
type SalesHistoryResponse struct {
	Values []SalesHistoryRow `json:"values"`          // History entries
	Count  int               `json:"count,omitempty"` // Total count (returned on first page only)
}

// CashoutHistoryRow represents cashout transaction log.
type CashoutHistoryRow map[string]any

// CashoutHistoryResponse represents balance cashouts history.
type CashoutHistoryResponse struct {
	Values []CashoutHistoryRow `json:"values"`          // History entries
	Count  int                 `json:"count,omitempty"` // Total count
}

// CashoutHistoryQuery holds paging configuration for GetCashoutHistory.
type CashoutHistoryQuery struct {
	Page    int `url:"page,omitempty"`    // Page index
	Count   int `url:"count,omitempty"`   // Page size (max 50)
	PerPage int `url:"perpage,omitempty"` // Page size alternative
	Limit   int `url:"limit,omitempty"`   // Page size alternative
}

// TransactionHistoryRow represents general ledger transaction.
type TransactionHistoryRow map[string]any

// TransactionHistoryResponse represents ledger details.
type TransactionHistoryResponse struct {
	Values []TransactionHistoryRow `json:"values"`          // History entries
	Count  int                     `json:"count,omitempty"` // Total count
}

// TransactionHistoryQuery holds query configurations for GetTransactionHistory.
type TransactionHistoryQuery struct {
	Page   int    `url:"page,omitempty"`   // Page index
	Limit  int    `url:"limit,omitempty"`  // Page size (max 50)
	Search string `url:"search,omitempty"` // Search filter
}

// TransactionDetailsQuery represents query params for GetTransactionDetails.
type TransactionDetailsQuery struct {
	TransactionID string `url:"transactionId"` // Transaction ID
}

// TransactionDetailsResponse wraps ledger entry transaction details.
type TransactionDetailsResponse struct {
	Transaction json.RawMessage `json:"transaction"` // Detailed transaction payload
}

// Disconnect invalidates the caller's JWT login session token immediately.
//
// Route: GET /user/disconnect
// Permission: Connected + API
func (c *Client) Disconnect(ctx context.Context) (*DisconnectResponse, error) {
	return aoni.GetJSON[DisconnectResponse](ctx, c.getClient(), "/user/disconnect")
}

// GetUserInfo returns account metadata, balance, name, and 2FA status.
//
// Route: GET /user/infos
// Permission: Connected + API
func (c *Client) GetUserInfo(ctx context.Context) (*UserInfoResponse, error) {
	return aoni.GetJSON[UserInfoResponse](ctx, c.getClient(), "/user/infos")
}

// GetBalance returns current account balance in cents.
//
// Route: GET /user/balance
// Permission: Connected + API
func (c *Client) GetBalance(ctx context.Context) (*UserBalanceResponse, error) {
	return aoni.GetJSON[UserBalanceResponse](ctx, c.getClient(), "/user/balance")
}

// GetNotifications returns unread notification metrics across categories.
//
// Route: GET /user/notifications
// Permission: Connected + API
func (c *Client) GetNotifications(ctx context.Context) (*NotificationsResponse, error) {
	return aoni.GetJSON[NotificationsResponse](ctx, c.getClient(), "/user/notifications")
}

// GetIPSessionList retrieves active and historical user login sessions.
//
// Route: GET /user/ipList
// Permission: Connected + API
func (c *Client) GetIPSessionList(ctx context.Context, query IPSessionListQuery) (*IPSessionListResponse, error) {
	return aoni.GetJSON[IPSessionListResponse](ctx, c.getClient(), "/user/ipList", aoni.WithQuery(query))
}

// GetPublicStoreProfile returns minimal public profile details of a storefront ID.
// Requires no session Bearer token authorization.
//
// Route: GET /user/store/{identifier}
// Permission: API Only (No session required)
func (c *Client) GetPublicStoreProfile(ctx context.Context, identifier string) (*PublicStoreProfile, error) {
	return aoni.GetJSON[PublicStoreProfile](
		ctx, c.getClient(), "/user/store/{identifier}",
		aoni.WithVar("identifier", identifier),
	)
}

// GetSalesInfos returns summary sales statistics for the user.
//
// Route: GET /user/getSalesInfos
// Permission: Connected + API
func (c *Client) GetSalesInfos(ctx context.Context) (*SalesInfosResponse, error) {
	return aoni.GetJSON[SalesInfosResponse](ctx, c.getClient(), "/user/getSalesInfos")
}

// GetSalesChartInfos returns aggregated sales data for plotting charts.
//
// Route: GET /user/getSalesChartInfos
// Permission: Connected + API
func (c *Client) GetSalesChartInfos(ctx context.Context, query SalesChartQuery) (*SalesChartResponse, error) {
	return aoni.GetJSON[SalesChartResponse](ctx, c.getClient(), "/user/getSalesChartInfos", aoni.WithQuery(query))
}

// GetBalanceHistory returns logs of deposits, payments, and balance adjustments.
//
// Route: GET /user/getBalanceHistory
// Permission: Connected + API
func (c *Client) GetBalanceHistory(ctx context.Context, query BalanceHistoryQuery) (*BalanceHistoryResponse, error) {
	return aoni.GetJSON[BalanceHistoryResponse](ctx, c.getClient(), "/user/getBalanceHistory", aoni.WithQuery(query))
}

// GetPurchaseHistory returns purchase checkout logs.
//
// Route: GET /user/getPurchaseHistory
// Permission: Connected + API
func (c *Client) GetPurchaseHistory(ctx context.Context, query PurchaseHistoryQuery) (*PurchaseHistoryResponse, error) {
	return aoni.GetJSON[PurchaseHistoryResponse](ctx, c.getClient(), "/user/getPurchaseHistory", aoni.WithQuery(query))
}

// GetSalesHistory returns marketplace sales logs.
// Search query filter requires at least 3 characters.
//
// Route: GET /user/getSalesHistory
// Permission: Connected + API
func (c *Client) GetSalesHistory(ctx context.Context, query SalesHistoryQuery) (*SalesHistoryResponse, error) {
	return aoni.GetJSON[SalesHistoryResponse](ctx, c.getClient(), "/user/getSalesHistory", aoni.WithQuery(query))
}

// GetSalesHistoryForUser returns the sales history of another user.
// Restricted to admin credentials.
//
// Route: GET /user/getSalesHistory/{userid}
// Permission: Connected + Admin
func (c *Client) GetSalesHistoryForUser(
	ctx context.Context,
	userID string,
	query SalesHistoryQuery,
) (*SalesHistoryResponse, error) {
	return aoni.GetJSON[SalesHistoryResponse](
		ctx, c.getClient(), "/user/getSalesHistory/{userid}",
		aoni.WithVar("userid", userID),
		aoni.WithQuery(query),
	)
}

// GetCashoutHistory returns cashout checkout transaction logs.
//
// Route: GET /user/getCashoutHistory
// Permission: Connected + API
func (c *Client) GetCashoutHistory(ctx context.Context, query CashoutHistoryQuery) (*CashoutHistoryResponse, error) {
	return aoni.GetJSON[CashoutHistoryResponse](ctx, c.getClient(), "/user/getCashoutHistory", aoni.WithQuery(query))
}

// GetTransactionHistory returns ledger details.
//
// Route: GET /user/getTransactionHistory
// Permission: Connected + API
func (c *Client) GetTransactionHistory(
	ctx context.Context,
	query TransactionHistoryQuery,
) (*TransactionHistoryResponse, error) {
	return aoni.GetJSON[TransactionHistoryResponse](
		ctx, c.getClient(), "/user/getTransactionHistory",
		aoni.WithQuery(query),
	)
}

// GetTransactionDetails returns details of a single ledger transaction ID.
//
// Route: GET /user/getTransactionDetails?transactionId={id}
// Permission: Connected + API
func (c *Client) GetTransactionDetails(ctx context.Context, transactionID string) (*TransactionDetailsResponse, error) {
	req := TransactionDetailsQuery{TransactionID: transactionID}

	return aoni.GetJSON[TransactionDetailsResponse](
		ctx, c.getClient(), "/user/getTransactionDetails",
		aoni.WithQuery(req),
	)
}
