package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wale/maker-checker/internal/auth"
	"github.com/wale/maker-checker/internal/config"
	"github.com/wale/maker-checker/internal/server"
	"github.com/wale/maker-checker/internal/service"
	"github.com/wale/maker-checker/internal/store/postgres"
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

	// Database
	ctx := context.Background()
	db, err := postgres.New(ctx, cfg.Database.DSN(), cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("connected to database")

	// Stores
	requestStore := postgres.NewRequestStore(db)
	approvalStore := postgres.NewApprovalStore(db)
	policyStore := postgres.NewPolicyStore(db)
	webhookStore := postgres.NewWebhookStore(db)
	auditStore := postgres.NewAuditStore(db)

	// Services
	policyService := service.NewPolicyService(policyStore)
	requestService := service.NewRequestService(requestStore, approvalStore, policyStore, auditStore)
	webhookService := service.NewWebhookService(webhookStore)

	// Auth provider
	var authProvider auth.Provider
	switch cfg.Auth.Mode {
	case "trust":
		authProvider = auth.NewTrustProvider(cfg.Auth.Trust.UserIDHeader, cfg.Auth.Trust.RolesHeader)
	default:
		slog.Error("unsupported auth mode", "mode", cfg.Auth.Mode)
		os.Exit(1)
	}

	// HTTP server
	srv := server.New(server.Config{
		RequestService: requestService,
		PolicyService:  policyService,
		WebhookService: webhookService,
		AuditStore:     auditStore,
		AuthProvider:   authProvider,
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
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
