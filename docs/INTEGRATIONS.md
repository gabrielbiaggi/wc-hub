# Integration contracts

| Module | Auth strategy | Initial capability | Mutations |
|---|---|---|---|
| Proxmox | scoped API token | nodes, VMs, storage, telemetry | queued and policy-gated |
| Docker | restricted socket proxy / mTLS agent | containers, images, stats | typed operations only |
| Kubernetes/K3s | service account / kubeconfig secret | nodes, workloads, events | RBAC + namespace constraints |
| GitHub | fine-grained PAT or GitHub App | repos, workflows, releases | explicit repository allowlist |
| Cloudflare | scoped API token | tunnels, DNS, health | zone/account allowlist |
| Terraform | ephemeral worker credentials | validate and plan | approved immutable plan only |
| SSH | encrypted key + known_hosts | audited PTY | remote targets only |
| MergerFS | constrained filesystem agent | browse, index, stream | read-only by default |

Adapters must implement health reporting and stable external IDs. Credentials are encrypted with envelope encryption and are only decrypted inside the adapter call boundary.

Runtime activation, secret filenames, allowlist formats and the Terraform worker HTTP contract are documented in [INTEGRATIONS_RUNTIME.md](INTEGRATIONS_RUNTIME.md).

