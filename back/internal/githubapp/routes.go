package githubapp
import("net/http";githubadapter "github.com/webcreations/wc-hub/back/internal/adapters/github")
type AuthMiddleware func(string,http.HandlerFunc)http.HandlerFunc
func MountRoutes(mux *http.ServeMux,auth AuthMiddleware,client *githubadapter.Client){handler:=&Handler{client};mux.HandleFunc("GET /api/v1/github/overview",auth("github.read",handler.Overview))}
