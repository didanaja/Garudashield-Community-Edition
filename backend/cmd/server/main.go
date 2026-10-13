// Command server runs the GarudaShield Demo Edition engine.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"garudashield/internal/api"
	"garudashield/internal/auth"
	"garudashield/internal/config"
)

func main() {
	var (
		hashPassword = flag.String("hash-password", "", "print a PBKDF2 hash for GARUDA_ADMIN_PASSWORD_HASH and exit")
		showVersion  = flag.Bool("version", false, "print the demo engine version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("GarudaShield Demo Engine " + api.Version)
		return
	}

	if *hashPassword != "" {
		h, err := auth.HashPassword(*hashPassword)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(h)
		return
	}

	// Load a local .env file if present. Real deployments should set environment
	// variables directly; this is a convenience for running the demo locally.
	if err := loadDotEnv(".env"); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not read .env:", err)
	}

	logger := newLogger()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	// Development convenience: accept a plaintext admin password from the
	// environment and hash it in memory at startup. Prefer
	// GARUDA_ADMIN_PASSWORD_HASH in any non-local environment.
	if cfg.AdminPasswordHash == "" && cfg.AdminPassword != "" {
		h, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			logger.Error("could not hash admin password", "error", err)
			os.Exit(1)
		}
		cfg.AdminPasswordHash = h
		logger.Warn("admin password taken from GARUDA_ADMIN_PASSWORD; use GARUDA_ADMIN_PASSWORD_HASH instead",
			"hint", "never commit plaintext secrets")
	}

	app, err := api.New(cfg, logger)
	if err != nil {
		logger.Error("failed to start engine", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("GarudaShield Demo Engine listening",
			"addr", cfg.ListenAddr,
			"edition", "Demo Edition",
			"upstream", proxyDisplay(cfg),
			"rate_limit", cfg.RateLimitEnabled,
			"fail_closed", cfg.FailClosed,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	if err := app.Store().Close(); err != nil {
		logger.Error("store close failed", "error", err)
	}
	logger.Info("stopped cleanly")
}

func proxyDisplay(cfg *config.Config) string {
	if cfg.UpstreamURL == nil {
		return "unconfigured"
	}
	return cfg.UpstreamURL.Host
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if v := os.Getenv("GARUDA_LOG_LEVEL"); v != "" {
		if l, err := parseLevel(v); err == nil {
			level = l
		}
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

func parseLevel(v string) (slog.Level, error) {
	switch v {
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "info":
		return slog.LevelInfo, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown level %q", v)
	}
}

// loadDotEnv reads a KEY=VALUE file and sets any variables that are not already
// present in the process environment. Existing environment variables always win.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
