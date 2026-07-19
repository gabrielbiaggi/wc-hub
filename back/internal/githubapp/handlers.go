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
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
