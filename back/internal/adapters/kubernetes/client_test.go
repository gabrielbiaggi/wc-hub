package kubernetes

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOverviewUsesRotatableTokenAndFiltersProblems(t *testing.T) {
	t.Helper()
	token := "service-account-token-with-safe-length"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"node-a","uid":"n1"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`))
		case "/apis/apps/v1/deployments":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"api","namespace":"default","uid":"d1"},"spec":{"replicas":2},"status":{"readyReplicas":2}}]}`))
		case "/api/v1/pods":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"ok","uid":"p1"},"status":{"phase":"Running","containerStatuses":[{"name":"app","ready":true,"restartCount":0}]}},{"metadata":{"name":"broken","uid":"p2"},"status":{"phase":"Failed","reason":"CrashLoopBackOff"}}]}`))
		case "/api/v1/events":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"warning","uid":"e1"},"type":"Warning","reason":"BackOff","message":"container restart"},{"metadata":{"name":"normal","uid":"e2"},"type":"Normal"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "token")
	caPath := filepath.Join(directory, "ca.pem")
	if err := os.WriteFile(tokenPath, []byte("  "+token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caPath, certificate, 0o600); err != nil {
		t.Fatal(err)
	}

	client, err := New(Config{Endpoint: server.URL, TokenPath: tokenPath, CAPath: caPath})
	if err != nil {
		t.Fatal(err)
	}
	overview, err := client.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Nodes) != 1 || len(overview.Deployments) != 1 {
		t.Fatalf("unexpected inventory: %#v", overview)
	}
	if len(overview.ProblemPods) != 1 || overview.ProblemPods[0].Metadata.Name != "broken" {
		t.Fatalf("unexpected problem pods: %#v", overview.ProblemPods)
	}
	if len(overview.Events) != 1 || overview.Events[0].Reason != "BackOff" {
		t.Fatalf("unexpected warning events: %#v", overview.Events)
	}
}

func TestNewRejectsInsecureEndpoint(t *testing.T) {
	if _, err := New(Config{Endpoint: "http://cluster.local", TokenPath: "token"}); err == nil {
		t.Fatal("expected insecure endpoint to be rejected")
	}
}
