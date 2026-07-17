package dockerapp

import (
	"context"
	"encoding/json"
	"net/http"

	dockeradapter "github.com/webcreations/wc-hub/back/internal/adapters/docker"
)

type Reader interface {
	Health(context.Context) (dockeradapter.Health, error)
	Inventory(context.Context) (dockeradapter.Inventory, error)
	ListContainers(context.Context) ([]dockeradapter.Container, error)
	ListImages(context.Context) ([]dockeradapter.Image, error)
	Stats(context.Context, []dockeradapter.Container) ([]dockeradapter.ContainerStats, []string)
}

type Handler struct{ reader Reader }

func NewHandler(reader Reader) *Handler { return &Handler{reader: reader} }

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	item, err := h.reader.Health(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_unreachable", "The restricted Docker API proxy is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) Inventory(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	item, err := h.reader.Inventory(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_inventory_failed", "Docker inventory could not be loaded from the restricted proxy.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) Containers(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	items, err := h.reader.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_containers_failed", "Docker containers could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Images(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	items, err := h.reader.ListImages(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_images_failed", "Docker images could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	containers, err := h.reader.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_stats_failed", "Docker container stats could not be loaded.")
		return
	}
	items, warnings := h.reader.Stats(r.Context(), containers)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "warnings": warnings})
}

func (h *Handler) available(w http.ResponseWriter) bool {
	if h.reader != nil {
		return true
	}
	writeError(w, http.StatusServiceUnavailable, "docker_unconfigured", "The Docker adapter has not been configured.")
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
