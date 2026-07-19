package cloudflareapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
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
