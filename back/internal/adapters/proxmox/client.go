package proxmox

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
func (c *Client) get(ctx context.Context, path string, destination any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
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
