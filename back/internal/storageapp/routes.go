package storageapp

import (
	mergerfsadapter "github.com/webcreations/wc-hub/back/internal/adapters/mergerfs"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *mergerfsadapter.Client) {
	h := &Handler{client}
	mux.HandleFunc("GET /api/v1/storage/browse", auth("storage.read", h.Browse))
	mux.HandleFunc("GET /api/v1/storage/index", auth("storage.read", h.Index))
	mux.HandleFunc("GET /api/v1/storage/stream", auth("storage.read", h.Stream))
	mux.HandleFunc("POST /api/v1/storage/directories", auth("storage.write", h.CreateDirectory))
	mux.HandleFunc("POST /api/v1/storage/upload", auth("storage.write", h.Upload))
	mux.HandleFunc("PATCH /api/v1/storage/entry", auth("storage.write", h.Rename))
	mux.HandleFunc("DELETE /api/v1/storage/entry", auth("storage.write", h.Delete))
	mux.HandleFunc("POST /api/v1/storage/rebalance", auth("storage.write", h.Rebalance))
}
