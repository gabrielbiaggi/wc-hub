INSERT INTO permissions (slug, description, risk) VALUES
  ('docker.read', 'Read Docker containers, images and runtime stats.', 'safe'),
  ('kubernetes.read', 'Read Kubernetes nodes, workloads and events.', 'safe'),
  ('cloudflare.read', 'Read allowlisted Cloudflare tunnels and DNS records.', 'safe'),
  ('github.read', 'Read allowlisted GitHub repositories, workflows and releases.', 'safe'),
  ('terraform.read', 'Read Terraform validate and plan results.', 'safe')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='god-admin' AND p.slug IN ('docker.read','kubernetes.read','cloudflare.read','github.read','terraform.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='operator' AND p.slug IN ('docker.read','kubernetes.read','cloudflare.read','github.read','terraform.read')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='auditor' AND p.slug IN ('docker.read','kubernetes.read','cloudflare.read','github.read','terraform.read')
ON CONFLICT DO NOTHING;
