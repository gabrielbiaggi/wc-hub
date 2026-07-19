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
}
