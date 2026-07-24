package dockerapp

import "net/http"

const ReadPermission = "docker.read"

type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

type PolicyRequest struct {
	Action       string
	Scope        string
	TargetName   string
	Confirmation string
	TOTPCode     string
}

type PolicyEnforcer func(w http.ResponseWriter, r *http.Request, req PolicyRequest) bool

// MountRoutes exports the Docker module as an isolated plugin. The global app
// only needs to call this function with its existing RBAC middleware and policy enforcer.
func MountRoutes(mux *http.ServeMux, authMiddleware AuthMiddleware, reader Reader, initErr ...string) {
	MountRoutesWithPolicy(mux, authMiddleware, nil, reader, initErr...)
}

// MountRoutesWithPolicy mounts routes with optional policy enforcement for critical operations
func MountRoutesWithPolicy(mux *http.ServeMux, authMiddleware AuthMiddleware, policyEnforcer PolicyEnforcer, reader Reader, initErr ...string) {
	handler := NewHandler(reader, policyEnforcer, initErr...)
	mux.HandleFunc("GET /api/v1/docker/health", authMiddleware(ReadPermission, handler.Health))
	mux.HandleFunc("GET /api/v1/docker/inventory", authMiddleware(ReadPermission, handler.Inventory))
	mux.HandleFunc("GET /api/v1/docker/containers", authMiddleware(ReadPermission, handler.Containers))
	mux.HandleFunc("GET /api/v1/docker/images", authMiddleware(ReadPermission, handler.Images))
	mux.HandleFunc("GET /api/v1/docker/stats", authMiddleware(ReadPermission, handler.Stats))
	mux.HandleFunc("POST /api/v1/docker/containers/{id}/{action}", authMiddleware("docker.manage", handler.ContainerAction))
	mux.HandleFunc("POST /api/v1/docker/containers/{id}/exec", authMiddleware("docker.manage", handler.Exec))
	mux.HandleFunc("GET /api/v1/docker/containers/{id}/update-stream", authMiddleware("docker.manage", handler.UpdateStream))
	mux.HandleFunc("POST /api/v1/docker/stacks/clone", authMiddleware("docker.clone", handler.CloneStack))
	mux.HandleFunc("POST /api/v1/docker/images/pull", authMiddleware("docker.manage", handler.PullImage))
	mux.HandleFunc("DELETE /api/v1/docker/images/{id}", authMiddleware("docker.manage", handler.DeleteImage))
	mux.HandleFunc("POST /api/v1/docker/images/prune", authMiddleware("docker.manage", handler.PruneImages))
	mux.HandleFunc("POST /api/v1/docker/volumes", authMiddleware("docker.manage", handler.CreateVolume))
	mux.HandleFunc("DELETE /api/v1/docker/volumes/{name}", authMiddleware("docker.manage", handler.DeleteVolume))
	mux.HandleFunc("POST /api/v1/docker/volumes/prune", authMiddleware("docker.manage", handler.PruneVolumes))
	mux.HandleFunc("POST /api/v1/docker/networks", authMiddleware("docker.manage", handler.CreateNetwork))
	mux.HandleFunc("DELETE /api/v1/docker/networks/{id}", authMiddleware("docker.manage", handler.DeleteNetwork))
	mux.HandleFunc("POST /api/v1/docker/networks/prune", authMiddleware("docker.manage", handler.PruneNetworks))
}
