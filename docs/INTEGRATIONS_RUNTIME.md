# Integration runtime activation

The API mounts every integration route at startup, but an adapter is enabled only when its required endpoint and credentials are present. Empty variables keep the corresponding integration unavailable without preventing the rest of WC Hub from starting.

Run database migrations before activation. Migration `000005_integration_read_permissions` creates the read-only permissions used by these routes.

## Secret boundary

Place file-based credentials under `ops/secrets/`. Docker Compose mounts this directory read-only at `/run/secrets`; do not commit real credentials. Token values passed through the environment must be injected by the deployment secret manager, not stored in `.env` in production.

The MergerFS host directory is mounted read-only. Set `MERGERFS_HOST_ROOT` to the host mount and keep `MERGERFS_ROOT=/mnt/drive` inside the container.

## Docker

Required variables:

- `DOCKER_PROXY_URL`: HTTPS URL for a restricted socket proxy or mTLS agent.
- `DOCKER_TLS_CA_PATH`: CA bundle, normally `/run/secrets/docker_ca.pem`.
- `DOCKER_CLIENT_CERT_PATH`: client certificate.
- `DOCKER_CLIENT_KEY_PATH`: client private key.

The proxy should expose only the read endpoints required for containers, images and stats. Never point WC Hub directly at an unrestricted Docker socket.

## Kubernetes

Required variables:

- `KUBERNETES_API_URL`: HTTPS API server URL.
- `KUBERNETES_TOKEN_PATH`: ServiceAccount bearer token file.
- `KUBERNETES_CA_PATH`: cluster CA bundle.

Grant the ServiceAccount only `get`, `list` and `watch` for nodes, pods, deployments and events. The API reads the token from disk for each request so mounted secret rotation does not require a restart.

## Cloudflare

Required variables:

- `CLOUDFLARE_API_TOKEN`: scoped read token injected at startup.
- `CLOUDFLARE_ACCOUNT_ALLOWLIST`: comma-separated account IDs.
- `CLOUDFLARE_ZONE_ALLOWLIST`: comma-separated zone IDs.
- `WC_HUB_ENCRYPTION_KEY`: 32-byte base64 key used to seal the token in memory.

The token needs only tunnel-read and DNS-read permissions for the allowlisted resources.

## GitHub

Required variables:

- `GITHUB_TOKEN`: fine-grained PAT or installation token.
- `GITHUB_REPOSITORY_ALLOWLIST`: comma-separated `owner/repository` names.

Scope the credential to metadata, Actions and releases read access for those repositories only.

## Terraform worker

Required variables:

- `TERRAFORM_WORKER_URL`: HTTPS URL of the isolated ephemeral worker.
- `TERRAFORM_WORKER_TOKEN`: worker bearer token.
- `TERRAFORM_WORKSPACE_ALLOWLIST`: comma-separated immutable workspace identifiers.

WC Hub never invokes the Terraform binary. It calls this worker contract:

- `POST /v1/runs` with JSON `{ "operation": "validate|plan", "workspace": "allowlisted-name" }` returns a run object.
- `GET /v1/runs` returns `{ "items": [run] }`.

A run contains `id`, `workspace`, `operation`, `status`, `created_at`, `summary` (`add`, `change`, `destroy`) and redacted `output`. The worker must create a clean ephemeral directory per run, obtain short-lived provider credentials, disable interactive input, redact secrets and destroy the directory and credentials after completion. Apply is intentionally unsupported.

## MergerFS

Required variables:

- `MERGERFS_HOST_ROOT`: host path mounted read-only into the API container.
- `MERGERFS_ROOT`: container path, normally `/mnt/drive`.

Browse, index and stream operations are confined to this root. Path traversal and symlink escapes are rejected.

## Smoke test

After configuring a module, restart the `back` service and sign in with a role containing its `*.read` permission. Confirm the corresponding page loads, then inspect API logs for adapter initialization errors. Keep unused integration variables empty.

## Development-only master login

For a single-user local test environment, set `WC_HUB_ENV=development` and `WC_HUB_DEV_MASTER_LOGIN=true`. The username is `allmight`; its password is calculated in memory as `HubDDMMYYYYHH` using `WC_HUB_DEV_MASTER_TIMEZONE` (default `America/Sao_Paulo`). No password or password hash for this hourly credential is stored. Its session expires at the next hour boundary.

The API refuses to start with this option in production or staging. Docker Compose binds the frontend to `127.0.0.1` by default; changing `WC_HUB_BIND_IP` can expose the predictable credential to other machines and is not recommended.
