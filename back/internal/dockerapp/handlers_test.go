package dockerapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	dockeradapter "github.com/webcreations/wc-hub/back/internal/adapters/docker"
)

type fakeReader struct{}

func (fakeReader) Health(context.Context) (dockeradapter.Health, error) {
	return dockeradapter.Health{Reachable: true, Version: "test"}, nil
}
func (fakeReader) Inventory(context.Context) (dockeradapter.Inventory, error) {
	return dockeradapter.Inventory{Containers: []dockeradapter.Container{}, Images: []dockeradapter.Image{}, Stats: []dockeradapter.ContainerStats{}, Warnings: []string{}}, nil
}
func (fakeReader) ListContainers(context.Context) ([]dockeradapter.Container, error) {
	return []dockeradapter.Container{}, nil
}
func (fakeReader) ListImages(context.Context) ([]dockeradapter.Image, error) {
	return []dockeradapter.Image{}, nil
}
func (fakeReader) Stats(context.Context, []dockeradapter.Container) ([]dockeradapter.ContainerStats, error) {
	return []dockeradapter.ContainerStats{}, nil
}

func TestMountRoutesAppliesExistingReadPermission(t *testing.T) {
	mux := http.NewServeMux()
	permissions := []string{}
	middleware := func(permission string, next http.HandlerFunc) http.HandlerFunc {
		permissions = append(permissions, permission)
		return next
	}
	MountRoutes(mux, middleware, fakeReader{})

	for _, path := range []string{"health", "inventory", "containers", "images", "stats"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/docker/"+path, nil)
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("route %s returned %d", path, response.Code)
		}
	}
	if len(permissions) != 7 {
		t.Fatalf("expected seven protected routes, got %d", len(permissions))
	}
	for _, permission := range permissions[:5] {
		if permission != ReadPermission {
			t.Fatalf("unexpected permission %q", permission)
		}
	}
	if permissions[5] != "docker.manage" {
		t.Fatalf("unexpected management permission %q", permissions[5])
	}
	if permissions[6] != "docker.manage" {
		t.Fatalf("unexpected exec permission %q", permissions[6])
	}
}

func TestHandlerReturnsServiceUnavailableWithoutReader(t *testing.T) {
	handler := NewHandler(nil)
	response := httptest.NewRecorder()
	handler.Inventory(response, httptest.NewRequest(http.MethodGet, "/api/v1/docker/inventory", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}
