# WC Hub network topology

Verified on 2026-07-21 after the `prox-ai` migration.

## Current paths

| Segment | Address | Role |
|---|---|---|
| Operator LAN | `10.10.50.0/24` | operator workstations and the Proxmox management interface |
| Proxmox `ai` | `10.10.50.21:8006` | certificate-valid Proxmox API endpoint |
| pfSense WAN | `192.168.68.107/22` | upstream side of the pfSense VM on `vmbr0` |
| pfSense LAN | `10.20.40.1/24` | gateway and DHCP server (`.100-.200`) for `vmbr1` |
| Proxmox `vmbr1` | `10.20.40.2/24` | host-side storage and management path |
| WC Hub VM | `10.20.40.4/24` | self-protected VM 101; gateway `10.20.40.1` |
| WC Hub local endpoint | `10.20.40.4:8088` | Docker-published control-plane endpoint |
| WC Hub public endpoint | `https://hub.webcreations.com.br` | Cloudflare Tunnel `prox-ai` ingress |

The pfSense VM routes its `10.20.40.0/24` LAN, but its WAN upstream and the operator LAN use different physical gateways. The Hub uses the Proxmox management endpoint at `10.10.50.21`; external browser access terminates through the existing `prox-ai` Cloudflare Tunnel.

Tailscale is intentionally disabled on VM 101 because the VM was cloned and must not reuse the source machine identity. New VMs behind pfSense should use one of these supported paths:

1. install/join Tailscale on the VM and configure its provider endpoint with the overlay address; or
2. advertise `10.20.40.0/24` through an approved Tailscale subnet router; or
3. add reciprocal static routes on both physical gateways and permit the traffic in pfSense.

Do not publish unauthenticated Docker/Kubernetes/Proxmox APIs on either LAN. Keep proxy tokens, service-account tokens and API signing keys in `ops/secrets`, which is Git-ignored and mounted read-only into the control plane.
