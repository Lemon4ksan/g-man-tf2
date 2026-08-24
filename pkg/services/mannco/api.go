// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"

	"github.com/lemon4ksan/aoni"
)

// API defines the declarative REST API endpoints for the Mannco.store platform.
// @aoni:service casing=snake_case
// @base_url "https://api.mannco.store/"
// @version "v1.0.0"
// @unwrap "content"
type API interface {
	// PostLogin authenticates using an API key and returns a JWT token.
	// @post "user/login"
	PostLogin(ctx context.Context, req LoginRequest, mods ...aoni.RequestModifier) (*LoginContent, error)

	// Disconnect invalidates the current session.
	// @get "user/disconnect"
	Disconnect(ctx context.Context, mods ...aoni.RequestModifier) (*DisconnectResponse, error)

	// GetUserInfo fetches current authenticated user details.
	// @get "user/infos"
	GetUserInfo(ctx context.Context, mods ...aoni.RequestModifier) (*UserInfoResponse, error)

	// GetBalance fetches current account balance.
	// @get "user/balance"
	GetBalance(ctx context.Context, mods ...aoni.RequestModifier) (*UserBalanceResponse, error)

	// GetNotifications summarizes unread notifications count.
	// @get "user/notifications"
	GetNotifications(ctx context.Context, mods ...aoni.RequestModifier) (*NotificationsResponse, error)

	// GetIPSessionList retrieves active and historical login sessions.
	// @get "user/ipList"
	GetIPSessionList(ctx context.Context, query IPSessionListQuery, mods ...aoni.RequestModifier) (*IPSessionListResponse, error)

	// GetPublicStoreProfile returns public store info by user identifier or slug.
	// @get "user/store/{identifier}"
	GetPublicStoreProfile(ctx context.Context, identifier string, mods ...aoni.RequestModifier) (*PublicStoreProfile, error)

	// GetSalesInfos returns summary sales statistics for the user.
	// @get "user/getSalesInfos"
	GetSalesInfos(ctx context.Context, mods ...aoni.RequestModifier) (*SalesInfosResponse, error)

	// GetSalesChartInfos returns aggregated sales data for plotting charts.
	// @get "user/getSalesChartInfos"
	GetSalesChartInfos(ctx context.Context, query SalesChartQuery, mods ...aoni.RequestModifier) (*SalesChartResponse, error)

	// GetBalanceHistory returns logs of deposits, payments, and balance adjustments.
	// @get "user/getBalanceHistory"
	GetBalanceHistory(ctx context.Context, query BalanceHistoryQuery, mods ...aoni.RequestModifier) (*BalanceHistoryResponse, error)

	// GetPurchaseHistory returns purchase checkout logs.
	// @get "user/getPurchaseHistory"
	GetPurchaseHistory(ctx context.Context, query PurchaseHistoryQuery, mods ...aoni.RequestModifier) (*PurchaseHistoryResponse, error)

	// GetSalesHistory returns marketplace sales logs.
	// @get "user/getSalesHistory"
	GetSalesHistory(ctx context.Context, query SalesHistoryQuery, mods ...aoni.RequestModifier) (*SalesHistoryResponse, error)

	// GetSalesHistoryForUser returns the sales history of another user (Admin only).
	// @get "user/getSalesHistory/{userid}"
	GetSalesHistoryForUser(ctx context.Context, userid string, query SalesHistoryQuery, mods ...aoni.RequestModifier) (*SalesHistoryResponse, error)

	// GetCashoutHistory returns cashout checkout transaction logs.
	// @get "user/getCashoutHistory"
	GetCashoutHistory(ctx context.Context, query CashoutHistoryQuery, mods ...aoni.RequestModifier) (*CashoutHistoryResponse, error)

	// GetTransactionHistory returns general account ledger details.
	// @get "user/getTransactionHistory"
	GetTransactionHistory(ctx context.Context, query TransactionHistoryQuery, mods ...aoni.RequestModifier) (*TransactionHistoryResponse, error)

	// GetTransactionDetails returns details of a single ledger transaction ID.
	// @get "user/getTransactionDetails"
	GetTransactionDetails(
		ctx context.Context,
		// @query "transactionId"
		transactionID string,
		mods ...aoni.RequestModifier,
	) (*TransactionDetailsResponse, error)

	// GetCryptoDepositHistory returns cryptocurrency top-ups logs.
	// @get "user/getCryptoDepositHistory"
	GetCryptoDepositHistory(ctx context.Context, query CryptoDepositHistoryQuery, mods ...aoni.RequestModifier) (*CryptoDepositHistoryResponse, error)

	// GetItemDetails contains the catalog information for a queried item ID or slug.
	// @get "item/details/{item}"
	GetItemDetails(ctx context.Context, item string, mods ...aoni.RequestModifier) (*ItemDetails, error)

	// GetItemSalesGraph returns historical sales summary chart points.
	// @get "item/salesGraph/{item}"
	GetItemSalesGraph(ctx context.Context, item string, period string, mods ...aoni.RequestModifier) (*ItemSalesGraph, error)

	// GetListingCountDirect returns the number of active sales listings for an item.
	// @get "item/listing/count/{item}"
	GetListingCountDirect(ctx context.Context, item string, mods ...aoni.RequestModifier) (*ListingCount, error)

	// GetListingCountForUser returns the number of active sales listings for an item by user ID.
	// @get "item/listing/count/{item}/{userid}"
	GetListingCountForUser(ctx context.Context, item string, userid string, mods ...aoni.RequestModifier) (*ListingCount, error)

	// GetItemListingsDirect fetches marketplace active listings sorted by price ascending.
	// @get "item/listing/{item}"
	GetItemListingsDirect(ctx context.Context, item string, query ListingsReq, mods ...aoni.RequestModifier) ([]Listing, error)

	// GetItemListingsForUser fetches marketplace active listings for an item by user ID.
	// @get "item/listing/{item}/{userid}"
	GetItemListingsForUser(ctx context.Context, item string, userid string, query ListingsReq, mods ...aoni.RequestModifier) ([]Listing, error)

	// GetBuyOrderList retrieves active buy orders grouped by pricing tiers.
	// @get "item/buyorderList/{item}"
	GetBuyOrderList(ctx context.Context, item string, mods ...aoni.RequestModifier) (*BuyOrderList, error)

	// GetItemPricing returns calculated lowest sale, highest buy order, Steam price, and suggested price.
	// @get "item/pricing/{item}"
	GetItemPricing(ctx context.Context, item string, mods ...aoni.RequestModifier) (*ItemPricing, error)

	// GetBulkPricingDirect returns calculated pricing for multiple items at once.
	// @get "item/pricing/bulk"
	GetBulkPricingDirect(ctx context.Context, items string, mods ...aoni.RequestModifier) (*BulkPricing, error)

	// GetBackpackDetailsTF2 queries full item details and backpack metrics for a TF2 asset ID.
	// @get "item/details/fromid/{backpackid}"
	GetBackpackDetailsTF2(ctx context.Context, backpackid string, mods ...aoni.RequestModifier) (*BackpackDetailsResponse, error)

	// GetBackpackDetailsCS2 queries full details for a CS2 asset ID.
	// @get "item/cs/details/fromid/{backpackid}"
	GetBackpackDetailsCS2(ctx context.Context, backpackid string, mods ...aoni.RequestModifier) (*BackpackDetailsResponse, error)

	// GetItemsOnSale returns items listed for sale by the user.
	// @get "inventory/onSale"
	GetItemsOnSale(ctx context.Context, mods ...aoni.RequestModifier) (*GetInventoryResponse, error)

	// GetItemsInInventory returns items currently in user inventory (not on sale).
	// @get "inventory/onInventory"
	GetItemsInInventory(ctx context.Context, mods ...aoni.RequestModifier) (*GetInventoryResponse, error)

	// SetPriceDirect updates pricing for inventory item asset IDs.
	// @post "inventory/price"
	SetPriceDirect(ctx context.Context, req SetPriceReq, mods ...aoni.RequestModifier) (*InventoryMessageResponse, error)

	// WithdrawDirect pulls items from inventory to the user's Steam Account.
	// @post "inventory/withdraw"
	WithdrawDirect(ctx context.Context, req WithdrawReq, mods ...aoni.RequestModifier) (*WithdrawResponse, error)

	// CreateBuyOrderDirect creates a new buy order.
	// @post "item/buyorder"
	CreateBuyOrderDirect(ctx context.Context, req CreateBuyOrderReq, mods ...aoni.RequestModifier) (*DetailsResponse, error)

	// UpdateBuyOrderDirect updates an existing buy order price or amount.
	// @post "item/buyorder/update"
	UpdateBuyOrderDirect(ctx context.Context, req UpdateBuyOrderReq, mods ...aoni.RequestModifier) (string, error)

	// RemoveBuyOrderDirect deletes an existing buy order.
	// @post "item/buyorder/remove"
	RemoveBuyOrderDirect(ctx context.Context, req RemoveBuyOrderReq, mods ...aoni.RequestModifier) (*DetailsResponse, error)

	// GetUserBuyOrdersForItem returns user's active buy order for a specific item.
	// @get "user/buyorder/{item}"
	GetUserBuyOrdersForItem(ctx context.Context, item string, mods ...aoni.RequestModifier) (*UserBuyOrderResponse, error)

	// GetUserBuyOrders retrieves all active buy orders for the authenticated user.
	// @get "user/getBuyorder"
	GetUserBuyOrders(ctx context.Context, query GetUserBuyOrdersQuery, mods ...aoni.RequestModifier) (*UserAllBuyOrdersResponse, error)

	// GetCart retrieves user's current shopping cart with full item metadata.
	// @get "cart/get"
	GetCart(ctx context.Context, mods ...aoni.RequestModifier) (*GetCartResponse, error)

	// AddToCartDirect inserts a single listing into user's shopping cart.
	// @post "cart/add"
	AddToCartDirect(ctx context.Context, req AddCartReq, mods ...aoni.RequestModifier) (*GetCartResponse, error)

	// BulkAddToCartDirect inserts multiple items into user's cart.
	// @post "cart/bulk"
	BulkAddToCartDirect(ctx context.Context, req BulkAddCartReq, mods ...aoni.RequestModifier) (*GetCartResponse, error)

	// RemoveFromCartDirect deletes a cart row from the user's cart.
	// @post "cart/remove"
	RemoveFromCartDirect(ctx context.Context, req RemoveCartReq, mods ...aoni.RequestModifier) (*GetCartResponse, error)

	// UpdateCart triggers an integrity scan on the cart.
	// @post "cart/update"
	UpdateCart(ctx context.Context, mods ...aoni.RequestModifier) (*UpdateCartResponse, error)

	// GetDepositInfo returns inventory items eligible for deposit for a specific game.
	// @get "deposit/{game}"
	GetDepositInfo(ctx context.Context, game int, mods ...aoni.RequestModifier) (*GetDepositInfoResponse, error)

	// GetInstantSellInfo returns enriched inventory data with instant-sell price values.
	// @get "deposit/instantSell/{game}"
	GetInstantSellInfo(ctx context.Context, game int, mods ...aoni.RequestModifier) (*GetDepositInfoResponse, error)

	// CreateDepositTrade creates a bot trade offer to deposit items onto the store.
	// @post "deposit/trade"
	CreateDepositTrade(ctx context.Context, req CreateDepositTradeReq, mods ...aoni.RequestModifier) (*CreateDepositTradeResponse, error)

	// CreateInstantSellTrade creates an instant sell bot trade offer.
	// @post "deposit/trade/instant"
	CreateInstantSellTrade(ctx context.Context, req CreateInstantSellTradeReq, mods ...aoni.RequestModifier) (*CreateDepositTradeResponse, error)

	// GetDepositTradeStatus retrieves the status and trade code of a deposit trade.
	// @get "deposit/tradeStatus/{tradeid}"
	GetDepositTradeStatus(ctx context.Context, tradeid int, mods ...aoni.RequestModifier) (*TradeStatusResponse, error)

	// GetActiveTrades returns pending, incomplete trades for the user.
	// @get "trades/active"
	GetActiveTrades(ctx context.Context, mods ...aoni.RequestModifier) (*GetTradesResponse, error)

	// GetAllTrades returns up to the last 500 historical trades for the user.
	// @get "trades/all"
	GetAllTrades(ctx context.Context, mods ...aoni.RequestModifier) (*GetTradesResponse, error)

	// ResendTrade triggers a retry for a failed or pending withdrawal trade.
	// @get "trade/resend"
	ResendTrade(ctx context.Context, id int, mods ...aoni.RequestModifier) (*ResendTradeResponse, error)

	// GetReceivedOffers returns trade offers received from other buyers.
	// @get "offers/received"
	GetReceivedOffers(ctx context.Context, mods ...aoni.RequestModifier) (*GetReceivedOffersResponse, error)

	// GetMyOffers returns active trade offers sent to other sellers.
	// @get "offers/my"
	GetMyOffers(ctx context.Context, mods ...aoni.RequestModifier) ([]Offer, error)

	// CreateOfferDirect initiates a purchase trade offer for an item on sale.
	// @post "offers/create"
	CreateOfferDirect(ctx context.Context, req CreateOfferReq, mods ...aoni.RequestModifier) (*OfferMessageResponse, error)

	// AcceptOfferDirect accepts a received offer (Seller action).
	// @post "offers/accept"
	AcceptOfferDirect(ctx context.Context, req OfferActionReq, mods ...aoni.RequestModifier) (*OfferMessageResponse, error)

	// DeclineOfferDirect declines an incoming trade offer (Seller action).
	// @post "offers/decline"
	DeclineOfferDirect(ctx context.Context, req OfferActionReq, mods ...aoni.RequestModifier) (*OfferMessageResponse, error)

	// RemoveOfferDirect cancels and removes an outgoing trade offer (Buyer action).
	// @post "offers/remove"
	RemoveOfferDirect(ctx context.Context, req OfferActionReq, mods ...aoni.RequestModifier) (*OfferMessageResponse, error)

	// InitiatePayment creates a checkout session to add balance credit or purchase cart listings.
	// @post "payment/{provider}"
	InitiatePayment(ctx context.Context, provider string, req PaymentReq, mods ...aoni.RequestModifier) (*PaymentResponse, error)
}
