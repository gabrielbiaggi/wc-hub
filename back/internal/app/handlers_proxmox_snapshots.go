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
