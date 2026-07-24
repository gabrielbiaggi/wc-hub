package app

import (
	"net/http"

	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	inventorydomain "github.com/webcreations/wc-hub/back/internal/inventory/domain"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

func (a *App) listIntegrations(w http.ResponseWriter, r *http.Request) {
	a.syncEnvironmentIntegrations(r.Context())
	items, err := a.inventory.ListIntegrations(r.Context())
	if err != nil {
		a.logger.Error("list integrations failed", "error", err)
		writeError(w, 500, "internal_error", "Could not load integrations.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) createIntegration(w http.ResponseWriter, r *http.Request) {
	var item inventorydomain.Integration
	if !decodeJSON(w, r, &item) {
		return
	}
	session := currentSession(r)
	created, err := a.inventory.CreateIntegration(r.Context(), item, session.User.ID)
	if err != nil {
		writeError(w, 400, "integration_invalid", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "integration.create", Scope: security.ScopeCloud, ResourceType: "integration", ResourceID: created.ID, TargetName: created.Name, Risk: security.RiskDangerous, Decision: "allowed", Reason: "integration metadata created; credentials not present", RequestID: requestID(r.Context()), SourceIP: remoteIP(r), Payload: map[string]any{"provider": created.Provider}})
	writeJSON(w, 201, created)
}
func (a *App) listHosts(w http.ResponseWriter, r *http.Request) {
	items, err := a.inventory.ListHosts(r.Context())
	if err != nil {
		a.logger.Error("list hosts failed", "error", err)
		writeError(w, 500, "internal_error", "Could not load hosts.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) createHost(w http.ResponseWriter, r *http.Request) {
	var item inventorydomain.Host
	if !decodeJSON(w, r, &item) {
		return
	}
	session := currentSession(r)
	if item.SelfProtected && item.Name != a.cfg.LocalTargetName {
		writeError(w, 400, "self_protection_mismatch", "Self-protected host name must match WC_HUB_LOCAL_TARGET_NAME.")
		return
	}
	created, err := a.inventory.CreateHost(r.Context(), item, session.User.ID)
	if err != nil {
		writeError(w, 400, "host_invalid", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "host.create", Scope: security.Scope(created.Scope), ResourceType: "host", ResourceID: created.ID, TargetName: created.Name, Risk: security.RiskDangerous, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r), Payload: map[string]any{"hostname": created.Hostname, "self_protected": created.SelfProtected}})
	writeJSON(w, 201, created)
}
