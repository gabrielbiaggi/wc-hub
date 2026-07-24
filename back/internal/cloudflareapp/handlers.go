package cloudflareapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	cloudflareadapter "github.com/webcreations/wc-hub/back/internal/adapters/cloudflare"
)

const defaultPermission = "cloudflare.read"

type Reader interface {
	AllowedAccounts() []string
	AllowedZones() []string
	AccountAllowed(string) bool
	ZoneAllowed(string) bool
	ListTunnels(context.Context, string) ([]cloudflareadapter.Tunnel, error)
	ListDNSRecords(context.Context, string) ([]cloudflareadapter.DNSRecord, error)
}

type DNSWriter interface {
	CreateDNSRecord(context.Context, string, cloudflareadapter.DNSRecordInput) (cloudflareadapter.DNSRecord, error)
	UpdateDNSRecord(context.Context, string, string, cloudflareadapter.DNSRecordInput) (cloudflareadapter.DNSRecord, error)
	DeleteDNSRecord(context.Context, string, string) error
}
type TunnelWriter interface {
	CreateTunnel(context.Context, string, cloudflareadapter.TunnelInput) (cloudflareadapter.Tunnel, error)
	UpdateTunnel(context.Context, string, string, cloudflareadapter.TunnelInput) (cloudflareadapter.Tunnel, error)
	DeleteTunnel(context.Context, string, string) error
}
type TunnelConfigurator interface {
	TunnelConfiguration(context.Context, string, string) (cloudflareadapter.TunnelConfiguration, error)
	UpdateTunnelConfiguration(context.Context, string, string, []cloudflareadapter.TunnelIngressRule) (cloudflareadapter.TunnelConfiguration, error)
}
type PrivateRouteManager interface {
	ListPrivateRoutes(context.Context, string) ([]cloudflareadapter.PrivateRoute, error)
	CreatePrivateRoute(context.Context, string, cloudflareadapter.PrivateRouteInput) (cloudflareadapter.PrivateRoute, error)
}
type ZoneAdministrator interface {
	ListZoneSettings(context.Context, string) ([]cloudflareadapter.ZoneSetting, error)
	UpdateZoneSetting(context.Context, string, string, any) (cloudflareadapter.ZoneSetting, error)
	PurgeCache(context.Context, string) error
	ListRulesets(context.Context, string) ([]cloudflareadapter.Ruleset, error)
}

// AuditEvent is intentionally provider-neutral so the global application can
// translate it into its hash-chained audit repository without this plugin
// importing application internals.
type AuditEvent struct {
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	TargetName   string         `json:"target_name,omitempty"`
	Decision     string         `json:"decision"`
	Reason       string         `json:"reason,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type AuditFunc func(context.Context, AuditEvent)

type Config struct {
	Permission string
	Timeout    time.Duration
	Audit      AuditFunc
}

type Handler struct {
	reader          Reader
	permission      string
	timeout         time.Duration
	audit           AuditFunc
	allowedAccounts map[string]struct{}
	allowedZones    map[string]struct{}
}

type OverviewResponse struct {
	GeneratedAt time.Time                     `json:"generated_at"`
	Status      string                        `json:"status"`
	Tunnels     []cloudflareadapter.Tunnel    `json:"tunnels"`
	DNSRecords  []cloudflareadapter.DNSRecord `json:"dns_records"`
	Targets     []TargetHealth                `json:"targets"`
	Summary     Summary                       `json:"summary"`
}

type TargetHealth struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	ItemCount int    `json:"item_count"`
}

type Summary struct {
	Accounts       int `json:"accounts"`
	Zones          int `json:"zones"`
	Tunnels        int `json:"tunnels"`
	HealthyTunnels int `json:"healthy_tunnels"`
	DNSRecords     int `json:"dns_records"`
	ProxiedRecords int `json:"proxied_records"`
}

func NewHandler(reader Reader, config Config) (*Handler, error) {
	if reader == nil {
		return nil, errors.New("Cloudflare reader is required")
	}
	permission := config.Permission
	if permission == "" {
		permission = defaultPermission
	}
	timeout := config.Timeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 20 * time.Second
	}
	accounts := set(reader.AllowedAccounts())
	zones := set(reader.AllowedZones())
	if len(accounts) == 0 && len(zones) == 0 {
		return nil, errors.New("Cloudflare handler requires an account or zone allowlist")
	}
	return &Handler{
		reader:          reader,
		permission:      permission,
		timeout:         timeout,
		audit:           config.Audit,
		allowedAccounts: accounts,
		allowedZones:    zones,
	}, nil
}

func (h *Handler) Permission() string { return h.permission }

func (h *Handler) Overview(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()

	response := OverviewResponse{
		GeneratedAt: time.Now().UTC(),
		Status:      "healthy",
		Tunnels:     []cloudflareadapter.Tunnel{},
		DNSRecords:  []cloudflareadapter.DNSRecord{},
		Targets:     []TargetHealth{},
		Summary: Summary{
			Accounts: len(h.allowedAccounts),
			Zones:    len(h.allowedZones),
		},
	}
	failed := 0
	for _, accountID := range sortedKeys(h.allowedAccounts) {
		// Defense in depth: validate the handler allowlist and the adapter
		// allowlist before any external request.
		if !h.reader.AccountAllowed(accountID) {
			failed++
			response.Targets = append(response.Targets, unavailableTarget("account", accountID))
			continue
		}
		tunnels, err := h.reader.ListTunnels(ctx, accountID)
		if err != nil {
			failed++
			response.Targets = append(response.Targets, unavailableTarget("account", accountID))
			continue
		}
		response.Tunnels = append(response.Tunnels, tunnels...)
		response.Targets = append(response.Targets, TargetHealth{Kind: "account", ID: accountID, Status: "healthy", Message: "Tunnel inventory available.", ItemCount: len(tunnels)})
	}
	for _, zoneID := range sortedKeys(h.allowedZones) {
		if !h.reader.ZoneAllowed(zoneID) {
			failed++
			response.Targets = append(response.Targets, unavailableTarget("zone", zoneID))
			continue
		}
		records, err := h.reader.ListDNSRecords(ctx, zoneID)
		if err != nil {
			failed++
			response.Targets = append(response.Targets, unavailableTarget("zone", zoneID))
			continue
		}
		response.DNSRecords = append(response.DNSRecords, records...)
		response.Targets = append(response.Targets, TargetHealth{Kind: "zone", ID: zoneID, Status: "healthy", Message: "DNS inventory available.", ItemCount: len(records)})
	}

	response.Summary.Tunnels = len(response.Tunnels)
	response.Summary.DNSRecords = len(response.DNSRecords)
	for _, tunnel := range response.Tunnels {
		if tunnel.Status == "healthy" {
			response.Summary.HealthyTunnels++
		}
	}
	for _, record := range response.DNSRecords {
		if record.Proxied {
			response.Summary.ProxiedRecords++
		}
	}
	if failed > 0 {
		response.Status = "degraded"
	}
	if failed == len(response.Targets) && failed > 0 {
		response.Status = "unavailable"
	}
	sort.Slice(response.Tunnels, func(i, j int) bool { return response.Tunnels[i].Name < response.Tunnels[j].Name })
	sort.Slice(response.DNSRecords, func(i, j int) bool {
		if response.DNSRecords[i].Name == response.DNSRecords[j].Name {
			return response.DNSRecords[i].Type < response.DNSRecords[j].Type
		}
		return response.DNSRecords[i].Name < response.DNSRecords[j].Name
	})

	decision := "allowed"
	if response.Status == "unavailable" {
		decision = "failed"
	}
	h.emit(ctx, AuditEvent{
		Action: "cloudflare.inventory.read", ResourceType: "cloudflare_integration",
		Decision: decision, Reason: "read-only allowlisted inventory request",
		Payload: map[string]any{"accounts": response.Summary.Accounts, "zones": response.Summary.Zones, "failed_targets": failed},
	})
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) Tunnels(w http.ResponseWriter, request *http.Request) {
	accountID := request.PathValue("account_id")
	if _, allowed := h.allowedAccounts[accountID]; !allowed || !h.reader.AccountAllowed(accountID) {
		h.emit(request.Context(), AuditEvent{Action: "cloudflare.tunnels.read", ResourceType: "cloudflare_account", TargetName: accountID, Decision: "denied", Reason: "account is not allowlisted"})
		writeError(w, http.StatusForbidden, "account_not_allowed", "The requested Cloudflare account is not allowlisted.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	tunnels, err := h.reader.ListTunnels(ctx, accountID)
	if err != nil {
		h.emit(ctx, AuditEvent{Action: "cloudflare.tunnels.read", ResourceType: "cloudflare_account", TargetName: accountID, Decision: "failed", Reason: "provider inventory unavailable"})
		writeProviderError(w)
		return
	}
	h.emit(ctx, AuditEvent{Action: "cloudflare.tunnels.read", ResourceType: "cloudflare_account", TargetName: accountID, Decision: "allowed", Reason: "read-only allowlisted request", Payload: map[string]any{"item_count": len(tunnels)}})
	writeJSON(w, http.StatusOK, map[string]any{"items": tunnels})
}
func (h *Handler) CreateTunnel(w http.ResponseWriter, request *http.Request) {
	h.writeTunnel(w, request, false)
}
func (h *Handler) UpdateTunnel(w http.ResponseWriter, request *http.Request) {
	h.writeTunnel(w, request, true)
}
func (h *Handler) writeTunnel(w http.ResponseWriter, request *http.Request, update bool) {
	accountID := request.PathValue("account_id")
	if _, allowed := h.allowedAccounts[accountID]; !allowed || !h.reader.AccountAllowed(accountID) {
		writeError(w, 403, "account_not_allowed", "A conta Cloudflare solicitada não está autorizada.")
		return
	}
	writer, ok := h.reader.(TunnelWriter)
	if !ok {
		writeError(w, 503, "cloudflare_tunnel_write_unavailable", "A permissão de escrita para tunnels não está configurada.")
		return
	}
	var input cloudflareadapter.TunnelInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "O payload do tunnel é inválido.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	action := "cloudflare.tunnel.create"
	var tunnel cloudflareadapter.Tunnel
	var err error
	if update {
		action = "cloudflare.tunnel.update"
		tunnel, err = writer.UpdateTunnel(ctx, accountID, request.PathValue("tunnel_id"), input)
	} else {
		tunnel, err = writer.CreateTunnel(ctx, accountID, input)
	}
	if err != nil {
		h.emit(ctx, AuditEvent{Action: action, ResourceType: "cloudflare_tunnel", TargetName: input.Name, Decision: "failed", Reason: "provider rejected mutation"})
		writeError(w, 502, "cloudflare_tunnel_write_failed", "A Cloudflare rejeitou a alteração do tunnel: "+err.Error())
		return
	}
	h.emit(ctx, AuditEvent{Action: action, ResourceType: "cloudflare_tunnel", TargetName: tunnel.Name, Decision: "allowed", Reason: "allowlisted account mutation", Payload: map[string]any{"account_id": accountID, "tunnel_id": tunnel.ID}})
	writeJSON(w, map[bool]int{false: 201, true: 200}[update], tunnel)
}
func (h *Handler) DeleteTunnel(w http.ResponseWriter, request *http.Request) {
	accountID, tunnelID := request.PathValue("account_id"), request.PathValue("tunnel_id")
	if _, allowed := h.allowedAccounts[accountID]; !allowed || !h.reader.AccountAllowed(accountID) {
		writeError(w, 403, "account_not_allowed", "A conta Cloudflare solicitada não está autorizada.")
		return
	}
	writer, ok := h.reader.(TunnelWriter)
	if !ok {
		writeError(w, 503, "cloudflare_tunnel_write_unavailable", "A permissão de escrita para tunnels não está configurada.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	if err := writer.DeleteTunnel(ctx, accountID, tunnelID); err != nil {
		h.emit(ctx, AuditEvent{Action: "cloudflare.tunnel.delete", ResourceType: "cloudflare_tunnel", TargetName: tunnelID, Decision: "failed", Reason: "provider rejected mutation"})
		writeError(w, 502, "cloudflare_tunnel_delete_failed", "A Cloudflare rejeitou a exclusão do tunnel: "+err.Error())
		return
	}
	h.emit(ctx, AuditEvent{Action: "cloudflare.tunnel.delete", ResourceType: "cloudflare_tunnel", TargetName: tunnelID, Decision: "allowed", Reason: "allowlisted account mutation", Payload: map[string]any{"account_id": accountID}})
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) TunnelConfiguration(w http.ResponseWriter, r *http.Request) {
	a, id := r.PathValue("account_id"), r.PathValue("tunnel_id")
	c, ok := h.reader.(TunnelConfigurator)
	if !ok || !h.reader.AccountAllowed(a) {
		writeError(w, 503, "cloudflare_tunnel_config_unavailable", "A configuração do tunnel não está disponível.")
		return
	}
	v, err := c.TunnelConfiguration(r.Context(), a, id)
	if err != nil {
		writeError(w, 502, "cloudflare_tunnel_config_failed", err.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (h *Handler) UpdateTunnelConfiguration(w http.ResponseWriter, r *http.Request) {
	a, id := r.PathValue("account_id"), r.PathValue("tunnel_id")
	c, ok := h.reader.(TunnelConfigurator)
	if !ok || !h.reader.AccountAllowed(a) {
		writeError(w, 503, "cloudflare_tunnel_config_unavailable", "A configuração do tunnel não está disponível.")
		return
	}
	var in struct {
		Ingress []cloudflareadapter.TunnelIngressRule `json:"ingress"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&in); err != nil {
		writeError(w, 400, "invalid_request", "Ingress inválido.")
		return
	}
	v, err := c.UpdateTunnelConfiguration(r.Context(), a, id, in.Ingress)
	if err != nil {
		writeError(w, 502, "cloudflare_tunnel_config_failed", err.Error())
		return
	}
	h.emit(r.Context(), AuditEvent{Action: "cloudflare.tunnel.configuration.update", ResourceType: "cloudflare_tunnel", TargetName: id, Decision: "allowed", Payload: map[string]any{"account_id": a, "rules": len(in.Ingress)}})
	writeJSON(w, 200, v)
}
func (h *Handler) PrivateRoutes(w http.ResponseWriter, r *http.Request) {
	a := r.PathValue("account_id")
	m, ok := h.reader.(PrivateRouteManager)
	if !ok || !h.reader.AccountAllowed(a) {
		writeError(w, 503, "cloudflare_private_routes_unavailable", "Rotas privadas não estão disponíveis.")
		return
	}
	items, err := m.ListPrivateRoutes(r.Context(), a)
	if err != nil {
		writeError(w, 502, "cloudflare_private_routes_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (h *Handler) CreatePrivateRoute(w http.ResponseWriter, r *http.Request) {
	a := r.PathValue("account_id")
	m, ok := h.reader.(PrivateRouteManager)
	if !ok || !h.reader.AccountAllowed(a) {
		writeError(w, 503, "cloudflare_private_routes_unavailable", "Rotas privadas não estão disponíveis.")
		return
	}
	var in cloudflareadapter.PrivateRouteInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeError(w, 400, "invalid_request", "Rota privada inválida.")
		return
	}
	item, err := m.CreatePrivateRoute(r.Context(), a, in)
	if err != nil {
		writeError(w, 502, "cloudflare_private_route_failed", err.Error())
		return
	}
	h.emit(r.Context(), AuditEvent{Action: "cloudflare.private_route.create", ResourceType: "cloudflare_private_route", TargetName: item.Network, Decision: "allowed"})
	writeJSON(w, 201, item)
}

func (h *Handler) DNSRecords(w http.ResponseWriter, request *http.Request) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed || !h.reader.ZoneAllowed(zoneID) {
		h.emit(request.Context(), AuditEvent{Action: "cloudflare.dns.read", ResourceType: "cloudflare_zone", TargetName: zoneID, Decision: "denied", Reason: "zone is not allowlisted"})
		writeError(w, http.StatusForbidden, "zone_not_allowed", "The requested Cloudflare zone is not allowlisted.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	records, err := h.reader.ListDNSRecords(ctx, zoneID)
	if err != nil {
		h.emit(ctx, AuditEvent{Action: "cloudflare.dns.read", ResourceType: "cloudflare_zone", TargetName: zoneID, Decision: "failed", Reason: "provider inventory unavailable"})
		writeProviderError(w)
		return
	}
	h.emit(ctx, AuditEvent{Action: "cloudflare.dns.read", ResourceType: "cloudflare_zone", TargetName: zoneID, Decision: "allowed", Reason: "read-only allowlisted request", Payload: map[string]any{"item_count": len(records)}})
	writeJSON(w, http.StatusOK, map[string]any{"items": records})
}

func (h *Handler) CreateDNSRecord(w http.ResponseWriter, request *http.Request) {
	h.writeDNSRecord(w, request, false)
}

func (h *Handler) UpdateDNSRecord(w http.ResponseWriter, request *http.Request) {
	h.writeDNSRecord(w, request, true)
}

func (h *Handler) writeDNSRecord(w http.ResponseWriter, request *http.Request, update bool) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed || !h.reader.ZoneAllowed(zoneID) {
		writeError(w, 403, "zone_not_allowed", "The requested Cloudflare zone is not allowlisted.")
		return
	}
	writer, ok := h.reader.(DNSWriter)
	if !ok {
		writeError(w, 503, "cloudflare_write_unavailable", "Cloudflare mutation access is not configured.")
		return
	}
	var input cloudflareadapter.DNSRecordInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "DNS record payload is invalid.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	var record cloudflareadapter.DNSRecord
	var err error
	action := "cloudflare.dns.create"
	if update {
		action = "cloudflare.dns.update"
		record, err = writer.UpdateDNSRecord(ctx, zoneID, request.PathValue("record_id"), input)
	} else {
		record, err = writer.CreateDNSRecord(ctx, zoneID, input)
	}
	if err != nil {
		h.emit(ctx, AuditEvent{Action: action, ResourceType: "cloudflare_dns_record", TargetName: input.Name, Decision: "failed", Reason: "provider rejected mutation"})
		writeError(w, 502, "cloudflare_dns_write_failed", "Cloudflare rejected the DNS change.")
		return
	}
	h.emit(ctx, AuditEvent{Action: action, ResourceType: "cloudflare_dns_record", TargetName: record.Name, Decision: "allowed", Reason: "allowlisted zone mutation", Payload: map[string]any{"zone_id": zoneID, "record_id": record.ID, "type": record.Type}})
	writeJSON(w, map[bool]int{false: 201, true: 200}[update], record)
}

func (h *Handler) DeleteDNSRecord(w http.ResponseWriter, request *http.Request) {
	zoneID, recordID := request.PathValue("zone_id"), request.PathValue("record_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed || !h.reader.ZoneAllowed(zoneID) {
		writeError(w, 403, "zone_not_allowed", "The requested Cloudflare zone is not allowlisted.")
		return
	}
	writer, ok := h.reader.(DNSWriter)
	if !ok {
		writeError(w, 503, "cloudflare_write_unavailable", "Cloudflare mutation access is not configured.")
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	if err := writer.DeleteDNSRecord(ctx, zoneID, recordID); err != nil {
		h.emit(ctx, AuditEvent{Action: "cloudflare.dns.delete", ResourceType: "cloudflare_dns_record", TargetName: recordID, Decision: "failed", Reason: "provider rejected mutation"})
		writeError(w, 502, "cloudflare_dns_delete_failed", "Cloudflare rejected the DNS deletion.")
		return
	}
	h.emit(ctx, AuditEvent{Action: "cloudflare.dns.delete", ResourceType: "cloudflare_dns_record", TargetName: recordID, Decision: "allowed", Reason: "allowlisted zone mutation", Payload: map[string]any{"zone_id": zoneID}})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ZoneSettings(w http.ResponseWriter, request *http.Request) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed {
		writeError(w, 403, "zone_not_allowed", "A zona solicitada não está autorizada.")
		return
	}
	admin, ok := h.reader.(ZoneAdministrator)
	if !ok {
		writeError(w, 503, "cloudflare_admin_unavailable", "A administração da zona não está configurada.")
		return
	}
	items, err := admin.ListZoneSettings(request.Context(), zoneID)
	if err != nil {
		writeProviderError(w)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (h *Handler) UpdateZoneSetting(w http.ResponseWriter, request *http.Request) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed {
		writeError(w, 403, "zone_not_allowed", "A zona solicitada não está autorizada.")
		return
	}
	admin, ok := h.reader.(ZoneAdministrator)
	if !ok {
		writeError(w, 503, "cloudflare_admin_unavailable", "A administração da zona não está configurada.")
		return
	}
	var input struct {
		Value any `json:"value"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "O valor da configuração é inválido.")
		return
	}
	item, err := admin.UpdateZoneSetting(request.Context(), zoneID, request.PathValue("setting"), input.Value)
	if err != nil {
		writeError(w, 502, "cloudflare_setting_failed", "A Cloudflare rejeitou a configuração.")
		return
	}
	h.emit(request.Context(), AuditEvent{Action: "cloudflare.zone.setting.update", ResourceType: "cloudflare_zone", TargetName: zoneID, Decision: "allowed", Payload: map[string]any{"setting": request.PathValue("setting")}})
	writeJSON(w, 200, item)
}
func (h *Handler) PurgeCache(w http.ResponseWriter, request *http.Request) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed {
		writeError(w, 403, "zone_not_allowed", "A zona solicitada não está autorizada.")
		return
	}
	admin, ok := h.reader.(ZoneAdministrator)
	if !ok {
		writeError(w, 503, "cloudflare_admin_unavailable", "A administração da zona não está configurada.")
		return
	}
	if err := admin.PurgeCache(request.Context(), zoneID); err != nil {
		writeError(w, 502, "cloudflare_purge_failed", "A Cloudflare rejeitou a limpeza de cache.")
		return
	}
	h.emit(request.Context(), AuditEvent{Action: "cloudflare.zone.cache.purge", ResourceType: "cloudflare_zone", TargetName: zoneID, Decision: "allowed"})
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (h *Handler) Rulesets(w http.ResponseWriter, request *http.Request) {
	zoneID := request.PathValue("zone_id")
	if _, allowed := h.allowedZones[zoneID]; !allowed {
		writeError(w, 403, "zone_not_allowed", "A zona solicitada não está autorizada.")
		return
	}
	admin, ok := h.reader.(ZoneAdministrator)
	if !ok {
		writeError(w, 503, "cloudflare_admin_unavailable", "A administração da zona não está configurada.")
		return
	}
	items, err := admin.ListRulesets(request.Context(), zoneID)
	if err != nil {
		writeProviderError(w)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	items, err := client.ListWorkers(r.Context(), accountID)
	if err != nil {
		writeError(w, 502, "cloudflare_workers_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (h *Handler) UploadWorker(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	name := r.PathValue("name")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Code) == "" {
		writeError(w, 400, "invalid_request", "Código JavaScript do Worker é obrigatório.")
		return
	}
	if err := client.UploadWorker(r.Context(), accountID, name, input.Code); err != nil {
		writeError(w, 502, "upload_worker_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "uploaded", "name": name})
}

func (h *Handler) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	name := r.PathValue("name")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	if err := client.DeleteWorker(r.Context(), accountID, name); err != nil {
		writeError(w, 502, "delete_worker_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "name": name})
}

func (h *Handler) ListKVNamespaces(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	items, err := client.ListKVNamespaces(r.Context(), accountID)
	if err != nil {
		writeError(w, 502, "cloudflare_kv_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (h *Handler) CreateKVNamespace(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	var input struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Title) == "" {
		writeError(w, 400, "invalid_request", "Título do KV Namespace é obrigatório.")
		return
	}
	id, err := client.CreateKVNamespace(r.Context(), accountID, input.Title)
	if err != nil {
		writeError(w, 502, "create_kv_failed", err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created", "id": id, "title": input.Title})
}

func (h *Handler) DeleteKVNamespace(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account_id")
	id := r.PathValue("id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	if err := client.DeleteKVNamespace(r.Context(), accountID, id); err != nil {
		writeError(w, 502, "delete_kv_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "id": id})
}

func (h *Handler) ListWAFRules(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zone_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	items, err := client.ListWAFRules(r.Context(), zoneID)
	if err != nil {
		writeError(w, 502, "cloudflare_waf_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (h *Handler) CreateWAFRule(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zone_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	var input cloudflareadapter.CreateWAFRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "Dados da regra WAF são inválidos.")
		return
	}
	id, err := client.CreateWAFRule(r.Context(), zoneID, input)
	if err != nil {
		writeError(w, 502, "create_waf_failed", err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created", "id": id})
}

func (h *Handler) DeleteWAFRule(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zone_id")
	id := r.PathValue("id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	if err := client.DeleteWAFRule(r.Context(), zoneID, id); err != nil {
		writeError(w, 502, "delete_waf_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "id": id})
}

func (h *Handler) ZoneAnalytics(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zone_id")
	client, ok := h.reader.(*cloudflareadapter.Client)
	if !ok || client == nil {
		writeError(w, 503, "cloudflare_unavailable", "Cloudflare adapter client unavailable.")
		return
	}
	analytics, err := client.GetZoneAnalytics(r.Context(), zoneID)
	if err != nil {
		writeError(w, 502, "cloudflare_analytics_failed", err.Error())
		return
	}
	writeJSON(w, 200, analytics)
}

func (h *Handler) emit(ctx context.Context, event AuditEvent) {
	if h.audit != nil {
		h.audit(ctx, event)
	}
}

func set(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func unavailableTarget(kind, id string) TargetHealth {
	return TargetHealth{Kind: kind, ID: id, Status: "unavailable", Message: "Provider inventory could not be loaded."}
}

func writeProviderError(w http.ResponseWriter) {
	writeError(w, http.StatusBadGateway, "provider_unavailable", "Cloudflare inventory is temporarily unavailable.")
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
