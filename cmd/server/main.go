package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lawale/quorum/console"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/config"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/server"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/store/mssql"
	"github.com/lawale/quorum/internal/store/postgres"
	"github.com/lawale/quorum/internal/webhook"
	"github.com/lawale/quorum/widgets"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Database and Stores
	ctx := context.Background()
	dsn := cfg.Database.DSN()
	maxOpen := cfg.Database.MaxOpenConns
	maxIdle := cfg.Database.MaxIdleConns

	var stores *store.Stores
	switch cfg.Database.Driver {
	case "postgres", "":
		stores, err = postgres.NewStores(ctx, dsn, maxOpen, maxIdle)
	case "mssql":
		stores, err = mssql.NewStores(ctx, dsn, maxOpen, maxIdle)
	default:
		slog.Error("unsupported database driver", "driver", cfg.Database.Driver)
		os.Exit(1)
	}
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer stores.Close()
	slog.Info("connected to database", "driver", cfg.Database.Driver)

	// Permission checker for external permission callback
	permissionChecker := auth.NewPermissionChecker(cfg.Webhook.Timeout)

	// Services
	policyService := service.NewPolicyService(stores.Policies)
	requestService := service.NewRequestService(stores.Requests, stores.Approvals, stores.Policies, stores.Audits, permissionChecker)
	webhookService := service.NewWebhookService(stores.Webhooks)

	// Webhook dispatcher
	dispatcher := webhook.NewDispatcher(stores.Webhooks, stores.Audits, cfg.Webhook.Timeout, cfg.Webhook.MaxRetries, cfg.Webhook.RetryInterval, cfg.Webhook.CallbackSecret)
	requestService.SetOnResolve(dispatcher.Dispatch)

	// Expiry worker
	expiryWorker := service.NewExpiryWorker(stores.Requests, stores.Audits, cfg.Expiry.CheckInterval)
	expiryWorker.SetOnExpire(dispatcher.Dispatch)

	// Metrics (optional)
	var (
		metricsInstance *metrics.Metrics
		metricsRegistry *prometheus.Registry
	)
	if cfg.Metrics.Enabled {
		metricsRegistry = prometheus.NewRegistry()
		metricsInstance = metrics.New(metricsRegistry)
		requestService.SetMetrics(metricsInstance)
		expiryWorker.SetMetrics(metricsInstance)
		dispatcher.SetMetrics(metricsInstance)
		slog.Info("metrics enabled", "path", cfg.Metrics.Path)
	}

	// Start background workers
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	dispatcher.Start(appCtx)
	expiryWorker.Start(appCtx)

	// Auth provider
	var authProvider auth.Provider
	switch cfg.Auth.Mode {
	case "trust":
		authProvider = auth.NewTrustProvider(cfg.Auth.Trust.UserIDHeader, cfg.Auth.Trust.RolesHeader)
	default:
		slog.Error("unsupported auth mode", "mode", cfg.Auth.Mode)
		os.Exit(1)
	}

	// Operator service for admin console (optional)
	var operatorService *service.OperatorService
	if cfg.Console.Enabled {
		operatorService = service.NewOperatorService(stores.Operators, cfg.Console.JWTSecret)
		slog.Info("admin console enabled")
	}

	// HTTP server
	srv := server.New(server.Config{
		RequestService:  requestService,
		PolicyService:   policyService,
		WebhookService:  webhookService,
		OperatorService: operatorService,
		AuditStore:      stores.Audits,
		AuthProvider:    authProvider,
		ConsoleEnabled:  cfg.Console.Enabled,
		ConsoleSPA:      console.Handler(),
		EmbedHandler:    widgets.Handler(),
		Metrics:         metricsInstance,
		MetricsPath:     cfg.Metrics.Path,
		Registry:        metricsRegistry,
	})

	httpServer := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("starting server", "addr", cfg.Server.Addr())
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
