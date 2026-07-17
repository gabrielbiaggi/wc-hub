package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	authapp "github.com/webcreations/wc-hub/back/internal/auth/application"
	authdomain "github.com/webcreations/wc-hub/back/internal/auth/domain"
	authrepo "github.com/webcreations/wc-hub/back/internal/auth/repository"
	inventorydomain "github.com/webcreations/wc-hub/back/internal/inventory/domain"
	inventoryrepo "github.com/webcreations/wc-hub/back/internal/inventory/repository"
	overview "github.com/webcreations/wc-hub/back/internal/overview/application"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

type contextKey string

const sessionContextKey contextKey = "session"
const sessionCookie = "wc_hub_session"

type App struct {
	cfg       config.Config
	logger    *slog.Logger
	overview  *overview.Service
	policy    *security.Engine
	db        *pgxpool.Pool
	auth      *authapp.Service
	audit     *auditrepo.Postgres
	inventory inventorydomain.Repository
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, func(), error) {
	if cfg.DatabaseURL == "" {
		return nil, nil, fmt.Errorf("WC_HUB_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, err
	}
	application := &App{cfg: cfg, logger: logger, overview: overview.New(cfg.Environment, cfg.SelfProtected), policy: security.NewEngine(cfg.LocalAllowlist), db: pool}
	application.auth = authapp.New(authrepo.NewPostgres(pool), cfg.SessionTTL)
	if err := application.auth.ConfigureTOTP(cfg.EncryptionKey, cfg.TOTPIssuer); err != nil {
		logger.Warn("TOTP enrollment disabled", "error", err)
	}
	application.audit = auditrepo.NewPostgres(pool)
	application.inventory = inventoryrepo.NewPostgres(pool)
	return application, func() { pool.Close() }, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/v1/auth/bootstrap-status", a.bootstrapStatus)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.bootstrap)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /api/v1/auth/session", a.protect("", a.session))
	mux.HandleFunc("POST /api/v1/auth/logout", a.protect("", a.logout))
	mux.HandleFunc("POST /api/v1/auth/totp/enroll", a.protect("", a.enrollTOTP))
	mux.HandleFunc("POST /api/v1/auth/totp/confirm", a.protect("", a.confirmTOTP))
	mux.HandleFunc("GET /api/v1/overview", a.protect("overview.read", a.getOverview))
	mux.HandleFunc("POST /api/v1/security/evaluate", a.protect("hosts.execute.safe", a.evaluatePolicy))
	mux.HandleFunc("GET /api/v1/modules", a.protect("overview.read", a.modules))
	mux.HandleFunc("GET /api/v1/integrations", a.protect("overview.read", a.listIntegrations))
	mux.HandleFunc("POST /api/v1/integrations", a.protect("hosts.execute.safe", a.createIntegration))
	mux.HandleFunc("GET /api/v1/hosts", a.protect("overview.read", a.listHosts))
	mux.HandleFunc("POST /api/v1/hosts", a.protect("hosts.execute.safe", a.createHost))
	mux.HandleFunc("GET /api/v1/audit", a.protect("audit.read", a.listAudit))
	return a.middleware(mux)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "time": time.Now().UTC(), "self_protected": a.cfg.SelfProtected})
}
func (a *App) getOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.overview.Snapshot())
}
func (a *App) modules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []string{"overview", "proxmox", "cloud", "kubernetes", "docker", "github", "tunnels", "terraform", "telemetry", "remote-access", "storage", "settings", "audit"})
}

func (a *App) protect(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, 401, "authentication_required", "Authentication is required.")
			return
		}
		session, err := a.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			a.clearCookie(w)
			writeError(w, 401, "session_expired", "Your session is invalid or expired.")
			return
		}
		if permission != "" && !session.User.Can(permission) {
			_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: r.Method + " " + r.URL.Path, Scope: security.ScopeLocal, ResourceType: "http_route", Risk: security.RiskDangerous, Decision: "denied", Reason: "missing permission: " + permission, RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
			writeError(w, 403, "permission_denied", "You do not have permission for this action.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && !a.auth.VerifyCSRF(session, r.Header.Get("X-CSRF-Token")) {
			writeError(w, 403, "csrf_failed", "The CSRF token is invalid.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, session)))
	}
}

func currentSession(r *http.Request) authdomain.Session {
	session, _ := r.Context().Value(sessionContextKey).(authdomain.Session)
	return session
}

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		ctx := context.WithValue(r.Context(), contextKey("request_id"), id)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if a.cfg.Environment == "development" {
			w.Header().Set("Access-Control-Allow-Origin", a.cfg.PublicURL)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
		a.logger.Info("http request", "request_id", id, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *App) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/api", Expires: expires, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
}
func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/api", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, 400, "invalid_request", "Request body is invalid.")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func newRequestID() string {
	value := make([]byte, 12)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
func requestID(ctx context.Context) string {
	value, _ := ctx.Value(contextKey("request_id")).(string)
	return value
}
func remoteIP(r *http.Request) string {
	value := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if value != "" && net.ParseIP(value) != nil {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}
func isAuthError(err error) bool {
	return errors.Is(err, authdomain.ErrInvalidCredentials) || errors.Is(err, authdomain.ErrUnauthorized)
}
