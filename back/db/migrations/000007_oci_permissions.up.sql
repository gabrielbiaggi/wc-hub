INSERT INTO permissions (slug, description, risk) VALUES
  ('oci.read', 'Read Oracle Cloud regions, compartments, compute, and network inventory.', 'safe'),
  ('oci.manage', 'Start, stop, reboot, and reset Oracle Cloud compute instances.', 'critical')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.slug = 'god-admin' AND p.slug IN ('oci.read', 'oci.manage')
ON CONFLICT DO NOTHING;
