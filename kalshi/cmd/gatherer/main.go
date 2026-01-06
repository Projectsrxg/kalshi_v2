package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/auth"
	"github.com/rickgao/kalshi-data/internal/config"
	"github.com/rickgao/kalshi-data/internal/connection"
	"github.com/rickgao/kalshi-data/internal/database"
	"github.com/rickgao/kalshi-data/internal/market"
	"github.com/rickgao/kalshi-data/internal/router"
	"github.com/rickgao/kalshi-data/internal/version"
	"github.com/rickgao/kalshi-data/internal/writer"
)

func main() {
	configPath := flag.String("config", "configs/gatherer.local.yaml", "path to config file")
	flag.Parse()

	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	logger.Info("starting gatherer",
		"version", version.Version,
		"commit", version.Commit,
		"config", *configPath,
	)

	// Load configuration
	cfg, err := config.LoadAndValidate(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded",
		"instance_id", cfg.Instance.ID,
		"api_url", cfg.API.RestURL,
	)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Connect to database
	logger.Info("connecting to database",
		"host", cfg.Database.Timescale.Host,
		"port", cfg.Database.Timescale.Port,
		"database", cfg.Database.Timescale.Name,
	)

	pools, err := database.NewPools(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pools.Close()

	logger.Info("database connected")

	// Load API credentials
	var privateKey *auth.Credentials
	if cfg.API.PrivateKeyPath != "" {
		var err error
		privateKey, err = auth.LoadCredentials(cfg.API.APIKey, cfg.API.PrivateKeyPath)
		if err != nil {
			logger.Error("failed to load API credentials", "error", err)
			os.Exit(1)
		}
		logger.Info("loaded API credentials", "key_id", cfg.API.APIKey)
	}

	// Create API client
	var apiClient *api.Client
	if privateKey != nil {
		apiClient = api.NewClient(
			cfg.API.RestURL,
			cfg.API.APIKey,
			privateKey.PrivateKey,
			api.WithLogger(logger),
			api.WithTimeout(30*time.Second),
			api.WithRetries(3, time.Second),
		)
	} else {
		apiClient = api.NewClient(
			cfg.API.RestURL,
			cfg.API.APIKey,
			nil,
			api.WithLogger(logger),
			api.WithTimeout(30*time.Second),
			api.WithRetries(3, time.Second),
		)
	}

	// Check exchange status
	logger.Info("checking exchange status")
	status, err := apiClient.GetExchangeStatus(ctx)
	if err != nil {
		logger.Error("failed to get exchange status", "error", err)
		os.Exit(1)
	}
	logger.Info("exchange status",
		"exchange_active", status.ExchangeActive,
		"trading_active", status.TradingActive,
	)

	// Create market registry
	registryCfg := market.Config{
		ReconcileInterval:  5 * time.Minute,
		PageSize:           1000,
		InitialLoadTimeout: 30 * time.Minute,
	}
	registry := market.NewRegistry(registryCfg, apiClient, logger)

	// Health server port (will start after all components are ready)
	healthPort := 8080
	if cfg.Metrics.Port > 0 {
		healthPort = cfg.Metrics.Port
	}

	// Start market registry (initial sync)
	logger.Info("starting market registry (initial sync)...")
	if err := registry.Start(ctx); err != nil {
		logger.Error("failed to start market registry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		registry.Stop(shutdownCtx)
	}()

	activeMarkets := registry.GetActiveMarkets()
	logger.Info("market registry started",
		"active_markets", len(activeMarkets),
	)

	// Create Connection Manager
	connMgrCfg := connection.DefaultManagerConfig()
	connMgrCfg.WSURL = cfg.API.WSURL
	connMgrCfg.KeyID = cfg.API.APIKey
	if privateKey != nil {
		connMgrCfg.PrivateKey = privateKey.PrivateKey
	}

	connMgr := connection.NewManager(connMgrCfg, registry, logger)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		connMgr.Stop(shutdownCtx)
	}()

	// Start health server (now that connMgr exists for debug endpoints)
	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthPort),
		Handler: createHealthHandler(pools, registry, connMgr, logger),
	}
	go func() {
		logger.Info("starting health server", "port", healthPort)
		if err := healthServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()

	// Create and Start Message Router BEFORE Connection Manager
	// (so it's ready to consume messages as soon as connections are established)
	routerCfg := router.DefaultRouterConfig()
	msgRouter := router.NewRouter(routerCfg, connMgr.Messages(), logger)

	logger.Info("starting message router...")
	if err := msgRouter.Start(ctx); err != nil {
		logger.Error("failed to start message router", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		msgRouter.Stop(shutdownCtx)
	}()
	logger.Info("message router started")

	// Create and Start Writers BEFORE Connection Manager
	writerCfg := writer.WriterConfig{
		BatchSize:     cfg.Writers.BatchSize,
		FlushInterval: cfg.Writers.FlushInterval,
	}
	if writerCfg.BatchSize == 0 {
		writerCfg.BatchSize = 1000
	}
	if writerCfg.FlushInterval == 0 {
		writerCfg.FlushInterval = 5 * time.Second
	}

	buffers := msgRouter.Buffers()

	tradeWriter := writer.NewTradeWriter(writerCfg, buffers.Trade, pools.Timescale, logger)
	orderbookWriter := writer.NewOrderbookWriter(writerCfg, buffers.Orderbook, pools.Timescale, logger)
	tickerWriter := writer.NewTickerWriter(writerCfg, buffers.Ticker, pools.Timescale, logger)

	logger.Info("starting writers...")
	if err := tradeWriter.Start(ctx); err != nil {
		logger.Error("failed to start trade writer", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		tradeWriter.Stop(shutdownCtx)
	}()

	if err := orderbookWriter.Start(ctx); err != nil {
		logger.Error("failed to start orderbook writer", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		orderbookWriter.Stop(shutdownCtx)
	}()

	if err := tickerWriter.Start(ctx); err != nil {
		logger.Error("failed to start ticker writer", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		tickerWriter.Stop(shutdownCtx)
	}()
	logger.Info("writers started")

	// NOW start Connection Manager (consumers are ready)
	logger.Info("starting connection manager...")
	if err := connMgr.Start(ctx); err != nil {
		logger.Error("failed to start connection manager", "error", err)
		os.Exit(1)
	}
	logger.Info("connection manager started")

	// TODO: Start Snapshot Poller (optional, for backup data)

	logger.Info("gatherer running",
		"instance_id", cfg.Instance.ID,
		"health_url", fmt.Sprintf("http://localhost:%d/health", healthPort),
	)

	// Wait for shutdown
	<-ctx.Done()

	logger.Info("shutting down...")

	// Graceful shutdown of health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	healthServer.Shutdown(shutdownCtx)

	logger.Info("gatherer stopped")
}

// createHealthHandler creates the HTTP handler for health checks.
func createHealthHandler(pools *database.Pools, registry market.Registry, connMgr connection.Manager, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		health := struct {
			Status     string                 `json:"status"`
			Components map[string]interface{} `json:"components"`
		}{
			Status:     "healthy",
			Components: make(map[string]interface{}),
		}

		// Check database
		if err := pools.Ping(ctx); err != nil {
			health.Status = "unhealthy"
			health.Components["timescaledb"] = map[string]string{
				"status": "disconnected",
				"error":  err.Error(),
			}
		} else {
			health.Components["timescaledb"] = "connected"
		}

		// Check market registry
		activeMarkets := registry.GetActiveMarkets()
		health.Components["market_registry"] = map[string]interface{}{
			"markets": len(activeMarkets),
		}
		if len(activeMarkets) == 0 {
			health.Status = "degraded"
		}

		// Set response
		w.Header().Set("Content-Type", "application/json")
		if health.Status == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(health)
	})

	mux.HandleFunc("/debug/markets", func(w http.ResponseWriter, r *http.Request) {
		markets := registry.GetActiveMarkets()

		// Limit to first 100 for debugging
		limit := 100
		if len(markets) > limit {
			markets = markets[:limit]
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   len(registry.GetActiveMarkets()),
			"showing": len(markets),
			"markets": markets,
		})
	})

	mux.HandleFunc("/debug/connections", func(w http.ResponseWriter, r *http.Request) {
		stats := connMgr.ConnStats()

		// Group by role for cleaner output
		byRole := make(map[string][]connection.ConnStat)
		for _, s := range stats {
			byRole[s.Role] = append(byRole[s.Role], s)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":   len(stats),
			"by_role": byRole,
		})
	})

	mux.HandleFunc("/debug/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		connIDStr := r.URL.Query().Get("conn")
		if connIDStr == "" {
			http.Error(w, "conn parameter required (e.g., ?conn=7)", http.StatusBadRequest)
			return
		}

		connID, err := strconv.Atoi(connIDStr)
		if err != nil {
			http.Error(w, "invalid conn parameter", http.StatusBadRequest)
			return
		}

		if err := connMgr.DisconnectConn(connID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "disconnected",
			"conn_id": connID,
			"message": "Connection closed. Markets will be redistributed and connection will reconnect.",
		})
	})

	return mux
}
