package app

import (
	"net/http"
	"strconv"
	"strings"
)

func (a *App) kubernetesOverview(w http.ResponseWriter, r *http.Request) {
	if a.kubernetes == nil {
		writeError(w, 503, "kubernetes_not_configured", "Kubernetes não está configurado.")
		return
	}
	overview, err := a.kubernetes.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "kubernetes_overview_failed", err.Error())
		return
	}
	writeJSON(w, 200, overview)
}

func (a *App) kubernetesPodLogs(w http.ResponseWriter, r *http.Request) {
	if a.kubernetes == nil {
		writeError(w, 503, "kubernetes_not_configured", "Kubernetes não está configurado.")
		return
	}
	
	namespace := r.PathValue("namespace")
	pod := r.PathValue("pod")
	container := r.URL.Query().Get("container")
	tailStr := r.URL.Query().Get("tail")
	
	if namespace == "" || pod == "" {
		writeError(w, 400, "missing_parameters", "Namespace e pod obrigatórios.")
		return
	}
	
	tail := 100
	if tailStr != "" {
		if parsed, err := strconv.Atoi(tailStr); err == nil && parsed > 0 && parsed <= 10000 {
			tail = parsed
		}
	}
	
	logs, err := a.kubernetes.PodLogs(r.Context(), namespace, pod, container, tail)
	if err != nil {
		writeError(w, 502, "kubernetes_logs_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"logs": logs})
}

func (a *App) kubernetesPodExec(w http.ResponseWriter, r *http.Request) {
	if a.kubernetes == nil {
		writeError(w, 503, "kubernetes_not_configured", "Kubernetes não está configurado.")
		return
	}
	
	namespace := r.PathValue("namespace")
	pod := r.PathValue("pod")
	
	if namespace == "" || pod == "" {
		writeError(w, 400, "missing_parameters", "Namespace e pod obrigatórios.")
		return
	}
	
	var in struct {
		Container string   `json:"container"`
		Command   []string `json:"command"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	
	if len(in.Command) == 0 {
		writeError(w, 400, "empty_command", "Comando vazio.")
		return
	}
	
	output, err := a.kubernetes.Exec(r.Context(), namespace, pod, in.Container, in.Command)
	if err != nil {
		writeError(w, 502, "kubernetes_exec_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"output": output})
}

func (a *App) kubernetesDeploymentAction(w http.ResponseWriter, r *http.Request) {
	if a.kubernetes == nil {
		writeError(w, 503, "kubernetes_not_configured", "Kubernetes não está configurado.")
		return
	}
	
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	action := r.PathValue("action")
	
	if namespace == "" || name == "" || action == "" {
		writeError(w, 400, "missing_parameters", "Namespace, name e action obrigatórios.")
		return
	}
	
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "scale" && action != "restart" {
		writeError(w, 400, "invalid_action", "Action deve ser scale ou restart.")
		return
	}
	
	var in struct {
		Replicas int `json:"replicas"`
	}
	if action == "scale" {
		if !decodeJSON(w, r, &in) {
			return
		}
		if in.Replicas < 0 || in.Replicas > 100 {
			writeError(w, 400, "invalid_replicas", "Replicas deve estar entre 0 e 100.")
			return
		}
	}
	
	if err := a.kubernetes.DeploymentAction(r.Context(), namespace, name, action, in.Replicas); err != nil {
		writeError(w, 502, "kubernetes_action_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "success", "action": action})
}
