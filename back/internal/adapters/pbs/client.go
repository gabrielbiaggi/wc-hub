package pbs

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
	"strings"
	"time"
)

type Config struct{ URL, TokenID, Secret, CAPath string }
type Client struct {
	baseURL, tokenID, secret string
	http                     *http.Client
}
type Datastore struct {
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	Comment       string  `json:"comment"`
	Total         int64   `json:"total"`
	Used          int64   `json:"used"`
	Available     int64   `json:"available"`
	Snapshots     int     `json:"snapshots"`
	Deduplication float64 `json:"deduplication"`
	Status        string  `json:"status"`
}
type Task struct {
	UPID       string `json:"upid"`
	WorkerType string `json:"worker_type"`
	Status     string `json:"status"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	User       string `json:"user"`
}
type Overview struct {
	CapturedAt time.Time   `json:"captured_at"`
	Datastores []Datastore `json:"datastores"`
	Tasks      []Task      `json:"tasks"`
	Warnings   []string    `json:"warnings"`
}
type envelope[T any] struct {
	Data T `json:"data"`
}

func New(c Config) (*Client, error) {
	base := strings.TrimRight(c.URL, "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || c.TokenID == "" || c.Secret == "" {
		return nil, fmt.Errorf("PBS URL and API token are required")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.CAPath != "" {
		pem, readErr := os.ReadFile(c.CAPath)
		if readErr != nil {
			return nil, readErr
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("invalid PBS CA")
		}
		tlsConfig.RootCAs = pool
	}
	return &Client{base, c.TokenID, c.Secret, &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsConfig}}}, nil
}
func (c *Client) Overview(ctx context.Context) (Overview, error) {
	result := Overview{CapturedAt: time.Now().UTC(), Datastores: []Datastore{}, Tasks: []Task{}, Warnings: []string{}}
	var configured []struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Comment string `json:"comment"`
	}
	if err := c.get(ctx, "/api2/json/config/datastore", &configured); err != nil {
		return result, err
	}
	for _, store := range configured {
		item := Datastore{Name: store.Name, Path: store.Path, Comment: store.Comment, Status: "unknown"}
		var status struct {
			Total         int64   `json:"total"`
			Used          int64   `json:"used"`
			Avail         int64   `json:"avail"`
			Deduplication float64 `json:"deduplication"`
		}
		if err := c.get(ctx, "/api2/json/admin/datastore/"+url.PathEscape(store.Name)+"/status", &status); err != nil {
			result.Warnings = append(result.Warnings, "Status indisponível para "+store.Name)
			result.Datastores = append(result.Datastores, item)
			continue
		}
		item.Total, item.Used, item.Available, item.Deduplication, item.Status = status.Total, status.Used, status.Avail, status.Deduplication, "healthy"
		var groups []json.RawMessage
		if err := c.get(ctx, "/api2/json/admin/datastore/"+url.PathEscape(store.Name)+"/groups", &groups); err == nil {
			item.Snapshots = len(groups)
		} else {
			result.Warnings = append(result.Warnings, "Snapshots indisponíveis para "+store.Name)
		}
		result.Datastores = append(result.Datastores, item)
	}
	if err := c.get(ctx, "/api2/json/nodes/localhost/tasks?limit=50", &result.Tasks); err != nil {
		result.Warnings = append(result.Warnings, "Histórico de tarefas indisponível")
	}
	return result, nil
}
func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PBSAPIToken="+c.tokenID+"="+c.secret)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PBS returned %d", resp.StatusCode)
	}
	var wrapped envelope[json.RawMessage]
	if err = json.Unmarshal(body, &wrapped); err != nil {
		return err
	}
	return json.Unmarshal(wrapped.Data, dest)
}
