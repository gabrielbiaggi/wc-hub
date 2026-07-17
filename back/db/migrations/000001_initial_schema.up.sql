CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE integration_status AS ENUM ('disabled', 'pending', 'connected', 'degraded', 'error');
CREATE TYPE resource_status AS ENUM ('unknown', 'offline', 'online', 'degraded', 'provisioning', 'stopped');
CREATE TYPE action_scope AS ENUM ('local', 'remote', 'cloud');
CREATE TYPE risk_level AS ENUM ('safe', 'dangerous', 'critical');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  totp_secret_ciphertext bytea,
  totp_enabled boolean NOT NULL DEFAULT false,
  disabled_at timestamptz,
  last_login_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  description text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  description text NOT NULL,
  risk risk_level NOT NULL DEFAULT 'safe'
);

CREATE TABLE user_roles (user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE, PRIMARY KEY (user_id, role_id));
CREATE TABLE role_permissions (role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE, permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE CASCADE, PRIMARY KEY (role_id, permission_id));

CREATE TABLE integrations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  provider text NOT NULL,
  status integration_status NOT NULL DEFAULT 'pending',
  config jsonb NOT NULL DEFAULT '{}',
  credentials_ciphertext bytea,
  last_checked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, name)
);

CREATE TABLE hosts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id uuid REFERENCES integrations(id) ON DELETE SET NULL,
  name text NOT NULL UNIQUE,
  hostname text NOT NULL,
  scope action_scope NOT NULL,
  status resource_status NOT NULL DEFAULT 'unknown',
  self_protected boolean NOT NULL DEFAULT false,
  labels jsonb NOT NULL DEFAULT '{}',
  facts jsonb NOT NULL DEFAULT '{}',
  last_seen_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT one_self_protected_local CHECK (NOT self_protected OR scope = 'local')
);

CREATE UNIQUE INDEX only_one_self_protected_host ON hosts (self_protected) WHERE self_protected;

CREATE TABLE clusters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id uuid REFERENCES integrations(id) ON DELETE SET NULL,
  name text NOT NULL,
  kind text NOT NULL,
  status resource_status NOT NULL DEFAULT 'unknown',
  version text,
  metadata jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_id uuid REFERENCES clusters(id) ON DELETE CASCADE,
  host_id uuid REFERENCES hosts(id) ON DELETE SET NULL,
  external_id text,
  name text NOT NULL,
  status resource_status NOT NULL DEFAULT 'unknown',
  cpu_cores integer,
  memory_bytes bigint,
  metadata jsonb NOT NULL DEFAULT '{}',
  last_seen_at timestamptz,
  UNIQUE (cluster_id, name)
);

CREATE TABLE virtual_machines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  external_id text NOT NULL,
  name text NOT NULL,
  status resource_status NOT NULL DEFAULT 'unknown',
  cpu_cores integer,
  memory_bytes bigint,
  disk_bytes bigint,
  addresses jsonb NOT NULL DEFAULT '[]',
  metadata jsonb NOT NULL DEFAULT '{}',
  UNIQUE (node_id, external_id)
);

CREATE TABLE containers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid REFERENCES hosts(id) ON DELETE SET NULL,
  cluster_id uuid REFERENCES clusters(id) ON DELETE SET NULL,
  external_id text NOT NULL,
  name text NOT NULL,
  runtime text NOT NULL,
  image text,
  status resource_status NOT NULL DEFAULT 'unknown',
  metadata jsonb NOT NULL DEFAULT '{}',
  UNIQUE (runtime, external_id)
);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  github_owner text,
  github_repo text,
  default_branch text NOT NULL DEFAULT 'main',
  environments jsonb NOT NULL DEFAULT '[]',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE terminal_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  host_id uuid NOT NULL REFERENCES hosts(id),
  status text NOT NULL DEFAULT 'requested',
  remote_address inet,
  recording_path text,
  started_at timestamptz,
  ended_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE terraform_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
  user_id uuid NOT NULL REFERENCES users(id),
  workspace text NOT NULL,
  action text NOT NULL,
  status text NOT NULL DEFAULT 'queued',
  plan_digest text,
  output_redacted text,
  approved_by uuid REFERENCES users(id),
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE storage_mounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid REFERENCES hosts(id) ON DELETE SET NULL,
  name text NOT NULL UNIQUE,
  root_path text NOT NULL,
  driver text NOT NULL DEFAULT 'mergerfs',
  read_only boolean NOT NULL DEFAULT true,
  status resource_status NOT NULL DEFAULT 'unknown',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE file_index (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  mount_id uuid NOT NULL REFERENCES storage_mounts(id) ON DELETE CASCADE,
  path text NOT NULL,
  name text NOT NULL,
  mime_type text,
  size_bytes bigint NOT NULL DEFAULT 0,
  is_directory boolean NOT NULL DEFAULT false,
  checksum text,
  modified_at timestamptz,
  indexed_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (mount_id, path)
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  action text NOT NULL,
  scope action_scope NOT NULL,
  resource_type text NOT NULL,
  resource_id text,
  target_name text,
  risk risk_level NOT NULL,
  decision text NOT NULL,
  reason text,
  request_id text,
  source_ip inet,
  payload_redacted jsonb NOT NULL DEFAULT '{}',
  previous_hash text,
  event_hash text NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_occurred_at_idx ON audit_logs (occurred_at DESC);
CREATE INDEX audit_logs_resource_idx ON audit_logs (resource_type, resource_id);

CREATE TABLE metrics_snapshots (
  captured_at timestamptz NOT NULL,
  resource_type text NOT NULL,
  resource_id uuid NOT NULL,
  metric text NOT NULL,
  value double precision NOT NULL,
  unit text,
  labels jsonb NOT NULL DEFAULT '{}',
  PRIMARY KEY (captured_at, resource_type, resource_id, metric)
);
CREATE INDEX metrics_snapshots_lookup_idx ON metrics_snapshots (resource_id, metric, captured_at DESC);

CREATE TABLE alerts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_type text NOT NULL,
  resource_id uuid,
  severity text NOT NULL,
  title text NOT NULL,
  description text,
  status text NOT NULL DEFAULT 'open',
  fingerprint text NOT NULL,
  acknowledged_by uuid REFERENCES users(id),
  acknowledged_at timestamptz,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (fingerprint, status)
);

