package vncapp

import (
	"encoding/json"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/vnc"
	"net/http"
	"time"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type AuditFunc func(*http.Request, string, string)
type Handler struct {
	gateway *adapter.Gateway
	audit   AuditFunc
}

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, gateway *adapter.Gateway, audit AuditFunc) {
	h := &Handler{gateway, audit}
	mux.HandleFunc("GET /api/v1/vnc/targets", auth("vnc.read", h.targets))
	mux.HandleFunc("GET /ws/vnc/{target}", auth("vnc.connect", h.connect))
}
func (h *Handler) targets(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		writeError(w, 503, "vnc_unconfigured", "O gateway VNC não está configurado.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": h.gateway.Targets()})
}
func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		writeError(w, 503, "vnc_unconfigured", "O gateway VNC não está configurado.")
		return
	}
	target := r.PathValue("target")
	if h.audit != nil {
		h.audit(r, "vnc.connect", target)
	}
	if err := h.gateway.Serve(w, r, target); err != nil && r.Context().Err() == nil {
		if h.audit != nil {
			h.audit(r, "vnc.connect.failed", target)
		}
	}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}, "at": time.Now().UTC()})
}
