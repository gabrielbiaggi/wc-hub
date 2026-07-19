CREATE TABLE monitor_targets (
  id text PRIMARY KEY,
  name text NOT NULL,
  target text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('http','tcp')),
  interval_seconds integer NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 15 AND 3600),
  enabled boolean NOT NULL DEFAULT true,
  last_status text NOT NULL DEFAULT 'unknown' CHECK (last_status IN ('unknown','up','down')),
  last_latency_ms integer,
  last_checked_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE monitor_settings (singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton), webhook_url text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now());
INSERT INTO monitor_settings(singleton) VALUES(true) ON CONFLICT DO NOTHING;
INSERT INTO permissions(slug,description,risk) VALUES
 ('vnc.read','Consultar alvos autorizados para desktop remoto VNC.','safe'),('vnc.connect','Abrir console gráfico VNC por gateway auditado.','critical'),('backup.read','Consultar saúde e inventário do Proxmox Backup Server.','safe'),('monitor.read','Consultar disponibilidade e latência de serviços.','safe'),('monitor.manage','Gerenciar alvos e webhook de monitoramento.','dangerous'),('power.read','Consultar o estado do no-break.','safe'),('power.manage','Enviar Wake-on-LAN para máquinas autorizadas.','critical') ON CONFLICT(slug) DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id) SELECT r.id,p.id FROM roles r CROSS JOIN permissions p WHERE r.slug='god-admin' AND p.slug IN ('vnc.read','vnc.connect','backup.read','monitor.read','monitor.manage','power.read','power.manage') ON CONFLICT DO NOTHING;
UPDATE permissions SET description=CASE slug WHEN 'vnc.read' THEN 'Consultar alvos autorizados para desktop remoto VNC.' WHEN 'vnc.connect' THEN 'Abrir console gráfico VNC por gateway auditado.' WHEN 'backup.read' THEN 'Consultar saúde e inventário do Proxmox Backup Server.' WHEN 'monitor.read' THEN 'Consultar disponibilidade e latência de serviços.' WHEN 'monitor.manage' THEN 'Gerenciar alvos e webhook de monitoramento.' WHEN 'power.read' THEN 'Consultar o estado do no-break.' WHEN 'power.manage' THEN 'Enviar Wake-on-LAN para máquinas autorizadas.' ELSE description END WHERE slug IN ('vnc.read','vnc.connect','backup.read','monitor.read','monitor.manage','power.read','power.manage');
