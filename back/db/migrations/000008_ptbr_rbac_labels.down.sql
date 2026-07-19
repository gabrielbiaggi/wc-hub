UPDATE roles SET
  name = CASE slug
    WHEN 'god-admin' THEN 'God administrator'
    WHEN 'operator' THEN 'Operator'
    WHEN 'auditor' THEN 'Auditor'
    ELSE name
  END,
  description = CASE slug
    WHEN 'god-admin' THEN 'Full platform operator; critical actions still require policy approval.'
    WHEN 'operator' THEN 'Manages non-critical infrastructure operations.'
    WHEN 'auditor' THEN 'Read-only access to telemetry and audit trails.'
    ELSE description
  END
WHERE slug IN ('god-admin', 'operator', 'auditor');

UPDATE permissions SET description = CASE slug
  WHEN 'overview.read' THEN 'Read global operational overview.'
  WHEN 'telemetry.read' THEN 'Read infrastructure telemetry.'
  WHEN 'hosts.execute.safe' THEN 'Run allowlisted commands on managed hosts.'
  WHEN 'hosts.execute.critical' THEN 'Request critical commands on remote hosts.'
  WHEN 'terraform.plan' THEN 'Create Terraform plans.'
  WHEN 'terraform.apply' THEN 'Apply approved Terraform plans.'
  WHEN 'terminal.open' THEN 'Open audited remote terminal sessions.'
  WHEN 'storage.read' THEN 'Browse configured storage mounts.'
  WHEN 'storage.write' THEN 'Modify files in writable mounts.'
  WHEN 'audit.read' THEN 'Read audit logs.'
  WHEN 'proxmox.read' THEN 'Read Proxmox inventory and status.'
  WHEN 'proxmox.sync' THEN 'Synchronize Proxmox inventory.'
  WHEN 'jobs.read' THEN 'Read job queue and scheduler state.'
  WHEN 'jobs.manage' THEN 'Enqueue and cancel operational jobs.'
  WHEN 'terminal.connect' THEN 'Create audited SSH terminal tickets.'
  WHEN 'agents.manage' THEN 'Provision and manage host agents.'
  WHEN 'docker.read' THEN 'Read Docker containers, images and runtime stats.'
  WHEN 'kubernetes.read' THEN 'Read Kubernetes nodes, workloads and events.'
  WHEN 'cloudflare.read' THEN 'Read allowlisted Cloudflare tunnels and DNS records.'
  WHEN 'github.read' THEN 'Read allowlisted GitHub repositories, workflows and releases.'
  WHEN 'terraform.read' THEN 'Read Terraform validate and plan results.'
  WHEN 'docker.manage' THEN 'Start, stop and restart Docker containers.'
  WHEN 'kubernetes.manage' THEN 'Scale and restart Kubernetes workloads.'
  WHEN 'cloudflare.manage' THEN 'Create, update and delete allowlisted Cloudflare resources.'
  WHEN 'github.manage' THEN 'Dispatch, cancel and rerun workflows in allowlisted repositories.'
  WHEN 'proxmox.manage' THEN 'Control virtual machines and containers in configured Proxmox clusters.'
  WHEN 'oci.read' THEN 'Read Oracle Cloud regions, compartments, compute, and network inventory.'
  WHEN 'oci.manage' THEN 'Start, stop, reboot, and reset Oracle Cloud compute instances.'
  ELSE description
END
WHERE slug IN (
  'overview.read','telemetry.read','hosts.execute.safe','hosts.execute.critical','terraform.plan',
  'terraform.apply','terminal.open','storage.read','storage.write','audit.read','proxmox.read',
  'proxmox.sync','jobs.read','jobs.manage','terminal.connect','agents.manage','docker.read',
  'kubernetes.read','cloudflare.read','github.read','terraform.read','docker.manage',
  'kubernetes.manage','cloudflare.manage','github.manage','proxmox.manage','oci.read','oci.manage'
);
