package githubapp
import("encoding/json";"net/http";githubadapter "github.com/webcreations/wc-hub/back/internal/adapters/github")
type Handler struct{client *githubadapter.Client}
func(h *Handler)Overview(w http.ResponseWriter,r *http.Request){if h.client==nil{writeError(w,503,"github_unconfigured","GitHub token and repository allowlist are not configured.");return};data,err:=h.client.Overview(r.Context());if err!=nil{writeError(w,502,"github_unavailable","GitHub inventory is temporarily unavailable.");return};writeJSON(w,200,data)}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_=json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
