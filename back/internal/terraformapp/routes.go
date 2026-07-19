package terraformapp

import (
	terraformadapter "github.com/webcreations/wc-hub/back/internal/adapters/terraform"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, runner *terraformadapter.Runner) {
	h := &Handler{runner}
	mux.HandleFunc("GET /api/v1/terraform/runs", auth("terraform.read", h.Runs))
	mux.HandleFunc("POST /api/v1/terraform/validate", auth("terraform.plan", h.Validate))
	mux.HandleFunc("POST /api/v1/terraform/plan", auth("terraform.plan", h.Plan))
	mux.HandleFunc("POST /api/v1/terraform/apply", auth("terraform.apply", h.Apply))
}
