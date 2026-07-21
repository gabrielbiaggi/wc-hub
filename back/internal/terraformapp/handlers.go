package terraformapp

import (
	"encoding/json"
	terraformadapter "github.com/webcreations/wc-hub/back/internal/adapters/terraform"
	"net/http"
	"strings"
)

type Handler struct {
	runner         *terraformadapter.Runner
	policyEnforcer PolicyEnforcer
}
type request struct {
	Workspace string `json:"workspace"`
}

func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		writeError(w, 503, "terraform_unconfigured", "Terraform ephemeral worker is not configured.")
		return
	}
	runs, err := h.runner.List(r.Context())
	if err != nil {
		writeError(w, 502, "terraform_worker_unavailable", "Terraform worker is unavailable.")
		return
	}
	writeJSON(w, 200, map[string]any{"items": runs, "workspaces": h.runner.Workspaces()})
}
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) { h.start(w, r, "validate") }
func (h *Handler) Plan(w http.ResponseWriter, r *http.Request)     { h.start(w, r, "plan") }
func (h *Handler) Apply(w http.ResponseWriter, r *http.Request)    { h.start(w, r, "apply") }
func (h *Handler) Destroy(w http.ResponseWriter, r *http.Request)  { h.start(w, r, "destroy") }
func (h *Handler) Output(w http.ResponseWriter, r *http.Request)   { h.start(w, r, "output") }
func (h *Handler) start(w http.ResponseWriter, r *http.Request, operation string) {
	if h.runner == nil {
		writeError(w, 503, "terraform_unconfigured", "Terraform ephemeral worker is not configured.")
		return
	}
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, 400, "invalid_request", "Workspace is required.")
		return
	}

	// Self-protection: validate terraform destroy and apply
	if h.policyEnforcer != nil && (operation == "destroy" || operation == "apply") {
		if !h.policyEnforcer(w, r, PolicyRequest{
			Action:       "terraform_" + operation,
			Scope:        "remote",
			TargetName:   "terraform/workspace/" + req.Workspace,
			Confirmation: r.Header.Get("X-Confirmation"),
			TOTPCode:     r.Header.Get("X-TOTP-Code"),
		}) {
			return
		}
	}

	run, err := h.runner.Start(r.Context(), operation, req.Workspace)
	if err != nil {
		if strings.Contains(err.Error(), "allowlisted") {
			writeError(w, 400, "terraform_request_rejected", "Terraform operation or workspace is not allowlisted.")
		} else {
			writeError(w, 502, "terraform_worker_unavailable", "Terraform worker could not start the operation.")
		}
		return
	}
	writeJSON(w, 202, run)
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
