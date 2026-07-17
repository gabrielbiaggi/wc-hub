package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/webcreations/wc-hub/back/internal/app"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
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
