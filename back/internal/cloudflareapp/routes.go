package cloudflareapp

import "net/http"

// AuthMiddleware matches the wc-hub application middleware contract. Keeping
// it as a function type lets the global router mount this package as a plugin
// without importing application internals.
type AuthMiddleware func(permission string, next http.HandlerFunc) http.HandlerFunc

// MountRoutes registers allowlisted Cloudflare inventory and mutation routes.
func MountRoutes(mux *http.ServeMux, authMiddleware AuthMiddleware, handler *Handler) {
	if mux == nil || authMiddleware == nil {
		panic("cloudflareapp: mux and auth middleware are required")
	}
	if handler == nil {
		unavailable := func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "cloudflare_unconfigured", "Cloudflare encrypted credentials and allowlists are not configured.")
		}
		mux.HandleFunc("GET /api/v1/cloudflare/overview", authMiddleware(defaultPermission, unavailable))
		mux.HandleFunc("GET /api/v1/cloudflare/accounts/{account_id}/tunnels", authMiddleware(defaultPermission, unavailable))
		mux.HandleFunc("GET /api/v1/cloudflare/zones/{zone_id}/dns-records", authMiddleware(defaultPermission, unavailable))
		return
	}
	permission := handler.Permission()
	mux.HandleFunc("GET /api/v1/cloudflare/overview", authMiddleware(permission, handler.Overview))
	mux.HandleFunc("GET /api/v1/cloudflare/accounts/{account_id}/tunnels", authMiddleware(permission, handler.Tunnels))
	mux.HandleFunc("GET /api/v1/cloudflare/zones/{zone_id}/dns-records", authMiddleware(permission, handler.DNSRecords))
	mux.HandleFunc("POST /api/v1/cloudflare/zones/{zone_id}/dns-records", authMiddleware("cloudflare.manage", handler.CreateDNSRecord))
	mux.HandleFunc("PUT /api/v1/cloudflare/zones/{zone_id}/dns-records/{record_id}", authMiddleware("cloudflare.manage", handler.UpdateDNSRecord))
	mux.HandleFunc("DELETE /api/v1/cloudflare/zones/{zone_id}/dns-records/{record_id}", authMiddleware("cloudflare.manage", handler.DeleteDNSRecord))
	mux.HandleFunc("GET /api/v1/cloudflare/zones/{zone_id}/settings", authMiddleware(permission, handler.ZoneSettings))
	mux.HandleFunc("PATCH /api/v1/cloudflare/zones/{zone_id}/settings/{setting}", authMiddleware("cloudflare.manage", handler.UpdateZoneSetting))
	mux.HandleFunc("POST /api/v1/cloudflare/zones/{zone_id}/purge-cache", authMiddleware("cloudflare.manage", handler.PurgeCache))
	mux.HandleFunc("GET /api/v1/cloudflare/zones/{zone_id}/rulesets", authMiddleware(permission, handler.Rulesets))
}
