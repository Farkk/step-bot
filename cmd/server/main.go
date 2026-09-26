package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"step-bot/internal/admin"
	"step-bot/internal/config"
	"step-bot/internal/httpserver"
	"step-bot/internal/max"
	"step-bot/internal/storage"
	"step-bot/internal/tasks"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		response, err := http.Get("http://127.0.0.1:8080/health/ready")
		if err != nil {
			os.Exit(1)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration", "error", err)
		os.Exit(1)
	}
	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if len(os.Args) > 1 && os.Args[1] == "create-owner" {
		if len(os.Args) != 5 || os.Getenv("ADMIN_INITIAL_PASSWORD") == "" {
			logger.Error("usage: create-owner COMPANY EMAIL NAME with ADMIN_INITIAL_PASSWORD set")
			os.Exit(2)
		}
		if err := admin.CreateOwner(db, os.Args[2], os.Args[3], os.Args[4], os.Getenv("ADMIN_INITIAL_PASSWORD")); err != nil {
			logger.Error("create owner", "error", err)
			os.Exit(1)
		}
		logger.Info("owner created")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := storage.Migrate(ctx, db, "/app/migrations"); err != nil {
			logger.Error("migration", "error", err)
			os.Exit(1)
		}
		logger.Info("migrations complete")
		return
	}
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpserver.New(db, cfg.S3Endpoint, cfg.WebhookSecret, cfg.BotToken, cfg.Env, "/app/static/app"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	inboxDone := make(chan struct{})
	go func() { defer close(workerDone); max.RunOutbox(ctx, db, max.Sender{Token: cfg.BotToken}, logger) }()
	go func() { defer close(inboxDone); tasks.RunMaxInbox(ctx, db, max.Sender{Token: cfg.BotToken}, logger) }()
	go func() {
		logger.Info("server started", "port", cfg.Port, "environment", cfg.Env)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	<-workerDone
	<-inboxDone
}
