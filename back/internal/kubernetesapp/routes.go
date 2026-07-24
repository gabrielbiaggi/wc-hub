package kubernetesapp

import (
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

type PolicyRequest struct {
	Action       string
	Scope        string
	TargetName   string
	Confirmation string
	TOTPCode     string
}

type PolicyEnforcer func(w http.ResponseWriter, r *http.Request, req PolicyRequest) bool

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *kubernetesadapter.Client, initErr ...string) {
	MountRoutesWithPolicy(mux, auth, nil, client, initErr...)
}

func MountRoutesWithPolicy(mux *http.ServeMux, auth AuthMiddleware, policyEnforcer PolicyEnforcer, client *kubernetesadapter.Client, initErr ...string) {
	message := ""
	if len(initErr) > 0 {
		message = initErr[0]
	}
	handler := &Handler{client: client, policyEnforcer: policyEnforcer, initErr: message}
	mux.HandleFunc("GET /api/v1/kubernetes/overview", auth("kubernetes.read", handler.Overview))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/deployments/{name}/{action}", auth("kubernetes.manage", handler.DeploymentAction))
	mux.HandleFunc("GET /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/logs", auth("kubernetes.read", handler.PodLogs))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/exec", auth("kubernetes.manage", handler.PodExec))
	mux.HandleFunc("POST /api/v1/kubernetes/apply", auth("kubernetes.manage", handler.ApplyManifest))
}
