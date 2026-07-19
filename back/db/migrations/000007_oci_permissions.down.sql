DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('oci.read', 'oci.manage'));
DELETE FROM permissions WHERE slug IN ('oci.read', 'oci.manage');
