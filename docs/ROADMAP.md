# Delivery roadmap

1. Authentication bootstrap, WebAuthn/TOTP and complete RBAC middleware.
2. Durable audit repository, hash-chain checkpoints and job queue.
3. Proxmox inventory plus node/VM telemetry.
4. Restricted host agent for Docker, hardware and GPU collectors.
5. Kubernetes, GitHub and Cloudflare adapters.
6. Audited SSH/WebSocket terminal and MergerFS browser.
7. Isolated Terraform plan/apply workers.
8. Alerts, notification routing and long-term metrics storage.

Each adapter ships read-only first. Mutations are enabled only after policy tests, audit coverage and recovery runbooks exist.

