package app

import (
	proxmox "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
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
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) proxmoxDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}
	if e := c.DeleteGuestSnapshot(r.Context(), n, k, id, r.PathValue("name")); e != nil {
		writeError(w, 502, "proxmox_snapshot_delete_failed", e.Error())
		return
	}
	w.WriteHeader(204)
}
func (a *App) proxmoxRollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	c, n, k, id, ok := a.snapshotTarget(r)
	if !ok {
		writeError(w, 400, "invalid_snapshot_target", "Destino inválido.")
		return
	}
	if e := c.RollbackGuestSnapshot(r.Context(), n, k, id, r.PathValue("name")); e != nil {
		writeError(w, 502, "proxmox_snapshot_rollback_failed", e.Error())
		return
	}
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
	if e := c.MigrateGuest(r.Context(), n, k, id, in.TargetNode, in.Online); e != nil {
		writeError(w, 502, "proxmox_migration_failed", e.Error())
		return
	}
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
	if e := c.ResizeDisk(r.Context(), n, k, id, in.Disk, in.Size); e != nil {
		writeError(w, 502, "proxmox_resize_failed", e.Error())
		return
	}
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
	c := a.proxmoxClient(cluster)
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
