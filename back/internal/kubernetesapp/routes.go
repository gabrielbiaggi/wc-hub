package kubernetesapp

import (
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware, client *kubernetesadapter.Client) {
	handler := &Handler{client: client}
	mux.HandleFunc("GET /api/v1/kubernetes/overview", auth("kubernetes.read", handler.Overview))
}
