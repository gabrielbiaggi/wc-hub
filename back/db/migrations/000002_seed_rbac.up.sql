INSERT INTO roles (slug, name, description) VALUES
  ('god-admin', 'God administrator', 'Full platform operator; critical actions still require policy approval.'),
  ('operator', 'Operator', 'Manages non-critical infrastructure operations.'),
  ('auditor', 'Auditor', 'Read-only access to telemetry and audit trails.');

INSERT INTO permissions (slug, description, risk) VALUES
  ('overview.read', 'Read global operational overview.', 'safe'),
  ('telemetry.read', 'Read infrastructure telemetry.', 'safe'),
  ('hosts.execute.safe', 'Run allowlisted commands on managed hosts.', 'dangerous'),
  ('hosts.execute.critical', 'Request critical commands on remote hosts.', 'critical'),
  ('terraform.plan', 'Create Terraform plans.', 'dangerous'),
  ('terraform.apply', 'Apply approved Terraform plans.', 'critical'),
  ('terminal.open', 'Open audited remote terminal sessions.', 'dangerous'),
  ('storage.read', 'Browse configured storage mounts.', 'safe'),
  ('storage.write', 'Modify files in writable mounts.', 'dangerous'),
  ('audit.read', 'Read audit logs.', 'safe');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.slug = 'god-admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.slug IN ('overview.read', 'telemetry.read', 'hosts.execute.safe', 'terraform.plan', 'terminal.open', 'storage.read') WHERE r.slug = 'operator';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.slug IN ('overview.read', 'telemetry.read', 'storage.read', 'audit.read') WHERE r.slug = 'auditor';

-- The bootstrap administrator is deliberately not seeded with a default password.
-- Create it through the one-time bootstrap command once authentication is enabled.

