// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andygrunwald/vdf"
	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-tf2/internal/stringpool"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

// ModuleName is the name of the schema manager module.
const ModuleName string = "tf2_schema"

// Config holds configuration parameters for the [Manager].
type Config struct {
	// UpdateInterval defines the time interval between schema updates.
	UpdateInterval time.Duration
	// LiteMode enables pruning of items_game data to reduce RAM usage.
	LiteMode bool
	// CachePath defines the path to the local schema cache file.
	CachePath string
	// PaintKitURL represents the URL used to fetch paintkit translation strings.
	PaintKitURL string
	// SchemaMirrorURL represents the backup URL for fetching schema overview data.
	SchemaMirrorURL string
	// ItemsMirrorURL represents the backup URL for fetching schema item lists.
	ItemsMirrorURL string
	// ItemsGameMirrorURL represents the backup URL for fetching items_game.txt.
	ItemsGameMirrorURL string
}

// DefaultConfig returns a [Config] containing production-ready defaults.
func DefaultConfig() Config {
	return Config{
		UpdateInterval:     24 * time.Hour,
		LiteMode:           false,
		CachePath:          "cache/tf2/json",
		PaintKitURL:        "https://raw.githubusercontent.com/SteamDatabase/GameTracking-TF2/master/tf/resource/tf_proto_obj_defs_english.txt",
		ItemsGameMirrorURL: "https://raw.githubusercontent.com/SteamDatabase/GameTracking-TF2/master/tf/scripts/items/items_game.txt",
	}
}

// WithModule returns a [steam.Option] that registers the [Manager] module with the client.
func WithModule(cfg Config) steam.Option {
	return steam.WithModule(NewManager(cfg))
}

// From returns the [Manager] module instance retrieved from the [steam.Client].
func From(c *steam.Client) *Manager {
	return steam.GetModule[*Manager](c)
}

// OverviewResult represents the strongly typed result from IEconItems_440/GetSchemaOverview.
type OverviewResult struct {
	Status                               int                   `json:"status"`
	ItemsGameURL                         string                `json:"items_game_url"`
	Items                                []*Item               `json:"items,omitempty"`
	Qualities                            map[string]int        `json:"qualities"`
	QualityNames                         map[string]string     `json:"qualityNames"`
	OriginNames                          []*OriginName         `json:"originNames"`
	Attributes                           []*AttributeSchema    `json:"attributes"`
	AttributeControlledAttachedParticles []*ParticleEffect     `json:"attribute_controlled_attached_particles"`
	ItemSets                             []*ItemSet            `json:"item_sets"`
	ItemLevels                           []*ItemLevel          `json:"item_levels"`
	KillEaterScoreTypes                  []*KillEaterScoreType `json:"kill_eater_score_types"`
	StringLookups                        []*StringLookup       `json:"string_lookups"`
}

// OverviewResponse wraps the Steam WebAPI response for schema overview.
type OverviewResponse struct {
	Result OverviewResult `json:"result"`
}

// ItemsResult represents the result payload for IEconItems_440/GetSchemaItems.
type ItemsResult struct {
	Status int     `json:"status"`
	Next   int     `json:"next"`
	Items  []*Item `json:"items"`
}

// ItemsResponse wraps the Steam WebAPI response for schema items.
type ItemsResponse struct {
	Result ItemsResult `json:"result"`
}

// Manager manages background updates and local caching of the TF2 item schema.
type Manager struct {
	module.Base

	config  Config
	service service.Doer
	rest    request.Requester
	pricedb *pricedb.Client

	mu            sync.RWMutex
	schema        *Schema
	refreshMu     sync.Mutex
	refreshChan   chan struct{}
	lastGCVersion uint32
}

// NewManager constructs a new [Manager] instance.
func NewManager(cfg Config) *Manager {
	if cfg.UpdateInterval < 1*time.Minute {
		cfg.UpdateInterval = 24 * time.Hour
	}

	return &Manager{
		Base:   module.New(ModuleName),
		config: cfg,
	}
}

// Name returns the unique module name [ModuleName].
func (m *Manager) Name() string { return ModuleName }

// Init initializes the module dependencies within the [module.InitContext].
func (m *Manager) Init(init module.InitContext) error {
	if err := m.Base.Init(init); err != nil {
		return err
	}

	m.service = init.Service()
	m.rest = init.Rest()

	if aoniClient := request.UnwrapClient(m.rest); aoniClient != nil {
		unlimitedClient := aoniClient.With(option.WithMaxResponseSize(0))
		m.rest = unlimitedClient
		m.pricedb = pricedb.NewClient(unlimitedClient)
	} else {
		unlimitedClient := aoni.NewClient(nil, option.WithMaxResponseSize(0))

		m.rest = unlimitedClient
		m.pricedb = pricedb.NewClient(unlimitedClient)
	}

	return nil
}

// StartAuthed starts background polling, updates, and events listening routines.
func (m *Manager) StartAuthed(ctx context.Context, _ module.AuthContext) error {
	m.Logger.Info("Starting TF2 Schema loading...")

	sub := m.Bus.Subscribe(&UpdateRequestedEvent{})

	m.Go(func(ctx context.Context) {
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.C():
				if !ok {
					return
				}

				req := ev.(*UpdateRequestedEvent)
				m.handleUpdateRequested(req)
			}
		}
	})

	if err := m.loadFromCache(); err != nil {
		m.Logger.InfoContext(ctx, "Cache not available or invalid, performing full refresh", log.Err(err))

		if err := m.Refresh(ctx); err != nil {
			return fmt.Errorf("initial schema fetch failed: %w", err)
		}
	} else {
		m.Logger.InfoContext(ctx, "Schema loaded from cache",
			log.Time("time", m.schema.Time),
			log.Int("items", m.schema.ItemCount()),
		)
	}

	m.Bus.Publish(&ReadyEvent{})

	m.Go(func(moduleCtx context.Context) {
		m.refreshLoop(moduleCtx)
	})

	return nil
}

func (m *Manager) handleUpdateRequested(req *UpdateRequestedEvent) {
	m.mu.Lock()
	hasSchema := m.schema != nil

	currentVersion := ""
	if hasSchema {
		currentVersion = m.schema.Version
	}

	lastGC := m.lastGCVersion
	m.mu.Unlock()

	if hasSchema && (req.ItemsGameURL == "" || currentVersion == req.ItemsGameURL || lastGC == req.Version) {
		m.Logger.Debug("Schema is already up-to-date, skipping update request",
			log.Uint32("requested_version", req.Version),
			log.Uint32("current_gc_version", lastGC),
			log.String("current_url", currentVersion),
		)

		if lastGC != req.Version {
			m.mu.Lock()
			m.lastGCVersion = req.Version
			m.mu.Unlock()
		}

		return
	}

	m.Logger.Info("Schema update requested",
		log.Uint32("version", req.Version),
		log.String("url", req.ItemsGameURL),
	)

	m.Go(func(ctx context.Context) {
		if err := m.doRefresh(ctx, req.ItemsGameURL); err != nil {
			if errors.Is(err, context.Canceled) {
				m.Logger.DebugContext(ctx, "Manual schema refresh cancelled due to shutdown")
			} else {
				m.Logger.ErrorContext(ctx, "Manual schema refresh failed", log.Err(err))
			}
		} else {
			m.mu.Lock()
			m.lastGCVersion = req.Version
			m.mu.Unlock()
			m.Logger.InfoContext(
				ctx,
				"Schema updated successfully after update request",
				log.Uint32("version", req.Version),
			)
		}
	})
}

// Get returns the current active [Schema] instance.
func (m *Manager) Get() *Schema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.schema
}

// Refresh manually triggers a full schema update.
func (m *Manager) Refresh(ctx context.Context) error {
	return m.doRefresh(ctx, m.config.ItemsGameMirrorURL)
}

func (m *Manager) doRefresh(ctx context.Context, itemsGameURL string) error {
	m.refreshMu.Lock()
	if m.refreshChan != nil {
		ch := m.refreshChan
		m.refreshMu.Unlock()

		m.Logger.DebugContext(ctx, "Schema refresh already in progress, waiting for completion...")

		select {
		case <-ch:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ch := make(chan struct{})
	m.refreshChan = ch
	m.refreshMu.Unlock()

	err := m.refreshSchema(ctx, itemsGameURL)

	m.refreshMu.Lock()
	m.refreshChan = nil

	close(ch)
	m.refreshMu.Unlock()

	return err
}

func (m *Manager) refreshSchema(ctx context.Context, itemsGameURL string) error {
	if err := m.refreshPriceDB(ctx); err == nil {
		return nil
	} else {
		if errors.Is(err, context.Canceled) {
			return err
		}

		m.Logger.WarnContext(ctx, "PriceDB schema fetch failed, falling back to items_game.txt", log.Err(err))
	}

	return m.refreshFromGame(ctx, itemsGameURL)
}

func (m *Manager) refreshPriceDB(ctx context.Context) error {
	m.Logger.DebugContext(ctx, "Fetching complete schema from PriceDB...")

	resp, err := m.pricedb.GetSchema(ctx)
	if err != nil {
		return fmt.Errorf("pricedb schema fetch failed: %w", err)
	}

	rawMap, ok := resp["raw"].(map[string]any)
	if !ok {
		return errors.New("invalid PriceDB response: missing 'raw'")
	}

	rawSchemaMap, ok := rawMap["schema"].(map[string]any)
	if !ok {
		return errors.New("invalid PriceDB response: missing 'raw.schema'")
	}

	schemaJSON, _ := json.Marshal(rawSchemaMap)

	var overviewResult OverviewResult
	if err := json.Unmarshal(schemaJSON, &overviewResult); err != nil {
		return fmt.Errorf("failed to parse overview from PriceDB: %w", err)
	}

	itemsGameURL, _ := rawSchemaMap["items_game_url"].(string)

	pkMap, _ := rawSchemaMap["paintkits"].(map[string]any)

	paintKits := make(map[string]string, len(pkMap))
	for k, v := range pkMap {
		if s, ok := v.(string); ok {
			paintKits[k] = stringpool.Intern(s)
		}
	}

	m.Logger.DebugContext(ctx, "Fetching items_game.txt...", log.String("url", itemsGameURL))

	itemsGame, err := m.getItemsGame(ctx, itemsGameURL)
	if err != nil {
		return fmt.Errorf("failed to fetch items_game.txt: %w", err)
	}

	extraItems := m.parseItemsGameItems(ctx, itemsGameURL)
	if len(extraItems) > 0 {
		existingDefindexes := make(map[int]bool, len(overviewResult.Items))
		for _, item := range overviewResult.Items {
			if item != nil {
				existingDefindexes[item.Defindex] = true
			}
		}

		mergedCount := 0
		for _, extra := range extraItems {
			if !existingDefindexes[extra.Defindex] {
				overviewResult.Items = append(overviewResult.Items, extra)
				mergedCount++
			}
		}

		if mergedCount > 0 {
			m.Logger.InfoContext(ctx, "Enriched schema with items from items_game.txt", log.Int("added", mergedCount))
		}
	}

	if err := m.buildSchemaDirect(&overviewResult, paintKits, itemsGame); err != nil {
		return err
	}

	m.mu.Lock()
	if v, ok := resp["version"].(string); ok && v != "" {
		m.schema.Version = stringpool.Intern(v)
	}

	if t, ok := resp["time"].(float64); ok && t > 0 {
		m.schema.Time = time.Unix(0, int64(t)*int64(time.Millisecond))
	}

	m.mu.Unlock()

	if err := m.saveToCache(); err != nil {
		m.Logger.WarnContext(ctx, "Failed to save schema to cache", log.Err(err))
	}

	m.Logger.InfoContext(ctx, "TF2 Schema updated successfully via PriceDB",
		log.String("version", m.schema.Version),
		log.Int("items", m.schema.ItemCount()),
	)
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
}

func (m *Manager) refreshFromGame(ctx context.Context, itemsGameURL string) error {
	m.Logger.InfoContext(ctx, "Building schema from Steam API and items_game.txt...")

	itemsGame, err := m.getItemsGame(ctx, itemsGameURL)
	if err != nil {
		m.Logger.WarnContext(ctx, "Failed to parse items_game.txt, continuing without it", log.Err(err))

		itemsGame = map[string]any{}
	}

	items, err := m.getSchemaItemsDirect(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch schema items: %w", err)
	}

	extraItems := m.parseItemsGameItems(ctx, itemsGameURL)
	if len(extraItems) > 0 {
		existingDefindexes := make(map[int]bool, len(items))
		for _, item := range items {
			if item != nil {
				existingDefindexes[item.Defindex] = true
			}
		}

		mergedCount := 0
		for _, extra := range extraItems {
			if !existingDefindexes[extra.Defindex] {
				items = append(items, extra)
				mergedCount++
			}
		}
	}

	paintKits, _ := m.getPaintKits(ctx)
	if paintKits == nil {
		paintKits = make(map[string]string)
	}

	overviewResp, err := m.getSchemaOverviewDirect(ctx)
	if err != nil {
		m.Logger.WarnContext(ctx, "Schema overview unavailable, using minimal overview", log.Err(err))

		overviewResp = &OverviewResponse{}
	}

	overviewResp.Result.Items = items

	if err := m.buildSchemaDirect(&overviewResp.Result, paintKits, itemsGame); err != nil {
		return err
	}

	if err := m.saveToCache(); err != nil {
		m.Logger.WarnContext(ctx, "Failed to save schema to cache", log.Err(err))
	}

	m.Logger.InfoContext(ctx, "TF2 Schema updated via items_game.txt",
		log.Int("items", m.schema.ItemCount()),
		log.Int("paintkits", len(paintKits)),
	)
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
}

func downloadRawURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "G-man Bot/1.0")

	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (m *Manager) parseTfEnglish(ctx context.Context) map[string]string {
	url := "https://raw.githubusercontent.com/SteamDatabase/GameTracking-TF2/master/tf/resource/tf_english.txt"

	m.Logger.InfoContext(ctx, "Fetching tf_english.txt for localization...")

	data, err := downloadRawURL(ctx, url)
	if err != nil {
		m.Logger.WarnContext(ctx, "Failed to fetch tf_english.txt", log.Err(err))
		return nil
	}

	m.Logger.InfoContext(ctx, "tf_english.txt downloaded, parsing...")

	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	inTokens := false
	bracketCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "{" {
			bracketCount++
			if bracketCount == 2 {
				inTokens = true
			}

			continue
		}

		if trimmed == "}" {
			bracketCount--
			if bracketCount < 2 {
				inTokens = false
			}

			continue
		}

		if bracketCount == 1 && trimmed == "\"Tokens\"" {
			inTokens = true
			continue
		}

		if !inTokens {
			continue
		}

		parts := strings.SplitN(trimmed, "\t", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(trimmed, " ", 2)
		}

		if len(parts) < 2 {
			continue
		}

		key := strings.Trim(parts[0], "\"")
		val := strings.Trim(parts[1], "\"")

		if key != "" && val != "" {
			result[key] = stringpool.Intern(val)
		}
	}

	m.Logger.InfoContext(ctx, "Loaded tf_english.txt translations", log.Int("count", len(result)))

	return result
}

func (m *Manager) parseItemsGameItems(ctx context.Context, url string) []*Item {
	url = generic.Coalesce(url, m.config.ItemsGameMirrorURL)

	data, err := downloadRawURL(ctx, url)
	if err != nil {
		m.Logger.WarnContext(ctx, "Failed to download items_game.txt for item parsing", log.Err(err))
		return nil
	}

	type gameItem struct {
		defindex      int
		name          string
		localizedName string
		itemClass     string
		itemSlot      string
		properName    bool
		craftClass    string
	}

	translations := m.parseTfEnglish(ctx)

	var found []gameItem

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	inItemsSection := false
	bracketCount := 0
	pendingDefindex := -1

	var current gameItem

	inItemBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "{" {
			bracketCount++
			if inItemsSection && bracketCount == 3 && pendingDefindex > 0 {
				inItemBlock = true
				current = gameItem{defindex: pendingDefindex}
				pendingDefindex = -1
			}

			continue
		}

		if trimmed == "}" {
			if inItemBlock && bracketCount == 3 {
				if current.defindex > 0 && current.name != "" {
					found = append(found, current)
				}

				inItemBlock = false
			}

			bracketCount--
			if bracketCount <= 1 {
				inItemsSection = false
			}

			continue
		}

		if bracketCount == 1 && trimmed == "\"items\"" {
			inItemsSection = true
			continue
		}

		if !inItemsSection {
			continue
		}

		if bracketCount == 2 && !inItemBlock {
			if m := rxDefindex.FindStringSubmatch(trimmed); len(m) == 2 {
				if di, e := strconv.Atoi(m[1]); e == nil {
					pendingDefindex = di
				}
			}

			continue
		}

		if !inItemBlock {
			continue
		}

		parts := strings.SplitN(trimmed, "\t", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(trimmed, " ", 2)
		}

		if len(parts) < 2 {
			continue
		}

		key := strings.Trim(parts[0], "\"")
		val := strings.Trim(parts[1], "\"")

		switch key {
		case "name":
			current.name = val
		case "localizedname":
			current.localizedName = val
		case "item_class":
			current.itemClass = val
		case "item_slot":
			current.itemSlot = val
		case "proper_name":
			current.properName = val == "1"
		case "craft_class":
			current.craftClass = val
		}
	}

	var result []*Item
	for _, gi := range found {
		displayName := gi.name

		if gi.localizedName != "" {
			if translations != nil {
				if loc, ok := translations[gi.localizedName]; ok {
					displayName = loc
				} else {
					baseToken := stripStyleSuffix(gi.localizedName)
					if baseToken != gi.localizedName {
						if loc, ok := translations[baseToken]; ok {
							displayName = loc
						} else {
							displayName = gi.localizedName
						}
					} else {
						displayName = gi.localizedName
					}
				}
			} else {
				displayName = gi.localizedName
			}
		} else if translations != nil {
			token := "#" + gi.name
			if loc, ok := translations[token]; ok {
				displayName = loc
			} else {
				camelName := toCamelCase(gi.name)

				token2 := "#TF_" + camelName
				if loc, ok := translations[token2]; ok {
					displayName = loc
				}
			}
		}

		item := &Item{
			Defindex:   gi.defindex,
			ItemName:   stringpool.Intern(displayName),
			ItemClass:  stringpool.Intern(gi.itemClass),
			ItemSlot:   stringpool.Intern(gi.itemSlot),
			ProperName: gi.properName,
			CraftClass: stringpool.Intern(gi.craftClass),
		}

		result = append(result, item)
	}

	m.Logger.InfoContext(ctx, "Parsed items from items_game.txt", log.Int("count", len(result)))

	return result
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}

	return strings.Join(parts, "")
}

func stripStyleSuffix(s string) string {
	re := regexp.MustCompile(`_Style\d+$`)
	return re.ReplaceAllString(s, "")
}

func (m *Manager) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Refresh(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					m.Logger.DebugContext(ctx, "Scheduled schema refresh cancelled due to shutdown")
				} else {
					m.Logger.ErrorContext(ctx, "Scheduled schema refresh failed", log.Err(err))
				}
			}
		}
	}
}

func (m *Manager) buildSchemaDirect(
	overview *OverviewResult,
	paintKits map[string]string,
	itemsGame map[string]any,
) error {
	raw := &Raw{
		ItemsGame: itemsGame,
	}

	raw.Schema.Items = overview.Items
	raw.Schema.Attributes = overview.Attributes
	raw.Schema.Qualities = overview.Qualities
	raw.Schema.QualityNames = overview.QualityNames
	raw.Schema.OriginNames = overview.OriginNames
	raw.Schema.ItemSets = overview.ItemSets
	raw.Schema.AttributeControlledAttachedParticles = overview.AttributeControlledAttachedParticles
	raw.Schema.ItemLevels = overview.ItemLevels
	raw.Schema.KillEaterScoreTypes = overview.KillEaterScoreTypes
	raw.Schema.StringLookups = overview.StringLookups
	raw.Schema.PaintKits = paintKits

	itemsByDefIndex := make(map[int]*Item, len(overview.Items))

	for _, item := range overview.Items {
		if item != nil && item.Defindex > 0 {
			item.InternStrings()

			if existing, found := itemsByDefIndex[item.Defindex]; found {
				if existing.ItemName == "" && item.ItemName != "" {
					existing.ItemName = item.ItemName
				}

				if existing.ItemClass == "" && item.ItemClass != "" {
					existing.ItemClass = item.ItemClass
				}

				if existing.CraftClass == "" && item.CraftClass != "" {
					existing.CraftClass = item.CraftClass
				}

				if existing.ItemQuality == 0 && item.ItemQuality != 0 {
					existing.ItemQuality = item.ItemQuality
				}
			} else {
				itemsByDefIndex[item.Defindex] = item
			}
		}
	}

	raw.Schema.Items = make([]*Item, 0, len(itemsByDefIndex))
	for _, itemPtr := range itemsByDefIndex {
		raw.Schema.Items = append(raw.Schema.Items, itemPtr)
	}

	if m.config.LiteMode {
		m.pruneItemsGame(raw)
	}

	newSchema := New(raw)
	newSchema.Version = overview.ItemsGameURL
	newSchema.Time = time.Now()

	m.mu.Lock()
	m.schema = newSchema
	m.mu.Unlock()

	debug.FreeOSMemory()

	return nil
}

func (m *Manager) pruneItemsGame(raw *Raw) {
	if raw.ItemsGame == nil {
		return
	}

	keysToRemove := []string{
		"game_info", "colors", "equip_regions_list", "equip_conflicts",
		"quest_objective_conditions", "item_series_types", "item_collections",
		"operations", "prefabs", "item_criteria_templates", "random_attribute_templates",
		"lootlist_job_template_definitions", "item_sets", "client_loot_lists",
		"revolving_loot_lists", "recipes", "achievement_rewards",
		"attribute_controlled_attached_particles", "armory_data", "item_levels",
		"kill_eater_score_types", "mvm_maps", "mvm_tours", "matchmaking_categories",
		"maps", "master_maps_list", "steam_packages", "community_market_item_remaps",
		"war_definitions",
	}

	for _, key := range keysToRemove {
		delete(raw.ItemsGame, key)
	}

	m.Logger.Debug("LiteMode: pruned items_game data to save memory")
}

func (m *Manager) getSchemaOverviewDirect(ctx context.Context) (*OverviewResponse, error) {
	params := struct {
		Language string `url:"language"`
	}{"English"}

	resp, err := service.WebAPI[OverviewResponse](
		ctx,
		m.service,
		"GET",
		"IEconItems_440",
		"GetSchemaOverview",
		1,
		params,
	)
	if err != nil {
		return nil, fmt.Errorf("overview fetch failed: %w", err)
	}

	return resp, nil
}

func (m *Manager) getSchemaItemsDirect(ctx context.Context) ([]*Item, error) {
	var allItems []*Item

	next := 0

	for {
		params := struct {
			Language string `url:"language"`
			Start    int    `url:"start"`
		}{"English", next}

		resp, err := service.WebAPI[ItemsResponse](
			ctx, m.service, "GET", "IEconItems_440", "GetSchemaItems", 1, params,
		)
		if err != nil {
			return nil, err
		}

		if resp != nil && len(resp.Result.Items) > 0 {
			allItems = append(allItems, resp.Result.Items...)
			m.Logger.Debug("Items progress", log.Int("count", len(allItems)))
		}

		if resp == nil || resp.Result.Next <= 0 {
			break
		}

		next = resp.Result.Next
	}

	return allItems, nil
}

func (m *Manager) getPaintKits(ctx context.Context) (map[string]string, error) {
	data, err := downloadRawURL(ctx, m.config.PaintKitURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paint kits: %w", err)
	}

	parsed, err := vdf.NewParser(bytes.NewReader(data)).Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse VDF: %w", err)
	}

	lang, ok := parsed["lang"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid VDF structure: missing 'lang'")
	}

	tokens, ok := lang["Tokens"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid VDF structure: missing 'Tokens'")
	}

	paintKits := make(map[string]string)
	seen := make(map[string]bool)

	for key, val := range tokens {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) != 2 {
			continue
		}

		subparts := strings.Split(parts[0], "_")
		if len(subparts) != 3 || subparts[0] != "9" {
			continue
		}

		def := subparts[1]

		name, ok := val.(string)
		if !ok {
			continue
		}

		if strings.HasPrefix(name, def+":") {
			continue
		}

		if !seen[name] {
			paintKits[def] = stringpool.Intern(name)
			seen[name] = true
		}
	}

	return paintKits, nil
}

var (
	rxDefindex = regexp.MustCompile(`^\s*"(\d+)"\s*$`)
	rxSeries   = regexp.MustCompile(`^\s*"set supply crate series"\s+"(\d+)"\s*$`)
)

func (m *Manager) getItemsGame(ctx context.Context, url string) (map[string]any, error) {
	url = generic.Coalesce(url, m.config.ItemsGameMirrorURL)

	data, err := downloadRawURL(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch items_game.txt: %w", err)
	}

	seriesMap := make(map[string]any)
	recipesMap := make(map[string]any)
	recipeBuf := new(strings.Builder)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var (
		inItemsSection, inRecipesSection bool
		currentDefindex, recipeDefindex  string
		recipeBracketDepth, bracketCount int
	)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if bracketCount == 1 {
			trimmedQ := strings.TrimSpace(line)

			if trimmedQ == "\"items\"" {
				inItemsSection = true
				inRecipesSection = false
				continue
			}

			if trimmedQ == "\"recipes\"" {
				inRecipesSection = true
				inItemsSection = false
				continue
			}
		}

		if trimmed == "{" {
			if inRecipesSection {
				if recipeBracketDepth == 0 && bracketCount == 2 {
					bracketCount++
					recipeBracketDepth = 1

					recipeBuf.Reset()
					recipeBuf.WriteString(line)
					recipeBuf.WriteString("\n")

					continue
				}

				if recipeBracketDepth > 0 {
					bracketCount++
					recipeBracketDepth++

					recipeBuf.WriteString(line)
					recipeBuf.WriteString("\n")

					continue
				}
			}

			bracketCount++

			continue
		}

		if trimmed == "}" {
			if inRecipesSection && recipeBracketDepth > 0 {
				recipeBracketDepth--

				recipeBuf.WriteString(line)
				recipeBuf.WriteString("\n")

				if recipeBracketDepth == 0 {
					bracketCount--

					if recipeDefindex != "" {
						recipesMap[recipeDefindex] = recipeBuf.String()
					}

					recipeDefindex = ""

					recipeBuf.Reset()

					continue
				}
			}

			bracketCount--

			if bracketCount == 2 {
				currentDefindex = ""
			}

			if bracketCount <= 1 {
				inItemsSection = false
				inRecipesSection = false
			}

			continue
		}

		if inRecipesSection && bracketCount == 2 && recipeBracketDepth == 0 {
			if match := rxDefindex.FindStringSubmatch(line); len(match) == 2 {
				recipeDefindex = match[1]
			}
		}

		if inRecipesSection && recipeBracketDepth > 0 {
			recipeBuf.WriteString(line)
			recipeBuf.WriteString("\n")
		}

		if inItemsSection {
			if bracketCount == 2 {
				if match := rxDefindex.FindStringSubmatch(line); len(match) == 2 {
					currentDefindex = match[1]
				}
			} else if bracketCount == 4 && currentDefindex != "" {
				if match := rxSeries.FindStringSubmatch(line); len(match) == 2 {
					series, _ := strconv.Atoi(match[1])

					seriesMap[currentDefindex] = map[string]any{
						"static_attrs": map[string]any{
							"set supply crate series": float64(series),
						},
					}

					currentDefindex = ""
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading items_game stream: %w", err)
	}

	return map[string]any{
		"items":   seriesMap,
		"recipes": recipesMap,
	}, nil
}

func (m *Manager) saveToCache() error {
	if m.config.CachePath == "" {
		return nil
	}

	m.mu.RLock()
	s := m.schema
	m.mu.RUnlock()

	if s == nil {
		return nil
	}

	data, err := json.Marshal(s.ToJSON())
	if err != nil {
		return err
	}

	return writeFile(m.config.CachePath+".json", data)
}

func (m *Manager) loadFromCache() error {
	if m.config.CachePath == "" {
		return errors.New("cache path not configured")
	}

	data, err := readFile(m.config.CachePath + ".json")
	if err != nil {
		return err
	}

	var raw Raw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Schema.Items) == 0 {
		return errors.New("cached schema is empty")
	}

	loadedSchema := New(&raw)

	m.mu.Lock()
	m.schema = loadedSchema
	m.mu.Unlock()

	return nil
}

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
