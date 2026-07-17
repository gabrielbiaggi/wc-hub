# Infrastructure runtime

## Proxmox source of truth

The Proxmox adapter uses `/api2/json` over HTTPS and requires an API token. TLS verification is always enabled; private certificate authorities are loaded through `PROXMOX_TLS_CA_PATH`. Password authentication and `InsecureSkipVerify` are intentionally unsupported.

Recommended token privileges are read-only equivalents for audit, cluster, datastore and VM inventory. The sync worker reads nodes, QEMU guests, LXC guests and storage, then commits the normalized inventory in one PostgreSQL transaction. A failed API call does not partially replace inventory.

```env
PROXMOX_API_URL=https://pve.example.internal:8006
PROXMOX_API_TOKEN_ID=wc-hub@pve!inventory
PROXMOX_API_TOKEN_SECRET=...
PROXMOX_TLS_CA_PATH=/run/secrets/proxmox_ca.pem
```

## Durable jobs and scheduler

Jobs are reserved with `FOR UPDATE SKIP LOCKED`, executed outside the HTTP lifecycle and retried with bounded exponential backoff. Unknown job kinds fail closed. The scheduler updates due schedules and emits normal durable jobs every 15 seconds; this keeps one execution model for manual and scheduled operations.

`proxmox.sync` is read-only but still requires RBAC, CSRF and audit. `telemetry.maintenance` marks stale agents offline. Worker concurrency is controlled by `WC_HUB_WORKER_COUNT`.

## Host agent

`wc-agent` is a separate small Go binary. It scrapes `node_exporter` and optional DCGM Exporter, filters metrics through a compile-time allowlist, and pushes batches using a random agent token stored only as a SHA-256 digest in WC Hub.

The action endpoint does not accept command strings. Its only typed operations are:

- `system.uptime`;
- `docker.ps` with a fixed output format;
- `journal.tail` with validated unit and bounded line count.

It never invokes a shell. Destructive commands do not exist in the action registry. A non-loopback action listener requires a server certificate and a client CA, enforcing mTLS. The Compose profile keeps it loopback-only because it currently operates in push mode.

Provision the self agent token through the offline CLI, never through the web UI:

```bash
docker compose run --rm back provision-agent-token wc-hub-local
```

Store the one-time output as `WC_AGENT_TOKEN`, then start Linux collection:

```bash
docker compose --profile agent up -d node-exporter host-agent
```

For an NVIDIA host with the NVIDIA Container Toolkit installed:

```bash
docker compose --profile agent --profile nvidia up -d node-exporter dcgm-exporter host-agent
```

Set `DCGM_EXPORTER_URL=http://dcgm-exporter:9400/metrics` for the agent.

## SSH terminal

The terminal uses this sequence:

1. authenticated user selects a remote host;
2. API checks `terminal.connect`, CSRF, exact host-name confirmation and server-side TOTP;
3. API creates a 45-second, single-use ticket stored as a SHA-256 digest;
4. WebSocket consumes the ticket atomically;
5. Go opens SSH with a server-held private key and mandatory `known_hosts` verification;
6. xterm.js exchanges typed input/output/resize frames;
7. ticket creation, open, close and session metadata are audited.

Local and self-protected hosts are rejected both when issuing and consuming a ticket. Configure:

```env
WC_HUB_SSH_PRIVATE_KEY_PATH=/run/secrets/ssh_private_key
WC_HUB_SSH_KNOWN_HOSTS_PATH=/run/secrets/ssh_known_hosts
```

Host facts require `ssh_address`, `ssh_user` and optional `ssh_port`. The terminal gateway refuses to start without both key and `known_hosts` files.

