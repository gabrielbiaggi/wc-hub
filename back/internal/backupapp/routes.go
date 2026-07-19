package backupapp

import (
	"context"
	"encoding/json"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/pbs"
	"net/http"
)

type Reader interface {
	Overview(context.Context) (adapter.Overview, error)
}
type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type Handler struct{ client Reader }

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client Reader) {
	h := &Handler{client}
	mux.HandleFunc("GET /api/v1/backups/overview", auth("backup.read", h.overview))
}
func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "pbs_unconfigured", "O Proxmox Backup Server não está configurado.")
		return
	}
	result, err := h.client.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "pbs_unavailable", "O Proxmox Backup Server não respondeu.")
		return
	}
	writeJSON(w, 200, result)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
