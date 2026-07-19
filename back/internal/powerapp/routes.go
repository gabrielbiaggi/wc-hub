package powerapp

import (
	"context"
	"encoding/json"
	"net/http"

	adapter "github.com/webcreations/wc-hub/back/internal/adapters/power"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type AuditFunc func(context.Context, string, string, string)

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *adapter.Client, audit AuditFunc) {
	handler := &handler{client: client, audit: audit}
	mux.HandleFunc("GET /api/v1/power/status", auth("power.read", handler.status))
	mux.HandleFunc("GET /api/v1/power/targets", auth("power.read", handler.targets))
	mux.HandleFunc("POST /api/v1/power/wake/{target}", auth("power.manage", handler.wake))
}

type handler struct {
	client *adapter.Client
	audit  AuditFunc
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, http.StatusServiceUnavailable, "power_unconfigured")
		return
	}
	writeJSON(w, http.StatusOK, h.client.Status(r.Context()))
}
func (h *handler) targets(w http.ResponseWriter, _ *http.Request) {
	if h.client == nil {
		writeError(w, http.StatusServiceUnavailable, "power_unconfigured")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": h.client.Targets()})
}
func (h *handler) wake(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, http.StatusServiceUnavailable, "power_unconfigured")
		return
	}
	target, err := h.client.Wake(r.Context(), r.PathValue("target"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "wake_rejected")
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), "power.wake", target.ID, target.MAC)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"target": target.ID, "status": "magic_packet_sent"})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}
