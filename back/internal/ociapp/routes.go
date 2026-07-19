package ociapp

import "net/http"

type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, handler *Handler) {
	mux.HandleFunc("GET /api/v1/oci/overview", auth("oci.read", handler.overview))
	mux.HandleFunc("POST /api/v1/oci/instances/{action}", auth("oci.manage", handler.instanceAction))
}
