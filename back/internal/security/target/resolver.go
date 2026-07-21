package target

import (
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Resolver struct {
	mu           sync.RWMutex
	hostname     string
	containerIDs []string
	podNames     []string
	vmIDs        []int
	workspaces   []string
	hostIDs      []string
	localIPs     []string
}

func NewResolver() *Resolver {
	r := &Resolver{
		containerIDs: []string{"wc-hub", "wc-hub-api", "wc-hub-backend", "wc_hub", "hub"},
		podNames:     []string{"wc-hub", "wc-hub-api", "wc-hub-backend", "hub"},
		workspaces:   []string{"wc-hub", "hub-infrastructure", "wc-hub-prod", "self"},
		hostIDs:      []string{"local", "self", "wc-hub", "localhost", "127.0.0.1"},
	}

	if hn, err := os.Hostname(); err == nil && hn != "" {
		r.hostname = strings.ToLower(hn)
		r.containerIDs = append(r.containerIDs, r.hostname)
		r.podNames = append(r.podNames, r.hostname)
		r.hostIDs = append(r.hostIDs, r.hostname)
	}

	if cid := os.Getenv("HUB_CONTAINER_ID"); cid != "" {
		r.containerIDs = append(r.containerIDs, strings.ToLower(cid))
	}
	if cname := os.Getenv("HUB_CONTAINER_NAME"); cname != "" {
		r.containerIDs = append(r.containerIDs, strings.ToLower(cname))
	}
	if pod := os.Getenv("HUB_POD_NAME"); pod != "" {
		r.podNames = append(r.podNames, strings.ToLower(pod))
	}
	if vmidStr := os.Getenv("HUB_PROXMOX_VMID"); vmidStr != "" {
		if id, err := strconv.Atoi(vmidStr); err == nil {
			r.vmIDs = append(r.vmIDs, id)
		}
	}
	if ws := os.Getenv("HUB_TERRAFORM_WORKSPACE"); ws != "" {
		r.workspaces = append(r.workspaces, strings.ToLower(ws))
	}
	if hid := os.Getenv("HUB_HOST_ID"); hid != "" {
		r.hostIDs = append(r.hostIDs, strings.ToLower(hid))
	}

	// Discover local IPs
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if addrs, err := iface.Addrs(); err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip != nil {
						r.localIPs = append(r.localIPs, ip.String())
					}
				}
			}
		}
	}

	return r
}

func (r *Resolver) IsSelfProtectedContainer(idOrName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := resourceName(idOrName)
	if target == "" {
		return false
	}

	for _, cid := range r.containerIDs {
		if target == cid || strings.HasPrefix(target, cid+"-") {
			return true
		}
	}
	return false
}

func (r *Resolver) IsSelfProtectedPod(podOrDeploymentName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := resourceName(podOrDeploymentName)
	if target == "" {
		return false
	}

	for _, p := range r.podNames {
		if target == p || strings.HasPrefix(target, p+"-") {
			return true
		}
	}
	return false
}

func (r *Resolver) IsSelfProtectedVM(vmid int, name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, id := range r.vmIDs {
		if vmid == id && vmid != 0 {
			return true
		}
	}

	targetName := strings.ToLower(strings.TrimSpace(name))
	if targetName != "" {
		for _, cid := range r.containerIDs {
			if targetName == cid || strings.Contains(targetName, cid) {
				return true
			}
		}
	}
	return false
}

func (r *Resolver) IsSelfProtectedWorkspace(workspaceName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := resourceName(workspaceName)
	if target == "" {
		return false
	}

	for _, ws := range r.workspaces {
		if target == ws {
			return true
		}
	}
	return false
}

// resourceName accepts both raw provider identifiers and the canonical audit
// targets used by handlers (for example docker/container/<id>). Keeping the
// normalization here prevents a handler prefix from bypassing self-protection.
func resourceName(value string) string {
	target := strings.ToLower(strings.Trim(strings.TrimSpace(value), "/"))
	if index := strings.LastIndexByte(target, '/'); index >= 0 {
		return target[index+1:]
	}
	return target
}

func (r *Resolver) IsSelfProtectedHost(idOrNameOrIP string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	target := strings.ToLower(strings.TrimSpace(idOrNameOrIP))
	if target == "" {
		return false
	}

	for _, hid := range r.hostIDs {
		if target == hid || strings.Contains(target, hid) {
			return true
		}
	}
	for _, ip := range r.localIPs {
		if target == ip {
			return true
		}
	}
	return false
}
