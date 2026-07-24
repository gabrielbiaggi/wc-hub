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
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	baseURL, tokenID string
	clusterName      string
	secret           []byte
	http             *http.Client
	tlsConfig        *tls.Config
}
type Config struct {
	URL                string
	ClusterName        string
	TokenID            string
	Secret             []byte
	CAPath             string
	InsecureSkipVerify bool
}
type Node struct {
	Cluster   string      `json:"cluster"`
	Node      string      `json:"node"`
	Status    string      `json:"status"`
	CPU       float64     `json:"cpu"`
	MaxCPU    int         `json:"maxcpu"`
	Memory    int64       `json:"mem"`
	MaxMemory int64       `json:"maxmem"`
	Uptime    int64       `json:"uptime"`
	Level     string      `json:"level"`
	Metrics   NodeMetrics `json:"metrics"`
}

// NodeMetrics is the latest point from Proxmox RRD. The regular /nodes
// endpoint intentionally omits live utilisation for restricted API tokens;
// the RRD endpoint remains read-only and gives a stable operational sample.
type NodeMetrics struct {
	CPU             float64 `json:"cpu_ratio"`
	Load1           float64 `json:"load1"`
	MemoryTotal     int64   `json:"memory_total_bytes"`
	MemoryAvailable int64   `json:"memory_available_bytes"`
	RootTotal       int64   `json:"root_total_bytes"`
	RootUsed        int64   `json:"root_used_bytes"`
	NetworkInBPS    float64 `json:"network_in_bytes_per_second"`
	NetworkOutBPS   float64 `json:"network_out_bytes_per_second"`
	IOWaitRatio     float64 `json:"io_wait_ratio"`
}
type VM struct {
	Cluster   string      `json:"cluster"`
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
	Cluster   string `json:"cluster"`
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
	Warnings   []string  `json:"warnings"`
}
type GuestSnapshot struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Snaptime    int64  `json:"snaptime"`
	VMState     string `json:"vmstate,omitempty"`
}
type CreateQEMUInput struct {
	Cluster  string `json:"cluster"`
	Node     string `json:"node"`
	VMID     int    `json:"vmid"`
	Name     string `json:"name"`
	Cores    int    `json:"cores"`
	MemoryMB int    `json:"memory_mb"`
	Storage  string `json:"storage"`
	DiskGB   int    `json:"disk_gb"`
	ISO      string `json:"iso"`
	Bridge   string `json:"bridge"`
	Start    bool   `json:"start"`
}
type CreateLXCInput struct {
	Cluster       string `json:"cluster"`
	Node          string `json:"node"`
	VMID          int    `json:"vmid"`
	Hostname      string `json:"hostname"`
	Cores         int    `json:"cores"`
	MemoryMB      int    `json:"memory_mb"`
	Storage       string `json:"storage"`
	RootFSGB      int    `json:"rootfs_gb"`
	Template      string `json:"template"`
	Bridge        string `json:"bridge"`
	Password      string `json:"password"`
	SSHPublicKeys string `json:"ssh_public_keys"`
	Unprivileged  bool   `json:"unprivileged"`
	Start         bool   `json:"start"`
}
type CloneInput struct {
	Cluster    string `json:"cluster"`
	Node       string `json:"node"`
	Kind       string `json:"kind"`
	SourceVMID int    `json:"source_vmid"`
	NewVMID    int    `json:"new_vmid"`
	Name       string `json:"name"`
	TargetNode string `json:"target_node"`
	Storage    string `json:"storage"`
	Full       bool   `json:"full"`
}
type envelope[T any] struct {
	Data T `json:"data"`
}

var (
	nodeNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
	tokenIDPattern  = regexp.MustCompile(`^[^@\s]+@[A-Za-z0-9_.-]+![A-Za-z0-9_.-]+$`)
)

func New(baseURL, tokenID string, secret []byte, caPath string) (*Client, error) {
	return NewWithConfig(Config{URL: baseURL, TokenID: tokenID, Secret: secret, CAPath: caPath})
}

func NewWithConfig(config Config) (*Client, error) {
	if config.URL == "" || config.TokenID == "" || len(config.Secret) == 0 {
		return nil, fmt.Errorf("Proxmox URL and API token are required")
	}
	if !tokenIDPattern.MatchString(config.TokenID) {
		return nil, fmt.Errorf("Proxmox token ID must use USER@REALM!TOKENID format")
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("Proxmox API URL must use HTTPS")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.InsecureSkipVerify} // #nosec G402 -- explicitly enabled only for homelab certificates.
	if config.CAPath != "" {
		pem, err := os.ReadFile(config.CAPath)
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
	return &Client{baseURL: strings.TrimRight(config.URL, "/"), clusterName: strings.TrimSpace(config.ClusterName), tokenID: config.TokenID, secret: append([]byte(nil), config.Secret...), http: &http.Client{Timeout: 20 * time.Second, Transport: transport}, tlsConfig: tlsConfig}, nil
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
		if key == "PROXMOX_API_URL" || key == "PROXMOX_CLUSTER_NAME" || key == "PROXMOX_API_TOKEN_ID" || key == "PROXMOX_API_TOKEN_SECRET" || key == "PROXMOX_TLS_CA_PATH" || key == "PROXMOX_TLS_INSECURE_SKIP_VERIFY" {
			values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Proxmox cluster config: %w", err)
	}
	insecure := false
	if raw := values["PROXMOX_TLS_INSECURE_SKIP_VERIFY"]; raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse PROXMOX_TLS_INSECURE_SKIP_VERIFY: %w", parseErr)
		}
		insecure = parsed
	}
	return NewWithConfig(Config{URL: values["PROXMOX_API_URL"], ClusterName: values["PROXMOX_CLUSTER_NAME"], TokenID: values["PROXMOX_API_TOKEN_ID"], Secret: []byte(values["PROXMOX_API_TOKEN_SECRET"]), CAPath: values["PROXMOX_TLS_CA_PATH"], InsecureSkipVerify: insecure})
}
func (c *Client) Configured() bool { return c != nil && c.baseURL != "" }
func (c *Client) ID() string {
	if c == nil {
		return "proxmox"
	}
	if c.clusterName != "" {
		return c.clusterName
	}
	parsed, err := url.Parse(c.baseURL)
	if err != nil || parsed == nil || parsed.Hostname() == "" {
		return "proxmox"
	}
	return parsed.Hostname()
}
func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	result := Snapshot{
		CapturedAt: time.Now().UTC(),
		Nodes:      []Node{},
		VMs:        []VM{},
		Containers: []VM{},
		Storage:    []Storage{},
		Warnings:   []string{},
	}
	if err := c.get(ctx, "/api2/json/nodes", &result.Nodes); err != nil {
		return result, err
	}
	for i := range result.Nodes {
		result.Nodes[i].Cluster = c.ID()
	}
	for _, node := range result.Nodes {
		if metrics, metricsErr := c.nodeMetrics(ctx, node.Node); metricsErr != nil {
			result.Warnings = append(result.Warnings, "Proxmox "+node.Node+" RRD: "+metricsErr.Error())
		} else {
			for index := range result.Nodes {
				if result.Nodes[index].Node == node.Node {
					result.Nodes[index].Metrics = metrics
					if metrics.CPU > 0 || result.Nodes[index].CPU == 0 {
						result.Nodes[index].CPU = metrics.CPU
					}
					if metrics.MemoryTotal > 0 {
						result.Nodes[index].MaxMemory = metrics.MemoryTotal
					}
					if metrics.MemoryTotal > 0 && metrics.MemoryAvailable >= 0 {
						result.Nodes[index].Memory = metrics.MemoryTotal - metrics.MemoryAvailable
					}
					break
				}
			}
		}
		var qemu, lxc []VM
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/qemu", &qemu); err != nil {
			return result, err
		}
		for i := range qemu {
			qemu[i].Cluster = c.ID()
			qemu[i].Node = node.Node
			qemu[i].Type = "qemu"
		}
		result.VMs = append(result.VMs, qemu...)
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/lxc", &lxc); err != nil {
			return result, err
		}
		for i := range lxc {
			lxc[i].Cluster = c.ID()
			lxc[i].Node = node.Node
			lxc[i].Type = "lxc"
		}
		result.Containers = append(result.Containers, lxc...)
		var stores []Storage
		if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node.Node)+"/storage", &stores); err != nil {
			return result, err
		}
		for i := range stores {
			stores[i].Cluster = c.ID()
			stores[i].Node = node.Node
		}
		result.Storage = append(result.Storage, stores...)
	}
	return result, nil
}

type rrdPoint struct {
	Time         int64   `json:"time"`
	CPU          float64 `json:"cpu"`
	LoadAvg      float64 `json:"loadavg"`
	MemTotal     float64 `json:"memtotal"`
	MemAvailable float64 `json:"memavailable"`
	RootTotal    float64 `json:"roottotal"`
	RootUsed     float64 `json:"rootused"`
	NetIn        float64 `json:"netin"`
	NetOut       float64 `json:"netout"`
	IOWait       float64 `json:"iowait"`
}

func (c *Client) nodeMetrics(ctx context.Context, node string) (NodeMetrics, error) {
	points := []rrdPoint{}
	if err := c.get(ctx, "/api2/json/nodes/"+url.PathEscape(node)+"/rrddata?timeframe=hour", &points); err != nil {
		return NodeMetrics{}, err
	}
	if len(points) == 0 {
		return NodeMetrics{}, fmt.Errorf("no RRD points returned")
	}
	var latest rrdPoint
	found := false
	for _, point := range points {
		// The newest RRD bucket can be allocated before Proxmox has populated
		// it, producing a valid timestamp with every metric at zero. Never let
		// that partial bucket erase the last real observation in the dashboard.
		if !rrdPointHasSample(point) {
			continue
		}
		if !found || point.Time > latest.Time {
			latest = point
			found = true
		}
	}
	if !found {
		return NodeMetrics{}, fmt.Errorf("RRD returned no complete metric point")
	}
	return NodeMetrics{CPU: latest.CPU, Load1: latest.LoadAvg, MemoryTotal: int64(latest.MemTotal), MemoryAvailable: int64(latest.MemAvailable), RootTotal: int64(latest.RootTotal), RootUsed: int64(latest.RootUsed), NetworkInBPS: latest.NetIn, NetworkOutBPS: latest.NetOut, IOWaitRatio: latest.IOWait}, nil
}

func rrdPointHasSample(point rrdPoint) bool {
	return point.MemTotal > 0 || point.RootTotal > 0 || point.CPU > 0 || point.LoadAvg > 0 || point.NetIn > 0 || point.NetOut > 0
}

func (c *Client) CreateQEMU(ctx context.Context, input CreateQEMUInput) error {
	if !nodeNamePattern.MatchString(input.Node) || input.VMID < 1 || strings.TrimSpace(input.Name) == "" || input.Cores < 1 || input.MemoryMB < 128 || input.DiskGB < 1 || !nodeNamePattern.MatchString(input.Storage) {
		return fmt.Errorf("invalid QEMU configuration")
	}
	bridge := input.Bridge
	if bridge == "" {
		bridge = "vmbr0"
	}
	values := url.Values{"vmid": {strconv.Itoa(input.VMID)}, "name": {input.Name}, "cores": {strconv.Itoa(input.Cores)}, "memory": {strconv.Itoa(input.MemoryMB)}, "scsihw": {"virtio-scsi-single"}, "scsi0": {fmt.Sprintf("%s:%d,discard=on,iothread=1", input.Storage, input.DiskGB)}, "net0": {"virtio,bridge=" + bridge}, "ostype": {"l26"}}
	if strings.TrimSpace(input.ISO) != "" {
		values.Set("ide2", input.ISO+",media=cdrom")
		values.Set("boot", "order=scsi0;ide2")
	}
	if input.Start {
		values.Set("start", "1")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/qemu", url.PathEscape(input.Node)), values, nil)
}

func (c *Client) CreateLXC(ctx context.Context, input CreateLXCInput) error {
	if !nodeNamePattern.MatchString(input.Node) || input.VMID < 1 || strings.TrimSpace(input.Hostname) == "" || input.Cores < 1 || input.MemoryMB < 128 || input.RootFSGB < 1 || !nodeNamePattern.MatchString(input.Storage) || strings.TrimSpace(input.Template) == "" {
		return fmt.Errorf("invalid LXC configuration")
	}
	bridge := input.Bridge
	if bridge == "" {
		bridge = "vmbr0"
	}
	values := url.Values{"vmid": {strconv.Itoa(input.VMID)}, "hostname": {input.Hostname}, "cores": {strconv.Itoa(input.Cores)}, "memory": {strconv.Itoa(input.MemoryMB)}, "rootfs": {fmt.Sprintf("%s:%d", input.Storage, input.RootFSGB)}, "ostemplate": {input.Template}, "net0": {"name=eth0,bridge=" + bridge + ",ip=dhcp"}}
	if input.Unprivileged {
		values.Set("unprivileged", "1")
	}
	if input.Start {
		values.Set("start", "1")
	}
	if input.Password != "" {
		values.Set("password", input.Password)
	}
	if input.SSHPublicKeys != "" {
		values.Set("ssh-public-keys", input.SSHPublicKeys)
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/lxc", url.PathEscape(input.Node)), values, nil)
}

func (c *Client) Clone(ctx context.Context, input CloneInput) error {
	if !nodeNamePattern.MatchString(input.Node) || (input.Kind != "qemu" && input.Kind != "lxc") || input.SourceVMID < 1 || input.NewVMID < 1 || strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("invalid clone configuration")
	}
	values := url.Values{"newid": {strconv.Itoa(input.NewVMID)}, "name": {input.Name}}
	if input.Full {
		values.Set("full", "1")
	}
	if input.TargetNode != "" {
		values.Set("target", input.TargetNode)
	}
	if input.Storage != "" {
		values.Set("storage", input.Storage)
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/clone", url.PathEscape(input.Node), input.Kind, input.SourceVMID), values, nil)
}

func (c *Client) DeleteGuest(ctx context.Context, node, kind string, vmid int, purge bool) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return fmt.Errorf("invalid guest")
	}
	path := fmt.Sprintf("/api2/json/nodes/%s/%s/%d", url.PathEscape(node), kind, vmid)
	if purge {
		path += "?purge=1&destroy-unreferenced-disks=1"
	}
	return c.request(ctx, http.MethodDelete, path, nil)
}
func (c *Client) ListGuestSnapshots(ctx context.Context, node, kind string, vmid int) ([]GuestSnapshot, error) {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return nil, fmt.Errorf("invalid guest")
	}
	var out []GuestSnapshot
	err := c.get(ctx, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/snapshot", url.PathEscape(node), kind, vmid), &out)
	return out, err
}
func (c *Client) CreateGuestSnapshot(ctx context.Context, node, kind string, vmid int, name, description string) error {
	if !nodeNamePattern.MatchString(node) || !nodeNamePattern.MatchString(name) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return fmt.Errorf("invalid snapshot")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/snapshot", url.PathEscape(node), kind, vmid), url.Values{"snapname": {name}, "description": {description}}, nil)
}
func (c *Client) DeleteGuestSnapshot(ctx context.Context, node, kind string, vmid int, name string) error {
	if !nodeNamePattern.MatchString(node) || !nodeNamePattern.MatchString(name) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return fmt.Errorf("invalid snapshot")
	}
	return c.request(ctx, http.MethodDelete, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/snapshot/%s", url.PathEscape(node), kind, vmid, url.PathEscape(name)), nil)
}
func (c *Client) RollbackGuestSnapshot(ctx context.Context, node, kind string, vmid int, name string) error {
	if !nodeNamePattern.MatchString(node) || !nodeNamePattern.MatchString(name) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return fmt.Errorf("invalid snapshot")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/snapshot/%s/rollback", url.PathEscape(node), kind, vmid, url.PathEscape(name)), nil, nil)
}
func (c *Client) MigrateGuest(ctx context.Context, node, kind string, vmid int, targetNode string, online bool) error {
	if !nodeNamePattern.MatchString(node) || !nodeNamePattern.MatchString(targetNode) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return fmt.Errorf("invalid migration parameters")
	}
	values := url.Values{"target": {targetNode}}
	if online {
		values.Set("online", "1")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/migrate", url.PathEscape(node), kind, vmid), values, nil)
}
func (c *Client) ResizeDisk(ctx context.Context, node, kind string, vmid int, disk, size string) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 || disk == "" || size == "" {
		return fmt.Errorf("invalid disk resize parameters")
	}
	values := url.Values{"disk": {disk}, "size": {size}}
	return c.requestForm(ctx, http.MethodPut, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/resize", url.PathEscape(node), kind, vmid), values, nil)
}
func (c *Client) UpdateGuestConfig(ctx context.Context, node, kind string, vmid int, config map[string]string) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 || len(config) == 0 {
		return fmt.Errorf("invalid config update parameters")
	}
	values := url.Values{}
	for key, value := range config {
		values.Set(key, value)
	}
	return c.requestForm(ctx, http.MethodPut, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/config", url.PathEscape(node), kind, vmid), values, nil)
}

type NetworkInterface struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Active    bool   `json:"active"`
	Autostart bool   `json:"autostart"`
	Bridge    string `json:"bridge,omitempty"`
	Address   string `json:"address,omitempty"`
	Netmask   string `json:"netmask,omitempty"`
	Gateway   string `json:"gateway,omitempty"`
}

func (c *Client) GetNodeNetworkConfig(ctx context.Context, node string) ([]NetworkInterface, error) {
	if !nodeNamePattern.MatchString(node) {
		return nil, fmt.Errorf("invalid node name")
	}
	var result struct {
		Data []struct {
			Iface       string `json:"iface"`
			Type        string `json:"type"`
			Active      int    `json:"active"`
			Autostart   int    `json:"autostart"`
			BridgePorts string `json:"bridge_ports,omitempty"`
			Address     string `json:"address,omitempty"`
			Netmask     string `json:"netmask,omitempty"`
			Gateway     string `json:"gateway,omitempty"`
		} `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api2/json/nodes/%s/network", url.PathEscape(node)), &result); err != nil {
		return nil, err
	}
	interfaces := make([]NetworkInterface, 0, len(result.Data))
	for _, iface := range result.Data {
		interfaces = append(interfaces, NetworkInterface{
			ID:        iface.Iface,
			Name:      iface.Iface,
			Type:      iface.Type,
			Active:    iface.Active == 1,
			Autostart: iface.Autostart == 1,
			Bridge:    iface.BridgePorts,
			Address:   iface.Address,
			Netmask:   iface.Netmask,
			Gateway:   iface.Gateway,
		})
	}
	return interfaces, nil
}

type FirewallRule struct {
	Pos     int    `json:"pos"`
	Type    string `json:"type"`
	Action  string `json:"action"`
	Enable  int    `json:"enable"`
	Proto   string `json:"proto,omitempty"`
	Source  string `json:"source,omitempty"`
	Dest    string `json:"dest,omitempty"`
	Dport   string `json:"dport,omitempty"`
	Sport   string `json:"sport,omitempty"`
	Comment string `json:"comment,omitempty"`
	IFace   string `json:"iface,omitempty"`
}

func (c *Client) GetGuestFirewallRules(ctx context.Context, node, kind string, vmid int) ([]FirewallRule, error) {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return nil, fmt.Errorf("invalid firewall target")
	}
	var result struct {
		Data []FirewallRule `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/firewall/rules", url.PathEscape(node), kind, vmid), &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) CreateGuestFirewallRule(ctx context.Context, node, kind string, vmid int, rule map[string]string) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 || len(rule) == 0 {
		return fmt.Errorf("invalid firewall rule parameters")
	}
	values := url.Values{}
	for key, value := range rule {
		values.Set(key, value)
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/firewall/rules", url.PathEscape(node), kind, vmid), values, nil)
}

func (c *Client) DeleteGuestFirewallRule(ctx context.Context, node, kind string, vmid, pos int) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 || pos < 0 {
		return fmt.Errorf("invalid firewall rule delete parameters")
	}
	return c.request(ctx, http.MethodDelete, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/firewall/rules/%d", url.PathEscape(node), kind, vmid, pos), nil)
}

type BackupInfo struct {
	VolID   string `json:"volid"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Format  string `json:"format"`
	Ctime   int64  `json:"ctime"`
	VMID    int    `json:"vmid"`
	Notes   string `json:"notes,omitempty"`
}

func (c *Client) GetNodeBackups(ctx context.Context, node, storage string) ([]BackupInfo, error) {
	if !nodeNamePattern.MatchString(node) || storage == "" {
		return nil, fmt.Errorf("invalid backup list parameters")
	}
	var result struct {
		Data []BackupInfo `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api2/json/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage)), &result); err != nil {
		return nil, err
	}
	// Filter only backups
	backups := make([]BackupInfo, 0)
	for _, item := range result.Data {
		if item.Content == "backup" {
			backups = append(backups, item)
		}
	}
	return backups, nil
}

func (c *Client) CreateGuestBackup(ctx context.Context, node, kind string, vmid int, storage, mode, compress string) error {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 || storage == "" {
		return fmt.Errorf("invalid backup parameters")
	}
	values := url.Values{
		"vmid":    {fmt.Sprintf("%d", vmid)},
		"storage": {storage},
		"mode":    {mode},
	}
	if compress != "" {
		values.Set("compress", compress)
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/vzdump", url.PathEscape(node)), values, nil)
}

func (c *Client) RestoreGuestBackup(ctx context.Context, node, storage, archive string, vmid int, force bool) error {
	if !nodeNamePattern.MatchString(node) || storage == "" || archive == "" || vmid < 1 {
		return fmt.Errorf("invalid restore parameters")
	}
	values := url.Values{
		"vmid":    {fmt.Sprintf("%d", vmid)},
		"archive": {archive},
	}
	if force {
		values.Set("force", "1")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/storage/%s/upload", url.PathEscape(node), url.PathEscape(storage)), values, nil)
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

// ServeVNC bridges a browser noVNC connection to Proxmox's short-lived native
// console websocket. The PVE API token and VNC ticket never leave this process.
func (c *Client) ServeVNC(w http.ResponseWriter, r *http.Request, node, kind string, vmid int) (bool, error) {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return false, fmt.Errorf("invalid Proxmox console target")
	}
	var ticket struct {
		Port   json.RawMessage `json:"port"`
		Ticket string          `json:"ticket"`
	}
	path := fmt.Sprintf("/api2/json/nodes/%s/%s/%d/vncproxy", url.PathEscape(node), kind, vmid)
	if err := c.requestForm(r.Context(), http.MethodPost, path, url.Values{"websocket": {"1"}}, &ticket); err != nil {
		return false, fmt.Errorf("create Proxmox console ticket: %w", err)
	}
	port, err := parseConsolePort(ticket.Port)
	if err != nil || ticket.Ticket == "" {
		return false, fmt.Errorf("Proxmox returned an incomplete console ticket")
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return false, fmt.Errorf("parse Proxmox console endpoint: %w", err)
	}
	endpoint.Scheme = "wss"
	endpoint.Path = fmt.Sprintf("/api2/json/nodes/%s/%s/%d/vncwebsocket", url.PathEscape(node), kind, vmid)
	query := endpoint.Query()
	query.Set("port", strconv.Itoa(port))
	query.Set("vncticket", ticket.Ticket)
	endpoint.RawQuery = query.Encode()

	dialer := websocket.Dialer{HandshakeTimeout: 20 * time.Second, TLSClientConfig: c.tlsConfig.Clone(), Subprotocols: []string{"binary"}}
	// A Proxmox API token is valid for vncwebsocket as well as vncproxy. Do
	// not request /access/ticket here: that endpoint creates a browser-login
	// cookie and rejects token-only authentication on current PVE versions.
	// Keeping the API token server-side gives the noVNC bridge native console
	// access without ever sending a PVE session cookie to the browser.
	header := http.Header{"Authorization": {"PVEAPIToken=" + c.tokenID + "=" + string(c.secret)}}
	upstream, response, err := dialer.DialContext(r.Context(), endpoint.String(), header)
	if err != nil {
		if response != nil {
			return false, fmt.Errorf("connect Proxmox console returned %d", response.StatusCode)
		}
		return false, fmt.Errorf("connect Proxmox console: %w", err)
	}
	defer upstream.Close()
	browser, err := (&websocket.Upgrader{ReadBufferSize: 32 << 10, WriteBufferSize: 32 << 10, CheckOrigin: consoleSameOrigin}).Upgrade(w, r, nil)
	if err != nil {
		return false, err
	}
	defer browser.Close()
	browser.SetReadLimit(2 << 20)
	return true, relayConsole(browser, upstream)
}

func (c *Client) accessTicket(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api2/json/access/ticket", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+string(c.secret))
	response, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("Proxmox request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		if readErr != nil {
			return "", readErr
		}
		return "", fmt.Errorf("Proxmox access ticket returned %d: %s", response.StatusCode, sanitize(body))
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == "PVEAuthCookie" && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("Proxmox did not return a PVEAuthCookie")
}

func parseConsolePort(raw json.RawMessage) (int, error) {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid Proxmox console port")
	}
	return port, nil
}

func relayConsole(browser, upstream *websocket.Conn) error {
	errors := make(chan error, 2)
	var once sync.Once
	closeBoth := func() { once.Do(func() { _ = browser.Close(); _ = upstream.Close() }) }
	copyFrames := func(from, to *websocket.Conn) {
		defer closeBoth()
		for {
			kind, payload, err := from.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			if kind != websocket.BinaryMessage {
				errors <- fmt.Errorf("Proxmox console bridge accepts binary websocket frames only")
				return
			}
			if err = to.WriteMessage(websocket.BinaryMessage, payload); err != nil {
				errors <- err
				return
			}
		}
	}
	go copyFrames(browser, upstream)
	go copyFrames(upstream, browser)
	return <-errors
}

func consoleSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}
type StorageContentItem struct {
	VolID   string `json:"volid"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	Format  string `json:"format,omitempty"`
	Ctime   int64  `json:"ctime,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

func (c *Client) GetStorageContent(ctx context.Context, node, storage string) ([]StorageContentItem, error) {
	if !nodeNamePattern.MatchString(node) || storage == "" {
		return nil, fmt.Errorf("invalid storage content parameters")
	}
	var out []StorageContentItem
	err := c.get(ctx, fmt.Sprintf("/api2/json/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage)), &out)
	return out, err
}

func (c *Client) DeleteStorageContent(ctx context.Context, node, storage, volumeID string) error {
	if !nodeNamePattern.MatchString(node) || storage == "" || volumeID == "" {
		return fmt.Errorf("invalid storage content delete parameters")
	}
	return c.request(ctx, http.MethodDelete, fmt.Sprintf("/api2/json/nodes/%s/storage/%s/content/%s", url.PathEscape(node), url.PathEscape(storage), url.PathEscape(volumeID)), nil)
}

type TaskStatus struct {
	UPID       string `json:"upid"`
	Node       string `json:"node"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus,omitempty"`
	PID        int    `json:"pid,omitempty"`
	StartTime  int64  `json:"starttime,omitempty"`
}

func (c *Client) GetTaskStatus(ctx context.Context, node, upid string) (*TaskStatus, error) {
	if !nodeNamePattern.MatchString(node) || upid == "" {
		return nil, fmt.Errorf("invalid task status parameters")
	}
	var status TaskStatus
	err := c.get(ctx, fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid)), &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

type TaskLogLine struct {
	N int    `json:"n"`
	T string `json:"t"`
}

func (c *Client) GetTaskLog(ctx context.Context, node, upid string) ([]string, error) {
	if !nodeNamePattern.MatchString(node) || upid == "" {
		return nil, fmt.Errorf("invalid task log parameters")
	}
	var lines []TaskLogLine
	err := c.get(ctx, fmt.Sprintf("/api2/json/nodes/%s/tasks/%s/log", url.PathEscape(node), url.PathEscape(upid)), &lines)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.T)
	}
	return out, nil
}

func (c *Client) ConvertToTemplate(ctx context.Context, node string, vmid int) error {
	if !nodeNamePattern.MatchString(node) || vmid < 1 {
		return fmt.Errorf("invalid guest template parameters")
	}
	return c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/template", url.PathEscape(node), vmid), nil, nil)
}

type AgentInterface struct {
	Name            string `json:"name"`
	HardwareAddress string `json:"hardware-address,omitempty"`
	IPAddresses     []struct {
		IPAddressType string `json:"ip-address-type"`
		IPAddress     string `json:"ip-address"`
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses,omitempty"`
}

func (c *Client) GetQEMUAgentInterfaces(ctx context.Context, node string, vmid int) ([]AgentInterface, error) {
	if !nodeNamePattern.MatchString(node) || vmid < 1 {
		return nil, fmt.Errorf("invalid QEMU agent parameters")
	}
	var result struct {
		Result []AgentInterface `json:"result"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/agent/network-get-interfaces", url.PathEscape(node), vmid), &result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

type VNCTicket struct {
	Ticket string `json:"ticket"`
	Port   string `json:"port"`
	UPID   string `json:"upid"`
	Cert   string `json:"cert,omitempty"`
	User   string `json:"user,omitempty"`
}

func (c *Client) CreateVNCProxy(ctx context.Context, node, kind string, vmid int) (*VNCTicket, error) {
	if !nodeNamePattern.MatchString(node) || (kind != "qemu" && kind != "lxc") || vmid < 1 {
		return nil, fmt.Errorf("invalid vnc proxy parameters")
	}
	var ticket VNCTicket
	err := c.requestForm(ctx, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%d/vncproxy", url.PathEscape(node), kind, vmid), url.Values{"websocket": {"1"}}, &ticket)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (c *Client) get(ctx context.Context, path string, destination any) error {
	return c.request(ctx, http.MethodGet, path, destination)
}
func (c *Client) request(ctx context.Context, method, path string, destination any) error {
	return c.requestForm(ctx, method, path, nil, destination)
}
func (c *Client) requestForm(ctx context.Context, method, path string, values url.Values, destination any) error {
	var requestBody io.Reader
	if values != nil {
		requestBody = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+string(c.secret))
	req.Header.Set("Accept", "application/json")
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
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
