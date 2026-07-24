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
	InstanceAction(context.Context, string, string, string) error
	LaunchInstance(context.Context, ociadapter.LaunchInstanceInput) (string, error)
	CreateAutonomousDatabase(context.Context, ociadapter.CreateAutonomousDatabaseInput) (string, error)
	TerminateInstance(ctx context.Context, instanceID, region string) error
	ListImages(ctx context.Context, compartmentID, region string) ([]ociadapter.ImageSummary, error)
	ListShapes(ctx context.Context, compartmentID, region string) ([]ociadapter.ShapeSummary, error)
	AutonomousDatabaseAction(ctx context.Context, dbID, action, region string) error
	DBSystemAction(ctx context.Context, dbID, action, region string) error
	CreateVCN(ctx context.Context, input ociadapter.CreateVCNInput) (string, error)
	DeleteVCN(ctx context.Context, vcnID, region string) error
	CreateSubnet(ctx context.Context, input ociadapter.CreateSubnetInput) (string, error)
	DeleteSubnet(ctx context.Context, subnetID, region string) error
	CreateBlockVolume(ctx context.Context, input ociadapter.CreateVolumeInput) (string, error)
	DeleteBlockVolume(ctx context.Context, volumeID, region string) error
}

func (h *Handler) terminateInstance(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	region := r.URL.Query().Get("region")
	if err := h.client.TerminateInstance(r.Context(), id, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_terminate_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.instance.terminate", ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "instance_id": id})
}

func (h *Handler) listImages(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	compartment := r.URL.Query().Get("compartment_id")
	region := r.URL.Query().Get("region")
	items, err := h.client.ListImages(r.Context(), compartment, region)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_images_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listShapes(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	compartment := r.URL.Query().Get("compartment_id")
	region := r.URL.Query().Get("region")
	items, err := h.client.ListShapes(r.Context(), compartment, region)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_shapes_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) autonomousDatabaseAction(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	action := r.PathValue("action")
	region := r.URL.Query().Get("region")
	if err := h.client.AutonomousDatabaseAction(r.Context(), id, action, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_autonomous_action_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.autonomous_database." + action, ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"action": action, "region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": action, "database_id": id})
}

func (h *Handler) dbSystemAction(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	action := r.PathValue("action")
	region := r.URL.Query().Get("region")
	if err := h.client.DBSystemAction(r.Context(), id, action, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_dbsystem_action_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.dbsystem." + action, ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"action": action, "region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": action, "dbsystem_id": id})
}

func (h *Handler) createVCN(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	var input ociadapter.CreateVCNInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Dados da VCN são inválidos.")
		return
	}
	id, err := h.client.CreateVCN(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_create_vcn_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.vcn.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"cidr": input.CIDRBlock}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "vcn_id": id})
}

func (h *Handler) deleteVCN(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	region := r.URL.Query().Get("region")
	if err := h.client.DeleteVCN(r.Context(), id, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_delete_vcn_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.vcn.delete", ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "vcn_id": id})
}

func (h *Handler) createSubnet(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	var input ociadapter.CreateSubnetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Dados da sub-rede são inválidos.")
		return
	}
	id, err := h.client.CreateSubnet(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_create_subnet_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.subnet.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"cidr": input.CIDRBlock}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "subnet_id": id})
}

func (h *Handler) deleteSubnet(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	region := r.URL.Query().Get("region")
	if err := h.client.DeleteSubnet(r.Context(), id, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_delete_subnet_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.subnet.delete", ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "subnet_id": id})
}

func (h *Handler) createBlockVolume(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	var input ociadapter.CreateVolumeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Dados do volume são inválidos.")
		return
	}
	id, err := h.client.CreateBlockVolume(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_create_volume_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.volume.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"size_gb": input.SizeGB}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "volume_id": id})
}

func (h *Handler) deleteBlockVolume(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	id := r.PathValue("id")
	region := r.URL.Query().Get("region")
	if err := h.client.DeleteBlockVolume(r.Context(), id, region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_delete_volume_failed", err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.volume.delete", ResourceID: id, TargetName: "Oracle Cloud", Payload: map[string]any{"region": region}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "volume_id": id})
}

func (h *Handler) launchInstance(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
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
		writeError(w, http.StatusBadGateway, "oci_launch_failed", "A Oracle Cloud rejeitou a criação da instância: "+err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.instance.create", ResourceID: id, TargetName: input.DisplayName, Payload: map[string]any{"shape": input.Shape, "compartment_id": input.CompartmentID, "public_ip": input.AssignPublicIP}})
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "instance_id": id})
}

func (h *Handler) createAutonomousDatabase(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
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
		writeError(w, http.StatusBadGateway, "oci_database_create_failed", "A Oracle Cloud rejeitou a criação do Autonomous Database: "+err.Error())
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
	client  Client
	audit   func(context.Context, AuditEvent)
	initErr string
}

func NewHandler(client Client, audit func(context.Context, AuditEvent), initErr ...string) *Handler {
	message := ""
	if len(initErr) > 0 {
		message = initErr[0]
	}
	return &Handler{client: client, audit: audit, initErr: message}
}

func (h *Handler) unconfiguredMessage() string {
	if h != nil && h.initErr != "" {
		return "OCI não configurada: " + h.initErr
	}
	return "As credenciais de assinatura da OCI não estão configuradas."
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	snapshot, err := h.client.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "oci_inventory_failed", "O inventário OCI não pôde ser carregado: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) instanceAction(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		writeError(w, http.StatusConflict, "oci_unconfigured", h.unconfiguredMessage())
		return
	}
	var input struct {
		InstanceID string `json:"instance_id"`
		Region     string `json:"region"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.InstanceID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "A valid OCI instance OCID is required.")
		return
	}
	action := r.PathValue("action")
	if err := h.client.InstanceAction(r.Context(), input.InstanceID, action, input.Region); err != nil {
		writeError(w, http.StatusBadGateway, "oci_action_failed", "A Oracle Cloud rejeitou a ação na instância: "+err.Error())
		return
	}
	if h.audit != nil {
		h.audit(r.Context(), AuditEvent{Action: "oci.instance." + action, ResourceID: input.InstanceID, TargetName: "Oracle Cloud", Payload: map[string]any{"action": action, "region": input.Region}})
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
