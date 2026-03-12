package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/wale/maker-checker/internal/config"
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

	_ = cfg
	fmt.Println("maker-checker service starting...")
	slog.Info("config loaded", "addr", cfg.Server.Addr(), "auth_mode", cfg.Auth.Mode)
}
