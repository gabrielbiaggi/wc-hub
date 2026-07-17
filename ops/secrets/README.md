# Runtime secrets

Place deployment-only files here; every file except this README is ignored by Git.

Expected names:

- `proxmox_ca.pem` — private CA that signs the PVE API certificate;
- `ssh_private_key` — dedicated unencrypted SSH key readable by the non-root backend container;
- `ssh_known_hosts` — host keys collected and verified out-of-band;
- `agent_server.crt`, `agent_server.key`, `agent_client_ca.pem` — only for a remotely reachable agent action listener.

Use restrictive host permissions and never reuse a personal SSH key.
