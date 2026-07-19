package terraform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunnerUsesTypedRunContract(t *testing.T) {
	token := "ephemeral-worker-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatal("missing worker authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["operation"] != "plan" || payload["workspace"] != "production" {
				t.Fatalf("unexpected payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"id":"run-1","workspace":"production","operation":"plan","status":"queued","summary":{"add":1,"change":0,"destroy":0}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/runs":
			if r.URL.Query().Get("limit") != "100" {
				t.Fatal("missing list limit")
			}
			_, _ = w.Write([]byte(`{"items":[{"id":"run-1","workspace":"production","operation":"plan","status":"succeeded","summary":{"add":1}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	runner, err := New(Config{WorkerURL: server.URL, WorkerToken: []byte(token), Workspaces: []string{"production"}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := runner.Start(context.Background(), "plan", "production")
	if err != nil || run.ID != "run-1" {
		t.Fatalf("unexpected start result: run=%#v err=%v", run, err)
	}
	runs, err := runner.List(context.Background())
	if err != nil || len(runs) != 1 || runs[0].Status != "succeeded" {
		t.Fatalf("unexpected list result: runs=%#v err=%v", runs, err)
	}
}

func TestRunnerRejectsMutationAndUnknownWorkspace(t *testing.T) {
	runner, err := New(Config{WorkerURL: "http://127.0.0.1:9999", WorkerToken: []byte("ephemeral-worker-token-value"), Workspaces: []string{"production"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runner.Start(context.Background(), "apply", "production"); err == nil {
		t.Fatal("expected apply to be rejected")
	}
	if _, err = runner.Start(context.Background(), "plan", "outside"); err == nil {
		t.Fatal("expected unknown workspace to be rejected")
	}
}
