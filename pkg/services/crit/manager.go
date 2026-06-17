// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"golang.org/x/time/rate"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// ModuleName is the unique name for this module in the Steam client.
const ModuleName = "crit_storefront"

// WithModule returns a steam.Option that registers the storefront manager in the client.
func WithModule(client *Client) steam.Option {
	return steam.WithModule(NewManager(client))
}

// From returns the crit manager module from the client.
func From(c *steam.Client) *Manager {
	return steam.GetModule[*Manager](c)
}

// OpType defines the operation type for a transaction job.
type OpType string

// Possible operation types for a transaction job.
const (
	OpCreate OpType = "create"
	OpUpdate OpType = "update"
	OpDelete OpType = "delete"
)

// Transaction represents an asynchronous operation job sent to the worker queue.
type Transaction struct {
	Op         OpType
	AssetID    string
	ListingID  string
	Currencies pricedb.Currencies
	ResultChan chan *TransactionResult
}

// TransactionResult encapsulates the result of an operation in the queue.
type TransactionResult struct {
	Listing *Listing
	Err     error
}

// BackpackProvider defines the backpack methods needed for stock checks.
type BackpackProvider interface {
	GetStock(sku string) int
	GetItemsBySKU(targetSKU string) []uint64
}

// PriceProvider defines the pricedb price retrieval methods.
type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
}

// ConfigProvider defines trading config manager retrieval.
type ConfigProvider interface {
	GetConfig() trading.Config
}

// Manager is the central storefront manager module for G-MAN.
// It implements module.Module, module.Auth, and listingsync.CritProvider interfaces.
type Manager struct {
	module.Base

	client *Client

	// State fields
	mu               sync.RWMutex
	steamID          id.ID
	customStoreSlug  string
	lastGroupCheck   time.Time
	groupCheckFailed bool

	// Active listings cache
	listingsMu     sync.RWMutex
	listings       map[string]Listing
	pendingUpdates map[string]pricedb.Currencies

	// Async request queue & rate limiter
	txQueue     chan Transaction
	rateLimiter *rate.Limiter

	// Cooldown parameters
	lastInvRefresh time.Time

	// External Providers (for event bus reaction)
	bp       BackpackProvider
	priceMgr PriceProvider
	cfgMgr   ConfigProvider
	commands commands.Registry
}

// NewManager creates a new storefront manager module instance.
func NewManager(client *Client) *Manager {
	return &Manager{
		Base:           module.New(ModuleName),
		client:         client,
		listings:       make(map[string]Listing),
		pendingUpdates: make(map[string]pricedb.Currencies),
		txQueue:        make(chan Transaction, 1000),
		rateLimiter:    rate.NewLimiter(rate.Limit(10), 10), // Max 10 requests/sec, burst size 10 (100ms average)
	}
}

// SetProviders registers dependencies for real-time inventory and pricing synchronization.
func (m *Manager) SetProviders(bp BackpackProvider, priceMgr PriceProvider, cfgMgr ConfigProvider) {
	m.mu.Lock()
	m.bp = bp
	m.priceMgr = priceMgr
	m.cfgMgr = cfgMgr
	dispatcher := m.commands
	m.mu.Unlock()

	// Dynamically override command descriptions from configuration if provided
	if dispatcher != nil && cfgMgr != nil {
		cfg := cfgMgr.GetConfig()
		for cmdName, desc := range cfg.CritCommandDescriptions {
			dispatcher.UpdateCommandDescription(cmdName, desc)
		}
	}
}

// Init registers chat commands and sets up logger.
func (m *Manager) Init(init module.InitContext) error {
	if err := m.Base.Init(init); err != nil {
		return err
	}

	m.Logger.Info("Initializing crit.tf storefront manager...")

	// Register commands if chat_commands module is present
	if dispatcher, err := module.Get[commands.Registry](init, "chat_commands"); err == nil {
		m.commands = dispatcher

		// Public / User commands
		dispatcher.Register("crit_url", m.handleURLCommand,
			commands.WithDescription("Gets storefront URL"),
		)

		// Admin / Trusted commands
		dispatcher.Register("crit_group", m.handleGroupInfoCommand,
			commands.WithAdmin(),
			commands.WithDescription("Displays storefront group information and members list"),
		)
		dispatcher.Register("crit_invite", m.handleInviteCommand,
			commands.WithAdmin(),
			commands.WithDescription("Invites a trader to the storefront group"),
			commands.WithArgsSchema(commands.Required[id.ID]("steamID")),
		)
		dispatcher.Register("crit_accept", m.handleAcceptInviteCommand,
			commands.WithAdmin(),
			commands.WithDescription("Accepts a pending storefront group invitation"),
			commands.WithArgsSchema(commands.Optional[int]("groupID")),
		)
		dispatcher.Register("crit_leave", m.handleLeaveGroupCommand,
			commands.WithAdmin(),
			commands.WithDescription("Leaves a storefront group"),
			commands.WithArgsSchema(commands.Optional[int]("groupID")),
		)
		m.Logger.Info("Successfully registered crit.tf storefront commands")
	}

	return nil
}

// Start starts background worker and event loop threads.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.Base.Start(ctx); err != nil {
		return err
	}

	// Start worker goroutine
	m.Go(func(ctx context.Context) {
		m.worker(ctx)
	})

	// Start event subscription loop
	m.Go(func(ctx context.Context) {
		m.eventLoop(ctx)
	})

	return nil
}

// StartAuthed captures the authenticated SteamID and schedules the async bootstrap sequence.
func (m *Manager) StartAuthed(ctx context.Context, auth module.AuthContext) error {
	m.mu.Lock()
	m.steamID = auth.SteamID()
	m.mu.Unlock()

	m.Logger.InfoContext(
		ctx,
		"Crit storefront manager authenticated. Scheduling bootstrap...",
		log.SteamID(m.steamID.Uint64()),
	)

	m.Go(func(ctx context.Context) {
		m.bootstrap(ctx)
	})

	return nil
}

// bootstrap performs the initial synchronization of listings and group configuration.
func (m *Manager) bootstrap(ctx context.Context) {
	m.Logger.InfoContext(ctx, "Starting storefront bootstrap...")

	// Fetch active listings
	listings, err := m.client.FetchMyListings(ctx)
	if err != nil {
		m.Logger.ErrorContext(ctx, "Failed to fetch active listings during bootstrap", log.Err(err))
	} else {
		m.listingsMu.Lock()

		m.listings = make(map[string]Listing)
		for _, l := range listings {
			m.listings[l.AssetID] = l
		}

		m.listingsMu.Unlock()
		m.Logger.InfoContext(ctx, "Successfully cached active listings", log.Int("count", len(listings)))
	}

	// Fetch group configuration
	m.checkGroup(ctx)

	m.Logger.InfoContext(ctx, "Storefront bootstrap completed")
}

// checkGroup retrieves store group details from the API, handling 404 group missing errors.
func (m *Manager) checkGroup(ctx context.Context) {
	group, err := m.client.GetMyGroup(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastGroupCheck = time.Now()
	if err != nil {
		var restError *aoni.APIError
		if errors.As(err, &restError) && restError.StatusCode == 404 {
			m.Logger.WarnContext(ctx,
				"Bot is not part of a store group (HTTP 404). Storefront URLs will fallback to standard profile link.",
			)
			m.groupCheckFailed = true
			m.customStoreSlug = ""
		} else {
			m.Logger.ErrorContext(ctx, "Failed to fetch group details from crit.tf", log.Err(err))
			m.groupCheckFailed = true
		}

		return
	}

	m.groupCheckFailed = false
	m.customStoreSlug = group.CustomStoreSlug
	m.Logger.InfoContext(ctx, "Successfully fetched storefront group info", log.String("slug", group.CustomStoreSlug))
}

// GetStorefrontURL thread-safely generates the showcase / storefront link.
// It returns a friendly URL if slug is cached, falls back to a profile-based URL under 404 cooldown (5 minutes),
// and triggers non-blocking background refreshes once the cooldown expires.
func (m *Manager) GetStorefrontURL(ctx context.Context) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.customStoreSlug != "" {
		return "https://crit.tf/group/" + m.customStoreSlug
	}

	if m.groupCheckFailed && time.Since(m.lastGroupCheck) < 5*time.Minute {
		if m.steamID.IsValid() {
			return fmt.Sprintf("https://crit.tf/profile/%d", m.steamID.Uint64())
		}

		return "https://crit.tf"
	}

	// Trigger async background refresh if cooldown has expired
	if m.steamID.IsValid() {
		m.lastGroupCheck = time.Now()

		moduleCtx := m.Ctx
		if moduleCtx == nil {
			moduleCtx = ctx
		}

		go func() {
			bgCtx, cancel := context.WithTimeout(moduleCtx, 10*time.Second)
			defer cancel()

			m.checkGroup(bgCtx)
		}()

		return fmt.Sprintf("https://crit.tf/profile/%d", m.steamID.Uint64())
	}

	return "https://crit.tf"
}

// RefreshInventory requests crit.tf to sync inventory from Steam, implementing a client-side 15-minute cooldown.
func (m *Manager) RefreshInventory(ctx context.Context) (*InventoryResponse, error) {
	m.mu.Lock()
	lastRefresh := m.lastInvRefresh
	timeSinceLast := time.Since(lastRefresh)

	if !lastRefresh.IsZero() && timeSinceLast < 15*time.Minute {
		remaining := 15*time.Minute - timeSinceLast
		m.mu.Unlock()
		m.Logger.WarnContext(
			ctx,
			"Inventory refresh requested too soon, blocked by cooldown",
			log.Duration("remaining", remaining),
		)

		return nil, fmt.Errorf("crit: inventory refresh is on cooldown, wait %s", remaining.Round(time.Second))
	}

	m.mu.Unlock()

	// Wait on rate limiter
	if err := m.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	res, err := m.client.RefreshInventory(ctx)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.lastInvRefresh = time.Now()
	m.mu.Unlock()

	return res, nil
}

// FindListingByAssetID returns cached listing for the specified asset ID.
func (m *Manager) FindListingByAssetID(assetID string) (Listing, bool) {
	m.listingsMu.RLock()
	defer m.listingsMu.RUnlock()

	l, exists := m.listings[assetID]

	return l, exists
}

// FetchCachedListings returns a copy of all active listings in the cache.
func (m *Manager) FetchCachedListings() []Listing {
	m.listingsMu.RLock()
	defer m.listingsMu.RUnlock()

	list := make([]Listing, 0, len(m.listings))
	for _, l := range m.listings {
		list = append(list, l)
	}

	return list
}

// EnqueueCreateListing pushes a create job onto the async queue.
func (m *Manager) EnqueueCreateListing(
	ctx context.Context,
	assetID string,
	currencies pricedb.Currencies,
) chan *TransactionResult {
	resChan := make(chan *TransactionResult, 1)
	m.txQueue <- Transaction{
		Op:         OpCreate,
		AssetID:    assetID,
		Currencies: currencies,
		ResultChan: resChan,
	}

	return resChan
}

// EnqueueUpdateListing pushes an update job onto the async queue.
func (m *Manager) EnqueueUpdateListing(
	ctx context.Context,
	assetID, listingID string,
	currencies pricedb.Currencies,
) chan *TransactionResult {
	resChan := make(chan *TransactionResult, 1)
	m.txQueue <- Transaction{
		Op:         OpUpdate,
		AssetID:    assetID,
		ListingID:  listingID,
		Currencies: currencies,
		ResultChan: resChan,
	}

	return resChan
}

// EnqueueDeleteListing pushes a delete job onto the async queue.
func (m *Manager) EnqueueDeleteListing(ctx context.Context, assetID, listingID string) chan *TransactionResult {
	resChan := make(chan *TransactionResult, 1)
	m.txQueue <- Transaction{
		Op:         OpDelete,
		AssetID:    assetID,
		ListingID:  listingID,
		ResultChan: resChan,
	}

	return resChan
}

// worker processes listings tasks sequentially while enforcing rate limits.
// It also provides self-healing recovery if an item is not found in crit.tf database.
func (m *Manager) worker(ctx context.Context) {
	defer func() {
		// Drain queue and close all pending result channels with cancellation error upon worker exit
		for {
			select {
			case tx := <-m.txQueue:
				if tx.ResultChan != nil {
					tx.ResultChan <- &TransactionResult{Err: context.Canceled}

					close(tx.ResultChan)
				}

			default:
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case tx, ok := <-m.txQueue:
			if !ok {
				return
			}

			// Enforce rate limiter before API request
			if err := m.rateLimiter.Wait(ctx); err != nil {
				if tx.ResultChan != nil {
					tx.ResultChan <- &TransactionResult{Err: err}

					close(tx.ResultChan)
				}

				m.listingsMu.Lock()
				delete(m.pendingUpdates, tx.AssetID)
				m.listingsMu.Unlock()

				continue
			}

			var (
				res *Listing
				err error
			)

			switch tx.Op {
			case OpCreate:
				res, err = m.client.CreateListing(ctx, tx.AssetID, tx.Currencies)
			case OpUpdate:
				res, err = m.client.UpdateListing(ctx, tx.ListingID, tx.Currencies)
			case OpDelete:
				err = m.client.DeleteListing(ctx, tx.ListingID)
			}

			// Self-healing Item Not Found HTTP 400 recovery
			var restError *aoni.APIError

			is400 := false
			if err != nil {
				is400 = (errors.As(err, &restError) && restError.StatusCode == 400) ||
					strings.Contains(err.Error(), "400") ||
					strings.Contains(err.Error(), "not found") ||
					strings.Contains(err.Error(), "item_not_found")
			}

			if tx.Op == OpCreate && err != nil && is400 {
				m.Logger.WarnContext(ctx,
					"Item not found on crit.tf during listing. Attempting inventory refresh...",
					log.String("asset_id", tx.AssetID),
					log.Err(err),
				)

				// Force a rate-limited inventory refresh
				_, refreshErr := m.RefreshInventory(ctx)
				if refreshErr != nil {
					m.Logger.ErrorContext(ctx, "Self-healing inventory refresh failed", log.Err(refreshErr))
				} else {
					m.Logger.InfoContext(ctx, "Self-healing inventory refresh successful. Re-enqueueing listing task.")
				}

				// Re-enqueue the listing creation task
				select {
				case m.txQueue <- tx:
					m.Logger.InfoContext(
						ctx,
						"Listing creation re-enqueued successfully",
						log.String("asset_id", tx.AssetID),
					)

				default:
					m.Logger.ErrorContext(
						ctx,
						"Queue full, failed to re-enqueue creation task",
						log.String("asset_id", tx.AssetID),
					)

					if tx.ResultChan != nil {
						tx.ResultChan <- &TransactionResult{Err: err}

						close(tx.ResultChan)
					}

					m.listingsMu.Lock()
					delete(m.pendingUpdates, tx.AssetID)
					m.listingsMu.Unlock()
				}

				continue
			}

			// Update cache under mutex upon successful operation, and always clean up pending updates map
			m.listingsMu.Lock()
			if err == nil {
				switch tx.Op {
				case OpCreate, OpUpdate:
					if res != nil {
						m.listings[tx.AssetID] = *res
					}
				case OpDelete:
					delete(m.listings, tx.AssetID)
				}
			}

			delete(m.pendingUpdates, tx.AssetID)
			m.listingsMu.Unlock()

			if tx.ResultChan != nil {
				tx.ResultChan <- &TransactionResult{Listing: res, Err: err}

				close(tx.ResultChan)
			}
		}
	}
}

// eventLoop handles event-driven listing updates based on the G-MAN internal event bus.
func (m *Manager) eventLoop(ctx context.Context) {
	sub := m.Bus.Subscribe(
		&tf2.ItemAcquiredEvent{},
		&tf2.ItemRemovedEvent{},
		&pricedb.PricelistUpdatedEvent{},
	)
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-sub.C():
			m.mu.RLock()
			bp := m.bp
			priceMgr := m.priceMgr
			cfgMgr := m.cfgMgr
			m.mu.RUnlock()

			if bp == nil || priceMgr == nil || cfgMgr == nil {
				continue // Skip if providers are not initialized
			}

			switch e := ev.(type) {
			case *tf2.ItemAcquiredEvent:
				if e.Item == nil {
					continue
				}

				skuStr := e.Item.SKU
				if skuStr == "" {
					skuStr = fmt.Sprintf("%d;%d", e.Item.DefIndex, e.Item.Quality)
				}

				assetID := strconv.FormatUint(e.Item.ID, 10)

				cfg := cfgMgr.GetConfig()
				enableSell := true

				minStock := 0
				if itemCfg, exists := cfg.Items[skuStr]; exists {
					enableSell = itemCfg.EnableSell
					minStock = itemCfg.MinStock
				}

				if !enableSell {
					continue
				}

				// Only list on crit.tf if we exceed MinStock limit
				physicalIDs := bp.GetItemsBySKU(skuStr)

				isTargetForSale := false
				for i, id := range physicalIDs {
					if id == e.Item.ID && i >= minStock {
						isTargetForSale = true
						break
					}
				}

				if isTargetForSale {
					price, ok := priceMgr.GetPrice(skuStr)
					if ok && price.Sell.Metal > 0 {
						m.Logger.InfoContext(ctx,
							"Event: Acquired item. Enqueueing creation job for crit.tf",
							log.String("sku", skuStr),
							log.String("asset_id", assetID),
						)
						m.EnqueueCreateListing(ctx, assetID, price.Sell)
					}
				}

			case *tf2.ItemRemovedEvent:
				assetID := strconv.FormatUint(e.ItemID, 10)
				if listing, exists := m.FindListingByAssetID(assetID); exists {
					listingID := fmt.Sprintf("%d", listing.ID)
					m.Logger.InfoContext(ctx,
						"Event: Item removed. Enqueueing deletion job for crit.tf",
						log.String("listing_id", listingID),
						log.String("asset_id", assetID),
					)
					m.EnqueueDeleteListing(ctx, assetID, listingID)
				}

			case *pricedb.PricelistUpdatedEvent:
				skuStr := e.SKU
				if e.Sell.Metal <= 0 {
					continue
				}

				m.listingsMu.Lock()
				for assetID, l := range m.listings {
					if l.SKU == skuStr {
						listingID := fmt.Sprintf("%d", l.ID)
						targetPrice := e.Sell

						// If the listing already matches the new price in the cache, no update needed
						if l.PriceKeys == targetPrice.Keys && float64(l.PriceMetal) == targetPrice.Metal {
							continue
						}

						// Check if there is already a pending update to this same price
						if pending, exists := m.pendingUpdates[assetID]; exists {
							if pending.Keys == targetPrice.Keys && pending.Metal == targetPrice.Metal {
								continue // Duplicate update to the same price is already enqueued!
							}
						}

						m.pendingUpdates[assetID] = targetPrice
						m.Logger.InfoContext(ctx,
							"Event: Price updated. Enqueueing update job for crit.tf",
							log.String("sku", skuStr),
							log.String("listing_id", listingID),
						)
						m.EnqueueUpdateListing(ctx, assetID, listingID, targetPrice)
					}
				}

				m.listingsMu.Unlock()
			}
		}
	}
}

// FetchMyListings fetches the user's listings from the crit.tf API, either from the cache or by making a network request.
func (m *Manager) FetchMyListings(ctx context.Context) ([]Listing, error) {
	m.listingsMu.RLock()

	if len(m.listings) > 0 {
		defer m.listingsMu.RUnlock()
		return m.FetchCachedListings(), nil
	}

	m.listingsMu.RUnlock()

	if err := m.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	return m.client.FetchMyListings(ctx)
}

// CreateListing creates a new listing on the crit.tf API.
func (m *Manager) CreateListing(ctx context.Context, assetID string, currencies pricedb.Currencies) (*Listing, error) {
	ch := m.EnqueueCreateListing(ctx, assetID, currencies)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.Listing, res.Err
	}
}

// UpdateListing updates an existing listing on the crit.tf API.
func (m *Manager) UpdateListing(
	ctx context.Context,
	listingID string,
	currencies pricedb.Currencies,
) (*Listing, error) {
	assetID := ""

	m.listingsMu.RLock()

	for aID, l := range m.listings {
		if fmt.Sprintf("%d", l.ID) == listingID {
			assetID = aID
			break
		}
	}

	m.listingsMu.RUnlock()

	ch := m.EnqueueUpdateListing(ctx, assetID, listingID, currencies)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.Listing, res.Err
	}
}

// DeleteListing deletes a listing on the crit.tf API.
func (m *Manager) DeleteListing(ctx context.Context, listingID string) error {
	assetID := ""

	m.listingsMu.RLock()

	for aID, l := range m.listings {
		if fmt.Sprintf("%d", l.ID) == listingID {
			assetID = aID
			break
		}
	}

	m.listingsMu.RUnlock()

	ch := m.EnqueueDeleteListing(ctx, assetID, listingID)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-ch:
		return res.Err
	}
}

// Group Administration Chat Command Handlers:

func (m *Manager) handleGroupInfoCommand(ctx context.Context, senderID uint64, args []string) (string, error) {
	group, err := m.client.GetMyGroup(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get group info: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Store Group: %s (ID: %d)\n", group.GroupName, group.ID)
	fmt.Fprintf(&sb, "Slug: %s\n", group.CustomStoreSlug)
	fmt.Fprintf(&sb, "Owner: %s (SteamID: %s)\n", group.OwnerName, group.OwnerSteamID)
	fmt.Fprintf(&sb, "Views: %d\n", group.ViewCount)
	fmt.Fprintf(&sb, "Description: %s\n\n", group.Description)

	accepted := []string{}
	pending := []string{}

	for _, member := range group.Members {
		name := member.DisplayName
		if name == "" {
			name = member.SteamID
		}

		info := fmt.Sprintf("- %s (%s) [%s]", name, member.SteamID, member.Role)
		switch member.InviteStatus {
		case "accepted":
			accepted = append(accepted, info)
		case "pending":
			pending = append(pending, info)
		}
	}

	sb.WriteString("Members (Accepted):\n")

	if len(accepted) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for _, info := range accepted {
			sb.WriteString(info + "\n")
		}
	}

	sb.WriteString("\nMembers (Pending):\n")

	if len(pending) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for _, info := range pending {
			sb.WriteString(info + "\n")
		}
	}

	return sb.String(), nil
}

func (m *Manager) handleInviteCommand(ctx context.Context, senderID uint64, steamID id.ID) (string, error) {
	group, err := m.client.GetMyGroup(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch group metadata: %w", err)
	}

	err = m.client.InviteToGroup(ctx, group.ID, steamID)
	if err != nil {
		return "", fmt.Errorf("invitation failed: %w", err)
	}

	return fmt.Sprintf("Successfully invited %s to group %s (ID: %d)", steamID, group.GroupName, group.ID), nil
}

func (m *Manager) handleAcceptInviteCommand(ctx context.Context, senderID uint64, groupID *int) (string, error) {
	if groupID == nil {
		invites, err := m.client.GetPendingInvites(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to fetch pending invites: %w", err)
		}

		if len(invites) == 0 {
			return "No pending storefront invitations found.", nil
		}

		var sb strings.Builder
		sb.WriteString("Pending Invitations:\n")

		for _, inv := range invites {
			fmt.Fprintf(&sb, "- Group: %s (ID: %d), Invited by: %s\n", inv.GroupName, inv.StoreGroupID, inv.InviterName)
		}

		sb.WriteString("Accept using: !crit_accept <groupID>")

		return sb.String(), nil
	}

	err := m.client.AcceptGroupInvite(ctx, *groupID)
	if err != nil {
		return "", fmt.Errorf("acceptance failed: %w", err)
	}

	// Fetch slug in the background
	m.Go(func(ctx context.Context) {
		m.checkGroup(ctx)
	})

	return fmt.Sprintf("Successfully accepted storefront group invitation for ID %d", *groupID), nil
}

func (m *Manager) handleLeaveGroupCommand(ctx context.Context, senderID uint64, groupID *int) (string, error) {
	var targetGroupID int

	if groupID == nil {
		group, err := m.client.GetMyGroup(ctx)
		if err != nil {
			return "Usage: !crit_leave <groupID> (or bot does not currently belong to a store group)", nil //nolint:nilerr
		}

		targetGroupID = group.ID
	} else {
		targetGroupID = *groupID
	}

	err := m.client.LeaveGroup(ctx, targetGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to leave group: %w", err)
	}

	m.mu.Lock()
	m.customStoreSlug = ""
	m.groupCheckFailed = true
	m.mu.Unlock()

	return fmt.Sprintf("Successfully left storefront group ID %d", targetGroupID), nil
}

func (m *Manager) handleURLCommand(ctx context.Context, senderID uint64, args []string) (string, error) {
	return "Storefront: " + m.GetStorefrontURL(ctx), nil
}
