# Deploy on Ubuntu Server

1. Create a dedicated unprivileged `wc-hub` user and clone the repository.
2. Install Docker Engine with the Compose plugin; do not expose its TCP API.
3. Copy `.env.example` to `.env`, replace every secret and keep mode `0600`.
4. Confirm `WC_HUB_SELF_PROTECTED=true` and set the exact local target name.
5. Run `docker compose config`, then `docker compose up -d --build`.
6. Verify `curl -fsS http://127.0.0.1:8088/healthz`.
7. Configure Cloudflare Tunnel from `infra/cloudflare/tunnel.example.yml`.
8. Require Cloudflare Access or an equivalent identity boundary before public use.

Back up PostgreSQL off-host. Test restore before enabling mutating integrations.

