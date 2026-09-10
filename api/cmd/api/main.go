package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexbarbatescu/payments-orchestrator/api/internal/config"
	"github.com/alexbarbatescu/payments-orchestrator/api/internal/db"
	"github.com/alexbarbatescu/payments-orchestrator/api/internal/httpapi"
	"github.com/alexbarbatescu/payments-orchestrator/api/internal/payment"
	"github.com/alexbarbatescu/payments-orchestrator/api/internal/recon"
	"github.com/alexbarbatescu/payments-orchestrator/api/internal/webhook"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gdb, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB); err != nil {
		return err
	}
	logger.Info("connected to postgres")

	store := payment.NewPostgresStore(gdb)
	svc := payment.NewService(store)
	if os.Getenv("RUN_WORKER") == "1" {
		go func() {
			_ = webhook.New(gdb, cfg.WebhookURL, cfg.WebhookSecret, logger).Run(ctx)
		}()
		go func() {
			_ = recon.New(gdb, logger).Run(ctx)
		}()
		logger.Info("worker loops started in api process")
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(logger, gdb, svc, store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
