package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsDirectSocketAndEmbeddedCredentials(t *testing.T) {
	for _, endpoint := range []string{"unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine", "https://user:secret@docker.example"} {
		if _, err := New(endpoint); err == nil {
			t.Fatalf("expected endpoint %q to be rejected", endpoint)
		}
	}
}

func TestNewRequiresCompleteMTLSIdentity(t *testing.T) {
	_, err := NewWithConfig(Config{Endpoint: "https://docker.example", ClientCertPath: "client.crt"})
	if err == nil || !strings.Contains(err.Error(), "together") {
		t.Fatalf("expected incomplete mTLS identity error, got %v", err)
	}
}

func TestInventoryNormalizesDockerAPIAndStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/_ping":
			_, _ = w.Write([]byte("OK"))
		case "/version":
			_, _ = w.Write([]byte(`{"Version":"27.5.1","ApiVersion":"1.47","Os":"linux","Arch":"amd64"}`))
		case "/containers/json":
			if r.URL.Query().Get("all") != "1" {
				t.Fatal("containers request must include stopped containers")
			}
			_, _ = w.Write([]byte(`[{"Id":"container-123456789","Names":["/api"],"Image":"wc-api:latest","ImageID":"sha256:image","Command":"./api","Created":1700000000,"State":"running","Status":"Up 2 hours","Ports":[{"IP":"127.0.0.1","PrivatePort":8080,"PublicPort":8088,"Type":"tcp"}],"Labels":{"com.example.role":"api"}}]`))
		case "/images/json":
			_, _ = w.Write([]byte(`[{"Id":"sha256:image","RepoTags":["wc-api:latest"],"RepoDigests":[],"Created":1700000000,"Size":1048576,"SharedSize":0,"Containers":1}]`))
		case "/containers/container-123456789/stats":
			_, _ = w.Write([]byte(`{"read":"2026-07-17T10:00:00Z","name":"/api","id":"container-123456789","cpu_stats":{"cpu_usage":{"total_usage":300,"percpu_usage":[150,150]},"system_cpu_usage":1000,"online_cpus":2},"precpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":500},"memory_stats":{"usage":1000,"limit":4000,"stats":{"cache":200}},"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}},"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":30},{"op":"Write","value":40}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := client.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Health.Version != "27.5.1" || len(inventory.Containers) != 1 || len(inventory.Images) != 1 || len(inventory.Stats) != 1 {
		t.Fatalf("unexpected inventory: %#v", inventory)
	}
	container := inventory.Containers[0]
	if container.ImageID != "sha256:image" || container.Ports[0].PrivatePort != 8080 {
		t.Fatalf("Docker API fields were not normalized: %#v", container)
	}
	stats := inventory.Stats[0]
	if stats.CPUPercent != 40 || stats.MemoryUsage != 800 || stats.MemoryPercent != 20 || stats.NetworkRX != 10 || stats.BlockWrite != 40 {
		t.Fatalf("stats were not normalized: %#v", stats)
	}
}

func TestFallsBackToUnixSocketWhenPrimaryTransportFails(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			if _, writeErr := io.WriteString(w, "OK"); writeErr != nil {
				t.Errorf("write Docker ping response: %v", writeErr)
			}
		case "/version":
			w.Header().Set("Content-Type", "application/json")
			if _, writeErr := io.WriteString(w, `{"Version":"27.5.1","ApiVersion":"1.47","Os":"linux","Arch":"amd64"}`); writeErr != nil {
				t.Errorf("write Docker version response: %v", writeErr)
			}
		default:
			http.NotFound(w, r)
		}
	})}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Cleanup(func() {
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			t.Errorf("close Unix socket HTTP server: %v", closeErr)
		}
		if resultErr := <-serveErr; resultErr != nil && !errors.Is(resultErr, http.ErrServerClosed) {
			t.Errorf("serve Unix socket HTTP server: %v", resultErr)
		}
	})

	client, err := NewWithConfig(Config{Endpoint: "http://127.0.0.1:1", FallbackSocketPath: socketPath, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !health.Reachable || health.Version != "27.5.1" {
		t.Fatalf("Unix socket fallback did not serve Docker health: %#v", health)
	}
}

func TestCounterDeltaDoesNotUnderflow(t *testing.T) {
	if value := counterDelta(10, 20); value != 0 {
		t.Fatalf("expected reset counter delta to be zero, got %f", value)
	}
}
