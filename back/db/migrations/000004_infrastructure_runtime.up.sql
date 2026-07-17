ALTER TABLE clusters ADD CONSTRAINT clusters_integration_name_unique UNIQUE (integration_id, name);
ALTER TABLE hosts ADD COLUMN agent_status text NOT NULL DEFAULT 'unregistered';

INSERT INTO permissions (slug, description, risk) VALUES
  ('proxmox.read', 'Read Proxmox inventory and status.', 'safe'),
  ('proxmox.sync', 'Synchronize Proxmox inventory.', 'dangerous'),
  ('jobs.read', 'Read job queue and scheduler state.', 'safe'),
  ('jobs.manage', 'Enqueue and cancel operational jobs.', 'dangerous'),
  ('terminal.connect', 'Create audited SSH terminal tickets.', 'dangerous'),
  ('agents.manage', 'Provision and manage host agents.', 'critical');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='god-admin' AND p.slug IN ('proxmox.read','proxmox.sync','jobs.read','jobs.manage','terminal.connect','agents.manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='operator' AND p.slug IN ('proxmox.read','proxmox.sync','jobs.read','jobs.manage','terminal.connect');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug='auditor' AND p.slug IN ('proxmox.read','jobs.read');

CREATE TABLE jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
  priority smallint NOT NULL DEFAULT 100,
  attempts integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  run_after timestamptz NOT NULL DEFAULT now(),
  locked_at timestamptz,
  locked_by text,
  started_at timestamptz,
  finished_at timestamptz,
  last_error text,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_reserve_idx ON jobs (priority, run_after, created_at) WHERE status='queued';
CREATE INDEX jobs_recent_idx ON jobs (created_at DESC);

CREATE TABLE schedules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  job_kind text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  interval_seconds integer NOT NULL CHECK (interval_seconds >= 30),
  enabled boolean NOT NULL DEFAULT true,
  next_run_at timestamptz NOT NULL DEFAULT now(),
  last_run_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO schedules (name,job_kind,interval_seconds,enabled)
VALUES ('proxmox-inventory-sync','proxmox.sync',300,false),('telemetry-maintenance','telemetry.maintenance',60,true);

CREATE TABLE agent_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  scopes text[] NOT NULL DEFAULT ARRAY['metrics:write'],
  expires_at timestamptz,
  last_used_at timestamptz,
  revoked_at timestamptz,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE terminal_tickets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash bytea NOT NULL UNIQUE,
  session_id uuid NOT NULL REFERENCES terminal_sessions(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX terminal_tickets_expiry_idx ON terminal_tickets (expires_at) WHERE used_at IS NULL;

CREATE TABLE provider_sync_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  integration_id uuid REFERENCES integrations(id) ON DELETE SET NULL,
  job_id uuid REFERENCES jobs(id) ON DELETE SET NULL,
  status text NOT NULL,
  resources_seen integer NOT NULL DEFAULT 0,
  error text,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

CREATE TABLE infrastructure_storage_pools (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id uuid NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
  node_id uuid REFERENCES nodes(id) ON DELETE CASCADE,
  external_id text NOT NULL,
  kind text NOT NULL,
  status resource_status NOT NULL DEFAULT 'unknown',
  total_bytes bigint NOT NULL DEFAULT 0,
  used_bytes bigint NOT NULL DEFAULT 0,
  available_bytes bigint NOT NULL DEFAULT 0,
  shared boolean NOT NULL DEFAULT false,
  last_seen_at timestamptz NOT NULL,
  UNIQUE(integration_id,node_id,external_id)
);
