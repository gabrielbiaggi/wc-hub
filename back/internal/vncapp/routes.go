package vncapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	proxmoxadapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/vnc"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc
type AuditFunc func(*http.Request, string, string)

type Target struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Status  string `json:"status,omitempty"`
	WSPath  string `json:"ws_path"`
}

type Handler struct {
	gateway *adapter.Gateway
	proxmox []*proxmoxadapter.Client
	audit   AuditFunc
}

// MountRoutes supports both explicit raw VNC targets and the native Proxmox
// console. Native console tickets are generated server-side for each session.
func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, gateway *adapter.Gateway, proxmoxClients []*proxmoxadapter.Client, audit AuditFunc) {
	h := &Handler{gateway: gateway, proxmox: proxmoxClients, audit: audit}
	mux.HandleFunc("GET /api/v1/vnc/targets", auth("vnc.read", h.targets))
	mux.HandleFunc("GET /ws/vnc/proxmox/{cluster}/{node}/{kind}/{vmid}", auth("vnc.connect", h.connectProxmox))
	mux.HandleFunc("GET /ws/vnc/{target}", auth("vnc.connect", h.connectStatic))
}

func (h *Handler) targets(w http.ResponseWriter, r *http.Request) {
	items := make([]Target, 0)
	if h.gateway != nil {
		for _, target := range h.gateway.Targets() {
			items = append(items, Target{ID: target.ID, Address: target.Address, Kind: "vnc", WSPath: "/ws/vnc/" + target.ID})
		}
	}
	for _, client := range h.proxmox {
		snapshot, err := client.Snapshot(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "proxmox_console_unavailable", fmt.Sprintf("Não foi possível listar consoles nativos do Proxmox (%s): %v", client.ID(), err))
			return
		}
		for _, guest := range append(snapshot.VMs, snapshot.Containers...) {
			if guest.Status != "running" || guest.Node == "" || (guest.Type != "qemu" && guest.Type != "lxc") {
				continue
			}
			path := fmt.Sprintf("/ws/vnc/proxmox/%s/%s/%s/%d", client.ID(), guest.Node, guest.Type, guest.VMID)
			items = append(items, Target{ID: "proxmox:" + client.ID() + ":" + guest.Type + ":" + strconv.Itoa(guest.VMID), Address: client.ID() + " / " + guest.Node, Name: guest.Name, Kind: "proxmox-" + guest.Type, Status: guest.Status, WSPath: path})
		}
	}
	if len(items) == 0 {
		writeError(w, http.StatusServiceUnavailable, "vnc_unconfigured", "Nenhum alvo VNC autorizado ou console Proxmox em execução foi encontrado.")
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) connectStatic(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		writeError(w, http.StatusServiceUnavailable, "vnc_unconfigured", "O gateway VNC não está configurado.")
		return
	}
	target := r.PathValue("target")
	h.auditConnection(r, "vnc.connect", target)
	if err := h.gateway.Serve(w, r, target); err != nil && r.Context().Err() == nil {
		h.auditConnection(r, "vnc.connect.failed", target)
	}
}

func (h *Handler) connectProxmox(w http.ResponseWriter, r *http.Request) {
	cluster, node, kind := r.PathValue("cluster"), r.PathValue("node"), r.PathValue("kind")
	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil || vmid < 1 {
		writeError(w, http.StatusBadRequest, "invalid_console_target", "O identificador da VM é inválido.")
		return
	}
	var client *proxmoxadapter.Client
	for _, candidate := range h.proxmox {
		if candidate.ID() == cluster {
			client = candidate
			break
		}
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "proxmox_cluster_not_found", "O cluster Proxmox solicitado não está configurado.")
		return
	}
	target := cluster + "/" + node + "/" + kind + "/" + strconv.Itoa(vmid)
	h.auditConnection(r, "vnc.proxmox.connect", target)
	upgraded, err := client.ServeVNC(w, r, node, kind, vmid)
	if err != nil && r.Context().Err() == nil {
		// Once upgraded, no HTTP body can be safely written into the websocket.
		if !upgraded {
			writeError(w, http.StatusBadGateway, "proxmox_console_failed", "Não foi possível abrir o console nativo do Proxmox: "+err.Error())
		}
		h.auditConnection(r, "vnc.proxmox.connect.failed", target)
	}
}

func (h *Handler) auditConnection(r *http.Request, action, target string) {
	if h.audit != nil {
		h.audit(r, action, target)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}, "at": time.Now().UTC()})
}
