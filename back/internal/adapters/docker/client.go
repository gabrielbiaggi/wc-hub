package docker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
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
)

const maxResponseBytes = 16 << 20

var containerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

type Config struct {
	Endpoint          string
	CACertificatePath string
	ClientCertPath    string
	ClientKeyPath     string
	Timeout           time.Duration
}

type Client struct {
	baseURL string
	http    *http.Client
}

type Health struct {
	Reachable  bool   `json:"reachable"`
	Version    string `json:"version,omitempty"`
	APIVersion string `json:"api_version,omitempty"`
	OSType     string `json:"os_type,omitempty"`
	Arch       string `json:"arch,omitempty"`
}

type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type Container struct {
	ID      string            `json:"id"`
	Names   []string          `json:"names"`
	Image   string            `json:"image"`
	ImageID string            `json:"image_id"`
	Command string            `json:"command"`
	Created int64             `json:"created"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Ports   []Port            `json:"ports"`
	Labels  map[string]string `json:"labels"`
}

type Image struct {
	ID          string   `json:"id"`
	RepoTags    []string `json:"repo_tags"`
	RepoDigests []string `json:"repo_digests"`
	Created     int64    `json:"created"`
	Size        int64    `json:"size"`
	SharedSize  int64    `json:"shared_size"`
	Containers  int64    `json:"containers"`
}

type ContainerStats struct {
	ContainerID   string    `json:"container_id"`
	Name          string    `json:"name"`
	ReadAt        time.Time `json:"read_at"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsage   uint64    `json:"memory_usage"`
	MemoryLimit   uint64    `json:"memory_limit"`
	MemoryPercent float64   `json:"memory_percent"`
	NetworkRX     uint64    `json:"network_rx"`
	NetworkTX     uint64    `json:"network_tx"`
	BlockRead     uint64    `json:"block_read"`
	BlockWrite    uint64    `json:"block_write"`
}

type Inventory struct {
	CapturedAt time.Time        `json:"captured_at"`
	Health     Health           `json:"health"`
	Containers []Container      `json:"containers"`
	Images     []Image          `json:"images"`
	Stats      []ContainerStats `json:"stats"`
	Warnings   []string         `json:"warnings"`
}

type versionResponse struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
	OSType     string `json:"Os"`
	Arch       string `json:"Arch"`
}

type rawContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	Command string   `json:"Command"`
	Created int64    `json:"Created"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

type rawImage struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	RepoDigests []string `json:"RepoDigests"`
	Created     int64    `json:"Created"`
	Size        int64    `json:"Size"`
	SharedSize  int64    `json:"SharedSize"`
	Containers  int64    `json:"Containers"`
}

type rawStats struct {
	Read     string `json:"read"`
	Name     string `json:"name"`
	ID       string `json:"id"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
		Stats struct {
			Cache uint64 `json:"cache"`
		} `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RXBytes uint64 `json:"rx_bytes"`
		TXBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IOServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

func New(endpoint string) (*Client, error) {
	return NewWithConfig(Config{Endpoint: endpoint})
}

func NewWithConfig(config Config) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("Docker endpoint must be an explicit HTTP(S) restricted proxy URL")
	}
	if parsed.User != nil {
		return nil, errors.New("Docker endpoint must not embed credentials")
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.CACertificatePath != "" {
		contents, err := os.ReadFile(config.CACertificatePath)
		if err != nil {
			return nil, fmt.Errorf("read Docker CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(contents) {
			return nil, errors.New("Docker CA bundle is invalid")
		}
		tlsConfig.RootCAs = pool
	}
	if (config.ClientCertPath == "") != (config.ClientKeyPath == "") {
		return nil, errors.New("Docker mTLS certificate and key must be configured together")
	}
	if config.ClientCertPath != "" {
		certificate, err := tls.LoadX509KeyPair(config.ClientCertPath, config.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load Docker mTLS identity: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := &http.Transport{
		TLSClientConfig:    tlsConfig,
		MaxIdleConns:       20,
		IdleConnTimeout:    60 * time.Second,
		DisableCompression: false,
	}
	return &Client{
		baseURL: strings.TrimRight(parsed.String(), "/"),
		http:    &http.Client{Timeout: timeout, Transport: transport},
	}, nil
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.http != nil
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	if !c.Configured() {
		return Health{}, errors.New("Docker adapter is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_ping", nil)
	if err != nil {
		return Health{}, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Health{}, fmt.Errorf("Docker ping: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return Health{}, err
	}
	if response.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "OK" {
		return Health{}, fmt.Errorf("Docker ping returned status %d", response.StatusCode)
	}
	var version versionResponse
	if err = c.get(ctx, "/version", &version); err != nil {
		return Health{}, err
	}
	return Health{Reachable: true, Version: version.Version, APIVersion: version.APIVersion, OSType: version.OSType, Arch: version.Arch}, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	rawItems := []rawContainer{}
	if err := c.get(ctx, "/containers/json?all=1&size=0", &rawItems); err != nil {
		return nil, err
	}
	items := make([]Container, 0, len(rawItems))
	for _, raw := range rawItems {
		item := Container{ID: raw.ID, Names: raw.Names, Image: raw.Image, ImageID: raw.ImageID, Command: raw.Command, Created: raw.Created, State: raw.State, Status: raw.Status, Labels: raw.Labels}
		if item.Names == nil {
			item.Names = []string{}
		}
		if item.Labels == nil {
			item.Labels = map[string]string{}
		}
		item.Ports = make([]Port, 0, len(raw.Ports))
		for _, port := range raw.Ports {
			item.Ports = append(item.Ports, Port{IP: port.IP, PrivatePort: port.PrivatePort, PublicPort: port.PublicPort, Type: port.Type})
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	rawItems := []rawImage{}
	if err := c.get(ctx, "/images/json?all=0", &rawItems); err != nil {
		return nil, err
	}
	items := make([]Image, 0, len(rawItems))
	for _, raw := range rawItems {
		item := Image{ID: raw.ID, RepoTags: raw.RepoTags, RepoDigests: raw.RepoDigests, Created: raw.Created, Size: raw.Size, SharedSize: raw.SharedSize, Containers: raw.Containers}
		if item.RepoTags == nil {
			item.RepoTags = []string{}
		}
		if item.RepoDigests == nil {
			item.RepoDigests = []string{}
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) ContainerAction(ctx context.Context, id, action string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if !containerIDPattern.MatchString(id) {
		return errors.New("Docker container ID is invalid")
	}
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("Docker container action is unsupported")
	}
	path := "/containers/" + id + "/" + action
	if action == "stop" || action == "restart" {
		path += "?t=15"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("Docker action request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotModified {
		return fmt.Errorf("Docker API returned %d: %s", response.StatusCode, sanitize(body))
	}
	return nil
}

func (c *Client) Stats(ctx context.Context, containers []Container) ([]ContainerStats, []string) {
	running := make([]Container, 0, len(containers))
	for _, container := range containers {
		if strings.EqualFold(container.State, "running") {
			running = append(running, container)
		}
	}
	stats := make([]ContainerStats, len(running))
	warnings := make([]string, 0)
	var wait sync.WaitGroup
	var lock sync.Mutex
	semaphore := make(chan struct{}, 4)
	for index, container := range running {
		wait.Add(1)
		go func(index int, container Container) {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			item, err := c.containerStats(ctx, container.ID)
			if err != nil {
				lock.Lock()
				warnings = append(warnings, "stats unavailable for "+shortID(container.ID))
				lock.Unlock()
				return
			}
			stats[index] = item
		}(index, container)
	}
	wait.Wait()
	filtered := stats[:0]
	for _, item := range stats {
		if item.ContainerID != "" {
			filtered = append(filtered, item)
		}
	}
	return filtered, warnings
}

func (c *Client) Inventory(ctx context.Context) (Inventory, error) {
	result := Inventory{CapturedAt: time.Now().UTC(), Warnings: []string{}}
	health, err := c.Health(ctx)
	if err != nil {
		return result, err
	}
	result.Health = health
	result.Containers, err = c.ListContainers(ctx)
	if err != nil {
		return result, err
	}
	result.Images, err = c.ListImages(ctx)
	if err != nil {
		return result, err
	}
	result.Stats, result.Warnings = c.Stats(ctx, result.Containers)
	return result, nil
}

func (c *Client) containerStats(ctx context.Context, id string) (ContainerStats, error) {
	var raw rawStats
	path := "/containers/" + url.PathEscape(id) + "/stats?stream=false&one-shot=true"
	if err := c.get(ctx, path, &raw); err != nil {
		return ContainerStats{}, err
	}
	return normalizeStats(raw), nil
}

func (c *Client) get(ctx context.Context, path string, destination any) error {
	if !c.Configured() {
		return errors.New("Docker adapter is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("Docker request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxResponseBytes {
		return errors.New("Docker response exceeded the safety limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Docker API returned %d: %s", response.StatusCode, sanitize(body))
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err = decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode Docker response: %w", err)
	}
	return nil
}

func normalizeStats(raw rawStats) ContainerStats {
	onlineCPUs := raw.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = uint32(len(raw.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuDelta := counterDelta(raw.CPUStats.CPUUsage.TotalUsage, raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := counterDelta(raw.CPUStats.SystemCPUUsage, raw.PreCPUStats.SystemCPUUsage)
	cpuPercent := 0.0
	if cpuDelta > 0 && systemDelta > 0 && onlineCPUs > 0 {
		cpuPercent = cpuDelta / systemDelta * float64(onlineCPUs) * 100
	}
	memoryUsage := raw.MemoryStats.Usage
	if cache := raw.MemoryStats.Stats.Cache; cache < memoryUsage {
		memoryUsage -= cache
	}
	memoryPercent := 0.0
	if raw.MemoryStats.Limit > 0 {
		memoryPercent = float64(memoryUsage) / float64(raw.MemoryStats.Limit) * 100
	}
	var networkRX, networkTX, blockRead, blockWrite uint64
	for _, network := range raw.Networks {
		networkRX += network.RXBytes
		networkTX += network.TXBytes
	}
	for _, entry := range raw.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			blockRead += entry.Value
		case "write":
			blockWrite += entry.Value
		}
	}
	readAt, _ := time.Parse(time.RFC3339Nano, raw.Read)
	return ContainerStats{
		ContainerID:   raw.ID,
		Name:          strings.TrimPrefix(raw.Name, "/"),
		ReadAt:        readAt,
		CPUPercent:    cpuPercent,
		MemoryUsage:   memoryUsage,
		MemoryLimit:   raw.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		NetworkRX:     networkRX,
		NetworkTX:     networkTX,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
	}
}

func counterDelta(current, previous uint64) float64 {
	if current <= previous {
		return 0
	}
	return float64(current - previous)
}

func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func sanitize(body []byte) string {
	value := strings.ReplaceAll(strings.TrimSpace(string(body)), "\n", " ")
	if len(value) > 240 {
		value = value[:240]
	}
	return strconv.QuoteToASCII(value)
}
