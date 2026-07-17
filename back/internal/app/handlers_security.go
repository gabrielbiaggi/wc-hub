package app

import (
	"net/http"

	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

func (a *App) evaluatePolicy(w http.ResponseWriter, r *http.Request) {
	var req security.ActionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	session := currentSession(r)
	if session.User.TOTPEnabled {
		verified, _ := a.auth.VerifyTOTP(r.Context(), session.User.ID, req.TOTPCode)
		req.TOTPVerified = verified
	}
	decision := a.policy.Evaluate(req)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: req.Action, Scope: req.Scope, ResourceType: "policy_evaluation", TargetName: req.TargetName, Risk: decision.Risk, Decision: map[bool]string{true: "allowed", false: "denied"}[decision.Allowed], Reason: decision.Reason, RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	status := 200
	if !decision.Allowed {
		status = 403
	}
	writeJSON(w, status, decision)
}
func (a *App) listAudit(w http.ResponseWriter, r *http.Request) {
	items, err := a.audit.List(r.Context(), 100)
	if err != nil {
		a.logger.Error("list audit failed", "error", err)
		writeError(w, 500, "internal_error", "Could not load audit log.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
