DROP TABLE IF EXISTS infrastructure_storage_pools, provider_sync_runs, terminal_tickets, agent_tokens, schedules, jobs CASCADE;
DELETE FROM permissions WHERE slug IN ('proxmox.read','proxmox.sync','jobs.read','jobs.manage','terminal.connect','agents.manage');
ALTER TABLE hosts DROP COLUMN IF EXISTS agent_status;
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_integration_name_unique;
