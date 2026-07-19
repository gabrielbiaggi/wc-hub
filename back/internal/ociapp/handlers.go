package ociapp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	ociadapter "github.com/webcreations/wc-hub/back/internal/adapters/oci"
)

type Client interface {
	Snapshot(context.Context) (ociadapter.Snapshot, error)
	InstanceAction(context.Context, string, string) error
	LaunchInstance(context.Context, ociadapter.LaunchInstanceInput) (string, error)
	CreateAutonomousDatabase(context.Context, ociadapter.CreateAutonomousDatabaseInput) (string, error)
}

func (h *Handler) launchInstance(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", "As credenciais de assinatura da OCI não estão configuradas.")
		return
	}
	var input ociadapter.LaunchInstanceInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Os dados da nova instância são inválidos.")
		return
	}
	id, err := h.client.LaunchInstance(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_launch_failed", "A Oracle Cloud rejeitou a criação da instância.")
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.instance.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"shape": input.Shape, "compartment_id": input.CompartmentID, "public_ip": input.AssignPublicIP}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "instance_id": id})
}

func (h *Handler) createAutonomousDatabase(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", "As credenciais de assinatura da OCI não estão configuradas.")
		return
	}
	var input ociadapter.CreateAutonomousDatabaseInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Os dados do Autonomous Database são inválidos.")
		return
	}
	id, err := h.client.CreateAutonomousDatabase(r.Context(), input)
	input.AdminPassword = ""
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_database_create_failed", "A Oracle Cloud rejeitou a criação do Autonomous Database.")
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.autonomous_database.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"workload": input.Workload, "compartment_id": input.CompartmentID, "free_tier": input.FreeTier}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "database_id": id})
}

type AuditEvent struct {
	Action     string
	ResourceID string
	TargetName string
	Payload    map[string]any
}

type Handler struct {
	client Client
	audit  func(context.Context, AuditEvent)
}

func NewHandler(client Client, audit func(context.Context, AuditEvent)) *Handler {
	return &Handler{client: client, audit: audit}
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", "OCI signing credentials are not configured.")
		return
	}
	snapshot, err := h.client.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_inventory_failed", "Oracle Cloud inventory could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) instanceAction(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", "OCI signing credentials are not configured.")
		return
	}
	var input struct {
		InstanceID string `json:"instance_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.InstanceID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "A valid OCI instance OCID is required.")
		return
	}
	action := r.PathValue("action")
	if err := h.client.InstanceAction(r.Context(), input.InstanceID, action); err != nil {
		writeError(w, http.StatusBadGateway, "oci_action_failed", "Oracle Cloud rejected the instance action.")
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.instance." + action, ResourceID: input.InstanceID, TargetName: "Oracle Cloud", Payload: map[string]any{"action": action}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": action})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
