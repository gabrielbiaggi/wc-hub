package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type RouteInfo struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Module        string `json:"module"`
	Permission    string `json:"permission"`
	ActionGuard   bool   `json:"action_guard"`
	OpenAPICovered bool   `json:"openapi_covered"`
}

func main() {
	routes := []RouteInfo{
		// Auth & Health
		{Method: "GET", Path: "/healthz", Module: "overview", Permission: "none", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/auth/bootstrap-status", Module: "auth", Permission: "none", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/bootstrap", Module: "auth", Permission: "none", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/login", Module: "auth", Permission: "none", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/logout", Module: "auth", Permission: "authenticated", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/auth/me", Module: "auth", Permission: "authenticated", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/totp/enroll", Module: "auth", Permission: "authenticated", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/totp/confirm", Module: "auth", Permission: "authenticated", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/auth/totp/disable", Module: "auth", Permission: "authenticated", ActionGuard: true, OpenAPICovered: true},

		// Docker
		{Method: "GET", Path: "/api/v1/docker/health", Module: "docker", Permission: "docker.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/docker/inventory", Module: "docker", Permission: "docker.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/docker/containers", Module: "docker", Permission: "docker.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/docker/images", Module: "docker", Permission: "docker.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/docker/stats", Module: "docker", Permission: "docker.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/docker/containers/{id}/{action}", Module: "docker", Permission: "docker.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/docker/containers/{id}/exec", Module: "docker", Permission: "docker.manage", ActionGuard: true, OpenAPICovered: true},

		// Kubernetes
		{Method: "GET", Path: "/api/v1/kubernetes/overview", Module: "kubernetes", Permission: "kubernetes.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/logs", Module: "kubernetes", Permission: "kubernetes.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/exec", Module: "kubernetes", Permission: "kubernetes.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/kubernetes/namespaces/{namespace}/deployments/{name}/{action}", Module: "kubernetes", Permission: "kubernetes.manage", ActionGuard: true, OpenAPICovered: true},

		// Proxmox
		{Method: "GET", Path: "/api/v1/proxmox/summary", Module: "proxmox", Permission: "proxmox.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/proxmox/sync", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/proxmox/inventory", Module: "proxmox", Permission: "proxmox.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/proxmox/nodes/{node}/{kind}/{vmid}/{action}", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/proxmox/qemu", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/proxmox/lxc", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/proxmox/clone", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "DELETE", Path: "/api/v1/proxmox/nodes/{node}/{kind}/{vmid}", Module: "proxmox", Permission: "proxmox.manage", ActionGuard: true, OpenAPICovered: true},

		// Terraform
		{Method: "GET", Path: "/api/v1/terraform/runs", Module: "terraform", Permission: "terraform.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/terraform/validate", Module: "terraform", Permission: "terraform.manage", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/terraform/plan", Module: "terraform", Permission: "terraform.manage", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/terraform/apply", Module: "terraform", Permission: "terraform.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/terraform/destroy", Module: "terraform", Permission: "terraform.manage", ActionGuard: true, OpenAPICovered: true},

		// Admin & RBAC
		{Method: "GET", Path: "/api/v1/admin/users", Module: "admin", Permission: "admin.users.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/admin/users", Module: "admin", Permission: "admin.users.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "PATCH", Path: "/api/v1/admin/users/{id}", Module: "admin", Permission: "admin.users.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "DELETE", Path: "/api/v1/admin/users/{id}", Module: "admin", Permission: "admin.users.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/admin/roles", Module: "admin", Permission: "admin.roles.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/admin/roles", Module: "admin", Permission: "admin.roles.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "PATCH", Path: "/api/v1/admin/roles/{id}", Module: "admin", Permission: "admin.roles.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "DELETE", Path: "/api/v1/admin/roles/{id}", Module: "admin", Permission: "admin.roles.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/admin/permissions", Module: "admin", Permission: "admin.roles.read", ActionGuard: false, OpenAPICovered: true},

		// Audit & Telemetry & Jobs
		{Method: "GET", Path: "/api/v1/audit", Module: "audit", Permission: "audit.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/jobs", Module: "jobs", Permission: "jobs.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/jobs", Module: "jobs", Permission: "jobs.manage", ActionGuard: false, OpenAPICovered: true},
		{Method: "GET", Path: "/api/v1/telemetry/hosts", Module: "telemetry", Permission: "telemetry.read", ActionGuard: false, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/hosts/{host_id}/agent-token", Module: "telemetry", Permission: "hosts.manage", ActionGuard: true, OpenAPICovered: true},
		{Method: "POST", Path: "/api/v1/terminal/tickets", Module: "terminal", Permission: "terminal.connect", ActionGuard: true, OpenAPICovered: true},
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Module == routes[j].Module {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Module < routes[j].Module
	})

	output, err := json.MarshalIndent(map[string]any{
		"total_endpoints": len(routes),
		"routes":          routes,
	}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating inventory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}
