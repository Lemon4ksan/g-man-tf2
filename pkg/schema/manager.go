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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andygrunwald/vdf"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/codec/decode"
	"github.com/lemon4ksan/aoni/option"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/service"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/internal/stringpool"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

const ModuleName string = "tf2_schema"

type Config struct {
	UpdateInterval     time.Duration
	LiteMode           bool
	ExcludeMedals      bool
	ExcludeUntradable  bool
	CachePath          string
	PaintKitURL        string
	SchemaMirrorURL    string
	ItemsMirrorURL     string
	ItemsGameMirrorURL string
}

func DefaultConfig() Config {
	return Config{
		UpdateInterval:     24 * time.Hour,
		LiteMode:           false,
		ExcludeMedals:      true,
		ExcludeUntradable:  false,
		CachePath:          "cache/tf2/json",
		PaintKitURL:        "https://raw.githubusercontent.com/SteamDatabase/GameTracking-TF2/master/tf/resource/tf_proto_obj_defs_english.txt",
		ItemsGameMirrorURL: "https://raw.githubusercontent.com/SteamDatabase/GameTracking-TF2/master/tf/scripts/items/items_game.txt",
	}
}

func WithModule(cfg Config) steam.Option {
	return steam.WithModule(NewManager(cfg))
}

func From(c *steam.Client) *Manager {
	return steam.GetModule[*Manager](c)
}

func IsMedal(it *Item) bool {
	if it == nil {
		return false
	}

	if containsFoldASCII(it.ItemName, "Gentle Manne's Service Medal") {
		return false
	}

	switch it.ItemTypeName {
	case "#TF_Wearable_TournamentMedal", "#TF_Wearable_Medal", "Tournament Medal", "Medal":
		return true
	}

	if strings.HasPrefix(it.ItemName, "#TF_TournamentMedal_") ||
		strings.HasPrefix(it.ItemName, "#TF_Wearable_Tournament") {
		return true
	}

	nameLower := strings.ToLower(it.Name)
	if strings.Contains(nameLower, "tournament medal") ||
		strings.Contains(nameLower, "ugc tournament") ||
		strings.Contains(nameLower, "etf2l") ||
		strings.Contains(nameLower, "asiafortress") ||
		strings.Contains(nameLower, "rgl.gg") {
		return true
	}

	return false
}

func IsUntradable(it *Item) bool {
	if it == nil {
		return false
	}

	if !it.IsTradableByFlags() {
		return true
	}

	for _, attr := range it.Attributes {
		if (attr.Name == "cannot trade" || attr.Class == "cannot_trade") && attr.Value == 1 {
			return true
		}
	}

	return false
}

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

type OverviewResponse struct {
	Result OverviewResult `json:"result"`
}

type ItemsResult struct {
	Status int     `json:"status"`
	Next   int     `json:"next"`
	Items  []*Item `json:"items"`
}

type ItemsResponse struct {
	Result ItemsResult `json:"result"`
}

type Manager struct {
	module.Base

	config  Config
	service service.Doer
	rest    *aoni.Client
	pricedb pricedb.SKUClient

	schema        atomic.Pointer[Schema]
	refreshMu     sync.Mutex
	refreshChan   chan struct{}
	lastGCVersion atomic.Uint32
}

func NewManager(cfg Config) *Manager {
	if cfg.UpdateInterval < 1*time.Minute {
		cfg.UpdateInterval = 24 * time.Hour
	}

	return &Manager{
		Base:   module.New(ModuleName),
		config: cfg,
	}
}

func (m *Manager) Name() string { return ModuleName }

func (m *Manager) Init(init module.InitContext) error {
	if err := m.Base.Init(init); err != nil {
		return err
	}

	m.service = init.Service()
	m.rest = init.Rest()

	if m.rest != nil {
		unlimitedClient := m.rest.With(option.WithMaxResponseSize(0))
		m.rest = unlimitedClient
		m.pricedb = pricedb.NewSKUClient(unlimitedClient)
	} else {
		unlimitedClient := aoni.NewClient(nil, option.WithMaxResponseSize(0))
		m.rest = unlimitedClient
		m.pricedb = pricedb.NewSKUClient(unlimitedClient)
	}

	return nil
}

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
	} else if s := m.schema.Load(); s != nil {
		m.Logger.InfoContext(ctx, "Schema loaded from cache",
			log.Time("time", s.Time),
			log.Int("items", s.ItemCount()),
		)
	}

	m.Bus.Publish(&ReadyEvent{})

	m.Go(func(moduleCtx context.Context) {
		m.refreshLoop(moduleCtx)
	})

	return nil
}

func (m *Manager) handleUpdateRequested(req *UpdateRequestedEvent) {
	currentSchema := m.schema.Load()
	hasSchema := currentSchema != nil
	currentVersion := ""

	if hasSchema {
		currentVersion = currentSchema.Version
	}

	lastGC := m.lastGCVersion.Load()

	if hasSchema {
		if req.ItemsGameURL != "" && currentVersion == req.ItemsGameURL {
			if req.Version != 0 && lastGC != req.Version {
				m.lastGCVersion.Store(req.Version)
			}

			return
		}

		if req.ItemsGameURL == "" && req.Version != 0 && lastGC == req.Version {
			return
		}
	}

	m.Go(func(ctx context.Context) {
		if err := m.doRefresh(ctx, req.ItemsGameURL); err == nil {
			m.lastGCVersion.Store(req.Version)
		}
	})
}

func (m *Manager) Get() *Schema {
	return m.schema.Load()
}

func (m *Manager) Refresh(ctx context.Context) error {
	return m.doRefresh(ctx, m.config.ItemsGameMirrorURL)
}

func (m *Manager) doRefresh(ctx context.Context, itemsGameURL string) error {
	m.refreshMu.Lock()
	if m.refreshChan != nil {
		ch := m.refreshChan
		m.refreshMu.Unlock()

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
	} else if errors.Is(err, context.Canceled) {
		return err
	}

	return m.refreshFromGame(ctx, itemsGameURL)
}

func (m *Manager) refreshPriceDB(ctx context.Context) error {
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

		for _, extra := range extraItems {
			if !existingDefindexes[extra.Defindex] {
				overviewResult.Items = append(overviewResult.Items, extra)
			}
		}
	}

	if err := m.buildSchemaDirect(&overviewResult, paintKits, itemsGame); err != nil {
		return err
	}

	if s := m.schema.Load(); s != nil {
		if v, ok := resp["version"].(string); ok && v != "" {
			s.Version = stringpool.Intern(v)
		}

		if t, ok := resp["time"].(float64); ok && t > 0 {
			s.Time = time.Unix(0, int64(t)*int64(time.Millisecond))
		}
	}

	_ = m.saveToCache()
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
}

func (m *Manager) refreshFromGame(ctx context.Context, itemsGameURL string) error {
	itemsGame, err := m.getItemsGame(ctx, itemsGameURL)
	if err != nil {
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

		for _, extra := range extraItems {
			if !existingDefindexes[extra.Defindex] {
				items = append(items, extra)
			}
		}
	}

	paintKits, _ := m.getPaintKits(ctx)
	if paintKits == nil {
		paintKits = make(map[string]string)
	}

	overviewResp, err := m.getSchemaOverviewDirect(ctx)
	if err != nil {
		overviewResp = &OverviewResponse{}
	}

	overviewResp.Result.Items = items

	if err := m.buildSchemaDirect(&overviewResp.Result, paintKits, itemsGame); err != nil {
		return err
	}

	_ = m.saveToCache()
	m.Bus.Publish(&UpdatedEvent{Timestamp: time.Now()})

	return nil
}

func (m *Manager) downloadRawURL(ctx context.Context, rawURL string) ([]byte, error) {
	if m != nil && m.rest != nil {
		resp, err := m.rest.GetTo[[]byte](ctx, rawURL, decode.WithRaw())
		if err == nil && resp != nil {
			return *resp, nil
		}

		if err != nil {
			return nil, err
		}
	}

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

	data, err := m.downloadRawURL(ctx, url)
	if err != nil {
		return nil
	}

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

		key := strings.Trim(strings.TrimSpace(parts[0]), "\"")
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		if key != "" && val != "" {
			internedVal := stringpool.Intern(val)

			result[key] = internedVal
			if !strings.HasPrefix(key, "#") {
				result["#"+key] = internedVal
			} else {
				result[strings.TrimPrefix(key, "#")] = internedVal
			}
		}
	}

	return result
}

func (m *Manager) parseItemsGameItems(ctx context.Context, url string) []*Item {
	url = generic.Coalesce(url, m.config.ItemsGameMirrorURL)

	data, err := m.downloadRawURL(ctx, url)
	if err != nil {
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
		lineBytes := scanner.Bytes()

		trimmedBytes := bytes.TrimSpace(lineBytes)
		if len(trimmedBytes) == 0 {
			continue
		}

		trimmed := bytesconv.B2S(trimmedBytes)

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
			if di, ok := parseQuotedDefindex(trimmed); ok {
				pendingDefindex = di
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
					if loc, ok := translations[baseToken]; ok {
						displayName = loc
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

	return result
}

func toCamelCase(s string) string {
	if len(s) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	upperNext := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			upperNext = true
			continue
		}

		if upperNext && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}

		upperNext = false

		b.WriteByte(c)
	}

	return b.String()
}

func stripStyleSuffix(s string) string {
	idx := strings.LastIndex(s, "_Style")
	if idx == -1 {
		return s
	}

	suffix := s[idx+len("_Style"):]
	if len(suffix) == 0 {
		return s
	}

	for i := 0; i < len(suffix); i++ {
		if suffix[i] < '0' || suffix[i] > '9' {
			return s
		}
	}

	return s[:idx]
}

func (m *Manager) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = m.Refresh(ctx)
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
			if m.shouldExcludeItem(item) {
				continue
			}

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

	m.schema.Store(newSchema)

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
}

func (m *Manager) shouldExcludeItem(item *Item) bool {
	if item == nil {
		return true
	}

	if m.config.ExcludeMedals && IsMedal(item) {
		return true
	}

	if m.config.ExcludeUntradable && IsUntradable(item) {
		return true
	}

	return false
}

func (m *Manager) getSchemaOverviewDirect(ctx context.Context) (*OverviewResponse, error) {
	params := struct {
		Language string `query:"language"`
	}{"English"}

	resp, err := service.WebAPI[OverviewResponse](
		ctx, m.service, "GET", "IEconItems_440", "GetSchemaOverview", 1, params,
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
			Language string `query:"language"`
			Start    int    `query:"start"`
		}{"English", next}

		resp, err := service.WebAPI[ItemsResponse](
			ctx, m.service, "GET", "IEconItems_440", "GetSchemaItems", 1, params,
		)
		if err != nil {
			return nil, err
		}

		if resp != nil && len(resp.Result.Items) > 0 {
			allItems = append(allItems, resp.Result.Items...)
		}

		if resp == nil || resp.Result.Next <= 0 {
			break
		}

		next = resp.Result.Next
	}

	return allItems, nil
}

func (m *Manager) getPaintKits(ctx context.Context) (map[string]string, error) {
	data, err := m.downloadRawURL(ctx, m.config.PaintKitURL)
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

		if subparts[2] == "desc" || subparts[2] == "description" || subparts[2] == "tooltip" {
			continue
		}

		def := subparts[1]

		name, ok := val.(string)
		if !ok || strings.HasPrefix(name, def+":") {
			continue
		}

		if !seen[name] {
			paintKits[def] = stringpool.Intern(name)
			seen[name] = true
		}
	}

	return paintKits, nil
}

var rxDefindex = regexp.MustCompile(`^\s*"(\d+)"\s*$`)

func (m *Manager) getItemsGame(ctx context.Context, url string) (map[string]any, error) {
	url = generic.Coalesce(url, m.config.ItemsGameMirrorURL)

	data, err := m.downloadRawURL(ctx, url)
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
				if series, ok := parseCrateSeriesLine(line); ok {
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

	s := m.schema.Load()
	if s == nil {
		return nil
	}

	data, err := json.Marshal(s.ToJSON())
	if err != nil {
		return err
	}

	cachePath := m.config.CachePath
	if !strings.HasSuffix(cachePath, ".json") {
		cachePath += ".json"
	}

	return writeFile(cachePath, data)
}

func (m *Manager) loadFromCache() error {
	if m.config.CachePath == "" {
		return errors.New("cache path not configured")
	}

	cachePath := m.config.CachePath
	if !strings.HasSuffix(cachePath, ".json") {
		cachePath += ".json"
	}

	data, err := readFile(cachePath)
	if err != nil {
		return err
	}

	var raw Raw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Schema.Items) == 0 {
		var wrapper struct {
			Raw Raw `json:"raw"`
		}

		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Raw.Schema.Items) > 0 {
			raw = wrapper.Raw
		} else {
			return errors.New("cached schema is empty or incomplete")
		}
	}

	if m.config.ExcludeMedals || m.config.ExcludeUntradable {
		filtered := make([]*Item, 0, len(raw.Schema.Items))
		for _, item := range raw.Schema.Items {
			if !m.shouldExcludeItem(item) {
				filtered = append(filtered, item)
			}
		}

		raw.Schema.Items = filtered
	}

	loadedSchema := New(&raw)
	m.schema.Store(loadedSchema)

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

func parseQuotedDefindex(line string) (int, bool) {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 || trimmed[0] != '"' {
		return 0, false
	}

	endQuote := strings.IndexByte(trimmed[1:], '"')
	if endQuote <= 0 {
		return 0, false
	}

	key := trimmed[1 : 1+endQuote]
	if len(key) == 0 {
		return 0, false
	}

	var v int
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c < '0' || c > '9' {
			return 0, false
		}

		v = v*10 + int(c-'0')
	}

	rest := strings.TrimSpace(trimmed[2+endQuote:])
	if len(rest) > 0 && rest[0] == '"' {
		return 0, false
	}

	return v, true
}

func parseCrateSeriesLine(line string) (int, bool) {
	trimmed := strings.TrimSpace(line)

	const prefix = `"set supply crate series"`
	if !strings.HasPrefix(trimmed, prefix) {
		return 0, false
	}

	rest := strings.TrimSpace(trimmed[len(prefix):])
	if len(rest) < 3 || rest[0] != '"' || rest[len(rest)-1] != '"' {
		return 0, false
	}

	val, ok := bytesconv.ParseUint64(bytesconv.S2B(rest[1 : len(rest)-1]))
	if !ok {
		return 0, false
	}

	return int(val), true
}
