package githubapp

import (
	"encoding/json"
	githubadapter "github.com/webcreations/wc-hub/back/internal/adapters/github"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct{ client *githubadapter.Client }

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub token and repository allowlist are not configured.")
		return
	}
	data, err := h.client.Overview(r.Context())
	if err != nil {
		writeError(w, 502, "github_unavailable", "GitHub inventory is temporarily unavailable.")
		return
	}
	writeJSON(w, 200, data)
}

func (h *Handler) RunAction(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("run_id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid_run", "Workflow run ID is invalid.")
		return
	}
	repository := strings.TrimSpace(r.PathValue("owner")) + "/" + strings.TrimSpace(r.PathValue("repo"))
	action := r.PathValue("action")
	if err = h.client.RunAction(r.Context(), repository, runID, action); err != nil {
		writeError(w, 502, "github_action_failed", "GitHub rejected the workflow action.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"repository": repository, "run_id": runID, "action": action, "status": "accepted"})
}
func (h *Handler) Commits(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	repository := repoFromPath(r)
	items, err := h.client.Commits(r.Context(), repository)
	if err != nil {
		writeError(w, 502, "github_commits_failed", "GitHub commits could not be loaded.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (h *Handler) Commit(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	item, err := h.client.Commit(r.Context(), repoFromPath(r), r.PathValue("sha"))
	if err != nil {
		writeError(w, 502, "github_commit_failed", "GitHub commit details could not be loaded.")
		return
	}
	writeJSON(w, 200, item)
}
func (h *Handler) Workflows(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	items, err := h.client.Workflows(r.Context(), repoFromPath(r))
	if err != nil {
		writeError(w, 502, "github_workflows_failed", "GitHub workflows could not be loaded.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (h *Handler) WorkflowAction(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("workflow_id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid_workflow", "Workflow ID is invalid.")
		return
	}
	var input struct {
		Ref    string            `json:"ref"`
		Inputs map[string]string `json:"inputs"`
	}
	if r.PathValue("action") == "dispatch" {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&input); err != nil {
			writeError(w, 400, "invalid_request", "Workflow inputs are invalid.")
			return
		}
	}
	if err = h.client.WorkflowAction(r.Context(), repoFromPath(r), id, r.PathValue("action"), input.Ref, input.Inputs); err != nil {
		writeError(w, 502, "github_workflow_action_failed", "GitHub rejected the workflow action.")
		return
	}
	writeJSON(w, 202, map[string]any{"status": "accepted", "action": r.PathValue("action")})
}
func (h *Handler) WorkflowFile(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	item, err := h.client.WorkflowFile(r.Context(), repoFromPath(r), r.URL.Query().Get("path"), r.URL.Query().Get("ref"))
	if err != nil {
		writeError(w, 502, "github_workflow_file_failed", "Workflow file could not be loaded.")
		return
	}
	writeJSON(w, 200, item)
}
func (h *Handler) UpdateWorkflowFile(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	var input struct {
		Path    string `json:"path"`
		Branch  string `json:"branch"`
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Content string `json:"content_base64"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 3<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "Workflow file update is invalid.")
		return
	}
	if err := h.client.UpdateWorkflowFile(r.Context(), repoFromPath(r), input.Path, input.Branch, input.SHA, input.Message, input.Content); err != nil {
		writeError(w, 502, "github_workflow_update_failed", "GitHub rejected the workflow file update.")
		return
	}
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}

func (h *Handler) WorkflowRunLogs(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("run_id"), 10, 64)
	if err != nil || runID <= 0 {
		writeError(w, 400, "invalid_request", "Run ID inválido.")
		return
	}
	logs, err := h.client.GetWorkflowRunLogs(r.Context(), repoFromPath(r), runID)
	if err != nil {
		writeError(w, 502, "github_logs_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"logs": logs})
}

func (h *Handler) CreateRelease(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	var input githubadapter.CreateReleaseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.TagName) == "" {
		writeError(w, 400, "invalid_request", "Tag Name é obrigatório.")
		return
	}
	rel, err := h.client.CreateRelease(r.Context(), repoFromPath(r), input)
	if err != nil {
		writeError(w, 502, "create_release_failed", err.Error())
		return
	}
	writeJSON(w, 201, rel)
}

func (h *Handler) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeError(w, 503, "github_unconfigured", "GitHub is not configured.")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid_request", "Release ID inválido.")
		return
	}
	if err := h.client.DeleteRelease(r.Context(), repoFromPath(r), id); err != nil {
		writeError(w, 502, "delete_release_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func repoFromPath(r *http.Request) string {
	return strings.TrimSpace(r.PathValue("owner")) + "/" + strings.TrimSpace(r.PathValue("repo"))
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
