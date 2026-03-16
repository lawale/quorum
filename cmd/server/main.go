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
	"github.com/lawale/quorum/internal/sse"
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
	tenantService := service.NewTenantService(stores.Tenants)
	if err := tenantService.LoadCache(ctx); err != nil {
		slog.Error("failed to load tenant cache", "error", err)
		os.Exit(1)
	}

	// Webhook dispatcher (outbox-backed, signal-driven)
	dispatcher := webhook.NewDispatcher(stores.Outbox, stores.Audits, webhook.Config{
		Timeout:         cfg.Webhook.Timeout,
		MaxRetries:      cfg.Webhook.MaxRetries,
		RetryDelay:      cfg.Webhook.RetryInterval,
		RetryWindow:     cfg.Webhook.RetryWindow,
		MaxRetryDelay:   cfg.Webhook.MaxRetryDelay,
		Heartbeat:       cfg.Webhook.Heartbeat,
		RetentionPeriod: cfg.Webhook.OutboxRetention,
	})
	requestService.SetWebhookDispatch(stores.RunInTx, dispatcher.Enqueue, dispatcher.Signal)

	// SSE event hub for real-time push to connected widgets
	eventHub := sse.NewHub()
	requestService.SetSSESignal(eventHub.Publish)

	// Expiry worker
	expiryWorker := service.NewExpiryWorker(stores.Requests, stores.Audits, cfg.Expiry.CheckInterval)
	expiryWorker.SetWebhookDispatch(stores.RunInTx, dispatcher.Enqueue, dispatcher.Signal)
	expiryWorker.SetSSESignal(eventHub.Publish)

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
	tenantService.StartCacheRefresh(appCtx, cfg.Tenant.CacheRefreshInterval)

	// Auth provider
	var authProvider auth.Provider
	switch cfg.Auth.Mode {
	case "trust":
		authProvider = auth.NewTrustProvider(cfg.Auth.Trust.UserIDHeader, cfg.Auth.Trust.RolesHeader, cfg.Auth.Trust.TenantIDHeader)
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
		TenantService:   tenantService,
		OperatorService: operatorService,
		AuditStore:      stores.Audits,
		OutboxStore:     stores.Outbox,
		SignalWorker:    dispatcher.Signal,
		AuthProvider:    authProvider,
		EventHub:        eventHub,
		ConsoleEnabled:  cfg.Console.Enabled,
		ConsoleSPA:      console.Handler(),
		EmbedHandler:    widgets.Handler(),
		Metrics:         metricsInstance,
		MetricsPath:     cfg.Metrics.Path,
		Registry:        metricsRegistry,
	})

	httpServer := &http.Server{
		Addr:        cfg.Server.Addr(),
		Handler:     srv.Handler(),
		ReadTimeout: 15 * time.Second,
		// WriteTimeout disabled (0) to support long-lived SSE connections.
		// The SSE handler manages per-write deadlines via http.ResponseController.
		IdleTimeout: 60 * time.Second,
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
