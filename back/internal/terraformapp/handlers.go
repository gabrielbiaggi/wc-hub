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
type runRequest struct {
	Workspace string            `json:"workspace"`
	Vars      map[string]string `json:"vars,omitempty"`
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

func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		writeError(w, 503, "terraform_unconfigured", "Terraform ephemeral worker is not configured.")
		return
	}
	workspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspace == "" {
		writeError(w, 400, "invalid_request", "Parâmetro workspace é obrigatório.")
		return
	}
	st, err := h.runner.GetState(r.Context(), workspace)
	if err != nil {
		writeError(w, 502, "terraform_state_failed", err.Error())
		return
	}
	writeJSON(w, 200, st)
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
	var req runRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
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

	run, err := h.runner.StartWithVars(r.Context(), operation, req.Workspace, req.Vars)
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
