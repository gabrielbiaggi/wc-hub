package proxmox

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL, tokenID string
	secret           []byte
	http             *http.Client
}
type Node struct {
	Node      string  `json:"node"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Memory    int64   `json:"mem"`
	MaxMemory int64   `json:"maxmem"`
	Uptime    int64   `json:"uptime"`
	Level     string  `json:"level"`
}
type VM struct {
	VMID      int         `json:"vmid"`
	Name      string      `json:"name"`
	Status    string      `json:"status"`
	CPU       float64     `json:"cpu"`
	CPUs      int         `json:"cpus"`
	Memory    int64       `json:"mem"`
	MaxMemory int64       `json:"maxmem"`
	MaxDisk   int64       `json:"maxdisk"`
	Uptime    int64       `json:"uptime"`
	Node      string      `json:"node,omitempty"`
	Type      string      `json:"type,omitempty"`
	Template  json.Number `json:"template,omitempty"`
}
type Storage struct {
	Storage   string `json:"storage"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Active    int    `json:"active"`
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
	Available int64  `json:"avail"`
	Shared    int    `json:"shared"`
	Node      string `json:"node,omitempty"`
}
type Snapshot struct {
	CapturedAt time.Time `json:"captured_at"`
	Nodes      []Node    `json:"nodes"`
	VMs        []VM      `json:"virtual_machines"`
	Containers []VM      `json:"containers"`
	Storage    []Storage `json:"storage"`
}
type envelope[T any] struct {
	Data T `json:"data"`
}

var nodeNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

func New(baseURL, tokenID string, secret []byte, caPath string) (*Client, error) {
	if baseURL == "" || tokenID == "" || len(secret) == 0 {
		return nil, fmt.Errorf("Proxmox URL and API token are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("Proxmox API URL must use HTTPS")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read Proxmox CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("invalid Proxmox CA bundle")
		}
		tlsConfig.RootCAs = pool
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig, MaxIdleConns: 20, IdleConnTimeout: 60 * time.Second}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), tokenID: tokenID, secret: append([]byte(nil), secret...), http: &http.Client{Timeout: 20 * time.Second, Transport: transport}}, nil
}

// NewFromEnvFile loads an additional cluster from a root-readable secret file.
// It intentionally accepts only the four Proxmox variables and never exposes
// their values through the API or logs.
func NewFromEnvFile(path string) (*Client, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Proxmox cluster config: %w", err)
	}
	defer file.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(io.LimitReader(file, 64<<10))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "PROXMOX_API_URL" || key == "PROXMOX_API_TOKEN_ID" || key == "PROXMOX_API_TOKEN_SECRET" || key == "PROXMOX_TLS_CA_PATH" {
			values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Proxmox cluster config: %w", err)
	}
	return New(values["PROXMOX_API_URL"], values["PROXMOX_API_TOKEN_ID"], []byte(values["PROXMOX_API_TOKEN_SECRET"]), values["PROXMOX_TLS_CA_PATH"])
}
func (c *Client) Configured() bool { return c != nil && c.baseURL != "" }
func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	result.CapturedAt = time.Now().UTC()
	if err := c.get(ctx, "/api2/json/nodes", &result.Nodes); err != nil {
		return result, err
	}
	for _, node := range result.Nodes {
		var qemu, lxc []VM
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/qemu", &qemu); err != nil {
			return result, err
		}
		for i := range qemu {
			qemu[i].Node = node.Node
			qemu[i].Type = "qemu"
		}
		result.VMs = append(result.VMs, qemu...)
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/lxc", &lxc); err != nil {
			return result, err
		}
		for i := range lxc {
			lxc[i].Node = node.Node
			lxc[i].Type = "lxc"
		}
		result.Containers = append(result.Containers, lxc...)
		var stores []Storage
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/storage", &stores); err != nil {
			return result, err
		}
		for i := range stores {
			stores[i].Node = node.Node
		}
		result.Storage = append(result.Storage, stores...)
	}
	return result, nil
}
func (c *Client) Version(ctx context.Context) (map[string]any, error) {
	var result map[string]any
	err := c.get(ctx, "/api2/json/version", &result)
	return result, err
}
func (c *Client) PowerAction(ctx context.Context, node, kind string, vmid int, action string) error {
	if !nodeNamePattern.MatchString(node) || vmid < 1 || vmid > 999999999 {
		return fmt.Errorf("invalid Proxmox resource")
	}
	if kind != "qemu" && kind != "lxc" {
		return fmt.Errorf("invalid Proxmox resource type")
	}
	allowed := map[string]bool{"start": true, "stop": true, "shutdown": true, "reboot": true}
	if kind == "qemu" {
		allowed["reset"] = true
	}
	if !allowed[action] {
		return fmt.Errorf("unsupported Proxmox power action")
	}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/status/%s", url.PathEscape(node), kind, vmid, action), nil)
}
func (c *Client) get(ctx context.Context, path string, destination any) error {
	return c.request(ctx, http.MethodGet, path, destination)
}
func (c *Client) request(ctx context.Context, method, path string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+string(c.secret))
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("proxmox request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("proxmox %s returned %d: %s", path, response.StatusCode, sanitize(body))
	}
	if destination == nil || len(body) == 0 {
		return nil
	}
	wrapper := envelope[json.RawMessage]{}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err = decoder.Decode(&wrapper); err != nil {
		return fmt.Errorf("decode Proxmox envelope: %w", err)
	}
	decoder = json.NewDecoder(strings.NewReader(string(wrapper.Data)))
	decoder.UseNumber()
	if err = decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode Proxmox data: %w", err)
	}
	return nil
}
func sanitize(body []byte) string {
	value := strings.ReplaceAll(string(body), "\n", " ")
	if len(value) > 240 {
		value = value[:240]
	}
	return strconv.QuoteToASCII(value)
}
