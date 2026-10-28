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

	"foundry-auth-proxy/internal/config"
	"foundry-auth-proxy/internal/identity"
	"foundry-auth-proxy/internal/login"
	"foundry-auth-proxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the JSON config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	resolver := identity.HeaderResolver{Header: cfg.UserHeader}

	handler, err := proxy.New(cfg, resolver, login.NewClient(cfg.FoundryURL), logger)
	if err != nil {
		logger.Error("build proxy", "err", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "addr", cfg.Listen, "foundry", cfg.FoundryURL)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("serve", "err", err)
		os.Exit(1)
	}
}
