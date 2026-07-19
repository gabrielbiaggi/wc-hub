DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE slug IN ('vnc.read','vnc.connect','backup.read','monitor.read','monitor.manage','power.read','power.manage'));
DELETE FROM permissions WHERE slug IN ('vnc.read','vnc.connect','backup.read','monitor.read','monitor.manage','power.read','power.manage');
DROP TABLE IF EXISTS monitor_settings;
DROP TABLE IF EXISTS monitor_targets;
