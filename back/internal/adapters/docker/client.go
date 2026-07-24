package docker

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	Endpoint           string
	SourceName         string
	FallbackSocketPath string
	CACertificatePath  string
	ClientCertPath     string
	ClientKeyPath      string
	Timeout            time.Duration
}

type Client struct {
	primary  endpoint
	fallback *endpoint
	source   string
}
type endpoint struct {
	name, baseURL string
	http          *http.Client
}

type Health struct {
	Reachable  bool   `json:"reachable"`
	Source     string `json:"source,omitempty"`
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
	Source     string           `json:"source,omitempty"`
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
	client := &Client{source: strings.TrimSpace(config.SourceName)}
	if rawEndpoint := strings.TrimSpace(config.Endpoint); rawEndpoint != "" {
		parsed, err := url.Parse(rawEndpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, errors.New("Docker endpoint must be an explicit HTTP(S) restricted proxy URL")
		}
		if parsed.User != nil {
			return nil, errors.New("Docker endpoint must not embed credentials")
		}
		client.primary = endpoint{name: "Docker endpoint " + parsed.Host, baseURL: strings.TrimRight(parsed.String(), "/"), http: &http.Client{Timeout: timeout, Transport: transport}}
	}
	if socketPath := strings.TrimSpace(config.FallbackSocketPath); socketPath != "" {
		dialer := &net.Dialer{Timeout: timeout}
		client.fallback = &endpoint{name: "Docker socket " + socketPath, baseURL: "http://docker", http: &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		}}}}
	}
	if client.primary.http == nil && client.fallback == nil {
		return nil, errors.New("Docker endpoint or Unix socket path is required")
	}
	if client.source == "" {
		if client.primary.name != "" {
			client.source = client.primary.name
		} else if client.fallback != nil {
			client.source = client.fallback.name
		}
	}
	return client, nil
}

func (c *Client) Configured() bool {
	return c != nil && (c.primary.http != nil || c.fallback != nil)
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	if !c.Configured() {
		return Health{}, errors.New("Docker adapter is not configured")
	}
	response, err := c.do(ctx, http.MethodGet, "/_ping", nil, nil)
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
	return Health{Reachable: true, Source: c.source, Version: version.Version, APIVersion: version.APIVersion, OSType: version.OSType, Arch: version.Arch}, nil
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
	method := http.MethodPost
	path := "/containers/" + id + "/" + action
	switch action {
	case "start":
	case "stop", "restart":
		path += "?t=15"
	case "kill":
	case "remove", "delete":
		method = http.MethodDelete
		path = "/containers/" + id + "?v=true&force=true"
	default:
		return errors.New("Docker container action is unsupported")
	}
	response, err := c.do(ctx, method, path, nil, http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("Docker action request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotModified && response.StatusCode != http.StatusOK {
		return fmt.Errorf("Docker API returned %d: %s", response.StatusCode, sanitize(body))
	}
	return nil
}

func (c *Client) Exec(ctx context.Context, id string, command []string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !containerIDPattern.MatchString(id) || len(command) == 0 || len(command) > 32 {
		return "", errors.New("Docker exec input is invalid")
	}
	for _, item := range command {
		if strings.TrimSpace(item) == "" || len(item) > 4096 {
			return "", errors.New("Docker exec command is invalid")
		}
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := c.postJSON(ctx, "/containers/"+id+"/exec", map[string]any{"AttachStdout": true, "AttachStderr": true, "Tty": false, "Cmd": command}, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("Docker exec session was not created")
	}
	raw, err := c.postRaw(ctx, "/exec/"+url.PathEscape(created.ID)+"/start", map[string]any{"Detach": false, "Tty": false})
	if err != nil {
		return "", err
	}
	return decodeDockerStream(raw)
}

func (c *Client) postJSON(ctx context.Context, path string, payload, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	response, err := c.do(ctx, http.MethodPost, path, body, http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("Docker request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Docker API returned %d: %s", response.StatusCode, sanitize(responseBody))
	}
	if destination != nil {
		return json.Unmarshal(responseBody, destination)
	}
	return nil
}

func (c *Client) postRaw(ctx context.Context, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	response, err := c.do(ctx, http.MethodPost, path, body, http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Docker API returned %d: %s", response.StatusCode, sanitize(result))
	}
	return result, nil
}

func decodeDockerStream(raw []byte) (string, error) {
	var output strings.Builder
	for len(raw) >= 8 {
		size := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if size < 0 || size > len(raw) {
			return "", errors.New("Docker exec stream is malformed")
		}
		output.Write(raw[:size])
		raw = raw[size:]
	}
	if output.Len() == 0 {
		output.Write(raw)
	}
	return output.String(), nil
}

func (c *Client) Stats(ctx context.Context, containers []Container) ([]ContainerStats, error) {
	running := make([]Container, 0, len(containers))
	for _, container := range containers {
		if strings.EqualFold(container.State, "running") {
			running = append(running, container)
		}
	}
	stats := make([]ContainerStats, len(running))
	errorsByContainer := make([]error, len(running))
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
				errorsByContainer[index] = fmt.Errorf("stats for container %s: %w", shortID(container.ID), err)
				lock.Unlock()
				return
			}
			stats[index] = item
		}(index, container)
	}
	wait.Wait()
	for _, err := range errorsByContainer {
		if err != nil {
			return nil, err
		}
	}
	filtered := stats[:0]
	for _, item := range stats {
		if item.ContainerID != "" {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (c *Client) Inventory(ctx context.Context) (Inventory, error) {
	result := Inventory{CapturedAt: time.Now().UTC(), Source: c.source, Warnings: []string{}}
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
	result.Stats, err = c.Stats(ctx, result.Containers)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) containerStats(ctx context.Context, id string) (ContainerStats, error) {
	var raw rawStats
	path := "/containers/" + url.PathEscape(id) + "/stats?stream=false&one-shot=true"
	if err := c.get(ctx, path, &raw); err != nil {
		return ContainerStats{}, err
	}
	return normalizeStats(raw)
}

func (c *Client) get(ctx context.Context, path string, destination any) error {
	if !c.Configured() {
		return errors.New("Docker adapter is not configured")
	}
	response, err := c.do(ctx, http.MethodGet, path, nil, http.Header{"Accept": []string{"application/json"}})
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

func (c *Client) do(ctx context.Context, method, path string, body []byte, headers http.Header) (*http.Response, error) {
	if !c.Configured() {
		return nil, errors.New("Docker adapter is not configured")
	}
	if c.primary.http == nil && c.fallback != nil {
		return c.doEndpoint(ctx, *c.fallback, method, path, body, headers)
	}
	response, primaryErr := c.doEndpoint(ctx, c.primary, method, path, body, headers)
	if primaryErr == nil || c.fallback == nil {
		return response, primaryErr
	}
	response, fallbackErr := c.doEndpoint(ctx, *c.fallback, method, path, body, headers)
	if fallbackErr == nil {
		return response, nil
	}
	return nil, fmt.Errorf("%s failed: %v; %s fallback failed: %w", c.primary.name, primaryErr, c.fallback.name, fallbackErr)
}

func (c *Client) doEndpoint(ctx context.Context, endpoint endpoint, method, path string, body []byte, headers http.Header) (*http.Response, error) {
	if endpoint.http == nil {
		return nil, errors.New("not configured")
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := endpoint.http.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func normalizeStats(raw rawStats) (ContainerStats, error) {
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
	if strings.TrimSpace(raw.Read) == "" {
		return ContainerStats{}, errors.New("Docker stats response has no timestamp")
	}
	readAt, err := time.Parse(time.RFC3339Nano, raw.Read)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("parse Docker stats timestamp: %w", err)
	}
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
	}, nil
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

func (c *Client) PullImageStream(ctx context.Context, image string, logWriter func(string)) error {
	if !c.Configured() {
		return errors.New("Docker adapter is not configured")
	}
	path := "/images/create?fromImage=" + url.QueryEscape(image)
	response, err := c.do(ctx, http.MethodPost, path, nil, http.Header{"Accept": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("Docker pull request failed: %w", err)
	}
	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	for decoder.More() {
		var msg struct {
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Error    string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			break
		}
		if msg.Error != "" {
			return fmt.Errorf("Docker pull error: %s", msg.Error)
		}
		out := msg.Status
		if msg.Progress != "" {
			out += " " + msg.Progress
		}
		if out != "" && logWriter != nil {
			logWriter(out)
		}
	}
	return nil
}

type ContainerInspect struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Image  string            `json:"Image"`
		Env    []string          `json:"Env"`
		Cmd    []string          `json:"Cmd"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
	} `json:"State"`
	HostConfig struct {
		Binds        []string `json:"Binds"`
		PortBindings map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
	} `json:"HostConfig"`
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*ContainerInspect, error) {
	var item ContainerInspect
	path := "/containers/" + url.PathEscape(id) + "/json"
	if err := c.get(ctx, path, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (c *Client) CloneStack(ctx context.Context, containerID, suffix string) (string, error) {
	inspect, err := c.InspectContainer(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspect container failed: %w", err)
	}

	cleanName := strings.TrimPrefix(inspect.Name, "/")
	if suffix == "" {
		suffix = "staging"
	}
	newContainerName := suffix + "-" + cleanName

	// Create payload — passed directly to postRaw (which marshals internally).
	payload := map[string]any{
		"Image": inspect.Config.Image,
		"Cmd":   inspect.Config.Cmd,
		"Env":   append(inspect.Config.Env, "ENV_SUFFIX="+suffix),
		"Labels": map[string]string{
			"com.wc-hub.cloned-from":   cleanName,
			"com.wc-hub.cloned-suffix": suffix,
		},
	}

	path := "/containers/create?name=" + url.QueryEscape(newContainerName)
	resp, err := c.postRaw(ctx, path, payload)
	if err != nil {
		// If container already exists, retry with a timestamp suffix.
		newContainerName = fmt.Sprintf("%s-%s-%d", suffix, cleanName, time.Now().Unix())
		path = "/containers/create?name=" + url.QueryEscape(newContainerName)
		resp, err = c.postRaw(ctx, path, payload)
		if err != nil {
			return "", fmt.Errorf("create cloned container failed: %w", err)
		}
	}

	var created struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(resp, &created)
	if created.ID != "" {
		_ = c.ContainerAction(ctx, created.ID, "start")
	}

	return newContainerName, nil
}

