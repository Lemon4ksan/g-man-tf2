// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schema

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andygrunwald/vdf"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/api"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"

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
	return func(c *steam.Client) {
		c.RegisterModule(NewManager(cfg))
	}
}

// From returns the [Manager] module instance retrieved from the [steam.Client].
func From(c *steam.Client) *Manager {
	return steam.GetModule[*Manager](c)
}

// Manager manages background updates and local caching of the TF2 item schema.
// Use [NewManager] to create an instance and configure it with [Config].
type Manager struct {
	module.Base

	config        Config
	svcClient     service.Doer
	restClient    rest.Requester
	pricedbClient *pricedb.Client

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
// Returns an error if context resolution fails.
func (m *Manager) Init(init module.InitContext) error {
	if err := m.Base.Init(init); err != nil {
		return err
	}

	m.svcClient = init.Service()
	m.restClient = init.Rest()

	type httpProvider interface {
		HTTP() rest.HTTPDoer
	}

	if hp, ok := m.restClient.(httpProvider); ok {
		m.pricedbClient = pricedb.NewClient(hp.HTTP())
	} else {
		m.pricedbClient = pricedb.NewClient(nil)
	}

	return nil
}

// StartAuthed starts background polling, updates, and events listening routines.
// Returns an error if the context is cancelled during initialization.
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
			log.Int("items", len(m.schema.Raw.Schema.Items)),
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
// Returns nil if the schema has not finished loading.
func (m *Manager) Get() *Schema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.schema
}

// Refresh manually triggers a full schema update from PriceDB and GitHub sources.
// Returns an error if the fetch fails or the context is cancelled.
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

		m.Logger.WarnContext(ctx, "PriceDB schema fetch failed, falling back to Steam API", log.Err(err))
	}

	return m.refreshLegacy(ctx, itemsGameURL)
}

func (m *Manager) refreshPriceDB(ctx context.Context) error {
	m.Logger.DebugContext(ctx, "Fetching complete schema from PriceDB...")

	resp, err := m.pricedbClient.GetSchema(ctx)
	if err != nil {
		return fmt.Errorf("pricedb schema fetch failed: %w", err)
	}

	raw, ok := resp["raw"].(map[string]any)
	if !ok {
		return errors.New("invalid PriceDB response: missing 'raw'")
	}

	rawSchema, ok := raw["schema"].(map[string]any)
	if !ok {
		return errors.New("invalid PriceDB response: missing 'raw.schema'")
	}

	items, _ := rawSchema["items"].([]any)
	itemsGameURL, _ := rawSchema["items_game_url"].(string)

	pkMap, _ := rawSchema["paintkits"].(map[string]any)

	paintKits := make(map[string]string, len(pkMap))
	for k, v := range pkMap {
		if s, ok := v.(string); ok {
			paintKits[k] = s
		}
	}

	m.Logger.DebugContext(ctx, "Fetching items_game.txt...", log.String("url", itemsGameURL))

	itemsGame, err := m.getItemsGame(ctx, itemsGameURL)
	if err != nil {
		return fmt.Errorf("failed to fetch items_game.txt: %w", err)
	}

	overview, err := m.getSchemaOverview(ctx)
	if err != nil {
		m.Logger.WarnContext(
			ctx,
			"Failed to fetch schema overview from Steam, using PriceDB raw schema instead",
			log.Err(err),
		)

		overview = map[string]any{"result": rawSchema}
	}

	if err := m.buildSchema(overview, items, paintKits, itemsGame); err != nil {
		return err
	}

	m.mu.Lock()
	if v, ok := resp["version"].(string); ok && v != "" {
		m.schema.Version = v
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
		log.Int("items", len(m.schema.Raw.Schema.Items)),
	)
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
}

func (m *Manager) refreshLegacy(ctx context.Context, itemsGameURL string) error {
	m.Logger.DebugContext(ctx, "Fetching schema components from Steam and GitHub (Legacy)...")

	overview, err := m.getSchemaOverview(ctx)
	if err != nil {
		return err
	}

	items, err := m.getSchemaItems(ctx)
	if err != nil {
		return err
	}

	var (
		paintkits map[string]string
		itemsGame map[string]any
	)

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error

		paintkits, err = m.getPaintKits(gCtx)

		return err
	})
	g.Go(func() error {
		var err error

		itemsGame, err = m.getItemsGame(gCtx, itemsGameURL)

		return err
	})

	if err := g.Wait(); err != nil {
		m.Bus.Publish(&UpdateFailedEvent{Error: err})
		return fmt.Errorf("parallel legacy fetch failed: %w", err)
	}

	if err := m.buildSchema(overview, items, paintkits, itemsGame); err != nil {
		return err
	}

	if err := m.saveToCache(); err != nil {
		m.Logger.WarnContext(ctx, "Failed to save schema to cache", log.Err(err))
	}

	m.Logger.InfoContext(
		ctx,
		"TF2 Schema updated successfully via Legacy API",
		log.Int("items", len(m.schema.Raw.Schema.Items)),
	)
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
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

func (m *Manager) buildSchema(
	overview map[string]any,
	items []any,
	paintKits map[string]string,
	itemsGame map[string]any,
) error {
	raw := &Raw{
		ItemsGame: itemsGame,
	}
	raw.Schema.PaintKits = paintKits

	var schemaData any = overview
	if result, ok := overview["result"]; ok {
		schemaData = result
	}

	overviewBytes, _ := json.Marshal(schemaData)
	if err := json.Unmarshal(overviewBytes, &raw.Schema); err != nil {
		return fmt.Errorf("failed to parse schema overview: %w", err)
	}

	version := ""
	if res, ok := overview["result"].(map[string]any); ok {
		if url, ok := res["items_game_url"].(string); ok {
			version = url
		}
	}

	strPool := make(map[string]string)
	intern := func(s string) string {
		if s == "" {
			return ""
		}

		if val, ok := strPool[s]; ok {
			return val
		}

		strPool[s] = s

		return s
	}

	raw.Schema.Items = make([]*Item, 0, len(items))
	for _, it := range items {
		var item Item

		config := &mapstructure.DecoderConfig{
			TagName:          "json",
			WeaklyTypedInput: true,
			Result:           &item,
		}

		decoder, err := mapstructure.NewDecoder(config)
		if err != nil {
			m.Logger.Error("Failed to build schema decoder", log.Err(err))
			return err
		}

		if err := decoder.Decode(it); err == nil {
			item.ItemClass = intern(item.ItemClass)
			item.CraftClass = intern(item.CraftClass)
			item.ItemName = intern(item.ItemName)
			item.ImageURL = intern(item.ImageURL)
			item.ImageURLLarge = intern(item.ImageURLLarge)

			for i, class := range item.UsedByClasses {
				item.UsedByClasses[i] = intern(class)
			}

			raw.Schema.Items = append(raw.Schema.Items, &item)
		}
	}

	strPool = nil

	if m.config.LiteMode {
		m.pruneItemsGame(raw)
	}

	newSchema := New(raw)
	newSchema.Version = version
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

func (m *Manager) getSchemaOverview(ctx context.Context) (map[string]any, error) {
	req := struct {
		Language string `url:"language"`
	}{"English"}

	resp, err := service.WebAPI[map[string]any](ctx, m.svcClient, "GET", "IEconItems_440", "GetSchemaOverview", 1, req)
	if err != nil {
		if m.isForbiddenError(err) {
			m.Logger.Warn("WebAPI returned 403. Attempting to fetch Overview from community mirror...")
			return m.fetchFromMirror(ctx, "overview")
		}

		return nil, fmt.Errorf("overview fetch failed: %w", err)
	}

	return *resp, nil
}

func (m *Manager) getSchemaItems(ctx context.Context) ([]any, error) {
	var allItems []any

	next := 0

	for {
		var resp *map[string]any

		err := m.withRetry(ctx, func() error {
			req := struct {
				Language string `url:"language"`
				Start    int    `url:"start"`
			}{"English", next}

			var err error

			resp, err = service.WebAPI[map[string]any](
				ctx,
				m.svcClient,
				"GET",
				"IEconItems_440",
				"GetSchemaItems",
				1,
				req,
			)

			return err
		})
		if err != nil {
			if m.isForbiddenError(err) {
				return m.fetchItemsFromMirror(ctx)
			}

			return nil, err
		}

		result, ok := (*resp)["result"].(map[string]any)
		if !ok {
			break
		}

		if items, ok := result["items"].([]any); ok {
			allItems = append(allItems, items...)
			m.Logger.Debug("Items progress", log.Int("count", len(allItems)))
		}

		nextVal, hasNext := result["next"].(float64)
		if !hasNext || nextVal <= 0 {
			break
		}

		next = int(nextVal)
	}

	return allItems, nil
}

func (m *Manager) getPaintKits(ctx context.Context) (map[string]string, error) {
	resp, err := m.restClient.Request(ctx, "GET", m.config.PaintKitURL, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paint kits: %w", err)
	}

	if resp == nil {
		return nil, errors.New("received nil response while fetching paint kits")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github returned status: %d", resp.StatusCode)
	}

	parser := vdf.NewParser(resp.Body)

	parsed, err := parser.Parse()
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
			paintKits[def] = name
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
	if url == "" {
		url = m.config.ItemsGameMirrorURL
	}

	resp, err := m.restClient.Request(ctx, "GET", url, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch items_game.txt: %w", err)
	}

	if resp == nil {
		return nil, errors.New("received nil response while fetching items_game.txt")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github returned status: %d", resp.StatusCode)
	}

	seriesMap := make(map[string]any)
	scanner := bufio.NewScanner(resp.Body)

	var currentDefindex string

	inItemsSection := false
	bracketCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "{" {
			bracketCount++
			continue
		}

		if trimmed == "}" {
			bracketCount--
			if bracketCount == 2 {
				currentDefindex = ""
			}

			if bracketCount == 1 {
				inItemsSection = false
			}

			continue
		}

		if trimmed == "\"items\"" && bracketCount == 1 {
			inItemsSection = true
			continue
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
		"items": seriesMap,
	}, nil
}

func (m *Manager) isForbiddenError(err error) bool {
	var apiErr *api.SteamAPIError
	if errors.As(err, &apiErr) {
		return true
	}

	restErr := &rest.APIError{}
	if errors.As(err, &restErr) {
		return true
	}

	return strings.Contains(err.Error(), "403")
}

func (m *Manager) fetchFromMirror(ctx context.Context, component string) (map[string]any, error) {
	var url string

	switch component {
	case "overview":
		url = m.config.SchemaMirrorURL
	default:
		return nil, fmt.Errorf("unknown mirror component: %s", component)
	}

	if url == "" {
		return nil, fmt.Errorf("mirror URL for %s not configured", component)
	}

	res, err := rest.GetJSON[map[string]any](ctx, m.restClient, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mirror fetch failed: %w", err)
	}

	return *res, nil
}

func (m *Manager) fetchItemsFromMirror(ctx context.Context) ([]any, error) {
	url := m.config.ItemsMirrorURL

	res, err := rest.GetJSON[[]any](ctx, m.restClient, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mirror items fetch failed: %w", err)
	}

	return *res, nil
}

func (m *Manager) withRetry(ctx context.Context, operation func() error) error {
	const maxRetries = 3

	backoff := 2 * time.Second

	var lastErr error

	for i := range maxRetries {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		if !m.isRetryable(err) {
			return err
		}

		m.Logger.Warn("Operation failed, retrying...",
			log.Err(err),
			log.Int("attempt", i+1),
			log.Duration("backoff", backoff),
		)

		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

func (m *Manager) isRetryable(err error) bool {
	if strings.Contains(err.Error(), "invalid character '<'") {
		return true
	}

	if strings.Contains(err.Error(), "429") {
		return true
	}

	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection refused") {
		return true
	}

	return false
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

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return writeFile(m.config.CachePath, data)
}

func (m *Manager) loadFromCache() error {
	if m.config.CachePath == "" {
		return errors.New("cache path not configured")
	}

	data, err := readFile(m.config.CachePath)
	if err != nil {
		return err
	}

	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if s.Raw == nil || len(s.Raw.Schema.Items) == 0 {
		return errors.New("cached schema is incomplete")
	}

	loadedSchema := New(s.Raw)
	loadedSchema.Version = s.Version
	loadedSchema.Time = s.Time

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
