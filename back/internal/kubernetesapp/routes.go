package kubernetesapp

import (
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *kubernetesadapter.Client, initErr ...string) {
	message := ""
	if len(initErr) > 0 {
		message = initErr[0]
	}
	handler := &Handler{client: client, initErr: message}
	mux.HandleFunc("GET /api/v1/kubernetes/overview", auth("kubernetes.read", handler.Overview))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/deployments/{name}/{action}", auth("kubernetes.manage", handler.DeploymentAction))
	mux.HandleFunc("GET /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/logs", auth("kubernetes.read", handler.PodLogs))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/exec", auth("kubernetes.manage", handler.PodExec))
}
