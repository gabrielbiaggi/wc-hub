package app

import (
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"

	adminrepo "github.com/webcreations/wc-hub/back/internal/admin/repository"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
)

var roleSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,47}$`)

func (a *App) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := currentSession(r)
		for _, role := range session.User.Roles {
			if role == "god-admin" {
				next(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "admin_required", "God administrator role is required.")
	}
}

func (a *App) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := adminrepo.NewPostgres(a.db).ListUsers(r.Context())
	if err != nil {
		a.adminFailure(w, "list users", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) createAdminUser(w http.ResponseWriter, r *http.Request) {
	if a.cfg.DevelopmentMasterLogin {
		writeError(w, http.StatusConflict, "single_user_mode", "WC Hub is configured for the allmight single-user identity.")
		return
	}
	var req struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Password    string   `json:"password"`
		RoleIDs     []string `json:"role_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil || len(strings.TrimSpace(req.DisplayName)) < 2 || len(req.Password) < 12 {
		writeError(w, 400, "invalid_user", "Use a valid email, a display name, and a password with at least 12 characters.")
		return
	}
	item, err := adminrepo.NewPostgres(a.db).CreateUser(r.Context(), req.Email, req.DisplayName, req.Password, req.RoleIDs)
	if err != nil {
		a.adminFailure(w, "create user", err)
		return
	}
	a.auditAdminMutation(r, "admin.user.create", "user", item.ID, item.Email, security.RiskDangerous)
	writeJSON(w, http.StatusCreated, item)
}

func (a *App) updateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Disabled    bool     `json:"disabled"`
		RoleIDs     []string `json:"role_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil || len(strings.TrimSpace(req.DisplayName)) < 2 {
		writeError(w, 400, "invalid_user", "Use a valid email and display name.")
		return
	}
	id := r.PathValue("id")
	session := currentSession(r)
	if a.cfg.DevelopmentMasterLogin && id != session.User.ID {
		writeError(w, http.StatusConflict, "single_user_mode", "Only the allmight identity may exist in single-user mode.")
		return
	}
	if a.cfg.DevelopmentMasterLogin && !strings.EqualFold(strings.TrimSpace(req.Email), a.cfg.DevelopmentMasterEmail) {
		writeError(w, http.StatusConflict, "master_identity_locked", "The allmight email is managed by WC_HUB_MASTER_EMAIL.")
		return
	}
	if id == session.User.ID && req.Disabled {
		writeError(w, 409, "self_lockout", "You cannot disable your own account.")
		return
	}
	repo := adminrepo.NewPostgres(a.db)
	if id == session.User.ID {
		roles, err := repo.ListRoles(r.Context())
		if err != nil {
			a.adminFailure(w, "validate user roles", err)
			return
		}
		keepsAdmin := false
		for _, role := range roles {
			if role.Slug != "god-admin" {
				continue
			}
			for _, selected := range req.RoleIDs {
				if selected == role.ID {
					keepsAdmin = true
				}
			}
		}
		if !keepsAdmin {
			writeError(w, 409, "self_lockout", "You cannot remove your own administrator role.")
			return
		}
	}
	item, err := repo.UpdateUser(r.Context(), id, req.Email, req.DisplayName, req.Disabled, req.RoleIDs)
	if err != nil {
		a.adminFailure(w, "update user", err)
		return
	}
	a.auditAdminMutation(r, "admin.user.update", "user", item.ID, item.Email, security.RiskDangerous)
	writeJSON(w, 200, item)
}

func (a *App) disableAdminUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == currentSession(r).User.ID {
		writeError(w, 409, "self_lockout", "You cannot disable your own account.")
		return
	}
	if err := adminrepo.NewPostgres(a.db).DisableUser(r.Context(), id); err != nil {
		a.adminFailure(w, "disable user", err)
		return
	}
	a.auditAdminMutation(r, "admin.user.disable", "user", id, "", security.RiskCritical)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) listAdminRoles(w http.ResponseWriter, r *http.Request) {
	items, err := adminrepo.NewPostgres(a.db).ListRoles(r.Context())
	if err != nil {
		a.adminFailure(w, "list roles", err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) createAdminRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug          string   `json:"slug"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !roleSlugPattern.MatchString(req.Slug) || len(strings.TrimSpace(req.Name)) < 2 {
		writeError(w, 400, "invalid_role", "Use a valid slug and role name.")
		return
	}
	repo := adminrepo.NewPostgres(a.db)
	item, err := repo.CreateRole(r.Context(), req.Slug, req.Name, req.Description)
	if err == nil && len(req.PermissionIDs) > 0 {
		item, err = repo.UpdateRole(r.Context(), item.ID, item.Name, item.Description, req.PermissionIDs)
	}
	if err != nil {
		a.adminFailure(w, "create role", err)
		return
	}
	a.auditAdminMutation(r, "admin.role.create", "role", item.ID, item.Slug, security.RiskDangerous)
	writeJSON(w, 201, item)
}
func (a *App) updateAdminRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		PermissionIDs []string `json:"permission_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(strings.TrimSpace(req.Name)) < 2 {
		writeError(w, 400, "invalid_role", "Role name is required.")
		return
	}
	item, err := adminrepo.NewPostgres(a.db).UpdateRole(r.Context(), r.PathValue("id"), req.Name, req.Description, req.PermissionIDs)
	if err != nil {
		a.adminFailure(w, "update role", err)
		return
	}
	a.auditAdminMutation(r, "admin.role.update", "role", item.ID, item.Slug, security.RiskDangerous)
	writeJSON(w, 200, item)
}
func (a *App) deleteAdminRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := adminrepo.NewPostgres(a.db).DeleteRole(r.Context(), id); err != nil {
		a.adminFailure(w, "delete role", err)
		return
	}
	a.auditAdminMutation(r, "admin.role.delete", "role", id, "", security.RiskCritical)
	w.WriteHeader(204)
}
func (a *App) listAdminPermissions(w http.ResponseWriter, r *http.Request) {
	items, err := adminrepo.NewPostgres(a.db).ListPermissions(r.Context())
	if err != nil {
		a.adminFailure(w, "list permissions", err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) listAlerts(w http.ResponseWriter, r *http.Request) {
	items, err := adminrepo.NewPostgres(a.db).ListAlerts(r.Context())
	if err != nil {
		a.adminFailure(w, "list alerts", err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) updateAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Status != "acknowledged" && req.Status != "resolved" && req.Status != "open" {
		writeError(w, 400, "invalid_status", "Status must be open, acknowledged, or resolved.")
		return
	}
	session := currentSession(r)
	item, err := adminrepo.NewPostgres(a.db).UpdateAlert(r.Context(), r.PathValue("id"), req.Status, session.User.ID)
	if err != nil {
		a.adminFailure(w, "update alert", err)
		return
	}
	a.auditAdminMutation(r, "alert."+req.Status, "alert", item.ID, item.Title, security.RiskDangerous)
	writeJSON(w, 200, item)
}

func (a *App) auditAdminMutation(r *http.Request, action, resourceType, resourceID, target string, risk security.Risk) {
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: action, Scope: security.ScopeLocal, ResourceType: resourceType, ResourceID: resourceID, TargetName: target, Risk: risk, Decision: "allowed", Reason: "administrative action", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
}
func (a *App) adminFailure(w http.ResponseWriter, operation string, err error) {
	a.logger.Error(operation+" failed", "error", err)
	switch {
	case errors.Is(err, adminrepo.ErrNotFound):
		writeError(w, 404, "not_found", "Administrative resource was not found.")
	case errors.Is(err, adminrepo.ErrProtectedRole):
		writeError(w, 409, "protected_role", "The built-in administrator role cannot be removed.")
	case errors.Is(err, adminrepo.ErrRoleInUse):
		writeError(w, 409, "role_in_use", "Remove this role from users before deleting it.")
	default:
		writeError(w, 500, "internal_error", "The administrative operation could not be completed.")
	}
}
