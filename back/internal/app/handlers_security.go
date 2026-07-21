package app

import (
	"net/http"

	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	dockerapp "github.com/webcreations/wc-hub/back/internal/dockerapp"
	kubernetesapp "github.com/webcreations/wc-hub/back/internal/kubernetesapp"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	terraformapp "github.com/webcreations/wc-hub/back/internal/terraformapp"
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

// enforcePolicy avalia uma ação crítica usando o security engine e registra no audit log.
// Retorna true se permitido, false se bloqueado (já escreve resposta 403).
// Uso: if !a.enforcePolicy(w, r, security.ActionRequest{...}) { return }
func (a *App) enforcePolicy(w http.ResponseWriter, r *http.Request, req security.ActionRequest) bool {
	session := currentSession(r)
	if session.User.TOTPEnabled {
		verified, _ := a.auth.VerifyTOTP(r.Context(), session.User.ID, req.TOTPCode)
		req.TOTPVerified = verified
	}
	decision := a.policy.Evaluate(req)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       req.Action,
		Scope:        req.Scope,
		ResourceType: "action_guard",
		TargetName:   req.TargetName,
		Risk:         decision.Risk,
		Decision:     map[bool]string{true: "allowed", false: "denied"}[decision.Allowed],
		Reason:       decision.Reason,
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
	})
	if !decision.Allowed {
		writeJSON(w, 403, decision)
		return false
	}
	return true
}

// policyEnforcerForPlugin adapts enforcePolicy for use in plugins
func (a *App) policyEnforcerForPlugin(w http.ResponseWriter, r *http.Request, req dockerapp.PolicyRequest) bool {
	isSelf := false
	if a.targetResolver != nil {
		isSelf = a.targetResolver.IsSelfProtectedContainer(req.TargetName)
	}
	return a.enforcePolicy(w, r, security.ActionRequest{
		Action:              req.Action,
		Scope:               security.Scope(req.Scope),
		TargetName:          req.TargetName,
		TargetSelfProtected: isSelf,
		Confirmation:        req.Confirmation,
		TOTPCode:            req.TOTPCode,
	})
}

func (a *App) policyEnforcerForK8s(w http.ResponseWriter, r *http.Request, req kubernetesapp.PolicyRequest) bool {
	isSelf := false
	if a.targetResolver != nil {
		isSelf = a.targetResolver.IsSelfProtectedPod(req.TargetName)
	}
	return a.enforcePolicy(w, r, security.ActionRequest{
		Action:              req.Action,
		Scope:               security.Scope(req.Scope),
		TargetName:          req.TargetName,
		TargetSelfProtected: isSelf,
		Confirmation:        req.Confirmation,
		TOTPCode:            req.TOTPCode,
	})
}

func (a *App) policyEnforcerForTerraform(w http.ResponseWriter, r *http.Request, req terraformapp.PolicyRequest) bool {
	isSelf := false
	if a.targetResolver != nil {
		isSelf = a.targetResolver.IsSelfProtectedWorkspace(req.TargetName)
	}
	return a.enforcePolicy(w, r, security.ActionRequest{
		Action:              req.Action,
		Scope:               security.Scope(req.Scope),
		TargetName:          req.TargetName,
		TargetSelfProtected: isSelf,
		Confirmation:        req.Confirmation,
		TOTPCode:            req.TOTPCode,
	})
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
