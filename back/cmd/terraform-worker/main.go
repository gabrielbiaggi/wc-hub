package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type summary struct {
	Add     int `json:"add"`
	Change  int `json:"change"`
	Destroy int `json:"destroy"`
}

type run struct {
	ID         string     `json:"id"`
	Workspace  string     `json:"workspace"`
	Operation  string     `json:"operation"`
	Status     string     `json:"status"`
	Output     string     `json:"output"`
	Summary    summary    `json:"summary"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type server struct {
	token         []byte
	root          string
	stateRoot     string
	terraformPath string
	timeout       time.Duration
	allowed       map[string]struct{}
	logger        *slog.Logger
	semaphore     chan struct{}
	mu            sync.RWMutex
	runs          []run
}

var workspaceName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	token := []byte(strings.TrimSpace(os.Getenv("TERRAFORM_WORKER_TOKEN")))
	if len(token) < 20 {
		logger.Error("TERRAFORM_WORKER_TOKEN must contain at least 20 characters")
		os.Exit(1)
	}
	allowed := map[string]struct{}{}
	for _, item := range strings.Split(os.Getenv("TERRAFORM_WORKSPACE_ALLOWLIST"), ",") {
		item = strings.TrimSpace(item)
		if item != "" && workspaceName.MatchString(item) {
			allowed[item] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		logger.Error("TERRAFORM_WORKSPACE_ALLOWLIST is empty")
		os.Exit(1)
	}
	s := &server{
		token:         token,
		root:          env("TERRAFORM_WORKSPACE_ROOT", "/workspaces"),
		terraformPath: env("TERRAFORM_BIN", "/bin/terraform"),
		stateRoot:     env("TERRAFORM_STATE_ROOT", "/state"),
		timeout:       10 * time.Minute,
		allowed:       allowed,
		logger:        logger,
		semaphore:     make(chan struct{}, 1),
		runs:          []run{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/runs", s.authorize(s.list))
	mux.HandleFunc("POST /v1/runs", s.authorize(s.start))
	httpServer := &http.Server{Addr: env("TERRAFORM_WORKER_ADDR", "127.0.0.1:8090"), Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 11 * time.Minute, IdleTimeout: 30 * time.Second}
	logger.Info("Terraform worker listening", "address", httpServer.Addr, "workspaces", len(allowed))
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Terraform worker stopped", "error", err)
		os.Exit(1)
	}
}

func (s *server) authorize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		valid := len(provided) == len(s.token) && subtle.ConstantTimeCompare([]byte(provided), s.token) == 1
		if !valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) list(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	items := append([]run(nil), s.runs...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) start(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Operation string `json:"operation"`
		Workspace string `json:"workspace"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if input.Operation != "validate" && input.Operation != "plan" && input.Operation != "apply" && input.Operation != "destroy" && input.Operation != "output" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "operation is not allowlisted"})
		return
	}
	if _, ok := s.allowed[input.Workspace]; !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "workspace is not allowlisted"})
		return
	}
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	default:
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "worker is busy"})
		return
	}
	item := run{ID: randomID(), Workspace: input.Workspace, Operation: input.Operation, Status: "running", CreatedAt: time.Now().UTC()}
	s.save(item)
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	output, changes, err := s.execute(ctx, input.Operation, input.Workspace)
	finished := time.Now().UTC()
	item.FinishedAt = &finished
	item.Output = redact(output)
	item.Summary = changes
	if err != nil {
		item.Status = "failed"
		if item.Output == "" {
			item.Output = "Terraform execution failed without provider output."
		}
		s.logger.Warn("Terraform run failed", "run_id", item.ID, "workspace", item.Workspace, "operation", item.Operation)
	} else {
		item.Status = "succeeded"
	}
	s.save(item)
	writeJSON(w, http.StatusOK, item)
}

func (s *server) execute(ctx context.Context, operation, workspace string) (string, summary, error) {
	source := filepath.Join(s.root, workspace)
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return "Workspace directory is unavailable.", summary{}, errors.New("workspace unavailable")
	}
	temp, err := os.MkdirTemp("", "wc-hub-tf-")
	if err != nil {
		return "", summary{}, err
	}
	defer os.RemoveAll(temp)
	if err = copyWorkspace(source, temp); err != nil {
		return "Workspace copy was rejected.", summary{}, err
	}
	cache := filepath.Join(os.TempDir(), "terraform-plugin-cache")
	_ = os.MkdirAll(cache, 0o700)
	environment := append(os.Environ(), "TF_IN_AUTOMATION=1", "TF_INPUT=0", "TF_PLUGIN_CACHE_DIR="+cache)
	initOutput, err := command(ctx, temp, environment, s.terraformPath, "init", "-backend=false", "-input=false", "-no-color")
	if err != nil {
		return initOutput, summary{}, err
	}
	if operation == "validate" {
		validateOutput, validateErr := command(ctx, temp, environment, s.terraformPath, "validate", "-no-color")
		return initOutput + "\n" + validateOutput, summary{}, validateErr
	}
	if err = os.MkdirAll(s.stateRoot, 0o700); err != nil {
		return initOutput, summary{}, err
	}
	statePath := filepath.Join(s.stateRoot, workspace+".tfstate")
	if operation == "output" {
		output, outputErr := command(ctx, temp, environment, s.terraformPath, "output", "-json", "-state="+statePath)
		return initOutput + "\n" + output, summary{}, outputErr
	}
	planArguments := []string{"plan", "-input=false", "-no-color", "-state=" + statePath, "-out=plan.bin"}
	if operation == "destroy" {
		planArguments = append(planArguments, "-destroy")
	}
	planOutput, planErr := command(ctx, temp, environment, s.terraformPath, planArguments...)
	if planErr != nil {
		return initOutput + "\n" + planOutput, summary{}, planErr
	}
	jsonOutput, showErr := command(ctx, temp, environment, s.terraformPath, "show", "-json", "plan.bin")
	if showErr != nil {
		return initOutput + "\n" + planOutput, summary{}, showErr
	}
	changes := summarizePlan([]byte(jsonOutput))
	if operation == "apply" || operation == "destroy" {
		applyOutput, applyErr := command(ctx, temp, environment, s.terraformPath, "apply", "-input=false", "-auto-approve", "-no-color", "plan.bin")
		return initOutput + "\n" + planOutput + "\n" + applyOutput, changes, applyErr
	}
	return initOutput + "\n" + planOutput, changes, nil
}

func command(ctx context.Context, dir string, environment []string, name string, arguments ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, arguments...)
	cmd.Dir = dir
	cmd.Env = environment
	contents, err := cmd.CombinedOutput()
	if len(contents) > 2<<20 {
		contents = contents[:2<<20]
	}
	return string(contents), err
}

func copyWorkspace(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("workspace path escaped root")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("workspace symlinks are not allowed")
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() || entry.Name() == ".terraform.lock.hcl" {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, io.LimitReader(input, 8<<20))
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func summarizePlan(contents []byte) summary {
	var plan struct {
		ResourceChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if json.Unmarshal(contents, &plan) != nil {
		return summary{}
	}
	result := summary{}
	for _, resource := range plan.ResourceChanges {
		actions := append([]string(nil), resource.Change.Actions...)
		sort.Strings(actions)
		switch strings.Join(actions, ",") {
		case "create":
			result.Add++
		case "delete":
			result.Destroy++
		case "create,delete":
			result.Change++
		case "update":
			result.Change++
		}
	}
	return result
}

var secretPattern = regexp.MustCompile(`(?i)(token|password|secret|private[_-]?key|authorization)(\s*[=:]\s*)([^\s,]+)`)

func redact(value string) string {
	value = secretPattern.ReplaceAllString(value, "$1$2[REDACTED]")
	if len(value) > 2<<20 {
		value = value[:2<<20] + "\n[OUTPUT TRUNCATED]"
	}
	return strings.TrimSpace(value)
}

func (s *server) save(item run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.runs {
		if s.runs[index].ID == item.ID {
			s.runs[index] = item
			return
		}
	}
	s.runs = append([]run{item}, s.runs...)
	if len(s.runs) > 100 {
		s.runs = s.runs[:100]
	}
}

func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
