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

func (h *Handler) DeploymentAction(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", "Kubernetes access is not configured.")
		return
	}
	request := struct {
		Replicas int `json:"replicas"`
	}{}
	if r.PathValue("action") == "scale" {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, 400, "invalid_request", "Replica count is required.")
			return
		}
	}
	if err := h.client.DeploymentAction(r.Context(), r.PathValue("namespace"), r.PathValue("name"), r.PathValue("action"), request.Replicas); err != nil {
		writeError(w, 502, "kubernetes_action_failed", "Kubernetes rejected the deployment action.")
		return
	}
	writeJSON(w, 202, map[string]any{"namespace": r.PathValue("namespace"), "name": r.PathValue("name"), "action": r.PathValue("action"), "status": "accepted"})
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
