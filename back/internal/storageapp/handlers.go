package storageapp

import (
	"context"
	"encoding/json"
	"errors"
	mergerfsadapter "github.com/webcreations/wc-hub/back/internal/adapters/mergerfs"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Handler struct{ client *mergerfsadapter.Client }

func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	items, err := h.client.Browse(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "path": r.URL.Query().Get("path")})
}
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.client.Index(r.Context(), r.URL.Query().Get("path"), limit)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	file, info, err := h.client.Open(r.URL.Query().Get("path"))
	if err != nil {
		h.failure(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	name := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(info.Name)
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Content-Type", info.MIMEType)
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err := io.Copy(w, file); err != nil && !errors.Is(err, context.Canceled) {
		return
	}
}
func (h *Handler) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	var input struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "Path and directory name are required.")
		return
	}
	entry, err := h.client.CreateDirectory(input.Path, input.Name)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 201, entry)
}
func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	var input struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "Path and new name are required.")
		return
	}
	entry, err := h.client.Rename(input.Path, input.Name)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 200, entry)
}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	if err := h.client.Delete(r.URL.Query().Get("path")); err != nil {
		h.failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "invalid_upload", "A file is required.")
		return
	}
	defer file.Close()
	entry, err := h.client.WriteFile(r.Context(), r.URL.Query().Get("path"), header.Filename, file)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, 201, entry)
}
func (h *Handler) ready(w http.ResponseWriter) bool {
	if h.client != nil {
		return true
	}
	writeError(w, 503, "storage_unconfigured", "MergerFS constrained root is not configured.")
	return false
}
func (h *Handler) failure(w http.ResponseWriter, err error) {
	if errors.Is(err, mergerfsadapter.ErrPathDenied) {
		writeError(w, 403, "storage_path_denied", "Storage path is outside the configured root.")
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, 404, "storage_not_found", "Storage entry was not found.")
		return
	}
	writeError(w, 409, "storage_unavailable", "Storage operation could not be completed: "+err.Error())
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
