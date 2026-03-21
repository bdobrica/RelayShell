package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	logLevel, err := parseLogLevel(os.Getenv("RELAY_LOG_LEVEL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid RELAY_LOG_LEVEL: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	app, err := newApp(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("RelayShell governor starting")
	if err := app.run(ctx); err != nil {
		logger.Error("RelayShell governor failed", "error", fmt.Errorf("run: %w", err))
		os.Exit(1)
	}
	logger.Info("RelayShell governor shutting down")
}

func parseLogLevel(raw string) (slog.Level, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return slog.LevelInfo, nil
	}

	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("supported values are debug, info, warn, error")
	}
}
