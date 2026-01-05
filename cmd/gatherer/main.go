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
	"syscall"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/config"
	"github.com/rickgao/kalshi-data/internal/database"
	"github.com/rickgao/kalshi-data/internal/market"
	"github.com/rickgao/kalshi-data/internal/version"
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

	// Create API client
	apiClient := api.NewClient(
		cfg.API.RestURL,
		cfg.API.APIKey,
		api.WithLogger(logger),
		api.WithTimeout(30*time.Second),
		api.WithRetries(3, time.Second),
	)

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

	// Start health server early so we can monitor sync progress
	healthPort := 8080
	if cfg.Metrics.Port > 0 {
		healthPort = cfg.Metrics.Port
	}

	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", healthPort),
		Handler: createHealthHandler(pools, registry, logger),
	}

	go func() {
		logger.Info("starting health server", "port", healthPort)
		if err := healthServer.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()

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

	// TODO: Start Connection Manager (WebSocket connections)
	// TODO: Start Message Router
	// TODO: Start Writers
	// TODO: Start Snapshot Poller

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
func createHealthHandler(pools *database.Pools, registry market.Registry, logger *slog.Logger) http.Handler {
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

	return mux
}
