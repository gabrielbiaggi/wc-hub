package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/webcreations/wc-hub/back/internal/telemetry/domain"
	"github.com/webcreations/wc-hub/back/pkg/promparse"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	listen, token, hubURL, nodeURL, dcgmURL, cert, key, clientCA string
	interval                                                     time.Duration
	selfProtected                                                bool
}
type actionRequest struct {
	Action string `json:"action"`
	Unit   string `json:"unit,omitempty"`
	Lines  int    `json:"lines,omitempty"`
}

var safeUnit = regexp.MustCompile(`^[a-zA-Z0-9_.@-]{1,128}$`)

func main() {
	cfg := load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.token == "" {
		logger.Error("WC_AGENT_TOKEN is required")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go pushLoop(ctx, cfg, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "self_protected": cfg.selfProtected})
	})
	mux.HandleFunc("POST /v1/action", auth(cfg.token, action(cfg, logger)))
	server := &http.Server{Addr: cfg.listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		var err error
		if cfg.cert != "" {
			tlsConfig, tlsErr := mutualTLS(cfg.clientCA)
			if tlsErr != nil {
				logger.Error("mTLS configuration failed", "error", tlsErr)
				cancel()
				return
			}
			server.TLSConfig = tlsConfig
			err = server.ListenAndServeTLS(cfg.cert, cfg.key)
		} else {
			host, _, _ := net.SplitHostPort(cfg.listen)
			if host != "127.0.0.1" && host != "localhost" {
				logger.Error("agent refuses non-loopback listener without mTLS")
				cancel()
				return
			}
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Error("agent server failed", "error", err)
			cancel()
		}
	}()
	<-ctx.Done()
	shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = server.Shutdown(shutdownCtx)
}
func load() config {
	interval, _ := time.ParseDuration(value("WC_AGENT_INTERVAL", "15s"))
	return config{listen: value("WC_AGENT_LISTEN", "127.0.0.1:9105"), token: os.Getenv("WC_AGENT_TOKEN"), hubURL: strings.TrimRight(os.Getenv("WC_HUB_URL"), "/"), nodeURL: value("NODE_EXPORTER_URL", "http://127.0.0.1:9100/metrics"), dcgmURL: os.Getenv("DCGM_EXPORTER_URL"), cert: os.Getenv("WC_AGENT_TLS_CERT"), key: os.Getenv("WC_AGENT_TLS_KEY"), clientCA: os.Getenv("WC_AGENT_CLIENT_CA"), interval: interval, selfProtected: value("WC_AGENT_SELF_PROTECTED", "false") == "true"}
}
func pushLoop(ctx context.Context, cfg config, logger *slog.Logger) {
	if cfg.hubURL == "" {
		logger.Warn("metrics push disabled: WC_HUB_URL not set")
		return
	}
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	for {
		if err := collectAndPush(ctx, cfg); err != nil {
			logger.Error("metrics push failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func collectAndPush(ctx context.Context, cfg config) error {
	samples, err := scrape(ctx, cfg.nodeURL, func(name string) bool { return strings.HasPrefix(name, "node_") })
	if err != nil {
		return err
	}
	if cfg.dcgmURL != "" {
		gpu, gpuErr := scrape(ctx, cfg.dcgmURL, func(name string) bool { return strings.HasPrefix(name, "DCGM_FI_DEV_") })
		if gpuErr == nil {
			samples = append(samples, gpu...)
		}
	}
	batch := domain.Batch{CapturedAt: time.Now().UTC()}
	for _, sample := range samples {
		batch.Samples = append(batch.Samples, domain.IngestSample{Name: sample.Name, Labels: sample.Labels, Value: sample.Value})
	}
	body, _ := json.Marshal(batch)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.hubURL+"/agent/v1/metrics", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 204 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("hub returned %d: %s", response.StatusCode, message)
	}
	return nil
}
func scrape(ctx context.Context, endpoint string, allow func(string) bool) ([]promparse.Sample, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("exporter returned %d", response.StatusCode)
	}
	return promparse.Parse(response.Body, allow)
}
func action(cfg config, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		decision := "denied"
		reason := ""
		actionName := "invalid"
		defer func() {
			logger.Info("agent action", "action", actionName, "decision", decision, "reason", reason, "duration_ms", time.Since(started).Milliseconds())
			go reportEvent(cfg, actionName, decision, reason)
		}()
		var req actionRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		actionName = req.Action
		var command *exec.Cmd
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		switch req.Action {
		case "system.uptime":
			command = exec.CommandContext(ctx, "uptime")
		case "docker.ps":
			command = exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}\\t{{.Names}}\\t{{.Status}}\\t{{.Image}}")
		case "journal.tail":
			if !safeUnit.MatchString(req.Unit) {
				http.Error(w, "invalid unit", 400)
				return
			}
			if req.Lines < 1 || req.Lines > 500 {
				req.Lines = 100
			}
			command = exec.CommandContext(ctx, "journalctl", "--no-pager", "-n", strconv.Itoa(req.Lines), "-u", req.Unit)
		default:
			reason = "action not allowlisted"
			http.Error(w, "action not allowlisted", 403)
			return
		}
		output, err := command.CombinedOutput()
		status := 200
		if err != nil {
			status = 500
			reason = err.Error()
		} else {
			decision = "allowed"
			reason = "typed allowlist action completed"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"output": string(output), "success": err == nil})
	}
}
func reportEvent(cfg config, action, decision, reason string) {
	if cfg.hubURL == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"action": action, "decision": decision, "reason": reason})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, cfg.hubURL+"/agent/v1/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err == nil {
		response.Body.Close()
	}
}
func auth(token string, next http.HandlerFunc) http.HandlerFunc {
	expected := sha256.Sum256([]byte(token))
	return func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		actual := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		next(w, r)
	}
}
func mutualTLS(caPath string) (*tls.Config, error) {
	pem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("invalid client CA")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}, nil
}
func value(key, fallback string) string {
	if result := os.Getenv(key); result != "" {
		return result
	}
	return fallback
}
