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
	mux.HandleFunc("DELETE /api/v1/oci/instances/{id}", auth("oci.manage", handler.terminateInstance))
	mux.HandleFunc("GET /api/v1/oci/images", auth("oci.read", handler.listImages))
	mux.HandleFunc("GET /api/v1/oci/shapes", auth("oci.read", handler.listShapes))
	mux.HandleFunc("POST /api/v1/oci/autonomous-databases/{id}/{action}", auth("oci.manage", handler.autonomousDatabaseAction))
	mux.HandleFunc("POST /api/v1/oci/autonomous-databases", auth("oci.manage", handler.createAutonomousDatabase))
	mux.HandleFunc("POST /api/v1/oci/db-systems/{id}/{action}", auth("oci.manage", handler.dbSystemAction))
	mux.HandleFunc("POST /api/v1/oci/vcns", auth("oci.manage", handler.createVCN))
	mux.HandleFunc("DELETE /api/v1/oci/vcns/{id}", auth("oci.manage", handler.deleteVCN))
	mux.HandleFunc("POST /api/v1/oci/subnets", auth("oci.manage", handler.createSubnet))
	mux.HandleFunc("DELETE /api/v1/oci/subnets/{id}", auth("oci.manage", handler.deleteSubnet))
	mux.HandleFunc("POST /api/v1/oci/volumes", auth("oci.manage", handler.createBlockVolume))
	mux.HandleFunc("DELETE /api/v1/oci/volumes/{id}", auth("oci.manage", handler.deleteBlockVolume))
}
