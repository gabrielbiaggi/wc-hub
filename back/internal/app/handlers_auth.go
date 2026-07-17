package app

import (
	"errors"
	"net/http"

	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
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
	open, err := a.auth.BootstrapOpen(r.Context())
	if err != nil {
		a.logger.Error("bootstrap status failed", "error", err)
		writeError(w, 500, "internal_error", "Could not check bootstrap status.")
		return
	}
	writeJSON(w, 200, map[string]bool{"bootstrap_required": open})
}
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request) {
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
	tokens, err := a.auth.Login(r.Context(), req.Email, req.Password, r.UserAgent(), remoteIP(r))
	if err != nil {
		_ = a.audit.Append(r.Context(), auditrepo.Record{Action: "auth.login", Scope: security.ScopeLocal, ResourceType: "session", TargetName: req.Email, Risk: security.RiskDangerous, Decision: "denied", Reason: "invalid credentials", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
		writeError(w, 401, "invalid_credentials", "Email or password is invalid.")
		return
	}
	a.setCookie(w, tokens.Session, tokens.Expires)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: tokens.User.ID, Action: "auth.login", Scope: security.ScopeLocal, ResourceType: "session", TargetName: tokens.User.Email, Risk: security.RiskSafe, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
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
