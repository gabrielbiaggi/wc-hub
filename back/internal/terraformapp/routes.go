package terraformapp

import (
	terraformadapter "github.com/webcreations/wc-hub/back/internal/adapters/terraform"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

type PolicyRequest struct {
	Action       string
	Scope        string
	TargetName   string
	Confirmation string
	TOTPCode     string
}

type PolicyEnforcer func(w http.ResponseWriter, r *http.Request, req PolicyRequest) bool

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, runner *terraformadapter.Runner) {
	MountRoutesWithPolicy(mux, auth, nil, runner)
}

func MountRoutesWithPolicy(mux *http.ServeMux, auth AuthMiddleware, policyEnforcer PolicyEnforcer, runner *terraformadapter.Runner) {
	h := &Handler{runner: runner, policyEnforcer: policyEnforcer}
	mux.HandleFunc("GET /api/v1/terraform/runs", auth("terraform.read", h.Runs))
	mux.HandleFunc("POST /api/v1/terraform/validate", auth("terraform.plan", h.Validate))
	mux.HandleFunc("POST /api/v1/terraform/plan", auth("terraform.plan", h.Plan))
	mux.HandleFunc("POST /api/v1/terraform/apply", auth("terraform.apply", h.Apply))
	mux.HandleFunc("POST /api/v1/terraform/destroy", auth("terraform.apply", h.Destroy))
	mux.HandleFunc("POST /api/v1/terraform/output", auth("terraform.read", h.Output))
}
