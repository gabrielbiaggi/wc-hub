CREATE TABLE worker_nodes (
  id text PRIMARY KEY,
  name text NOT NULL,
  hardware_fingerprint text NOT NULL UNIQUE,
  public_key text NOT NULL,
  ip_address text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'pending_approval' CHECK (status IN ('pending_approval', 'approved', 'rejected')),
  approved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE stack_clones (
  id text PRIMARY KEY,
  source_stack text NOT NULL,
  target_stack text NOT NULL,
  suffix text NOT NULL,
  port_mappings text NOT NULL DEFAULT '{}',
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO permissions(slug, description, risk) VALUES
  ('worker.read', 'Consultar estado e requisições de onboarding de workers remotos.', 'safe'),
  ('worker.manage', 'Aprovar ou rejeitar requisições de bootstrap de workers remotos.', 'critical'),
  ('docker.clone', 'Duplicar e clonar stacks Docker/K3s para ambientes isolados.', 'dangerous')
ON CONFLICT(slug) DO NOTHING;

INSERT INTO role_permissions(role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='god-admin' AND p.slug IN ('worker.read', 'worker.manage', 'docker.clone')
ON CONFLICT DO NOTHING;
