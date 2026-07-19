package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOverviewOnlyRequestsAllowlistedRepository(t *testing.T) {
	token := "github-fine-grained-token-value"
	paths := []string{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" {
			t.Fatalf("unexpected host: %s", r.URL.Host)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatal("missing bearer token")
		}
		paths = append(paths, r.URL.RequestURI())
		body := `[]`
		switch r.URL.Path {
		case "/repos/acme/control-plane":
			body = `{"id":1,"full_name":"acme/control-plane","default_branch":"main"}`
		case "/repos/acme/control-plane/actions/runs":
			body = `{"workflow_runs":[{"id":2,"name":"CI","status":"completed","conclusion":"success"}]}`
		case "/repos/acme/control-plane/releases":
			body = `[{"id":3,"tag_name":"v1.0.0"}]`
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	client, err := New(Config{Token: []byte(token), Repositories: []string{"acme/control-plane", "acme/control-plane"}, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	overview, err := client.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 || len(overview.Projects) != 1 || len(overview.Projects[0].WorkflowRuns) != 1 || len(overview.Projects[0].Releases) != 1 {
		t.Fatalf("unexpected overview: paths=%v result=%#v", paths, overview)
	}
}

func TestNewRejectsInvalidRepository(t *testing.T) {
	_, err := New(Config{Token: []byte("github-fine-grained-token-value"), Repositories: []string{"../outside"}})
	if err == nil {
		t.Fatal("expected invalid allowlist entry to be rejected")
	}
}
