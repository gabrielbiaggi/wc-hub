# Security model

## Non-negotiable invariants

1. A target marked `self_protected` can only have `scope=local`.
2. There can be only one self-protected host in the database.
3. Destructive actions and commands against that host are denied before confirmation is evaluated.
4. The local executor accepts only allowlisted binaries and validates arguments separately.
5. Browser clients never receive provider, SSH, TOTP or encryption secrets.
6. Every requested and completed mutation creates an audit event.
7. TOTP verification happens server-side; client-provided verification flags are ignored.

Changing `WC_HUB_SELF_PROTECTED` is a break-glass deployment operation, not a UI feature. Production policy should refuse startup with it disabled unless a separate, offline recovery procedure is active.

## Action matrix

| Scope | Safe read | Allowlisted command | Destructive action |
|---|---:|---:|---:|
| Local self-protected | allow | policy + RBAC | always deny |
| Remote | allow | RBAC + audit | exact target confirmation + TOTP + RBAC |
| Cloud | allow | not applicable | exact resource confirmation + TOTP + RBAC |

The command allowlist is binary-based, not a shell string prefix. Never invoke `/bin/sh -c` with user input. Reject shell operators, command substitution, redirections, environment injection, path traversal and uncontrolled flags. Prefer typed operations (`DockerListContainers`) over generic commands.

## Terminal

Remote terminals require short-lived server-side sessions, host key verification, size and idle limits, concurrency limits, redacted recording metadata and explicit RBAC. The local host never exposes a raw shell. WebSocket origin checks and expiring single-use tokens are mandatory.

## Identity and TOTP

The deployed single-operator mode reserves the username `allmight` for
`WC_HUB_MASTER_EMAIL`. Its memorizable hourly password follows
`HubDDMMYYYYHH` in `WC_HUB_DEV_MASTER_TIMEZONE` and is never stored, hashed,
logged, or returned by the API. Because this first factor is intentionally
predictable, TOTP is the mandatory secret factor after enrollment.
The initial session exists only to enroll TOTP; once enrolled, every later
login requires both the hourly credential and the six-digit authenticator code.
Additional user creation is rejected while this mode is enabled.

The first administrator is created through a transactionally locked, one-time bootstrap. Passwords use bcrypt cost 12. Sessions use random opaque tokens stored only as SHA-256 digests; browser cookies are HttpOnly, SameSite Strict and must use `Secure` in production. Mutations require a rotating CSRF token.

TOTP follows RFC 6238 with a 30-second period and a one-step clock window. Secrets are encrypted with AES-256-GCM using `WC_HUB_ENCRYPTION_KEY`; this key must be a base64-encoded 32-byte value stored outside the repository. Critical policy decisions verify the submitted code on the server and never trust a boolean from the browser.

## Docker and Terraform

Do not mount `/var/run/docker.sock` in the API. Use a least-privileged socket proxy or remote agent. Run Terraform in isolated workers with pinned images, ephemeral workspaces and scoped credentials. Apply must reference an immutable reviewed plan digest; destroy is a critical action.

## Audit integrity

Audit rows include previous and current hashes to support a verifiable chain. Production should periodically export signed checkpoints to storage the hub cannot mutate. Sensitive payload fields are redacted before persistence.
