package cloudflareapp

import "net/http"

// AuthMiddleware matches the wc-hub application middleware contract. Keeping
// it as a function type lets the global router mount this package as a plugin
// without importing application internals.
type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

// MountRoutes registers only read-only Cloudflare routes. The caller owns
// authentication, RBAC, CSRF and global request middleware.
func MountRoutes(mux *http.ServeMux, authMiddleware AuthMiddleware, handler *Handler) {
	if mux == nil || authMiddleware == nil || handler == nil {
		panic("cloudflareapp: mux, auth middleware and handler are required")
	}
	permission := handler.Permission()
	mux.HandleFunc("GET /api/v1/cloudflare/overview", authMiddleware(permission, handler.Overview))
	mux.HandleFunc("GET /api/v1/cloudflare/accounts/{account_id}/tunnels", authMiddleware(permission, handler.Tunnels))
	mux.HandleFunc("GET /api/v1/cloudflare/zones/{zone_id}/dns-records", authMiddleware(permission, handler.DNSRecords))
}
