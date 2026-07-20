package app

import (
	"net/http"
	"strconv"
	"strings"
)

func (a *App) githubOverview(w http.ResponseWriter, r *http.Request) {
	if a.github == nil {
		writeError(w, 503, "github_not_configured", "GitHub não está configurado.")
		return
	}
	overview, err := a.github.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "github_overview_failed", err.Error())
		return
	}
	writeJSON(w, 200, overview)
}

func (a *App) githubCommits(w http.ResponseWriter, r *http.Request) {
	if a.github == nil {
		writeError(w, 503, "github_not_configured", "GitHub não está configurado.")
		return
	}
	
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeError(w, 400, "missing_repo", "Parâmetro repo obrigatório.")
		return
	}
	
	commits, err := a.github.Commits(r.Context(), repo)
	if err != nil {
		writeError(w, 502, "github_commits_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, commits)
}

func (a *App) githubWorkflows(w http.ResponseWriter, r *http.Request) {
	if a.github == nil {
		writeError(w, 503, "github_not_configured", "GitHub não está configurado.")
		return
	}
	
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeError(w, 400, "missing_repo", "Parâmetro repo obrigatório.")
		return
	}
	
	workflows, err := a.github.Workflows(r.Context(), repo)
	if err != nil {
		writeError(w, 502, "github_workflows_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, workflows)
}

func (a *App) githubWorkflowAction(w http.ResponseWriter, r *http.Request) {
	if a.github == nil {
		writeError(w, 503, "github_not_configured", "GitHub não está configurado.")
		return
	}
	
	var in struct {
		Repo       string            `json:"repo"`
		WorkflowID int64             `json:"workflow_id"`
		Action     string            `json:"action"`
		Ref        string            `json:"ref"`
		Inputs     map[string]string `json:"inputs"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	
	if in.Repo == "" || in.WorkflowID <= 0 || in.Action == "" {
		writeError(w, 400, "missing_parameters", "Repo, workflow_id e action obrigatórios.")
		return
	}
	
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action != "dispatch" {
		writeError(w, 400, "invalid_action", "Action deve ser dispatch.")
		return
	}
	
	if in.Ref == "" {
		in.Ref = "main"
	}
	
	if err := a.github.WorkflowAction(r.Context(), in.Repo, in.WorkflowID, action, in.Ref, in.Inputs); err != nil {
		writeError(w, 502, "github_workflow_action_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "dispatched"})
}

func (a *App) githubRunAction(w http.ResponseWriter, r *http.Request) {
	if a.github == nil {
		writeError(w, 503, "github_not_configured", "GitHub não está configurado.")
		return
	}
	
	var in struct {
		Repo   string `json:"repo"`
		RunID  int64  `json:"run_id"`
		Action string `json:"action"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	
	if in.Repo == "" || in.RunID <= 0 || in.Action == "" {
		writeError(w, 400, "missing_parameters", "Repo, run_id e action obrigatórios.")
		return
	}
	
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action != "cancel" && action != "rerun" {
		writeError(w, 400, "invalid_action", "Action deve ser cancel ou rerun.")
		return
	}
	
	if err := a.github.RunAction(r.Context(), in.Repo, in.RunID, action); err != nil {
		writeError(w, 502, "github_run_action_failed", err.Error())
		return
	}
	
	writeJSON(w, 200, map[string]string{"status": "success", "action": action})
}
