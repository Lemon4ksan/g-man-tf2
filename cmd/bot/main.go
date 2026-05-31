// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/behavior/achievements"
	"github.com/lemon4ksan/g-man/pkg/behavior/guard"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	webtrading "github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/critlistener"
	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/stock"
	"github.com/lemon4ksan/g-man-tf2/pkg/crafting"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/pricing"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/bptf"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/manualprices"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/rep"
	tf2jsonfile "github.com/lemon4ksan/g-man-tf2/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// Config holds the configuration for the bot loaded from environment variables.
type Config struct {
	Username       string
	Password       string
	SharedSecret   string
	IdentitySecret string
	DeviceID       string
	StoragePath    string

	// TF2/BPTF specific integrations
	BptfAPIKey    string
	BptfUserToken string
	MptfAPIKey    string

	// Crit.tf integrations
	CritAPIKey                 string
	TradeRequestEventStreamURL string
}

// Bot encapsulates all core systems, storage, loggers, and coordinates the session lifecycle.
type Bot struct {
	cfg             Config
	store           storage.Provider
	logger          log.Logger
	client          *steam.Client
	sub             *bus.Subscription
	orchestrator    *behavior.Orchestrator
	wg              sync.WaitGroup
	tradeCfgManager *tf2trading.ConfigManager
	bptfClient      *bptf.Client
	bansManager     *rep.BansManager
	bptfChecker     *bptf.BackpackTFChecker
	pdbManager      *pricedb.Manager
	pdbClient       *pricedb.Client
	costBasis       *tf2jsonfile.CostBasisStore
	critClient      *crit.Client
	manualPrices    *manualprices.Manager
}

// NewBot creates and initializes a new bot instance using the provided configuration
// and injected storage and logger dependencies.
func NewBot(cfg Config, store storage.Provider, logger log.Logger) (*Bot, error) {
	// 1. Initialize TF2 Trade Configuration
	tradeCfgManager, err := tf2trading.NewConfigManager("trading_config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize trade config: %w", err)
	}

	logger = logger.With(log.String("module", "bot"))

	// 2. Setup standard HTTP clients and TF2 API services
	httpClient := &http.Client{Timeout: 30 * time.Second}
	bptfClient := bptf.New(httpClient, cfg.BptfAPIKey, cfg.BptfUserToken)
	pdbClient := pricedb.NewClient(httpClient)
	critClient := crit.NewClient(httpClient, cfg.CritAPIKey)

	pdbManager := pricedb.NewManager(pdbClient, logger)
	bansManager := rep.NewBansManager(bptfClient, cfg.MptfAPIKey)
	bptfChecker := bptf.NewBackpackTFChecker(bptfClient)

	// Initialize Cost Basis Store for stagnant stock discount strategy
	costBasis, err := tf2jsonfile.NewCostBasisStore("cost_basis.json", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cost basis store: %w", err)
	}

	// 3. Configure the Steam Client with all necessary modules
	clientCfg := steam.DefaultConfig()
	clientCfg.Storage = store

	opts := []steam.Option{
		steam.WithLogger(logger),
		apps.WithModule(),
		gc.WithModule(),
		tf2.WithModule(),
		schema.WithModule(schema.DefaultConfig()),
		backpack.WithModule(),
		guard.WithModule(guard.DefaultGuardConfig(cfg.SharedSecret, cfg.IdentitySecret, cfg.DeviceID)),
		webtrading.WithModule(webtrading.Config{PollInterval: 30 * time.Second}),
	}

	client, err := steam.NewClient(clientCfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("steam client initialization failed: %w", err)
	}

	manualPricesStore, err := tf2jsonfile.NewManualPricesStore("cache/tf2/manual_prices.json", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize manual prices store: %w", err)
	}

	getStaticFixedPrices := func() map[string]manualprices.PriceEntry {
		cfg := tradeCfgManager.GetConfig()
		staticPrices := make(map[string]manualprices.PriceEntry)

		for sku, itemCfg := range cfg.Items {
			if itemCfg.FixedBuyPrice != nil && itemCfg.FixedSellPrice != nil {
				staticPrices[sku] = manualprices.PriceEntry{
					Buy:  *itemCfg.FixedBuyPrice,
					Sell: *itemCfg.FixedSellPrice,
				}
			}
		}

		return staticPrices
	}

	priceAdapter := &pdbPriceProviderAdapter{mgr: pdbManager}
	manualPrices := manualprices.New(manualPricesStore, priceAdapter, getStaticFixedPrices, logger)

	bot := &Bot{
		cfg:             cfg,
		store:           store,
		logger:          logger,
		client:          client,
		tradeCfgManager: tradeCfgManager,
		bptfClient:      bptfClient,
		bansManager:     bansManager,
		bptfChecker:     bptfChecker,
		pdbManager:      pdbManager,
		pdbClient:       pdbClient,
		costBasis:       costBasis,
		critClient:      critClient,
		manualPrices:    manualPrices,
	}

	return bot, nil
}

// Run starts the bot's background systems, establishes connection and logs on to Steam.
// It blocks until the context is canceled or a termination signal is received.
func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Starting core client services...")

	if err := b.client.Run(); err != nil {
		return fmt.Errorf("client run failed: %w", err)
	}

	server, err := b.discoverCMServer(ctx)
	if err != nil {
		return fmt.Errorf("cm discovery failed: %w", err)
	}

	b.logger.Info("Optimal CM server found",
		log.String("endpoint", server.Endpoint),
		log.Float64("load", server.Load),
	)

	// Start cost basis persistence background thread
	b.wg.Go(func() {
		b.costBasis.Start(ctx)
	})

	// Start config hot-reloader
	b.tradeCfgManager.StartWatching(ctx, 10*time.Second, b.logger)

	// Start manual prices watcher and load initial values
	if err := b.manualPrices.LoadAndApply(); err != nil {
		b.logger.Error("Failed to apply initial manual prices", log.Err(err))
	}

	b.manualPrices.StartWatcher(ctx, 10*time.Second)

	b.sub = b.client.Bus().Subscribe(&auth.LoggedOnEvent{}, &auth.LoggedOffEvent{})

	// Explicitly track background task execution using a WaitGroup
	b.wg.Go(func() {
		b.handleEvents(ctx)
	})

	b.logger.Info("Connecting and authenticating with Steam...",
		log.String("username", b.cfg.Username),
	)

	details := &auth.LogOnDetails{
		AccountName: b.cfg.Username,
		Password:    b.cfg.Password,
	}
	if err := b.client.ConnectAndLogin(ctx, server, details); err != nil {
		return fmt.Errorf("connect and login failed: %w", err)
	}

	b.logger.Info("Bot logged in. Starting background behaviors...")

	b.setupOrchestrator()

	if err := b.orchestrator.Start(ctx); err != nil {
		return fmt.Errorf("orchestrator start failed: %w", err)
	}

	b.logger.Info("Bot fully operational")

	b.waitForShutdown(ctx)

	return nil
}

// Close gracefully shuts down the bot, stopping the orchestrator and closing the client connection.
func (b *Bot) Close() {
	if b.orchestrator != nil {
		b.orchestrator.Stop()
		b.logger.Info("Behavior orchestrator stopped")
	}

	if b.sub != nil {
		b.sub.Unsubscribe()
	}

	// Wait for event handler loop to exit before closing the client
	b.wg.Wait()

	if err := b.client.Close(); err != nil {
		b.logger.Error("Error during client shutdown", log.Err(err))
	} else {
		b.logger.Info("Client session closed")
	}

	b.logger.Info("Bot shut down successfully")
}

func (b *Bot) discoverCMServer(ctx context.Context) (socket.CMServer, error) {
	dirCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	b.logger.Info("Discovering optimal Steam Connection Manager server...")
	dir := directory.New(b.client.Service())

	return dir.GetOptimalCMServer(dirCtx)
}

func (b *Bot) setupOrchestrator() {
	b.orchestrator = behavior.NewOrchestrator(b.logger, b.client.Bus())
	b.orchestrator.Register(b.pdbManager)

	bp := backpack.From(b.client)
	tf2Mod := tf2.From(b.client)
	webTradeManager := webtrading.From(b.client)
	guardian := guard.From(b.client)

	craftingManager := crafting.NewManager(bp, tf2Mod)
	metalManager := crafting.NewMetalManager(bp, craftingManager, b.logger)

	guardBehaviorCfg := guard.Config{
		AutoAcceptTypes: []guard.ConfirmationType{
			guard.ConfTypeTrade,
			guard.ConfTypeLogin,
		},
		PollOnStart: true,
	}

	b.orchestrator.Install(
		guard.AutoAccept(guardian, guardBehaviorCfg),
		stock.Control(bp, b.pdbManager, b.tradeCfgManager, b.costBasis, craftingManager, stock.DefaultConfig()),
		achievements.Simulate(tf2Mod, tf2.AchievementConfig()),
	)

	if b.cfg.CritAPIKey != "" {
		b.orchestrator.Install(
			critlistener.Listen(
				b.critClient,
				b.pdbManager,
				bp,
				b.tradeCfgManager,
				webTradeManager,
				b.cfg.TradeRequestEventStreamURL,
			),
		)
	} else {
		b.logger.Warn("Crit.tf API key is not configured, event stream listener behavior will not be started")
	}

	// 5. Setup the TF2 Trading Engine Middlewares
	tradeEngine := engine.New()

	tradeCfg := b.tradeCfgManager.GetConfig()

	stockCfg := tf2trading.StockConfig{
		MaxTotal:   tradeCfg.GlobalMaxStock,
		DefaultMax: tradeCfg.DefaultMaxStock,
		MaxPerSKU:  make(map[string]int),
	}
	for sku, c := range tradeCfg.Items {
		stockCfg.MaxPerSKU[sku] = c.MaxStock
	}

	schemaFunc := func() *schema.Schema {
		if m := schema.From(b.client); m != nil {
			return m.Get()
		}

		return nil
	}

	tradeEngine.Use(
		tf2trading.EscrowMiddleware(webTradeManager, b.logger),
		tf2trading.BanCheckMiddleware(b.bansManager, b.logger),
		tf2trading.PricerMiddleware(b.pdbManager, schemaFunc, b.logger),
		tf2trading.HalloweenSpellMiddleware(b.pdbClient, schemaFunc, b.tradeCfgManager.GetConfig, b.logger),
		tf2trading.DupeCheckMiddleware(b.bptfChecker, b.logger),
		tf2trading.StockLimitMiddleware(bp, stockCfg, b.logger),
		tf2trading.SmartCounterMiddleware(b.tradeCfgManager, metalManager, bp, webTradeManager, b.logger),
	)

	// 6. Connect the Engine to the Trade Manager
	// We use the built-in engine.BotHandler to bridge our engine with the SDK's processor.
	webTradeManager.SetOfferHandler(context.Background(), engine.NewBotHandler(tradeEngine, b.logger), bp)
}

func (b *Bot) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-b.sub.C():
			if !ok {
				return
			}

			switch ev := event.(type) {
			case *auth.LoggedOnEvent:
				b.logger.Info("Login successful", log.Uint64("steam_id", ev.SteamID))
			case *auth.LoggedOffEvent:
				b.logger.Info("Logged off")
			}
		}
	}
}

func (b *Bot) waitForShutdown(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		b.logger.Info("Shutdown triggered by context cancellation")
	case sig := <-sigChan:
		b.logger.Info("System signal received, shutting down gracefully", log.String("signal", sig.String()))
	}
}

func loadEnvConfig() (Config, error) {
	username, password := os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS")
	if username == "" || password == "" {
		return Config{}, errors.New("STEAM_USER and STEAM_PASS environment variables are required")
	}

	storagePath := os.Getenv("STEAM_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage.json"
	}

	return Config{
		Username:                   username,
		Password:                   password,
		SharedSecret:               os.Getenv("STEAM_SHARED_SECRET"),
		IdentitySecret:             os.Getenv("STEAM_IDENTITY_SECRET"),
		DeviceID:                   os.Getenv("STEAM_DEVICE_ID"),
		StoragePath:                storagePath,
		BptfAPIKey:                 os.Getenv("BPTF_API_KEY"),
		BptfUserToken:              os.Getenv("BPTF_USER_TOKEN"),
		MptfAPIKey:                 os.Getenv("MPTF_API_KEY"),
		CritAPIKey:                 os.Getenv("CRIT_API_KEY"),
		TradeRequestEventStreamURL: os.Getenv("TRADE_REQUEST_EVENT_STREAM_URL"),
	}, nil
}

func main() {
	cfg, err := loadEnvConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	store, err := jsonfile.New(cfg.StoragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := store.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close storage: %v\n", err)
		}
	}()

	logCfg := log.DefaultConfig(log.LevelDebug)
	logCfg.FullPath = true
	logCfg.OmitFields = []string{"account", "steam_id", "job_id", "correlation_id", "queue_delay_ms"}

	logger := log.New(logCfg)
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	bot, err := NewBot(cfg, store, logger)
	if err != nil {
		logger.Error("Failed to create bot", log.Err(err))
		return
	}
	defer bot.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bot.Run(ctx); err != nil {
		logger.Error("Bot runtime error", log.Err(err))
		return
	}
}

type pdbPriceProviderAdapter struct {
	mgr *pricedb.Manager
}

func (a *pdbPriceProviderAdapter) SetPrice(sku string, buy, sell currency.Currency, source pricing.Source) {
	a.mgr.SetPrice(sku, pricedb.Currencies{
		Keys:  int(buy.Keys),
		Metal: buy.Metal,
	}, pricedb.Currencies{
		Keys:  int(sell.Keys),
		Metal: sell.Metal,
	}, source)
}
