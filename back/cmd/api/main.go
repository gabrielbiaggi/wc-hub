package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/webcreations/wc-hub/back/internal/app"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
	telemetryrepo "github.com/webcreations/wc-hub/back/internal/telemetry/repository"
)

func main() {
	cfg := config.Load()
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1" + cfg.HTTPAddr + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "provision-agent-token" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			panic(err)
		}
		defer pool.Close()
		var hostID string
		if err = pool.QueryRow(ctx, `SELECT id::text FROM hosts WHERE name=$1`, os.Args[2]).Scan(&hostID); err != nil {
			panic(err)
		}
		token, err := telemetryrepo.NewPostgres(pool).ProvisionToken(ctx, hostID, "")
		if err != nil {
			panic(err)
		}
		fmt.Println(token)
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel()}))
	application, cleanup, err := app.New(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: application.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("control plane listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
