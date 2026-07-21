package app

import (
	"encoding/json"
	proxmoxadapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	telemetrydomain "github.com/webcreations/wc-hub/back/internal/telemetry/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *App) proxmoxSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := a.proxmox.Summary(r.Context(), a.proxmoxClient != nil)
	if err != nil {
		writeError(w, 500, "proxmox_summary_failed", err.Error())
		return
	}
	writeJSON(w, 200, summary)
}
func (a *App) proxmoxSync(w http.ResponseWriter, r *http.Request) {
	if a.proxmoxClient == nil {
		writeError(w, 409, "proxmox_unconfigured", "Proxmox não configurado: "+a.adapterError("proxmox"))
		return
	}
	session := currentSession(r)
	job, err := a.jobs.Enqueue(r.Context(), jobs.Enqueue{Kind: "proxmox.sync", Payload: map[string]any{"requested_at": time.Now().UTC()}, Priority: 50, CreatedBy: session.User.ID})
	if err != nil {
		writeError(w, 500, "enqueue_failed", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox.sync.enqueue", Scope: security.ScopeRemote, ResourceType: "job", ResourceID: job.ID, TargetName: "Proxmox", Risk: security.RiskDangerous, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 202, job)
}
func (a *App) proxmoxInventory(w http.ResponseWriter, r *http.Request) {
	if len(a.proxmoxClients) == 0 {
		writeError(w, 409, "proxmox_unconfigured", "Proxmox não configurado: "+a.adapterError("proxmox"))
		return
	}
	snapshot := proxmoxadapter.Snapshot{
		CapturedAt: time.Now().UTC(),
		Nodes:      []proxmoxadapter.Node{},
		VMs:        []proxmoxadapter.VM{},
		Containers: []proxmoxadapter.VM{},
		Storage:    []proxmoxadapter.Storage{},
		Warnings:   []string{},
	}
	for _, client := range a.proxmoxClients {
		cluster, err := client.Snapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "proxmox_inventory_failed", "Cluster "+client.ID()+": "+err.Error())
			return
		}
		snapshot.Nodes = append(snapshot.Nodes, cluster.Nodes...)
		snapshot.VMs = append(snapshot.VMs, cluster.VMs...)
		snapshot.Containers = append(snapshot.Containers, cluster.Containers...)
		snapshot.Storage = append(snapshot.Storage, cluster.Storage...)
	}
	writeJSON(w, 200, snapshot)
}
func (a *App) proxmoxPowerAction(w http.ResponseWriter, r *http.Request) {
	if len(a.proxmoxClients) == 0 {
		writeError(w, 409, "proxmox_unconfigured", "Proxmox não configurado: "+a.adapterError("proxmox"))
		return
	}
	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil {
		writeError(w, 400, "invalid_resource", "Proxmox VMID is invalid.")
		return
	}
	node, kind, action := r.PathValue("node"), r.PathValue("kind"), r.PathValue("action")
	var target *proxmoxadapter.Client
	requestedCluster := strings.TrimSpace(r.URL.Query().Get("cluster"))
	for _, client := range a.proxmoxClients {
		if requestedCluster != "" && client.ID() != requestedCluster {
			continue
		}
		snapshot, snapshotErr := client.Snapshot(r.Context())
		if snapshotErr != nil {
			writeError(w, http.StatusBadGateway, "proxmox_inventory_failed", "Cluster "+client.ID()+": "+snapshotErr.Error())
			return
		}
		for _, candidate := range snapshot.Nodes {
			if candidate.Node == node {
				target = client
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		writeError(w, 404, "proxmox_node_not_found", "The Proxmox node was not found in configured clusters.")
		return
	}

	// Self-protection: avaliar ações destrutivas (shutdown, reboot) antes de executar
	actionLower := strings.ToLower(action)
	if actionLower == "shutdown" || actionLower == "reboot" || actionLower == "reset" || actionLower == "stop" {
		targetName := target.ID() + "/" + node + "/" + kind + "/" + strconv.Itoa(vmid)
		isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(vmid, targetName)
		if !a.enforcePolicy(w, r, security.ActionRequest{
			Action:              actionLower,
			Scope:               security.ScopeRemote,
			TargetName:          targetName,
			TargetSelfProtected: isSelf,
			Confirmation:        r.Header.Get("X-Confirmation"),
			TOTPCode:            r.Header.Get("X-TOTP-Code"),
		}) {
			return
		}
	}

	if err = target.PowerAction(r.Context(), node, kind, vmid, action); err != nil {
		writeError(w, 502, "proxmox_action_failed", "Proxmox rejeitou a ação de energia: "+err.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox." + kind + "." + action, Scope: security.ScopeRemote, ResourceType: kind, ResourceID: strconv.Itoa(vmid), TargetName: node, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 202, map[string]any{"node": node, "kind": kind, "vmid": vmid, "action": action, "status": "accepted"})
}

func (a *App) proxmoxClientFor(cluster string) *proxmoxadapter.Client {
	for _, client := range a.proxmoxClients {
		if client.ID() == cluster {
			return client
		}
	}
	return nil
}

func (a *App) proxmoxCreateQEMU(w http.ResponseWriter, r *http.Request) {
	var input proxmoxadapter.CreateQEMUInput
	if !decodeJSON(w, r, &input) {
		return
	}
	target := a.proxmoxClientFor(input.Cluster)
	if target == nil {
		writeError(w, 404, "proxmox_cluster_not_found", "O cluster Proxmox não foi encontrado.")
		return
	}
	if err := target.CreateQEMU(r.Context(), input); err != nil {
		writeError(w, 502, "proxmox_create_failed", "O Proxmox rejeitou a criação da VM: "+err.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox.qemu.create", Scope: security.ScopeRemote, ResourceType: "qemu", ResourceID: strconv.Itoa(input.VMID), TargetName: input.Cluster + "/" + input.Node, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r), Payload: map[string]any{"name": input.Name, "cores": input.Cores, "memory_mb": input.MemoryMB, "disk_gb": input.DiskGB}})
	writeJSON(w, 202, map[string]any{"status": "accepted", "vmid": input.VMID})
}

func (a *App) proxmoxCreateLXC(w http.ResponseWriter, r *http.Request) {
	var input proxmoxadapter.CreateLXCInput
	if !decodeJSON(w, r, &input) {
		return
	}
	target := a.proxmoxClientFor(input.Cluster)
	if target == nil {
		writeError(w, 404, "proxmox_cluster_not_found", "O cluster Proxmox não foi encontrado.")
		return
	}
	if err := target.CreateLXC(r.Context(), input); err != nil {
		writeError(w, 502, "proxmox_create_failed", "O Proxmox rejeitou a criação do container LXC: "+err.Error())
		return
	}
	input.Password = ""
	input.SSHPublicKeys = ""
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox.lxc.create", Scope: security.ScopeRemote, ResourceType: "lxc", ResourceID: strconv.Itoa(input.VMID), TargetName: input.Cluster + "/" + input.Node, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r), Payload: map[string]any{"hostname": input.Hostname, "cores": input.Cores, "memory_mb": input.MemoryMB, "rootfs_gb": input.RootFSGB}})
	writeJSON(w, 202, map[string]any{"status": "accepted", "vmid": input.VMID})
}

func (a *App) proxmoxClone(w http.ResponseWriter, r *http.Request) {
	var input proxmoxadapter.CloneInput
	if !decodeJSON(w, r, &input) {
		return
	}
	target := a.proxmoxClientFor(input.Cluster)
	if target == nil {
		writeError(w, 404, "proxmox_cluster_not_found", "O cluster Proxmox não foi encontrado.")
		return
	}
	if err := target.Clone(r.Context(), input); err != nil {
		writeError(w, 502, "proxmox_clone_failed", "O Proxmox rejeitou a clonagem: "+err.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox." + input.Kind + ".clone", Scope: security.ScopeRemote, ResourceType: input.Kind, ResourceID: strconv.Itoa(input.NewVMID), TargetName: input.Cluster + "/" + input.Node, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r), Payload: map[string]any{"source_vmid": input.SourceVMID, "name": input.Name, "full": input.Full}})
	writeJSON(w, 202, map[string]any{"status": "accepted", "vmid": input.NewVMID})
}

func (a *App) proxmoxDeleteGuest(w http.ResponseWriter, r *http.Request) {
	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil {
		writeError(w, 400, "invalid_resource", "VMID inválido.")
		return
	}
	cluster, node, kind := r.URL.Query().Get("cluster"), r.PathValue("node"), r.PathValue("kind")
	target := a.proxmoxClientFor(cluster)
	if target == nil {
		writeError(w, 404, "proxmox_cluster_not_found", "O cluster Proxmox não foi encontrado.")
		return
	}

	// Self-protection: avaliar ação antes de executar
	targetName := cluster + "/" + node + "/" + kind + "/" + strconv.Itoa(vmid)
	isSelf := a.targetResolver != nil && a.targetResolver.IsSelfProtectedVM(vmid, targetName)
	if !a.enforcePolicy(w, r, security.ActionRequest{
		Action:              "delete_vm",
		Scope:               security.ScopeRemote,
		TargetName:          targetName,
		TargetSelfProtected: isSelf,
		Confirmation:        r.Header.Get("X-Confirmation"),
		TOTPCode:            r.Header.Get("X-TOTP-Code"),
	}) {
		return
	}

	if err = target.DeleteGuest(r.Context(), node, kind, vmid, r.URL.Query().Get("purge") == "true"); err != nil {
		writeError(w, 502, "proxmox_delete_failed", "O Proxmox rejeitou a exclusão: "+err.Error())
		return
	}
	session := currentSession(r)
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "proxmox." + kind + ".delete", Scope: security.ScopeRemote, ResourceType: kind, ResourceID: strconv.Itoa(vmid), TargetName: cluster + "/" + node, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	w.WriteHeader(204)
}
func (a *App) listJobs(w http.ResponseWriter, r *http.Request) {
	items, err := a.jobs.List(r.Context(), 100)
	if err != nil {
		writeError(w, 500, "jobs_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) createJob(w http.ResponseWriter, r *http.Request) {
	var input jobs.Enqueue
	if !decodeJSON(w, r, &input) {
		return
	}
	allowed := map[string]bool{"proxmox.sync": true, "telemetry.maintenance": true}
	if !allowed[input.Kind] {
		writeError(w, 400, "job_kind_denied", "Job kind is not manually allowlisted.")
		return
	}
	session := currentSession(r)
	input.CreatedBy = session.User.ID
	job, err := a.jobs.Enqueue(r.Context(), input)
	if err != nil {
		writeError(w, 500, "enqueue_failed", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "job.enqueue", Scope: security.ScopeRemote, ResourceType: "job", ResourceID: job.ID, TargetName: job.Kind, Risk: security.RiskDangerous, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 202, job)
}

type strongConfirmation struct {
	Confirmation string `json:"confirmation"`
	TOTPCode     string `json:"totp_code"`
}

func (a *App) provisionAgentToken(w http.ResponseWriter, r *http.Request) {
	var req strongConfirmation
	if !decodeJSON(w, r, &req) {
		return
	}
	session := currentSession(r)
	hostID := r.PathValue("host_id")
	var name string
	var self bool
	if err := a.db.QueryRow(r.Context(), `SELECT name,self_protected FROM hosts WHERE id=$1`, hostID).Scan(&name, &self); err != nil {
		writeError(w, 404, "host_not_found", "Host was not found.")
		return
	}
	if self {
		writeError(w, 403, "self_protected", "Agent token rotation for the self target requires offline administration.")
		return
	}
	if req.Confirmation != name || !session.User.TOTPEnabled {
		writeError(w, 403, "strong_confirmation_failed", "Exact host name and enabled TOTP are required.")
		return
	}
	verified, _ := a.auth.VerifyTOTP(r.Context(), session.User.ID, req.TOTPCode)
	if !verified {
		writeError(w, 403, "totp_invalid", "A valid TOTP code is required.")
		return
	}
	token, err := a.telemetry.ProvisionToken(r.Context(), hostID, session.User.ID)
	if err != nil {
		writeError(w, 500, "agent_token_failed", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "agent.token.provision", Scope: security.ScopeRemote, ResourceType: "host", ResourceID: hostID, TargetName: name, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 201, map[string]string{"token": token})
}
func (a *App) ingestAgentMetrics(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	hostID, err := a.telemetry.Authenticate(r.Context(), token)
	if err != nil {
		writeError(w, 401, "agent_unauthorized", "Agent token is invalid.")
		return
	}
	var batch telemetrydomain.Batch
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
	if err = decoder.Decode(&batch); err != nil {
		writeError(w, 400, "metrics_invalid", "Metrics batch is invalid.")
		return
	}
	if err = a.telemetry.Ingest(r.Context(), hostID, batch); err != nil {
		writeError(w, 400, "metrics_rejected", err.Error())
		return
	}
	w.WriteHeader(204)
}
func (a *App) ingestAgentEvent(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	hostID, err := a.telemetry.Authenticate(r.Context(), token)
	if err != nil {
		writeError(w, 401, "agent_unauthorized", "Agent token is invalid.")
		return
	}
	var event struct {
		Action   string `json:"action"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &event) {
		return
	}
	allowed := map[string]bool{"system.uptime": true, "docker.ps": true, "journal.tail": true, "invalid": true}
	if !allowed[event.Action] || !(event.Decision == "allowed" || event.Decision == "denied") {
		writeError(w, 400, "event_invalid", "Agent event is invalid.")
		return
	}
	var name string
	_ = a.db.QueryRow(r.Context(), `SELECT name FROM hosts WHERE id=$1`, hostID).Scan(&name)
	risk := security.RiskSafe
	if event.Action != "system.uptime" {
		risk = security.RiskDangerous
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{Action: "agent." + event.Action, Scope: security.ScopeRemote, ResourceType: "host", ResourceID: hostID, TargetName: name, Risk: risk, Decision: event.Decision, Reason: event.Reason, RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	w.WriteHeader(204)
}
func (a *App) hostTelemetry(w http.ResponseWriter, r *http.Request) {
	items, err := a.telemetry.Latest(r.Context())
	if err != nil {
		writeError(w, 500, "telemetry_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) createTerminalTicket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID string `json:"host_id"`
		strongConfirmation
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	session := currentSession(r)
	var name string
	var self bool
	var scope string
	if err := a.db.QueryRow(r.Context(), `SELECT name,self_protected,scope::text FROM hosts WHERE id=$1`, req.HostID).Scan(&name, &self, &scope); err != nil {
		writeError(w, 404, "host_not_found", "Host was not found.")
		return
	}
	if self || scope == "local" {
		writeError(w, 403, "self_protected", "Terminal access to the local target is blocked.")
		return
	}
	if a.terminalGateway == nil {
		writeError(w, 503, "terminal_unavailable", "SSH key and known_hosts are not configured.")
		return
	}
	if req.Confirmation != name || !session.User.TOTPEnabled {
		writeError(w, 403, "strong_confirmation_failed", "Exact host name and enabled TOTP are required.")
		return
	}
	verified, _ := a.auth.VerifyTOTP(r.Context(), session.User.ID, req.TOTPCode)
	if !verified {
		writeError(w, 403, "totp_invalid", "A valid TOTP code is required.")
		return
	}
	token, sessionID, err := a.terminal.CreateTicket(r.Context(), session.User.ID, req.HostID, remoteIP(r))
	if err != nil {
		writeError(w, 400, "terminal_ticket_failed", err.Error())
		return
	}
	_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: "terminal.ticket.create", Scope: security.ScopeRemote, ResourceType: "terminal_session", ResourceID: sessionID, TargetName: name, Risk: security.RiskDangerous, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	writeJSON(w, 201, map[string]any{"ticket": token, "session_id": sessionID, "expires_in": 45})
}
func (a *App) terminalSessions(w http.ResponseWriter, r *http.Request) {
	items, err := a.terminal.Recent(r.Context())
	if err != nil {
		writeError(w, 500, "terminal_sessions_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (a *App) terminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if a.terminalGateway == nil {
		http.Error(w, "terminal gateway is not configured", http.StatusServiceUnavailable)
		return
	}
	a.terminalGateway.ServeHTTP(w, r)
}
