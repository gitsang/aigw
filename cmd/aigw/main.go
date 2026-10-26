package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gitsang/aigw/internal/config"
	"github.com/gitsang/aigw/internal/crypto"
	"github.com/gitsang/aigw/internal/health"
	"github.com/gitsang/aigw/internal/server"
	"github.com/gitsang/aigw/internal/store"
)

func main() {
	configPath := flag.String("config", envOr("AIGW_CONFIG", "configs/config.yml"), "path to config file")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}
	secretCipher, err := crypto.New(cfg.Security.EncryptionKey)
	if err != nil {
		log.Error("initialize encryption", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.Database.URL, cfg.Database.MaxConns, cfg.Database.MinConns, secretCipher)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer st.Close()
	if cfg.Database.Migrate {
		if err := st.Migrate(ctx); err != nil {
			log.Error("migrate database", "error", err)
			os.Exit(1)
		}
		log.Info("database schema is up to date")
	}

	prober := health.New(st, cfg.Health.Interval, cfg.Health.Timeout, cfg.Health.Concurrency,
		func(format string, args ...any) { log.Info("health", "message", sprintf(format, args...)) })
	go prober.Run(ctx)

	srv := server.New(cfg, st, log)
	if err := srv.ListenAndServe(ctx); err != nil {
		log.Error("http server", "error", err)
		os.Exit(1)
	}
	log.Info("shutdown complete")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func sprintf(format string, args ...any) string {
	return fmtSprintf(format, args...)
}
