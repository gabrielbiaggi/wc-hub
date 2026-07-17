package kubernetesapp

import (
	"encoding/json"
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	"net/http"
)

type Handler struct{ client *kubernetesadapter.Client }

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", "Kubernetes service account access is not configured.")
		return
	}
	overview, err := h.client.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "kubernetes_unavailable", "Kubernetes inventory is temporarily unavailable.")
		return
	}
	writeJSON(w, 200, overview)
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
