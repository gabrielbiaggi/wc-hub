INSERT INTO permissions (slug, description, risk) VALUES
  ('docker.manage', 'Start, stop and restart Docker containers.', 'dangerous'),
  ('kubernetes.manage', 'Scale and restart Kubernetes workloads.', 'dangerous'),
  ('cloudflare.manage', 'Create, update and delete allowlisted Cloudflare resources.', 'critical'),
  ('github.manage', 'Dispatch, cancel and rerun workflows in allowlisted repositories.', 'dangerous'),
  ('proxmox.manage', 'Control virtual machines and containers in configured Proxmox clusters.', 'critical')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug = 'god-admin' AND p.slug IN ('docker.manage','kubernetes.manage','cloudflare.manage','github.manage','proxmox.manage')
ON CONFLICT DO NOTHING;
