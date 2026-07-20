package operationsapp

import (
	"encoding/json"
	"net/http"
)

type AuthMiddleware func(string, http.HandlerFunc) http.HandlerFunc

type Operation struct {
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	Resource     string `json:"resource"`
	Name         string `json:"name"`
	Permission   string `json:"permission"`
	Risk         string `json:"risk"`
	Confirmation string `json:"confirmation"`
	Execution    string `json:"execution"`
	Status       string `json:"status"`
	Route        string `json:"route,omitempty"`
}

// Catalog is the single source of truth for the administrative coverage plan.
// "available" entries are wired to a real route today; "planned" entries are
// intentionally visible but cannot be invoked until their typed contract,
// provider capability check and audit test exist.
func Catalog() []Operation {
	return []Operation{
		{"proxmox.qemu.create", "Proxmox", "QEMU", "Criar VM", "proxmox.manage", "critical", "confirmar recurso", "job", "available", "/api/v1/proxmox/qemu"},
		{"proxmox.guest.power", "Proxmox", "QEMU/LXC", "Controlar energia", "proxmox.manage", "critical", "confirmar recurso", "job", "available", "/api/v1/proxmox/nodes/{node}/{kind}/{vmid}/{action}"},
		{"proxmox.guest.snapshot", "Proxmox", "QEMU/LXC", "Snapshots e restore", "proxmox.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"proxmox.network.manage", "Proxmox", "Rede/Firewall", "Gerenciar rede, SDN e firewall", "proxmox.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"proxmox.storage.manage", "Proxmox", "Storage", "Gerenciar storage e conteúdo", "proxmox.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"cloudflare.dns.manage", "Cloudflare", "DNS", "Criar, editar e excluir DNS", "cloudflare.manage", "critical", "confirmar exclusão", "direct", "available", "/api/v1/cloudflare/zones/{zone_id}/dns-records"},
		{"cloudflare.tunnel.manage", "Cloudflare", "Zero Trust Tunnel", "Criar, editar, excluir e rotacionar tunnel", "cloudflare.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"cloudflare.access.manage", "Cloudflare", "Zero Trust Access", "Gerenciar aplicações, policies e service tokens", "cloudflare.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"cloudflare.rules.manage", "Cloudflare", "WAF/Rules", "Gerenciar regras de segurança e cache", "cloudflare.manage", "critical", "confirmar recurso", "job", "planned", ""},
		{"docker.container.manage", "Docker", "Container", "Controlar e executar em container", "docker.manage", "dangerous", "confirmar execução", "direct", "available", "/api/v1/docker/containers/{id}/{action}"},
		{"docker.compose.manage", "Docker", "Compose/Stack", "Deploy, diff e rollback de stacks", "docker.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"kubernetes.workload.manage", "Kubernetes", "Workload", "Scale, restart, logs e exec", "kubernetes.manage", "dangerous", "confirmar execução", "direct", "available", "/api/v1/kubernetes/namespaces/{namespace}/deployments/{name}/{action}"},
		{"kubernetes.manifest.apply", "Kubernetes", "Manifest/Helm", "Aplicar manifestos e releases Helm", "kubernetes.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"github.workflow.manage", "GitHub", "Actions", "Disparar, cancelar e editar workflows", "github.manage", "dangerous", "confirmar execução", "direct", "available", "/api/v1/github/repos/{owner}/{repo}/workflows/{workflow_id}/{action}"},
		{"github.repository.manage", "GitHub", "Repositório", "PRs, branches, proteção e segredos", "github.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"oci.compute.manage", "OCI", "Compute", "Criar e controlar instâncias", "oci.manage", "critical", "confirmação forte", "job", "available", "/api/v1/oci/instances"},
		{"oci.network.manage", "OCI", "Networking", "VCN, subnets, NSG e load balancers", "oci.manage", "critical", "confirmação forte", "job", "planned", ""},
		{"terraform.apply", "Terraform", "Workspace", "Plan, apply e destroy", "terraform.apply", "critical", "plano imutável", "job", "available", "/api/v1/terraform/apply"},
		{"storage.file.manage", "MergerFS", "Arquivos", "Upload, download, renomear e excluir", "storage.write", "dangerous", "confirmar exclusão", "direct", "available", "/api/v1/storage"},
	}
}

func MountRoutes(mux *http.ServeMux, auth AuthMiddleware) {
	mux.HandleFunc("GET /api/v1/operations/catalog", auth("overview.read", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": Catalog()})
	}))
}
