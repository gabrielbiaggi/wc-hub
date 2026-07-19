package dockerapp

import "net/http"

const ReadPermission = "docker.read"

type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

// MountRoutes exports the Docker module as an isolated plugin. The global app
// only needs to call this function with its existing RBAC middleware.
func MountRoutes(mux *http.ServeMux, authMiddleware AuthMiddleware, reader Reader) {
	handler := NewHandler(reader)
	mux.HandleFunc("GET /api/v1/docker/health", authMiddleware(ReadPermission, handler.Health))
	mux.HandleFunc("GET /api/v1/docker/inventory", authMiddleware(ReadPermission, handler.Inventory))
	mux.HandleFunc("GET /api/v1/docker/containers", authMiddleware(ReadPermission, handler.Containers))
	mux.HandleFunc("GET /api/v1/docker/images", authMiddleware(ReadPermission, handler.Images))
	mux.HandleFunc("GET /api/v1/docker/stats", authMiddleware(ReadPermission, handler.Stats))
	mux.HandleFunc("POST /api/v1/docker/containers/{id}/{action}", authMiddleware("docker.manage", handler.ContainerAction))
}
