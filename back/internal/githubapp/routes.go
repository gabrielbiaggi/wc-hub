package githubapp

import (
	githubadapter "github.com/webcreations/wc-hub/back/internal/adapters/github"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *githubadapter.Client) {
	handler := &Handler{client: client}
	mux.HandleFunc("GET /api/v1/github/overview", auth("github.read", handler.Overview))
	mux.HandleFunc("POST /api/v1/github/repos/{owner}/{repo}/actions/runs/{run_id}/{action}", auth("github.manage", handler.RunAction))
	mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/commits", auth("github.read", handler.Commits))
	mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/commits/{sha}", auth("github.read", handler.Commit))
	mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/workflows", auth("github.read", handler.Workflows))
	mux.HandleFunc("POST /api/v1/github/repos/{owner}/{repo}/workflows/{workflow_id}/{action}", auth("github.manage", handler.WorkflowAction))
	mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/workflow-file", auth("github.read", handler.WorkflowFile))
	mux.HandleFunc("PUT /api/v1/github/repos/{owner}/{repo}/workflow-file", auth("github.manage", handler.UpdateWorkflowFile))
	mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/actions/runs/{run_id}/logs", auth("github.read", handler.WorkflowRunLogs))
	mux.HandleFunc("POST /api/v1/github/repos/{owner}/{repo}/releases", auth("github.manage", handler.CreateRelease))
	mux.HandleFunc("DELETE /api/v1/github/repos/{owner}/{repo}/releases/{id}", auth("github.manage", handler.DeleteRelease))
}
