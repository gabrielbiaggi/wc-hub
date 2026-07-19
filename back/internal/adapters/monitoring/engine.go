package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Target struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Target          string     `json:"target"`
	Kind            string     `json:"kind"`
	LastStatus      string     `json:"lastStatus"`
	LastError       string     `json:"lastError"`
	IntervalSeconds int        `json:"intervalSeconds"`
	LastLatencyMS   int        `json:"lastLatencyMS"`
	Enabled         bool       `json:"enabled"`
	LastCheckedAt   *time.Time `json:"lastCheckedAt"`
}
type Store interface {
	Targets(context.Context) ([]Target, error)
	Result(context.Context, string, string, int, string) error
	Webhook(context.Context) (string, error)
}
type Engine struct {
	store    Store
	client   *http.Client
	interval time.Duration
}

func New(store Store, interval time.Duration) *Engine {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Engine{store: store, client: &http.Client{Timeout: 10 * time.Second}, interval: interval}
}
func (e *Engine) Start(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	go func() {
		defer ticker.Stop()
		e.run(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.run(ctx)
			}
		}
	}()
}
func (e *Engine) run(ctx context.Context) {
	targets, err := e.store.Targets(ctx)
	if err != nil {
		return
	}
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		if target.LastCheckedAt != nil && time.Since(*target.LastCheckedAt) < time.Duration(target.IntervalSeconds)*time.Second {
			continue
		}
		status, latency, reason := e.check(ctx, target)
		if err = e.store.Result(ctx, target.ID, status, latency, reason); err == nil && status == "down" && target.LastStatus != "down" {
			e.notify(ctx, target, reason)
		}
	}
}
func (e *Engine) check(ctx context.Context, target Target) (string, int, string) {
	start := time.Now()
	switch target.Kind {
	case "http":
		parsed, err := url.Parse(target.Target)
		if err != nil || !(parsed.Scheme == "http" || parsed.Scheme == "https") || parsed.Host == "" {
			return "down", 0, "URL HTTP inválida"
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target.Target, nil)
		resp, err := e.client.Do(req)
		if err != nil {
			return "down", 0, err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			return "down", int(time.Since(start).Milliseconds()), fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "up", int(time.Since(start).Milliseconds()), ""
	case "tcp":
		address := strings.TrimSpace(target.Target)
		if _, _, err := net.SplitHostPort(address); err != nil {
			return "down", 0, "endereço TCP inválido"
		}
		connection, err := net.DialTimeout("tcp", address, 8*time.Second)
		if err != nil {
			return "down", 0, err.Error()
		}
		_ = connection.Close()
		return "up", int(time.Since(start).Milliseconds()), ""
	default:
		return "down", 0, "tipo inválido"
	}
}
func (e *Engine) notify(ctx context.Context, target Target, reason string) {
	hook, err := e.store.Webhook(ctx)
	if err != nil || hook == "" {
		return
	}
	parsed, err := url.Parse(hook)
	if err != nil || parsed.Scheme != "https" {
		return
	}
	payload, err := json.Marshal(map[string]string{
		"content": fmt.Sprintf("🚨 WC Hub: %s está indisponível. %s", target.Name, reason),
	})
	if err != nil {
		return
	}
	body := strings.NewReader(string(payload))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, body)
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err == nil && response != nil {
		response.Body.Close()
	}
}
