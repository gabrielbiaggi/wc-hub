# WC Hub network topology

Verified on 2026-07-19.

## Current paths

| Segment | Address | Role |
|---|---|---|
| Operator LAN | `10.10.50.0/24` | Windows host and WC Hub frontend |
| WC Hub | `10.10.50.16:8088` | Docker-published control-plane endpoint |
| pfSense WAN | `192.168.68.107/22` | upstream side of the pfSense VM on `vmbr0` |
| pfSense LAN | `10.20.30.1/24` | gateway for infrastructure VMs on `vmbr1` |
| Ubuntu infrastructure VM | `10.20.30.4/24` | Docker/Kubernetes host; gateway `10.20.30.1` |

The pfSense VM routes its `10.20.30.0/24` LAN, but its WAN upstream and the operator LAN use different physical gateways. There is currently no direct layer-3 route from pfSense to `10.10.50.0/24`. A Windows firewall exception alone cannot create that route.

WC Hub reaches the existing Proxmox and Ubuntu providers through their Tailscale addresses. New VMs behind pfSense should use one of these supported paths:

1. install/join Tailscale on the VM and configure its provider endpoint with the overlay address; or
2. advertise `10.20.30.0/24` through an approved Tailscale subnet router; or
3. add reciprocal static routes on both physical gateways and permit the traffic in pfSense.

Do not publish unauthenticated Docker/Kubernetes/Proxmox APIs on either LAN. Keep proxy tokens, service-account tokens and API signing keys in `ops/secrets`, which is Git-ignored and mounted read-only into the control plane.
