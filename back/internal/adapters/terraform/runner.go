package terraform

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var workspacePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

type Config struct {
	WorkerURL   string
	WorkerToken []byte
	Workspaces  []string
	HTTPClient  *http.Client
}
type Runner struct {
	baseURL    *url.URL
	token      []byte
	workspaces map[string]struct{}
	http       *http.Client
}
type ChangeSummary struct {
	Add     int `json:"add"`
	Change  int `json:"change"`
	Destroy int `json:"destroy"`
}
type Run struct {
	ID         string        `json:"id"`
	Workspace  string        `json:"workspace"`
	Operation  string        `json:"operation"`
	Status     string        `json:"status"`
	Output     string        `json:"output"`
	Summary    ChangeSummary `json:"summary"`
	CreatedAt  time.Time     `json:"created_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
}

func New(config Config) (*Runner, error) {
	parsed, err := url.Parse(strings.TrimRight(config.WorkerURL, "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("Terraform worker URL is invalid")
	}
	host := parsed.Hostname()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())) {
		return nil, errors.New("Terraform worker must use HTTPS or loopback HTTP")
	}
	if len(config.WorkerToken) < 20 {
		return nil, errors.New("Terraform worker token is required")
	}
	allowed := map[string]struct{}{}
	for _, value := range config.Workspaces {
		value = strings.TrimSpace(value)
		if !workspacePattern.MatchString(value) {
			return nil, errors.New("invalid Terraform workspace allowlist entry")
		}
		allowed[value] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, errors.New("Terraform workspace allowlist is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 10, IdleConnTimeout: 60 * time.Second}}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != parsed.Scheme || !strings.EqualFold(req.URL.Host, parsed.Host) {
			return errors.New("Terraform worker redirect rejected")
		}
		if len(via) > 4 {
			return errors.New("too many Terraform worker redirects")
		}
		return nil
	}
	return &Runner{baseURL: parsed, token: append([]byte(nil), config.WorkerToken...), workspaces: allowed, http: &copyClient}, nil
}
func (r *Runner) Workspaces() []string {
	values := make([]string, 0, len(r.workspaces))
	for value := range r.workspaces {
		values = append(values, value)
	}
	return values
}
func (r *Runner) Start(ctx context.Context, operation, workspace string) (Run, error) {
	if operation != "validate" && operation != "plan" && operation != "apply" && operation != "destroy" && operation != "output" {
		return Run{}, errors.New("Terraform operation is not allowlisted")
	}
	if _, ok := r.workspaces[workspace]; !ok {
		return Run{}, errors.New("Terraform workspace is not allowlisted")
	}
	var run Run
	err := r.request(ctx, http.MethodPost, "/v1/runs", map[string]string{"operation": operation, "workspace": workspace}, &run)
	return run, err
}
func (r *Runner) List(ctx context.Context) ([]Run, error) {
	result := struct {
		Items []Run `json:"items"`
	}{Items: []Run{}}
	err := r.request(ctx, http.MethodGet, "/v1/runs?limit=100", nil, &result)
	return result.Items, err
}
func (r *Runner) request(ctx context.Context, method, path string, payload any, destination any) error {
	endpoint := *r.baseURL
	clean := path
	if i := strings.Index(path, "?"); i >= 0 {
		clean = path[:i]
		endpoint.RawQuery = path[i+1:]
	}
	endpoint.Path = strings.TrimRight(r.baseURL.Path, "/") + clean
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(r.token))
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("Terraform worker request: %w", err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Terraform worker returned %d", response.StatusCode)
	}
	if err = json.Unmarshal(contents, destination); err != nil {
		return fmt.Errorf("decode Terraform worker response: %w", err)
	}
	return nil
}
