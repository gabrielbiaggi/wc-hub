DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('worker.read', 'worker.manage', 'docker.clone'));
DELETE FROM permissions WHERE slug IN ('worker.read', 'worker.manage', 'docker.clone');
DROP TABLE IF EXISTS stack_clones;
DROP TABLE IF EXISTS worker_nodes;
