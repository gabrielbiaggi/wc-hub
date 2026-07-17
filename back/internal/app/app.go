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
	proxmoxadapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	authapp "github.com/webcreations/wc-hub/back/internal/auth/application"
	authdomain "github.com/webcreations/wc-hub/back/internal/auth/domain"
	authrepo "github.com/webcreations/wc-hub/back/internal/auth/repository"
	inventorydomain "github.com/webcreations/wc-hub/back/internal/inventory/domain"
	inventoryrepo "github.com/webcreations/wc-hub/back/internal/inventory/repository"
	jobapp "github.com/webcreations/wc-hub/back/internal/jobs/application"
	jobdomain "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	jobrepo "github.com/webcreations/wc-hub/back/internal/jobs/repository"
	overview "github.com/webcreations/wc-hub/back/internal/overview/application"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
	proxmoxapp "github.com/webcreations/wc-hub/back/internal/proxmox/application"
	proxmoxrepo "github.com/webcreations/wc-hub/back/internal/proxmox/repository"
	schedulerrepo "github.com/webcreations/wc-hub/back/internal/scheduler/repository"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	telemetryapp "github.com/webcreations/wc-hub/back/internal/telemetry/application"
	telemetryrepo "github.com/webcreations/wc-hub/back/internal/telemetry/repository"
	terminalapp "github.com/webcreations/wc-hub/back/internal/terminal/application"
	terminalrepo "github.com/webcreations/wc-hub/back/internal/terminal/repository"
)

type contextKey string

const sessionContextKey contextKey = "session"
const sessionCookie = "wc_hub_session"

type App struct {
	cfg             config.Config
	logger          *slog.Logger
	overview        *overview.Service
	policy          *security.Engine
	db              *pgxpool.Pool
	auth            *authapp.Service
	audit           *auditrepo.Postgres
	inventory       inventorydomain.Repository
	jobs            *jobrepo.Postgres
	proxmox         *proxmoxrepo.Postgres
	proxmoxClient   *proxmoxadapter.Client
	telemetry       *telemetryrepo.Postgres
	terminal        *terminalrepo.Postgres
	terminalGateway *terminalapp.Gateway
	cancelWorkers   context.CancelFunc
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
	application.jobs = jobrepo.NewPostgres(pool)
	application.proxmox = proxmoxrepo.NewPostgres(pool)
	application.telemetry = telemetryrepo.NewPostgres(pool)
	application.terminal = terminalrepo.NewPostgres(pool)
	if cfg.ProxmoxURL != "" {
		client, clientErr := proxmoxadapter.New(cfg.ProxmoxURL, cfg.ProxmoxTokenID, []byte(cfg.ProxmoxSecret), cfg.ProxmoxTLSCA)
		if clientErr != nil {
			logger.Error("Proxmox adapter disabled", "error", clientErr)
		} else {
			application.proxmoxClient = client
			if _, err = pool.Exec(ctx, `UPDATE schedules SET enabled=true,next_run_at=LEAST(next_run_at,now()) WHERE name='proxmox-inventory-sync'`); err != nil {
				logger.Error("enable Proxmox schedule failed", "error", err)
			}
		}
	}
	if gateway, gatewayErr := terminalapp.NewGateway(application.terminal, cfg.SSHPrivateKeyPath, cfg.SSHKnownHostsPath, cfg.PublicURL); gatewayErr != nil {
		logger.Warn("SSH terminal disabled", "error", gatewayErr)
	} else {
		application.terminalGateway = gateway
		gateway.SetAudit(func(ctx context.Context, target terminalrepo.Target, status, reason string) {
			_ = application.audit.Append(ctx, auditrepo.Record{ActorID: target.UserID, Action: "terminal.session." + status, Scope: security.ScopeRemote, ResourceType: "terminal_session", ResourceID: target.SessionID, TargetName: target.HostName, Risk: security.RiskDangerous, Decision: "allowed", Reason: reason})
		})
	}
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	application.cancelWorkers = cancelWorkers
	handlers := []jobdomain.Handler{proxmoxapp.NewSyncHandler(application.proxmoxClient, application.proxmox, cfg.ProxmoxURL), telemetryapp.NewMaintenanceHandler(application.telemetry)}
	runner := jobapp.NewRunner(application.jobs, handlers, logger, cfg.WorkerID, cfg.WorkerCount)
	runner.SetResultHook(func(ctx context.Context, job jobdomain.Job, status string, jobErr error) {
		reason := ""
		if jobErr != nil {
			reason = jobErr.Error()
		}
		_ = application.audit.Append(ctx, auditrepo.Record{Action: "job." + status, Scope: security.ScopeRemote, ResourceType: "job", ResourceID: job.ID, TargetName: job.Kind, Risk: security.RiskDangerous, Decision: map[string]string{"succeeded": "allowed", "failed": "denied"}[status], Reason: reason})
	})
	runner.Start(workerCtx)
	schedulerrepo.NewPostgres(pool, application.jobs).Start(workerCtx)
	return application, func() { cancelWorkers(); pool.Close() }, nil
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
	mux.HandleFunc("GET /api/v1/admin/users", a.protect("", a.adminOnly(a.listAdminUsers)))
	mux.HandleFunc("POST /api/v1/admin/users", a.protect("", a.adminOnly(a.createAdminUser)))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", a.protect("", a.adminOnly(a.updateAdminUser)))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", a.protect("", a.adminOnly(a.disableAdminUser)))
	mux.HandleFunc("GET /api/v1/admin/roles", a.protect("", a.adminOnly(a.listAdminRoles)))
	mux.HandleFunc("POST /api/v1/admin/roles", a.protect("", a.adminOnly(a.createAdminRole)))
	mux.HandleFunc("PATCH /api/v1/admin/roles/{id}", a.protect("", a.adminOnly(a.updateAdminRole)))
	mux.HandleFunc("DELETE /api/v1/admin/roles/{id}", a.protect("", a.adminOnly(a.deleteAdminRole)))
	mux.HandleFunc("GET /api/v1/admin/permissions", a.protect("", a.adminOnly(a.listAdminPermissions)))
	mux.HandleFunc("GET /api/v1/alerts", a.protect("overview.read", a.listAlerts))
	mux.HandleFunc("PATCH /api/v1/alerts/{id}", a.protect("hosts.execute.safe", a.updateAlert))
	mux.HandleFunc("GET /api/v1/proxmox/summary", a.protect("proxmox.read", a.proxmoxSummary))
	mux.HandleFunc("POST /api/v1/proxmox/sync", a.protect("proxmox.sync", a.proxmoxSync))
	mux.HandleFunc("GET /api/v1/jobs", a.protect("jobs.read", a.listJobs))
	mux.HandleFunc("POST /api/v1/jobs", a.protect("jobs.manage", a.createJob))
	mux.HandleFunc("POST /api/v1/agents/hosts/{host_id}/token", a.protect("agents.manage", a.provisionAgentToken))
	mux.HandleFunc("POST /agent/v1/metrics", a.ingestAgentMetrics)
	mux.HandleFunc("POST /agent/v1/events", a.ingestAgentEvent)
	mux.HandleFunc("GET /api/v1/telemetry/hosts", a.protect("telemetry.read", a.hostTelemetry))
	mux.HandleFunc("POST /api/v1/terminal/tickets", a.protect("terminal.connect", a.createTerminalTicket))
	mux.HandleFunc("GET /api/v1/terminal/sessions", a.protect("audit.read", a.terminalSessions))
	mux.HandleFunc("GET /ws/terminal", a.terminalWebSocket)
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
