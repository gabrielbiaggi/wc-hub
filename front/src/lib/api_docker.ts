import { api } from "@/lib/api";

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

export const getDockerHealth = async () =>
  (await api.get<DockerHealth>("/v1/docker/health")).data;

export const getDockerInventory = async () =>
  (await api.get<DockerInventory>("/v1/docker/inventory")).data;

export const getDockerContainers = async () =>
  (await api.get<{ items: DockerContainer[] }>("/v1/docker/containers")).data
    .items;

export const getDockerImages = async () =>
  (await api.get<{ items: DockerImage[] }>("/v1/docker/images")).data.items;

export const getDockerStats = async () =>
  (
    await api.get<{ items: DockerContainerStats[]; warnings: string[] }>(
      "/v1/docker/stats",
    )
  ).data;

export type DockerContainerAction =
  "start" | "stop" | "restart" | "kill" | "remove";

export const runDockerContainerAction = async (
  id: string,
  action: DockerContainerAction,
  headers?: Record<string, string>,
) =>
  (
    await api.post<{ status: string }>(
      `/v1/docker/containers/${encodeURIComponent(id)}/${action}`,
      {},
      { headers },
    )
  ).data;

export const execDockerContainer = async (
  id: string,
  command: string[],
  headers?: Record<string, string>,
) =>
  (
    await api.post<{ output: string }>(
      `/v1/docker/containers/${encodeURIComponent(id)}/exec`,
      { command },
      { headers, timeout: 45_000 },
    )
  ).data;

export const cloneDockerStack = async (container_id: string, suffix: string) =>
  (
    await api.post<{ status: string; new_stack_name: string }>(
      "/v1/docker/stacks/clone",
      { container_id, suffix },
    )
  ).data;

export interface WorkerNode {
  id: string;
  name: string;
  hardware_fingerprint: string;
  public_key: string;
  ip_address: string;
  status: "pending_approval" | "approved" | "rejected";
  approved_at?: string;
  created_at: string;
  updated_at: string;
}

export const getPendingWorkers = async () =>
  (await api.get<{ items: WorkerNode[] }>("/v1/workers/pending")).data.items;

export const approveWorker = async (id: string) =>
  (await api.post<WorkerNode>(`/v1/workers/${encodeURIComponent(id)}/approve`)).data;

export const rejectWorker = async (id: string) =>
  (await api.post<WorkerNode>(`/v1/workers/${encodeURIComponent(id)}/reject`)).data;

