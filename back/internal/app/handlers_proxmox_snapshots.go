package app

import (
	proxmox "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	"net/http"
	"strconv"
)

func (a *App) snapshotTarget(r *http.Request) (*proxmox.Client, string, string, int, bool) {
	c := a.proxmoxClientFor(r.URL.Query().Get("cluster"))
	id, e := strconv.Atoi(r.PathValue("vmid"))
	if c == nil || e != nil {
		return nil, "", "", 0, false
	}
	return c, r.PathValue("node"), r.PathValue("kind"), id, true
}
func (a *App) proxmoxSnapshots(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}
	items, e := c.ListGuestSnapshots(r.Context(), n, k, id)
	if e != nil {
		writeError(w, 502, "proxmox_snapshot_failed", e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) proxmoxCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if e := c.CreateGuestSnapshot(r.Context(), n, k, id, in.Name, in.Description); e != nil {
		writeError(w, 502, "proxmox_snapshot_create_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox." + k + ".snapshot.create",
		Scope:        security.ScopeRemote,
		ResourceType: "snapshot",
		ResourceID:   in.Name,
		TargetName:   c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id),
		Risk:         security.RiskDangerous,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
	})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) proxmoxDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}

	// Self-protection: snapshots são críticos para recovery
	snapshotName := r.PathValue("name")
	targetName := c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id) + "/snapshot/" + snapshotName
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(id, "")
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "delete_snapshot",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}

	if e := c.DeleteGuestSnapshot(r.Context(), n, k, id, snapshotName); e != nil {
		writeError(w, 502, "proxmox_snapshot_delete_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox." + k + ".snapshot.delete",
		Scope:        security.ScopeRemote,
		ResourceType: "snapshot",
		ResourceID:   snapshotName,
		TargetName:   targetName,
		Risk:         security.RiskCritical,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
	})
	w.WriteHeader(204)
}
func (a *App) proxmoxRollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}

	// Self-protection: rollback pode causar perda de dados
	snapshotName := r.PathValue("name")
	targetName := c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id) + "/snapshot/" + snapshotName
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(id, "")
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "rollback_snapshot",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}

	if e := c.RollbackGuestSnapshot(r.Context(), n, k, id, snapshotName); e != nil {
		writeError(w, 502, "proxmox_snapshot_rollback_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox." + k + ".snapshot.rollback",
		Scope:        security.ScopeRemote,
		ResourceType: "snapshot",
		ResourceID:   snapshotName,
		TargetName:   targetName,
		Risk:         security.RiskCritical,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
	})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) proxmoxMigrateGuest(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_migration_target", "Destino inválido.")
		return
	}
	var in struct {
		TargetNode string `json:"target_node"`
		Online     bool   `json:"online"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	targetName := c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id)
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(id, targetName)
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "proxmox_migrate",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}
	if e := c.MigrateGuest(r.Context(), n, k, id, in.TargetNode, in.Online); e != nil {
		writeError(w, 502, "proxmox_migration_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox." + k + ".migrate",
		Scope:        security.ScopeRemote,
		ResourceType: k,
		ResourceID:   strconv.Itoa(id),
		TargetName:   c.ID() + "/" + n + " → " + in.TargetNode,
		Risk:         security.RiskDangerous,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
		Payload:      map[string]any{"online": in.Online},
	})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) proxmoxResizeDisk(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_resize_target", "Destino inválido.")
		return
	}
	var in struct {
		Disk string `json:"disk"`
		Size string `json:"size"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	targetName := c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id)
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(id, targetName)
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "proxmox_resize",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}
	if e := c.ResizeDisk(r.Context(), n, k, id, in.Disk, in.Size); e != nil {
		writeError(w, 502, "proxmox_resize_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox." + k + ".resize",
		Scope:        security.ScopeRemote,
		ResourceType: k,
		ResourceID:   strconv.Itoa(id),
		TargetName:   c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id),
		Risk:         security.RiskDangerous,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
		Payload:      map[string]any{"disk": in.Disk, "size": in.Size},
	})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) proxmoxUpdateConfig(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_config_target", "Destino inválido.")
		return
	}
	var in struct {
		Config map[string]string `json:"config"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Config) == 0 {
		writeError(w, 400, "empty_config", "Configuração vazia.")
		return
	}
	targetName := c.ID() + "/" + n + "/" + k + "/" + strconv.Itoa(id)
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(id, targetName)
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "proxmox_config_update",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}
	if e := c.UpdateGuestConfig(r.Context(), n, k, id, in.Config); e != nil {
		writeError(w, 502, "proxmox_config_update_failed", e.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (a *App) proxmoxNodeNetwork(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		writeError(w, 400, "missing_cluster", "Parâmetro cluster obrigatório.")
		return
	}
	c := a.proxmoxClientFor(cluster)
	if c == nil {
		writeError(w, 404, "cluster_not_found", "Cluster não encontrado.")
		return
	}
	node := r.PathValue("node")
	if node == "" {
		writeError(w, 400, "missing_node", "Node obrigatório.")
		return
	}
	interfaces, e := c.GetNodeNetworkConfig(r.Context(), node)
	if e != nil {
		writeError(w, 502, "proxmox_network_failed", e.Error())
		return
	}
	writeJSON(w, 200, interfaces)
}

func (a *App) proxmoxGuestFirewallRules(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_firewall_target", "Destino inválido.")
		return
	}
	rules, e := c.GetGuestFirewallRules(r.Context(), n, k, id)
	if e != nil {
		writeError(w, 502, "proxmox_firewall_read_failed", e.Error())
		return
	}
	writeJSON(w, 200, rules)
}

func (a *App) proxmoxCreateFirewallRule(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_firewall_target", "Destino inválido.")
		return
	}
	var in struct {
		Rule map[string]string `json:"rule"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Rule) == 0 {
		writeError(w, 400, "empty_rule", "Regra vazia.")
		return
	}
	if e := c.CreateGuestFirewallRule(r.Context(), n, k, id, in.Rule); e != nil {
		writeError(w, 502, "proxmox_firewall_create_failed", e.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created"})
}

func (a *App) proxmoxDeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_firewall_target", "Destino inválido.")
		return
	}
	posStr := r.PathValue("pos")
	pos, err := strconv.Atoi(posStr)
	if err != nil || pos < 0 {
		writeError(w, 400, "invalid_position", "Posição inválida.")
		return
	}
	if e := c.DeleteGuestFirewallRule(r.Context(), n, k, id, pos); e != nil {
		writeError(w, 502, "proxmox_firewall_delete_failed", e.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (a *App) proxmoxNodeBackups(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	storage := r.URL.Query().Get("storage")
	if cluster == "" || storage == "" {
		writeError(w, 400, "missing_parameters", "Parâmetros cluster e storage obrigatórios.")
		return
	}
	c := a.proxmoxClientFor(cluster)
	if c == nil {
		writeError(w, 404, "cluster_not_found", "Cluster não encontrado.")
		return
	}
	node := r.PathValue("node")
	if node == "" {
		writeError(w, 400, "missing_node", "Node obrigatório.")
		return
	}
	backups, e := c.GetNodeBackups(r.Context(), node, storage)
	if e != nil {
		writeError(w, 502, "proxmox_backup_list_failed", e.Error())
		return
	}
	writeJSON(w, 200, backups)
}

func (a *App) proxmoxCreateBackup(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_backup_target", "Destino inválido.")
		return
	}
	var in struct {
		Storage  string `json:"storage"`
		Mode     string `json:"mode"`
		Compress string `json:"compress"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Storage == "" {
		writeError(w, 400, "missing_storage", "Storage obrigatório.")
		return
	}
	if in.Mode == "" {
		in.Mode = "snapshot"
	}
	if e := c.CreateGuestBackup(r.Context(), n, k, id, in.Storage, in.Mode, in.Compress); e != nil {
		writeError(w, 502, "proxmox_backup_create_failed", e.Error())
		return
	}
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}

func (a *App) proxmoxRestoreBackup(w http.ResponseWriter, r *http.Request) {
	node := r.PathValue("node")
	if node == "" {
		writeError(w, 400, "invalid_node", "Node inválido.")
		return
	}
	c := a.proxmoxClient
	if c == nil {
		writeError(w, 503, "proxmox_unconfigured", "Proxmox não configurado.")
		return
	}
	var in struct {
		Storage string `json:"storage"`
		Archive string `json:"archive"`
		VMID    int    `json:"vmid"`
		Force   bool   `json:"force"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Storage == "" || in.Archive == "" || in.VMID <= 0 {
		writeError(w, 400, "missing_params", "Storage, Archive e VMID são obrigatórios.")
		return
	}
	if e := c.RestoreGuestBackup(r.Context(), node, in.Storage, in.Archive, in.VMID, in.Force); e != nil {
		writeError(w, 502, "proxmox_restore_failed", e.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{
		ActorID:      session.User.ID,
		Action:       "proxmox.backup.restore",
		Scope:        security.ScopeRemote,
		ResourceType: "virtual_machine",
		ResourceID:   strconv.Itoa(in.VMID),
		TargetName:   c.ID() + "/" + node + "/" + strconv.Itoa(in.VMID),
		Risk:         security.RiskCritical,
		Decision:     "allowed",
		RequestID:    requestID(r.Context()),
		SourceIP:     remoteIP(r),
	})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
