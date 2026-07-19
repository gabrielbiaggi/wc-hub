package kubernetesapp

import (
	"encoding/json"
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	"net/http"
)

type Handler struct {
	client  *kubernetesadapter.Client
	initErr string
}

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", h.unconfiguredMessage())
		return
	}
	overview, err := h.client.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "kubernetes_unavailable", "O inventário Kubernetes não pôde ser carregado: "+err.Error())
		return
	}
	writeJSON(w, 200, overview)
}

func (h *Handler) PodLogs(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", h.unconfiguredMessage())
		return
	}
	logs, err := h.client.PodLogs(r.Context(), r.PathValue("namespace"), r.PathValue("pod"), r.URL.Query().Get("container"), 500)
	if err != nil {
		writeError(w, 502, "kubernetes_logs_failed", "Não foi possível carregar os logs do pod: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"output": logs})
}

func (h *Handler) PodExec(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", h.unconfiguredMessage())
		return
	}
	var input struct {
		Container string   `json:"container"`
		Command   []string `json:"command"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "O comando é inválido.")
		return
	}
	output, err := h.client.Exec(r.Context(), r.PathValue("namespace"), r.PathValue("pod"), input.Container, input.Command)
	if err != nil {
		writeError(w, 502, "kubernetes_exec_failed", "O Kubernetes rejeitou a execução no pod: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"output": output})
}

func (h *Handler) DeploymentAction(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "kubernetes_unconfigured", h.unconfiguredMessage())
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
		writeError(w, 502, "kubernetes_action_failed", "O Kubernetes rejeitou a ação no deployment: "+err.Error())
		return
	}
	writeJSON(w, 202, map[string]any{"namespace": r.PathValue("namespace"), "name": r.PathValue("name"), "action": r.PathValue("action"), "status": "accepted"})
}
func (h *Handler) unconfiguredMessage() string {
	if h.initErr != "" {
		return "Kubernetes não configurado: " + h.initErr
	}
	return "O Kubernetes não está configurado."
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
