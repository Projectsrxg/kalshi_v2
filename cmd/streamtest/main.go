// streamtest connects to Kalshi WebSocket and streams parsed messages to console.
// Usage: go run ./cmd/streamtest --config configs/gatherer.local.yaml
//
// Required environment variables:
//
//	KALSHI_API_KEY         - Your API key ID from Kalshi dashboard
//	KALSHI_PRIVATE_KEY_PATH - Path to your RSA private key PEM file
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rickgao/kalshi-data/internal/api"
	"github.com/rickgao/kalshi-data/internal/auth"
	"github.com/rickgao/kalshi-data/internal/config"
	"github.com/rickgao/kalshi-data/internal/connection"
	"github.com/rickgao/kalshi-data/internal/market"
	"github.com/rickgao/kalshi-data/internal/router"
)

func main() {
	configPath := flag.String("config", "configs/gatherer.example.yaml", "path to config file")
	verbose := flag.Bool("verbose", false, "print full message JSON")
	flag.Parse()

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("received shutdown signal")
		cancel()
	}()

	// Load authentication credentials (need private key for REST API too)
	var privateKey *auth.Credentials
	if cfg.API.PrivateKeyPath != "" {
		var err error
		privateKey, err = auth.LoadCredentials(cfg.API.APIKey, cfg.API.PrivateKeyPath)
		if err != nil {
			logger.Error("failed to load credentials for REST API", "error", err)
			os.Exit(1)
		}
	}

	// Create API client with authentication
	var apiClient *api.Client
	if privateKey != nil {
		apiClient = api.NewClient(cfg.API.RestURL, cfg.API.APIKey, privateKey.PrivateKey, api.WithLogger(logger))
	} else {
		apiClient = api.NewClient(cfg.API.RestURL, cfg.API.APIKey, nil, api.WithLogger(logger))
	}

	// Create Market Registry
	registry := market.NewRegistry(market.Config{
		ReconcileInterval:  cfg.Connections.ReconnectBaseDelay,
		PageSize:           1000,
		InitialLoadTimeout: 30 * time.Minute,
	}, apiClient, logger)

	// Start registry - this does initial sync
	logger.Info("starting market registry sync...")
	if err := registry.Start(ctx); err != nil {
		logger.Error("failed to start market registry", "error", err)
		os.Exit(1)
	}

	activeMarkets := registry.GetActiveMarkets()
	logger.Info("market registry ready", "active_markets", len(activeMarkets))

	// Check we have credentials for WebSocket (already loaded for REST API)
	if privateKey == nil {
		logger.Error("API credentials required for WebSocket",
			"api_key_set", cfg.API.APIKey != "",
			"private_key_path_set", cfg.API.PrivateKeyPath != "",
		)
		logger.Info("Set environment variables: KALSHI_API_KEY and KALSHI_PRIVATE_KEY_PATH")
		os.Exit(1)
	}
	logger.Info("using API credentials", "key_id", cfg.API.APIKey)

	// Create Connection Manager (use credentials already loaded for REST API)
	connCfg := connection.DefaultManagerConfig()
	connCfg.WSURL = cfg.API.WSURL
	connCfg.KeyID = privateKey.KeyID
	connCfg.PrivateKey = privateKey.PrivateKey
	connCfg.MessageBufferSize = 10000

	connMgr := connection.NewManager(connCfg, registry, logger)

	// Create Router using Connection Manager's message channel
	rtr := router.NewRouter(router.RouterConfig{
		OrderbookBufferSize: 1000,
		TradeBufferSize:     1000,
		TickerBufferSize:    1000,
	}, connMgr.Messages(), logger)

	// Start Connection Manager (will auto-subscribe to active markets)
	logger.Info("starting connection manager")
	if err := connMgr.Start(ctx); err != nil {
		logger.Error("failed to start connection manager", "error", err)
		os.Exit(1)
	}

	// Start Router
	logger.Info("starting router")
	if err := rtr.Start(ctx); err != nil {
		logger.Error("failed to start router", "error", err)
		os.Exit(1)
	}

	// Get buffers
	buffers := rtr.Buffers()

	// Start console printers
	go printOrderbook(ctx, buffers.Orderbook, *verbose, logger)
	go printTrades(ctx, buffers.Trade, *verbose, logger)
	go printTickers(ctx, buffers.Ticker, *verbose, logger)

	// Stats printer
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				routerStats := rtr.Stats()
				connStats := connMgr.Stats()
				logger.Info("stats",
					"conn_connected", connStats.ConnectedCount,
					"markets_subscribed", connStats.MarketsSubscribed,
					"router_received", routerStats.MessagesReceived,
					"router_routed", routerStats.MessagesRouted,
					"parse_errors", routerStats.ParseErrors,
					"orderbook_buf", routerStats.OrderbookBuffer.Count,
					"trade_buf", routerStats.TradeBuffer.Count,
					"ticker_buf", routerStats.TickerBuffer.Count,
				)
			}
		}
	}()

	logger.Info("streaming started - press Ctrl+C to stop")

	// Wait for shutdown
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down...")
	rtr.Stop(shutdownCtx)
	connMgr.Stop(shutdownCtx)
	registry.Stop(shutdownCtx)

	logger.Info("shutdown complete")
}

func printOrderbook(ctx context.Context, buf *router.GrowableBuffer[router.OrderbookMsg], verbose bool, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := buf.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if verbose {
				data, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("[ORDERBOOK] %s\n", data)
			} else {
				if msg.Type == "delta" {
					fmt.Printf("[ORDERBOOK DELTA] ticker=%s side=%s price=%s delta=%d seq=%d\n",
						msg.Ticker, msg.Side, msg.PriceDollars, msg.Delta, msg.Seq)
				} else {
					fmt.Printf("[ORDERBOOK SNAPSHOT] ticker=%s yes_levels=%d no_levels=%d seq=%d\n",
						msg.Ticker, len(msg.Yes), len(msg.No), msg.Seq)
				}
			}
		}
	}
}

func printTrades(ctx context.Context, buf *router.GrowableBuffer[router.TradeMsg], verbose bool, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := buf.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if verbose {
				data, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("[TRADE] %s\n", data)
			} else {
				fmt.Printf("[TRADE] ticker=%s id=%s size=%d yes_price=%s taker=%s\n",
					msg.Ticker, msg.TradeID, msg.Size, msg.YesPriceDollars, msg.TakerSide)
			}
		}
	}
}

func printTickers(ctx context.Context, buf *router.GrowableBuffer[router.TickerMsg], verbose bool, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := buf.TryReceive()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if verbose {
				data, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("[TICKER] %s\n", data)
			} else {
				fmt.Printf("[TICKER] ticker=%s price=%s bid=%s ask=%s vol=%d\n",
					msg.Ticker, msg.PriceDollars, msg.YesBidDollars, msg.YesAskDollars, msg.Volume)
			}
		}
	}
}
