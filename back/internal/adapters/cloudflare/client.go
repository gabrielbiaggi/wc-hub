// Package cloudflare implements the read-only Cloudflare API boundary.
//
// The client deliberately exposes no mutation methods. Account and zone
// allowlists are enforced before a request is built, and the API token only
// exists in plaintext while do is executing.
package cloudflare

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.cloudflare.com/client/v4"

var externalIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

var (
	ErrAccountNotAllowed = errors.New("cloudflare account is not allowlisted")
	ErrZoneNotAllowed    = errors.New("cloudflare zone is not allowlisted")
	ErrUnauthorized      = errors.New("cloudflare credential was rejected")
	ErrUpstream          = errors.New("cloudflare API request failed")
)

// TokenEnvelope is the persisted envelope-encrypted representation of the
// scoped API token. The data-encryption key is wrapped separately from the
// credential ciphertext so plaintext credentials never need to be stored.
type TokenEnvelope struct {
	WrappedKey      []byte `json:"wrapped_key"`
	KeyNonce        []byte `json:"key_nonce"`
	Ciphertext      []byte `json:"ciphertext"`
	CredentialNonce []byte `json:"credential_nonce"`
}

// CredentialSource loads only encrypted credential material. Implementations
// may read this envelope from the integrations repository or a secret store.
type CredentialSource interface {
	EncryptedToken(context.Context) (TokenEnvelope, error)
}

type CredentialSourceFunc func(context.Context) (TokenEnvelope, error)

func (f CredentialSourceFunc) EncryptedToken(ctx context.Context) (TokenEnvelope, error) {
	return f(ctx)
}

// EnvelopeDecryptor unwraps a credential at the immediate adapter boundary.
// Implementations must return an owned byte slice because the client zeroes it
// after every request.
type EnvelopeDecryptor interface {
	Decrypt(context.Context, TokenEnvelope) ([]byte, error)
}

// AESGCMEnvelopeDecryptor unwraps a random 256-bit DEK using a 256-bit KEK and
// then decrypts the credential with that DEK.
type AESGCMEnvelopeDecryptor struct {
	kek []byte
}

func NewAESGCMEnvelopeDecryptor(kek []byte) (*AESGCMEnvelopeDecryptor, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("Cloudflare envelope KEK must contain 32 bytes")
	}
	return &AESGCMEnvelopeDecryptor{kek: append([]byte(nil), kek...)}, nil
}

func (d *AESGCMEnvelopeDecryptor) Decrypt(_ context.Context, envelope TokenEnvelope) ([]byte, error) {
	if d == nil || len(d.kek) != 32 {
		return nil, fmt.Errorf("Cloudflare envelope decryptor is not configured")
	}
	keyAEAD, err := newGCM(d.kek)
	if err != nil {
		return nil, fmt.Errorf("initialize Cloudflare key decryptor: %w", err)
	}
	if len(envelope.KeyNonce) != keyAEAD.NonceSize() || len(envelope.WrappedKey) == 0 {
		return nil, fmt.Errorf("invalid Cloudflare key envelope")
	}
	dek, err := keyAEAD.Open(nil, envelope.KeyNonce, envelope.WrappedKey, []byte("wc-hub/cloudflare/dek/v1"))
	if err != nil {
		return nil, fmt.Errorf("decrypt Cloudflare key envelope")
	}
	defer zero(dek)
	if len(dek) != 32 {
		return nil, fmt.Errorf("invalid Cloudflare data key")
	}
	credentialAEAD, err := newGCM(dek)
	if err != nil {
		return nil, fmt.Errorf("initialize Cloudflare credential decryptor: %w", err)
	}
	if len(envelope.CredentialNonce) != credentialAEAD.NonceSize() || len(envelope.Ciphertext) == 0 {
		return nil, fmt.Errorf("invalid Cloudflare credential envelope")
	}
	plaintext, err := credentialAEAD.Open(nil, envelope.CredentialNonce, envelope.Ciphertext, []byte("wc-hub/cloudflare/token/v1"))
	if err != nil {
		return nil, fmt.Errorf("decrypt Cloudflare credential envelope")
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type Config struct {
	CredentialSource CredentialSource
	Decryptor        EnvelopeDecryptor
	AllowedAccounts  []string
	AllowedZones     []string
	HTTPClient       *http.Client
	// BaseURL exists for controlled integration tests. Production callers
	// should leave it empty so the fixed Cloudflare endpoint is used.
	BaseURL string
}

type Client struct {
	baseURL         *url.URL
	credentials     CredentialSource
	decryptor       EnvelopeDecryptor
	allowedAccounts map[string]struct{}
	allowedZones    map[string]struct{}
	http            *http.Client
}

type TunnelConnection struct {
	Colocation    string    `json:"colocation"`
	ConnectionID  string    `json:"connection_id"`
	ClientVersion string    `json:"client_version"`
	OpenedAt      time.Time `json:"opened_at"`
	Pending       bool      `json:"pending"`
}

type Tunnel struct {
	ID          string             `json:"id"`
	AccountID   string             `json:"account_id"`
	Name        string             `json:"name"`
	Status      string             `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
	Connections []TunnelConnection `json:"connections"`
}

type DNSRecord struct {
	ID         string    `json:"id"`
	ZoneID     string    `json:"zone_id"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	Proxied    bool      `json:"proxied"`
	Proxiable  bool      `json:"proxiable"`
	TTL        int       `json:"ttl"`
	Comment    string    `json:"comment,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
}

type ClientError struct {
	StatusCode int
	Code       int
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("cloudflare API request failed (status=%d, code=%d)", e.StatusCode, e.Code)
}

func (e *ClientError) Unwrap() error {
	if e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	return ErrUpstream
}

func New(config Config) (*Client, error) {
	if config.CredentialSource == nil || config.Decryptor == nil {
		return nil, fmt.Errorf("Cloudflare encrypted credential source and decryptor are required")
	}
	base := strings.TrimRight(config.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("Cloudflare API URL must be an absolute HTTPS URL without user info")
	}
	if !strings.EqualFold(parsed.Hostname(), "api.cloudflare.com") {
		return nil, fmt.Errorf("Cloudflare API URL host must be api.cloudflare.com")
	}
	accounts, err := buildAllowlist(config.AllowedAccounts)
	if err != nil {
		return nil, fmt.Errorf("invalid Cloudflare account allowlist: %w", err)
	}
	zones, err := buildAllowlist(config.AllowedZones)
	if err != nil {
		return nil, fmt.Errorf("invalid Cloudflare zone allowlist: %w", err)
	}
	if len(accounts) == 0 && len(zones) == 0 {
		return nil, fmt.Errorf("at least one Cloudflare account or zone must be allowlisted")
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
				MaxIdleConns:       10,
				IdleConnTimeout:    60 * time.Second,
				DisableCompression: false,
			},
		}
	} else {
		copyClient := *httpClient
		httpClient = &copyClient
		if httpClient.Timeout <= 0 || httpClient.Timeout > 30*time.Second {
			httpClient.Timeout = 20 * time.Second
		}
	}
	previousRedirect := httpClient.CheckRedirect
	httpClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "https" || !strings.EqualFold(request.URL.Host, parsed.Host) {
			return fmt.Errorf("Cloudflare redirect target rejected")
		}
		if previousRedirect != nil {
			return previousRedirect(request, via)
		}
		if len(via) >= 5 {
			return fmt.Errorf("too many Cloudflare redirects")
		}
		return nil
	}

	return &Client{
		baseURL:         parsed,
		credentials:     config.CredentialSource,
		decryptor:       config.Decryptor,
		allowedAccounts: accounts,
		allowedZones:    zones,
		http:            httpClient,
	}, nil
}

func (c *Client) AllowedAccounts() []string { return allowlistValues(c.allowedAccounts) }
func (c *Client) AllowedZones() []string    { return allowlistValues(c.allowedZones) }

func (c *Client) AccountAllowed(accountID string) bool {
	_, ok := c.allowedAccounts[accountID]
	return ok
}

func (c *Client) ZoneAllowed(zoneID string) bool {
	_, ok := c.allowedZones[zoneID]
	return ok
}

func (c *Client) ListTunnels(ctx context.Context, accountID string) ([]Tunnel, error) {
	if !c.AccountAllowed(accountID) {
		return nil, ErrAccountNotAllowed
	}
	path := "/accounts/" + url.PathEscape(accountID) + "/tunnels"
	query := url.Values{"is_deleted": {"false"}, "per_page": {"1000"}}
	var tunnels []tunnelWire
	if err := c.list(ctx, path, query, &tunnels); err != nil {
		return nil, err
	}
	result := make([]Tunnel, 0, len(tunnels))
	for _, item := range tunnels {
		connections := make([]TunnelConnection, 0, len(item.Connections))
		for _, connection := range item.Connections {
			connections = append(connections, TunnelConnection{
				Colocation:    connection.Colocation,
				ConnectionID:  connection.ConnectionID,
				ClientVersion: connection.ClientVersion,
				OpenedAt:      connection.OpenedAt,
				Pending:       connection.Pending,
			})
		}
		result = append(result, Tunnel{
			ID:          item.ID,
			AccountID:   accountID,
			Name:        item.Name,
			Status:      normalizeTunnelStatus(item.Status),
			CreatedAt:   item.CreatedAt,
			Connections: connections,
		})
	}
	return result, nil
}

func (c *Client) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	if !c.ZoneAllowed(zoneID) {
		return nil, ErrZoneNotAllowed
	}
	path := "/zones/" + url.PathEscape(zoneID) + "/dns_records"
	query := url.Values{"per_page": {"1000"}}
	var records []dnsRecordWire
	if err := c.list(ctx, path, query, &records); err != nil {
		return nil, err
	}
	result := make([]DNSRecord, 0, len(records))
	for _, item := range records {
		result = append(result, DNSRecord{
			ID:         item.ID,
			ZoneID:     zoneID,
			Type:       item.Type,
			Name:       item.Name,
			Content:    item.Content,
			Proxied:    item.Proxied,
			Proxiable:  item.Proxiable,
			TTL:        item.TTL,
			Comment:    item.Comment,
			ModifiedAt: item.ModifiedAt,
		})
	}
	return result, nil
}

func (c *Client) list(ctx context.Context, path string, query url.Values, destination any) error {
	page := 1
	all := make([]json.RawMessage, 0)
	for {
		query.Set("page", fmt.Sprintf("%d", page))
		var items []json.RawMessage
		info, err := c.do(ctx, path, query, &items)
		if err != nil {
			return err
		}
		all = append(all, items...)
		perPage := 1000
		if parsed, parseErr := strconv.Atoi(query.Get("per_page")); parseErr == nil && parsed > 0 {
			perPage = parsed
		}
		if len(items) == 0 || (info.TotalCount > 0 && len(all) >= info.TotalCount) || (info.TotalPages > 0 && info.TotalPages <= page) || len(items) < perPage {
			break
		}
		page++
		if page > 100 {
			return fmt.Errorf("Cloudflare pagination limit exceeded: %w", ErrUpstream)
		}
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		return fmt.Errorf("prepare Cloudflare response: %w", ErrUpstream)
	}
	if err = json.Unmarshal(encoded, destination); err != nil {
		return fmt.Errorf("decode Cloudflare response: %w", ErrUpstream)
	}
	return nil
}

func (c *Client) do(ctx context.Context, path string, query url.Values, destination any) (resultInfo, error) {
	envelope, err := c.credentials.EncryptedToken(ctx)
	if err != nil {
		return resultInfo{}, fmt.Errorf("load encrypted Cloudflare credential")
	}
	token, err := c.decryptor.Decrypt(ctx, envelope)
	if err != nil {
		return resultInfo{}, fmt.Errorf("decrypt Cloudflare credential")
	}
	defer zero(token)
	if len(token) < 20 || len(token) > 256 {
		return resultInfo{}, fmt.Errorf("invalid Cloudflare credential")
	}

	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return resultInfo{}, fmt.Errorf("build Cloudflare request: %w", ErrUpstream)
	}
	request.Header.Set("Authorization", "Bearer "+string(token))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "wc-hub/cloudflare-readonly")
	defer request.Header.Del("Authorization")

	response, err := c.http.Do(request)
	if err != nil {
		return resultInfo{}, fmt.Errorf("send Cloudflare request: %w", ErrUpstream)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return resultInfo{}, fmt.Errorf("read Cloudflare response: %w", ErrUpstream)
	}
	var decoded apiEnvelope[json.RawMessage]
	if err = json.Unmarshal(body, &decoded); err != nil {
		return resultInfo{}, fmt.Errorf("decode Cloudflare envelope: %w", ErrUpstream)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !decoded.Success {
		code := 0
		if len(decoded.Errors) > 0 {
			code = decoded.Errors[0].Code
		}
		return resultInfo{}, &ClientError{StatusCode: response.StatusCode, Code: code}
	}
	if err = json.Unmarshal(decoded.Result, destination); err != nil {
		return resultInfo{}, fmt.Errorf("decode Cloudflare result: %w", ErrUpstream)
	}
	return decoded.ResultInfo, nil
}

type apiEnvelope[T any] struct {
	Success    bool       `json:"success"`
	Errors     []apiError `json:"errors"`
	Result     T          `json:"result"`
	ResultInfo resultInfo `json:"result_info"`
}

type apiError struct {
	Code int `json:"code"`
}

type resultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

type tunnelWire struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	Connections []tunnelConnectionWire `json:"connections"`
}

type tunnelConnectionWire struct {
	Colocation    string    `json:"colo_name"`
	ConnectionID  string    `json:"uuid"`
	ClientVersion string    `json:"client_version"`
	OpenedAt      time.Time `json:"opened_at"`
	Pending       bool      `json:"is_pending_reconnect"`
}

type dnsRecordWire struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	Proxied    bool      `json:"proxied"`
	Proxiable  bool      `json:"proxiable"`
	TTL        int       `json:"ttl"`
	Comment    string    `json:"comment"`
	ModifiedAt time.Time `json:"modified_on"`
}

func buildAllowlist(values []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !externalIDPattern.MatchString(value) {
			return nil, fmt.Errorf("identifier contains unsupported characters")
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func allowlistValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func normalizeTunnelStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy", "degraded", "down", "inactive":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "unknown"
	}
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
