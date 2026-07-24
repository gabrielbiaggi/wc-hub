import axios from "axios";

let csrfToken = sessionStorage.getItem("wc_hub_csrf") ?? "";
export const setCSRFToken = (token: string) => {
  csrfToken = token;
  token
    ? sessionStorage.setItem("wc_hub_csrf", token)
    : sessionStorage.removeItem("wc_hub_csrf");
};
export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || "/api",
  timeout: 30_000,
  withCredentials: true,
  headers: { Accept: "application/json" },
});
api.interceptors.request.use((config) => {
  if (
    csrfToken &&
    config.method &&
    !["get", "head", "options"].includes(config.method)
  )
    config.headers.set("X-CSRF-Token", csrfToken);
  return config;
});

export interface User {
  id: string;
  email: string;
  display_name: string;
  totp_enabled: boolean;
  roles: string[];
  permissions: string[];
}
export interface AuthResponse {
  user: User;
  csrf_token: string;
  expires_at: string;
}
export interface Metric {
  label: string;
  value: number;
  unit: string;
  delta: number;
  status: string;
}
export interface Activity {
  id: string;
  source: string;
  message: string;
  severity: string;
  at: string;
}
export interface Overview {
  generated_at: string;
  environment: string;
  self_protected: boolean;
  metrics: Metric[];
  activity: Activity[];
  series: number[];
}
export interface Integration {
  id: string;
  name: string;
  provider: string;
  status: string;
  config: Record<string, unknown>;
  last_checked_at?: string;
  created_at: string;
}
export interface Host {
  id: string;
  integration_id?: string;
  name: string;
  hostname: string;
  scope: "local" | "remote" | "cloud";
  status: string;
  self_protected: boolean;
  labels: Record<string, unknown>;
  facts: Record<string, unknown>;
  last_seen_at?: string;
  created_at: string;
}
export interface AuditEntry {
  id: string;
  actor_email?: string;
  action: string;
  scope: string;
  resource_type: string;
  resource_id?: string;
  target_name?: string;
  risk: string;
  decision: string;
  reason?: string;
  request_id?: string;
  occurred_at: string;
  event_hash: string;
}
export interface AdminUser {
  id: string;
  email: string;
  display_name: string;
  totp_enabled: boolean;
  disabled_at?: string;
  last_login_at?: string;
  created_at: string;
  roles: string[];
}
export interface Role {
  id: string;
  slug: string;
  name: string;
  description: string;
  permissions: string[];
  user_count: number;
}
export interface Permission {
  id: string;
  slug: string;
  description: string;
  risk: "safe" | "dangerous" | "critical";
}
export interface Alert {
  id: string;
  resource_type: string;
  resource_id?: string;
  severity: string;
  title: string;
  description: string;
  status: "open" | "acknowledged" | "resolved";
  acknowledged_at?: string;
  resolved_at?: string;
  created_at: string;
}
export interface ProxmoxSummary {
  configured: boolean;
  status: string;
  last_checked_at?: string;
  nodes: number;
  virtual_machines: number;
  containers: number;
  storage_pools: number;
}
export interface ProxmoxNode {
  cluster: string;
  node: string;
  status: string;
  cpu: number;
  maxcpu: number;
  mem: number;
  maxmem: number;
  uptime: number;
  level: string;
}
export interface ProxmoxGuest {
  cluster: string;
  vmid: number;
  name: string;
  status: string;
  cpu: number;
  cpus: number;
  mem: number;
  maxmem: number;
  maxdisk: number;
  uptime: number;
  node: string;
  type: "qemu" | "lxc";
  template?: number;
}
export interface ProxmoxStorage {
  cluster: string;
  storage: string;
  type: string;
  status: string;
  active: number;
  total: number;
  used: number;
  avail: number;
  shared: number;
  node: string;
}
export interface ProxmoxInventory {
  captured_at: string;
  nodes: ProxmoxNode[];
  virtual_machines: ProxmoxGuest[];
  containers: ProxmoxGuest[];
  storage: ProxmoxStorage[];
  warnings?: string[];
}
export interface Job {
  id: string;
  kind: string;
  payload: Record<string, unknown>;
  status: string;
  priority: number;
  attempts: number;
  max_attempts: number;
  run_after: string;
  locked_by?: string;
  last_error?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}
export interface HostMetric {
  host_id: string;
  host_name: string;
  metric: string;
  value: number;
  unit: string;
  captured_at: string;
}

export const getOverview = async () =>
  (await api.get<Overview>("/v1/overview")).data;
export const getIntegrations = async () =>
  (await api.get<{ items: Integration[] }>("/v1/integrations")).data.items;
export const createIntegration = async (
  body: Pick<Integration, "name" | "provider"> & {
    config?: Record<string, unknown>;
  },
) => (await api.post<Integration>("/v1/integrations", body)).data;
export const getHosts = async () =>
  (await api.get<{ items: Host[] }>("/v1/hosts")).data.items;
export const createHost = async (
  body: Pick<Host, "name" | "hostname" | "scope" | "self_protected"> &
    Partial<Host>,
) => (await api.post<Host>("/v1/hosts", body)).data;
export const getAudit = async () =>
  (await api.get<{ items: AuditEntry[] }>("/v1/audit")).data.items;
export const getAdminUsers = async () =>
  (await api.get<{ items: AdminUser[] }>("/v1/admin/users")).data.items;
export const createAdminUser = async (body: {
  email: string;
  display_name: string;
  password: string;
  role_ids: string[];
}) => (await api.post<AdminUser>("/v1/admin/users", body)).data;
export const updateAdminUser = async (
  id: string,
  body: {
    email: string;
    display_name: string;
    disabled: boolean;
    role_ids: string[];
  },
) => (await api.patch<AdminUser>(`/v1/admin/users/${id}`, body)).data;
export const disableAdminUser = async (id: string) =>
  api.delete(`/v1/admin/users/${id}`);
export const getRoles = async () =>
  (await api.get<{ items: Role[] }>("/v1/admin/roles")).data.items;
export const createRole = async (body: {
  slug: string;
  name: string;
  description: string;
  permission_ids: string[];
}) => (await api.post<Role>("/v1/admin/roles", body)).data;
export const updateRole = async (
  id: string,
  body: { name: string; description: string; permission_ids: string[] },
) => (await api.patch<Role>(`/v1/admin/roles/${id}`, body)).data;
export const deleteRole = async (id: string) =>
  api.delete(`/v1/admin/roles/${id}`);
export const getPermissions = async () =>
  (await api.get<{ items: Permission[] }>("/v1/admin/permissions")).data.items;
export const getAlerts = async () =>
  (await api.get<{ items: Alert[] }>("/v1/alerts")).data.items;
export const updateAlert = async (id: string, status: Alert["status"]) =>
  (await api.patch<Alert>(`/v1/alerts/${id}`, { status })).data;
export const enrollTOTP = async () =>
  (
    await api.post<{ secret: string; otpauth_uri: string }>(
      "/v1/auth/totp/enroll",
      {},
    )
  ).data;
export const confirmTOTP = async (code: string) =>
  (await api.post<{ totp_enabled: boolean }>("/v1/auth/totp/confirm", { code }))
    .data;
export const getProxmoxSummary = async () =>
  (await api.get<ProxmoxSummary>("/v1/proxmox/summary")).data;
export const syncProxmox = async () =>
  (await api.post<Job>("/v1/proxmox/sync", {})).data;
export const getProxmoxInventory = async () =>
  (await api.get<ProxmoxInventory>("/v1/proxmox/inventory")).data;
export function buildActionHeaders(
  confirmation?: string,
  totpCode?: string,
): Record<string, string> {
  const headers: Record<string, string> = {};
  if (confirmation) headers["X-Confirmation"] = confirmation;
  if (totpCode) headers["X-TOTP-Code"] = totpCode;
  return headers;
}

export const runProxmoxPowerAction = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  action: "start" | "stop" | "shutdown" | "reboot" | "reset",
  extraHeaders?: Record<string, string>,
) =>
  (
    await api.post<{ status: string }>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/${action}`,
      {},
      { params: { cluster }, headers: extraHeaders },
    )
  ).data;
export interface ProxmoxQEMUInput {
  cluster: string;
  node: string;
  vmid: number;
  name: string;
  cores: number;
  memory_mb: number;
  storage: string;
  disk_gb: number;
  iso: string;
  bridge: string;
  start: boolean;
}
export interface ProxmoxLXCInput {
  cluster: string;
  node: string;
  vmid: number;
  hostname: string;
  cores: number;
  memory_mb: number;
  storage: string;
  rootfs_gb: number;
  template: string;
  bridge: string;
  password: string;
  ssh_public_keys: string;
  unprivileged: boolean;
  start: boolean;
}
export interface ProxmoxCloneInput {
  cluster: string;
  node: string;
  kind: "qemu" | "lxc";
  source_vmid: number;
  new_vmid: number;
  name: string;
  target_node: string;
  storage: string;
  full: boolean;
}
export const createProxmoxQEMU = async (input: ProxmoxQEMUInput) =>
  (await api.post("/v1/proxmox/qemu", input)).data;
export const createProxmoxLXC = async (input: ProxmoxLXCInput) =>
  (await api.post("/v1/proxmox/lxc", input)).data;
export const cloneProxmoxGuest = async (input: ProxmoxCloneInput) =>
  (await api.post("/v1/proxmox/clone", input)).data;
export const deleteProxmoxGuest = async (
  guest: ProxmoxGuest,
  purge = true,
  extraHeaders?: Record<string, string>,
) =>
  api.delete(
    `/v1/proxmox/nodes/${encodeURIComponent(guest.node)}/${guest.type}/${guest.vmid}`,
    { params: { cluster: guest.cluster, purge }, headers: extraHeaders },
  );
export interface ProxmoxSnapshot {
  name: string;
  description: string;
  snaptime: number;
  parent?: string;
  vmstate?: number;
}
export const getProxmoxSnapshots = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
) =>
  (
    await api.get<{ items: ProxmoxSnapshot[] }>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/snapshots`,
      { params: { cluster } },
    )
  ).data.items;
export const createProxmoxSnapshot = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  name: string,
  description: string,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/snapshots`,
      { name, description },
      { params: { cluster } },
    )
  ).data;
export const rollbackProxmoxSnapshot = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  name: string,
  headers?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/snapshots/${encodeURIComponent(name)}/rollback`,
      {},
      { params: { cluster }, headers },
    )
  ).data;
export const deleteProxmoxSnapshot = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  name: string,
  headers?: Record<string, string>,
) =>
  api.delete(
    `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/snapshots/${encodeURIComponent(name)}`,
    { params: { cluster }, headers },
  );
export const migrateProxmoxGuest = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  targetNode: string,
  online: boolean,
  headers?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/migrate`,
      { target_node: targetNode, online },
      { params: { cluster }, headers },
    )
  ).data;
export const resizeProxmoxDisk = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  disk: string,
  size: string,
  headers?: Record<string, string>,
) =>
  (
    await api.put(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/resize`,
      { disk, size },
      { params: { cluster }, headers },
    )
  ).data;
export const updateProxmoxConfig = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  config: Record<string, string>,
  headers?: Record<string, string>,
) =>
  (
    await api.put(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/config`,
      { config },
      { params: { cluster }, headers },
    )
  ).data;
export interface ProxmoxNetworkInterface {
  id: string;
  name: string;
  type: string;
  active: boolean;
  autostart: boolean;
  bridge?: string;
  address?: string;
  netmask?: string;
  gateway?: string;
}
export const getProxmoxNodeNetwork = async (
  cluster: string,
  node: string,
): Promise<ProxmoxNetworkInterface[]> =>
  (
    await api.get(`/v1/proxmox/nodes/${encodeURIComponent(node)}/network`, {
      params: { cluster },
    })
  ).data;
export interface ProxmoxFirewallRule {
  pos: number;
  type: string;
  action: string;
  enable: number;
  proto?: string;
  source?: string;
  dest?: string;
  dport?: string;
  sport?: string;
  comment?: string;
  iface?: string;
}
export const getProxmoxGuestFirewallRules = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
): Promise<ProxmoxFirewallRule[]> =>
  (
    await api.get(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/firewall/rules`,
      { params: { cluster } },
    )
  ).data;
export const createProxmoxFirewallRule = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  rule: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/firewall/rules`,
      { rule },
      { params: { cluster } },
    )
  ).data;
export const deleteProxmoxFirewallRule = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  pos: number,
) =>
  (
    await api.delete(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/firewall/rules/${pos}`,
      { params: { cluster } },
    )
  ).data;
export interface ProxmoxBackupInfo {
  volid: string;
  content: string;
  size: number;
  format: string;
  ctime: number;
  vmid: number;
  notes?: string;
}
export const getProxmoxNodeBackups = async (
  cluster: string,
  node: string,
  storage: string,
): Promise<ProxmoxBackupInfo[]> =>
  (
    await api.get(`/v1/proxmox/nodes/${encodeURIComponent(node)}/backups`, {
      params: { cluster, storage },
    })
  ).data;
export const createProxmoxBackup = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
  storage: string,
  mode: string,
  compress: string,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/backup`,
      { storage, mode, compress },
      { params: { cluster } },
    )
  ).data;

export interface ProxmoxStorageContentItem {
  volid: string;
  content: string;
  size: number;
  format?: string;
  ctime?: number;
  notes?: string;
}

export const getProxmoxStorageContent = async (
  cluster: string,
  node: string,
  storage: string,
): Promise<ProxmoxStorageContentItem[]> =>
  (
    await api.get<{ items: ProxmoxStorageContentItem[] }>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/storage/${encodeURIComponent(storage)}/content`,
      { params: { cluster } },
    )
  ).data.items;

export const deleteProxmoxStorageContent = async (
  cluster: string,
  node: string,
  storage: string,
  volid: string,
) =>
  (
    await api.delete(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/storage/${encodeURIComponent(storage)}/content`,
      { params: { cluster, volid } },
    )
  ).data;

export interface ProxmoxTaskStatus {
  upid: string;
  node: string;
  status: string;
  exitstatus?: string;
  pid?: number;
  starttime?: number;
}

export const getProxmoxTaskStatus = async (
  cluster: string,
  node: string,
  upid: string,
): Promise<ProxmoxTaskStatus> =>
  (
    await api.get<ProxmoxTaskStatus>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/tasks/${encodeURIComponent(upid)}/status`,
      { params: { cluster } },
    )
  ).data;

export const getProxmoxTaskLog = async (
  cluster: string,
  node: string,
  upid: string,
): Promise<string[]> =>
  (
    await api.get<{ lines: string[] }>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/tasks/${encodeURIComponent(upid)}/log`,
      { params: { cluster } },
    )
  ).data.lines;

export const convertToProxmoxTemplate = async (
  cluster: string,
  node: string,
  vmid: number,
) =>
  (
    await api.post(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/qemu/${vmid}/template`,
      {},
      { params: { cluster } },
    )
  ).data;

export interface ProxmoxAgentInterface {
  name: string;
  "hardware-address"?: string;
  "ip-addresses"?: {
    "ip-address-type": string;
    "ip-address": string;
    prefix: number;
  }[];
}

export const getProxmoxGuestAgentInterfaces = async (
  cluster: string,
  node: string,
  vmid: number,
): Promise<ProxmoxAgentInterface[]> =>
  (
    await api.get<{ interfaces: ProxmoxAgentInterface[] }>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/qemu/${vmid}/agent/interfaces`,
      { params: { cluster } },
    )
  ).data.interfaces;

export interface ProxmoxVNCTicket {
  ticket: string;
  port: string;
  upid: string;
  cert?: string;
  user?: string;
}

export const createProxmoxVNCProxyTicket = async (
  cluster: string,
  node: string,
  kind: "qemu" | "lxc",
  vmid: number,
): Promise<ProxmoxVNCTicket> =>
  (
    await api.post<ProxmoxVNCTicket>(
      `/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/vncproxy`,
      {},
      { params: { cluster } },
    )
  ).data;

export interface DockerHealth {
  reachable: boolean;
  source?: string;
  version?: string;
  api_version?: string;
  os_type?: string;
  arch?: string;
}
export interface DockerPort {
  ip?: string;
  private_port: number;
  public_port?: number;
  type: string;
}
export interface DockerContainer {
  id: string;
  names: string[];
  image: string;
  image_id: string;
  command: string;
  created: number;
  state: string;
  status: string;
  ports: DockerPort[];
  labels: Record<string, string>;
}
export interface DockerImage {
  id: string;
  repo_tags: string[];
  repo_digests: string[];
  created: number;
  size: number;
  shared_size: number;
  containers: number;
}
export interface DockerContainerStats {
  container_id: string;
  name: string;
  read_at: string;
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
  memory_percent: number;
  network_rx: number;
  network_tx: number;
  block_read: number;
  block_write: number;
}
export interface DockerInventory {
  captured_at: string;
  source?: string;
  health: DockerHealth;
  containers: DockerContainer[];
  images: DockerImage[];
  stats: DockerContainerStats[];
  warnings: string[];
}
export const getDockerInventory = async (): Promise<DockerInventory> =>
  (await api.get("/v1/docker/inventory")).data;
export const dockerContainerAction = async (
  id: string,
  action: "start" | "stop" | "restart" | "kill" | "remove",
  extraHeaders?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/docker/containers/${id}/${action}`,
      {},
      { headers: extraHeaders },
    )
  ).data;
export const dockerContainerExec = async (
  id: string,
  command: string[],
  extraHeaders?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/docker/containers/${id}/exec`,
      { command },
      { headers: extraHeaders },
    )
  ).data;
export interface KubernetesMetadata {
  name: string;
  namespace: string;
  uid: string;
  created_at: string;
  labels: Record<string, string>;
}
export interface KubernetesNode {
  metadata: KubernetesMetadata;
  status: string;
  roles: string[];
  capacity: { cpu: string; memory: string; pods: string };
  allocatable: { cpu: string; memory: string; pods: string };
  kubelet_version: string;
  os_image: string;
}
export interface KubernetesDeployment {
  metadata: KubernetesMetadata;
  replicas: number;
  ready_replicas: number;
  available_replicas: number;
  updated_replicas: number;
}
export interface KubernetesPod {
  metadata: KubernetesMetadata;
  status: string;
  phase: string;
  node_name: string;
  pod_ip: string;
  containers: Array<{
    name: string;
    image: string;
    ready: boolean;
    restart_count: number;
    state: string;
  }>;
  conditions: Array<{ type: string; status: string; reason: string }>;
}
export interface KubernetesEvent {
  metadata: KubernetesMetadata;
  involved_object: { kind: string; name: string; namespace: string };
  reason: string;
  message: string;
  type: string;
  count: number;
  first_timestamp: string;
  last_timestamp: string;
}
export interface KubernetesOverview {
  captured_at: string;
  source: string;
  nodes: KubernetesNode[];
  deployments: KubernetesDeployment[];
  pods: KubernetesPod[];
  events: KubernetesEvent[];
  warnings: string[];
}
export const getKubernetesOverview = async (): Promise<KubernetesOverview> =>
  (await api.get("/v1/kubernetes/overview")).data;
export const getKubernetesPodLogs = async (
  namespace: string,
  pod: string,
  container?: string,
  tail?: number,
) =>
  (
    await api.get(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/logs`,
      { params: { container, tail } },
    )
  ).data;
export const kubernetesPodExec = async (
  namespace: string,
  pod: string,
  container: string,
  command: string[],
  extraHeaders?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/exec`,
      { container, command },
      { headers: extraHeaders },
    )
  ).data;
export const kubernetesDeploymentAction = async (
  namespace: string,
  name: string,
  action: "scale" | "restart" | "delete",
  replicas?: number,
  extraHeaders?: Record<string, string>,
) =>
  (
    await api.post(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/deployments/${encodeURIComponent(name)}/${action}`,
      { replicas },
      { headers: extraHeaders },
    )
  ).data;
export interface GitHubCommit {
  sha: string;
  author: string;
  author_email: string;
  message: string;
  timestamp: string;
  url: string;
}
export interface GitHubWorkflow {
  id: number;
  name: string;
  path: string;
  state: string;
  created_at: string;
  updated_at: string;
}
export interface GitHubWorkflowRun {
  id: number;
  name: string;
  display_title: string;
  status: string;
  conclusion: string;
  workflow_id: number;
  event: string;
  head_branch: string;
  head_sha: string;
  run_number: number;
  run_attempt: number;
  created_at: string;
  updated_at: string;
  url: string;
}
export interface GitHubRelease {
  id: number;
  tag_name: string;
  name: string;
  draft: boolean;
  prerelease: boolean;
  created_at: string;
  published_at: string;
  body: string;
  url: string;
}
export interface GitHubProject {
  name: string;
  full_name: string;
  description: string;
  private: boolean;
  default_branch: string;
  language: string;
  stargazers_count: number;
  open_issues_count: number;
  commits: GitHubCommit[];
  workflows: GitHubWorkflow[];
  runs: GitHubWorkflowRun[];
  releases: GitHubRelease[];
  warnings: string[];
}
export interface GitHubOverview {
  captured_at: string;
  projects: GitHubProject[];
  warnings: string[];
}
export const getGitHubOverview = async (): Promise<GitHubOverview> =>
  (await api.get("/v1/github/overview")).data;
export const getGitHubCommits = async (repo: string): Promise<GitHubCommit[]> =>
  (await api.get("/v1/github/commits", { params: { repo } })).data;
export const getGitHubWorkflows = async (
  repo: string,
): Promise<GitHubWorkflow[]> =>
  (await api.get("/v1/github/workflows", { params: { repo } })).data;
export const githubWorkflowAction = async (
  repo: string,
  workflowId: number,
  action: "dispatch",
  ref: string,
  inputs?: Record<string, string>,
) =>
  (
    await api.post("/v1/github/workflow/action", {
      repo,
      workflow_id: workflowId,
      action,
      ref,
      inputs,
    })
  ).data;
export const githubRunAction = async (
  repo: string,
  runId: number,
  action: "cancel" | "rerun",
) =>
  (await api.post("/v1/github/run/action", { repo, run_id: runId, action }))
    .data;
export const getJobs = async () =>
  (await api.get<{ items: Job[] }>("/v1/jobs")).data.items;
export const enqueueJob = async (kind: string) =>
  (
    await api.post<Job>("/v1/jobs", {
      kind,
      payload: {},
      priority: 100,
      max_attempts: 5,
    })
  ).data;
export const getHostTelemetry = async () =>
  (await api.get<{ items: HostMetric[] }>("/v1/telemetry/hosts")).data.items;
export const createTerminalTicket = async (
  host_id: string,
  confirmation: string,
  totp_code: string,
) =>
  (
    await api.post<{ ticket: string; session_id: string; expires_in: number }>(
      "/v1/terminal/tickets",
      { host_id, confirmation, totp_code },
    )
  ).data;
export interface TerminalSession {
  id: string;
  user_email: string;
  host_name: string;
  status: string;
  started_at?: string;
  ended_at?: string;
  created_at: string;
}
export const getTerminalSessions = async () =>
  (await api.get<{ items: TerminalSession[] }>("/v1/terminal/sessions")).data
    .items;
export const provisionAgentToken = async (
  hostID: string,
  confirmation: string,
  totpCode: string,
) =>
  (
    await api.post<{ token: string }>(
      `/v1/agents/hosts/${encodeURIComponent(hostID)}/token`,
      { confirmation, totp_code: totpCode },
    )
  ).data;
export interface SecurityActionRequest {
  action: string;
  command?: string;
  scope: "local" | "remote" | "cloud";
  target_name: string;
  target_self_protected: boolean;
  confirmation?: string;
  totp_code?: string;
}
export interface SecurityDecision {
  allowed: boolean;
  risk: "safe" | "dangerous" | "critical";
  reason: string;
  requires_confirmation: boolean;
  requires_totp: boolean;
}
export const evaluateSecurityPolicy = async (input: SecurityActionRequest) =>
  (await api.post<SecurityDecision>("/v1/security/evaluate", input)).data;
export const getModules = async () =>
  (await api.get<string[]>("/v1/modules")).data;

export * from "./api_docker";
export * from "./api_kubernetes";
export * from "./api_cloudflare";
export * from "./api_github";
export * from "./api_terraform";
export * from "./api_storage";
export * from "./api_oci";
export * from "./api_vnc";
export * from "./api_backup";
export * from "./api_monitor";
export * from "./api_power";
export * from "./api_operations";
