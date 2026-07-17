package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	overview "github.com/webcreations/wc-hub/back/internal/overview/application"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	overview *overview.Service
	policy   *security.Engine
	db       *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, func(), error) {
	var pool *pgxpool.Pool
	var err error
	if cfg.DatabaseURL != "" {
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, err
		}
	}
	application := &App{cfg: cfg, logger: logger, overview: overview.New(cfg.Environment, cfg.SelfProtected), policy: security.NewEngine(cfg.LocalAllowlist), db: pool}
	cleanup := func() {
		if pool != nil {
			pool.Close()
		}
	}
	return application, cleanup, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/v1/overview", a.getOverview)
	mux.HandleFunc("POST /api/v1/security/evaluate", a.evaluatePolicy)
	mux.HandleFunc("GET /api/v1/modules", a.modules)
	return a.middleware(mux)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC(), "self_protected": a.cfg.SelfProtected})
}
func (a *App) getOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.overview.Snapshot())
}
func (a *App) evaluatePolicy(w http.ResponseWriter, r *http.Request) {
	var req security.ActionRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	decision := a.policy.Evaluate(req)
	status := 200
	if !decision.Allowed {
		status = 403
	}
	writeJSON(w, status, decision)
}
func (a *App) modules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []string{"overview", "proxmox", "cloud", "kubernetes", "docker", "github", "tunnels", "terraform", "telemetry", "remote-access", "storage", "settings", "audit"})
}

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if a.cfg.Environment == "development" {
			w.Header().Set("Access-Control-Allow-Origin", a.cfg.PublicURL)
		}
		next.ServeHTTP(w, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
