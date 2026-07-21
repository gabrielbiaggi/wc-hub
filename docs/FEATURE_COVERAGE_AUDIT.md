# Feature coverage audit

Audit date: 2026-07-19.

This matrix distinguishes compiled UI from a complete runtime feature. A feature is complete only when its backend contract, RBAC, error handling, frontend and automated test boundary exist. Real-provider validation additionally requires operator credentials and infrastructure.

## Delivered

| Area | Coverage | Evidence |
|---|---|---|
| Password auth, TOTP, sessions and CSRF | Implemented | PostgreSQL identities and sessions, encrypted TOTP, HttpOnly/SameSite cookie, CSRF and progressive IP/account login throttling |
| RBAC and administration | Implemented | users, roles, permissions, alerts and audited admin handlers |
| Durable audit and jobs | Implemented | hash-chained audit records, PostgreSQL queue, workers and scheduler |
| Overview | Live data | PostgreSQL inventory, telemetry, alerts and audit activity; demonstration counters were removed |
| Proxmox | Inventory plus control | multiple configured clusters, nodes, QEMU/LXC, storage, sync and audited start/stop/shutdown/reboot/reset |
| Docker | Inventory plus control | proxy inventory/stats and audited container start/stop/restart |
| Kubernetes | Inventory plus control | nodes, deployments, problem pods/events and audited scale/restart |
| Cloudflare | Inventory plus DNS control | encrypted token envelope, allowlists, tunnels, health and DNS create/update/delete |
| GitHub | Delivery control | explicit repository allowlist, repository permissions/details, Actions, releases and run rerun/cancel |
| MergerFS | File management | confined browse/index/stream plus upload, mkdir, rename and delete |
| Oracle Cloud | Inventory plus compute control | signed OCI SDK client, regions, ADs, compartments, instances, VCNs/subnets and audited power actions |
| Terraform | Validate, plan and apply | in-repository isolated worker, workspace allowlist, persistent non-root state volume and confirmation-gated apply |
| SSH terminal | Implemented | short-lived ticket, TOTP gate, known_hosts verification, WebSocket PTY and audit |
| Development master login | Local-only | `allmight`, hourly in-memory password, no stored password hash, god-admin role, hourly session expiry and production startup guard |

## Remaining gaps

### P0 - required before production exposure

1. **WebAuthn is absent.** TOTP exists, but hardware/passkey authentication is not implemented.
2. **The hourly master formula is development-only.** It is intentionally refused outside `development`, `local` or `test`; production needs a non-predictable identity flow.
3. **TLS/reverse proxy is required before exposure beyond trusted LAN/Tailscale.** The current LAN endpoint is plain HTTP by operator request.

### P1 - roadmap extensions

1. **Credential rotation UI remains metadata-only.** Runtime credentials are injected as ignored Docker secrets; values are never returned to the browser.
2. **External notification routing is absent.** Alert acknowledgement/resolution exists, but email/webhook/Slack escalation does not.
3. **Command palette is partial.** Resource-wide search and the complete command catalog are not wired.
4. **Provider health history is not durable/unified.** Live provider errors are displayed, but long-term SLA/history and universal alert generation remain.
5. **Provider write surfaces are intentionally curated.** Tokens/ACLs have administrative capability, while the UI currently exposes the common audited operations listed above rather than every vendor API endpoint.

### P2 - quality and operations

1. The production build reports an `OverviewView` chunk slightly above 500 kB; route/vendor chunking should be tuned.
2. Handler-level contract tests should be expanded beyond the current adapter and browser coverage.
3. A infraestrutura interna de pré-push deve executar Compose/Playwright com credenciais descartáveis; credenciais reais de providers permanecem fora desses testes.

## Runtime verification on 2026-07-19

- `go test ./...` and `go vet ./...`: passed.
- `npm run typecheck` and `npm run build`: passed; only the known Overview chunk warning remains.
- Docker Compose migrations through `000007_oci_permissions`: passed.
- Playwright master-login navigation across Docker, Kubernetes, Cloudflare, GitHub, Terraform, Proxmox, MergerFS and OCI: passed with no API 5xx or browser console errors.
- Live reads: Docker 4 containers/6 images; Cloudflare 7 tunnels/41 DNS; GitHub 12 allowlisted repositories; OCI 3 instances/2 VCNs/2 subnets/3 subscribed regions; Proxmox and Kubernetes responded successfully.
- Live writes: Terraform validate/plan/apply succeeded with zero changes in the acceptance workspace; MergerFS mkdir/rename/delete succeeded and cleaned up.
- Frontend is published only on `10.10.50.16:8088`; the duplicate localhost stack was stopped without deleting its volumes.
