# Emergency and break-glass

If the hub misbehaves, use the Proxmox console or an independent management host. Do not weaken self-protection from the UI. Stop the Compose project, preserve database and logs, revoke integration tokens, then investigate offline.

The independent recovery path must not depend on WC Hub, its VM, its DNS or its credentials. Keep a tested Proxmox administrator account and offline recovery documentation.

