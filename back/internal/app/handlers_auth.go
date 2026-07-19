package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	authapp "github.com/webcreations/wc-hub/back/internal/auth/application"
	authdomain "github.com/webcreations/wc-hub/back/internal/auth/domain"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

type authRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}
type totpRequest struct {
	Code string `json:"code"`
}

func (a *App) bootstrapStatus(w http.ResponseWriter, r *http.Request) {
	if a.cfg.DevelopmentMasterLogin {
		writeJSON(w, http.StatusOK, map[string]bool{"bootstrap_required": false})
		return
	}
	open, err := a.auth.BootstrapOpen(r.Context())
	if err != nil {
		a.logger.Error("bootstrap status failed", "error", err)
		writeError(w, 500, "internal_error", "Could not check bootstrap status.")
		return
	}
	writeJSON(w, 200, map[string]bool{"bootstrap_required": open})
}
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request) {
	if a.cfg.DevelopmentMasterLogin {
		writeError(w, http.StatusConflict, "bootstrap_disabled", "Bootstrap is disabled while the development master login is enabled.")
		return
	}
	var req authRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	tokens, err := a.auth.Bootstrap(r.Context(), authdomain.Credentials{Email: req.Email, DisplayName: req.DisplayName, Password: req.Password}, r.UserAgent(), remoteIP(r))
	if err != nil {
		status := 400
		code := "bootstrap_failed"
		if errors.Is(err, authdomain.ErrBootstrapClosed) {
			status = 409
			code = "bootstrap_closed"
		}
		writeError(w, status, code, err.Error())
		return
	}
	a.setCookie(w, tokens.Session, tokens.Expires)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: tokens.User.ID, Action: "auth.bootstrap", Scope: security.ScopeLocal, ResourceType: "user", ResourceID: tokens.User.ID, TargetName: tokens.User.Email, Risk: security.RiskCritical, Decision: "allowed", Reason: "first administrator created", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 201, tokens)
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	identity := strings.ToLower(strings.TrimSpace(req.Email))
	limiterKeys := []string{"ip:" + remoteIP(r), "account:" + identity}
	if allowed, retry := a.loginLimiter.allow(limiterKeys, time.Now()); !allowed {
		seconds := int(retry.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		_ = a.audit.Append(r.Context(), auditrepo.Record{Action: "auth.login.throttled", Scope: security.ScopeLocal, ResourceType: "session", TargetName: identity, Risk: security.RiskCritical, Decision: "denied", Reason: "progressive login backoff", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
		writeError(w, http.StatusTooManyRequests, "login_throttled", "Too many failed login attempts. Try again later.")
		return
	}
	var tokens authapp.Tokens
	var err error
	developmentMaster := a.cfg.DevelopmentMasterLogin && strings.EqualFold(strings.TrimSpace(req.Email), authdomain.DevelopmentMasterUsername)
	if developmentMaster {
		tokens, err = a.auth.LoginDevelopmentMaster(r.Context(), req.Email, req.Password, r.UserAgent(), remoteIP(r), time.Now(), a.developmentMasterLocation)
	} else {
		tokens, err = a.auth.Login(r.Context(), req.Email, req.Password, r.UserAgent(), remoteIP(r))
	}
	if err != nil {
		a.loginLimiter.failure(limiterKeys, time.Now())
		action := "auth.login"
		risk := security.RiskDangerous
		if developmentMaster {
			action = "auth.development_master.login"
			risk = security.RiskCritical
		}
		_ = a.audit.Append(r.Context(), auditrepo.Record{Action: action, Scope: security.ScopeLocal, ResourceType: "session", TargetName: req.Email, Risk: risk, Decision: "denied", Reason: "invalid credentials", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
		writeError(w, 401, "invalid_credentials", "Email or password is invalid.")
		return
	}
	a.loginLimiter.success(limiterKeys)
	a.setCookie(w, tokens.Session, tokens.Expires)
	action := "auth.login"
	risk := security.RiskSafe
	reason := ""
	if developmentMaster {
		action = "auth.development_master.login"
		risk = security.RiskCritical
		reason = "development-only hourly master credential"
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: tokens.User.ID, Action: action, Scope: security.ScopeLocal, ResourceType: "session", TargetName: tokens.User.Email, Risk: risk, Decision: "allowed", Reason: reason, RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 200, tokens)
}
func (a *App) session(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	csrf, err := a.auth.RefreshCSRF(r.Context(), session.ID)
	if err != nil {
		writeError(w, 500, "internal_error", "Could not refresh session token.")
		return
	}
	writeJSON(w, 200, map[string]any{"user": session.User, "expires_at": session.Expires, "csrf_token": csrf})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		_ = a.auth.Logout(r.Context(), cookie.Value)
	}
	a.clearCookie(w)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "auth.logout", Scope: security.ScopeLocal, ResourceType: "session", ResourceID: session.ID, Risk: security.RiskSafe, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	w.WriteHeader(204)
}
func (a *App) enrollTOTP(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	secret, uri, err := a.auth.EnrollTOTP(r.Context(), session.User.ID, session.User.Email)
	if err != nil {
		writeError(w, 503, "totp_unavailable", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "auth.totp.enroll", Scope: security.ScopeLocal, ResourceType: "user", ResourceID: session.User.ID, Risk: security.RiskCritical, Decision: "allowed", Reason: "encrypted pending secret created", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 200, map[string]string{"secret": secret, "otpauth_uri": uri})
}
func (a *App) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	var req totpRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	session := currentSession(r)
	if err := a.auth.ConfirmTOTP(r.Context(), session.User.ID, req.Code); err != nil {
		writeError(w, 400, "totp_invalid", "The verification code is invalid.")
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "auth.totp.enable", Scope: security.ScopeLocal, ResourceType: "user", ResourceID: session.User.ID, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 200, map[string]bool{"totp_enabled": true})
}
