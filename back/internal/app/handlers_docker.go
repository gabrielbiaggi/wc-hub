package app

import (
	"net/http"
	"strings"
)

func (a *App) dockerInventory(w http.ResponseWriter, r *http.Request) {
	if a.docker == nil {
		writeError(w, 503, "docker_not_configured", "Docker não está configurado.")
		return
	}
	inventory, err := a.docker.Inventory(r.Context())
	if err != nil {
		writeError(w, 502, "docker_inventory_failed", err.Error())
		return
	}
	writeJSON(w, 200, inventory)
}

func (a *App) dockerContainerAction(w http.ResponseWriter, r *http.Request) {
	if a.docker == nil {
		writeError(w, 503, "docker_not_configured", "Docker não está configurado.")
		return
	}
	id := r.PathValue("id")
	action := r.PathValue("action")
	
	if id == "" || action == "" {
		writeError(w, 400, "missing_parameters", "Container ID e action obrigatórios.")
		return
	}
	
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "start" && action != "stop" && action != "restart" {
		writeError(w, 400, "invalid_action", "Action deve ser start, stop ou restart.")
		return
	}
	
	if err := a.docker.ContainerAction(r.Context(), id, action); err != nil {
		writeError(w, 502, "docker_action_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "success", "action": action})
}

func (a *App) dockerContainerExec(w http.ResponseWriter, r *http.Request) {
	if a.docker == nil {
		writeError(w, 503, "docker_not_configured", "Docker não está configurado.")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, 400, "missing_id", "Container ID obrigatório.")
		return
	}
	
	var in struct {
		Command []string `json:"command"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	
	if len(in.Command) == 0 {
		writeError(w, 400, "empty_command", "Comando vazio.")
		return
	}
	
	output, err := a.docker.Exec(r.Context(), id, in.Command)
	if err != nil {
		writeError(w, 502, "docker_exec_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"output": output})
}
