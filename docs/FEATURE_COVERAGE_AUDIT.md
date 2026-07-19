# Feature coverage audit

Audit date: 2026-07-19.

This matrix distinguishes compiled UI from a complete runtime feature. A feature is complete only when its backend contract, RBAC, error handling, frontend and automated test boundary exist. Real-provider validation additionally requires operator credentials and infrastructure.

## Delivered

| Area | Coverage | Evidence |
|---|---|---|
| Password auth, TOTP, sessions and CSRF | Implemented | PostgreSQL identities and sessions, encrypted TOTP secret, HttpOnly/SameSite cookie and CSRF middleware |
| RBAC and administration | Implemented | users, roles, permissions, alerts and audited admin handlers |
| Durable audit and jobs | Implemented | hash-chained audit records, PostgreSQL queue, workers and scheduler |
| Proxmox | Read-only plus queued sync | nodes, VMs, storage and inventory synchronization |
| Docker | Read-only | restricted HTTPS/mTLS client, containers, images, health and stats |
| Kubernetes | Read-only | ServiceAccount token, CA validation, nodes, deployments, problem pods and warning events |
| Cloudflare | Read-only | encrypted token envelope, account/zone allowlists, tunnels and DNS |
| GitHub | Read-only | repository allowlist, metadata, Actions runs and releases |
| MergerFS | Read-only | browse, index and stream with traversal/symlink confinement |
| SSH terminal | Implemented | short-lived ticket, TOTP gate, known_hosts verification, WebSocket PTY and audit |
| Terraform control-plane client | Partial | validate/plan requests, workspace allowlist and run history are implemented; the external worker is not in this repository |
| Development master login | Local-only | `allmight`, hourly in-memory password, no stored password hash, god-admin role, hourly session expiry and production startup guard |

## Gaps found

### P0 - required before claiming production completeness

1. **Overview uses demonstration values.** `internal/overview/application.Service.Snapshot` returns fixed metrics and activity instead of aggregating PostgreSQL telemetry, alerts and integration health.
2. **WebAuthn is absent.** The roadmap lists WebAuthn/TOTP, but only password plus TOTP is implemented.
3. **No end-to-end CI suite.** Adapter unit tests and builds exist, but there is no automated browser/API/PostgreSQL Compose test in CI.
4. **Development master is not a production identity.** Its formula is predictable by design. The application therefore refuses to start with it outside `development`, `local` or `test`.
5. **Login throttling is absent.** Authentication failures are audited, but the API does not yet enforce per-IP/account rate limits or progressive backoff.

### P1 - incomplete roadmap capabilities

1. **Oracle Cloud is a placeholder.** `/cloud` renders `ModuleView.vue`; there is no OCI adapter, API client, RBAC permission or provider test.
2. **Terraform worker is external and absent.** WC Hub implements the HTTP contract but does not provide the sandboxed ephemeral executor or apply approval flow.
3. **Integration credential lifecycle is environment-only.** The UI stores integration metadata; it does not create, rotate or revoke encrypted credentials through the control plane.
4. **Notification routing is absent.** The operational inbox supports persistent alert acknowledgement/resolution, but email, webhook, Slack or escalation policies are not implemented.
5. **Command palette is partial.** `Ctrl/Cmd+K` opens a shell with one telemetry shortcut; search, commands and resource navigation are not wired.
6. **Provider health is not unified.** Individual adapters expose status/error behavior, but there is no durable cross-provider health history and alert generation for every integration.

### P2 - quality and operations

1. The production build reports an `OverviewView` chunk slightly above 500 kB; route/vendor chunking should be tuned.
2. Handler-level tests are strongest for Docker; Kubernetes, GitHub, Cloudflare, Terraform and storage would benefit from explicit RBAC/unconfigured/error response contract tests.
3. Real-provider smoke tests remain required for each configured endpoint because CI cannot safely contain production credentials.

## Runtime verification on 2026-07-19

- `go test ./...`: passed.
- `go vet ./...`: passed.
- `npm run build`: passed, with the known chunk-size warning.
- `docker compose config --quiet`: passed.
- Docker Desktop container execution: blocked by the Windows WSL backend returning `HCS_E_CONNECTION_TIMEOUT`; Docker Desktop remained in `starting` after a forced restart and `wsl --shutdown`.

Once WSL is healthy, the pending local acceptance test is: build the isolated Compose project, wait for migrations and health checks, authenticate as `allmight`, verify the session expires at the hour boundary, navigate every real route, check browser console errors and confirm unconfigured adapters fail gracefully.
