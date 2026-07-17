# Host agent rollout

1. Register the host in Inventory with the correct `local`, `remote` or `cloud` scope.
2. For a remote host, provision an agent token through the UI/API with exact-name confirmation and TOTP. For the self host, use the offline CLI.
3. Install Docker and start `node_exporter` plus `wc-agent`; do not expose exporter ports publicly.
4. For NVIDIA, install the NVIDIA driver and Container Toolkit before enabling the `nvidia` profile.
5. Confirm the host becomes `online` and expected metrics appear in Telemetry.
6. Revoke the token in PostgreSQL or the forthcoming token-management UI if a machine is reprovisioned.

For a standalone action listener, issue a dedicated server certificate and configure `WC_AGENT_TLS_CERT`, `WC_AGENT_TLS_KEY` and `WC_AGENT_CLIENT_CA`. The agent refuses a non-loopback listener without mTLS.

