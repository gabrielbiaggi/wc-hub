package ociapp

import "net/http"

type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, handler *Handler, initErr ...string) {
	if handler == nil {
		handler = NewHandler(nil, nil, initErr...)
	}
	mux.HandleFunc("GET /api/v1/oci/overview", auth("oci.read", handler.overview))
	mux.HandleFunc("POST /api/v1/oci/instances/{action}", auth("oci.manage", handler.instanceAction))
	mux.HandleFunc("POST /api/v1/oci/instances", auth("oci.manage", handler.launchInstance))
	mux.HandleFunc("POST /api/v1/oci/autonomous-databases", auth("oci.manage", handler.createAutonomousDatabase))
}
