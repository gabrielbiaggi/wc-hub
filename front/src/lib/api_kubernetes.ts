import { api } from "@/lib/api";
export interface KubeMetadata {
  name: string;
  namespace: string;
  uid: string;
  creationTimestamp: string;
  labels: Record<string, string>;
}
export interface KubeCondition {
  type: string;
  status: string;
  reason: string;
  message: string;
  lastTransitionTime: string;
}
export interface KubeNode {
  metadata: KubeMetadata;
  status: {
    conditions: KubeCondition[];
    nodeInfo: { kubeletVersion: string; osImage: string; architecture: string };
    capacity: Record<string, string>;
  };
}
export interface KubeDeployment {
  metadata: KubeMetadata;
  spec: { replicas: number };
  status: {
    replicas: number;
    readyReplicas: number;
    availableReplicas: number;
    unavailableReplicas: number;
  };
}
export interface KubePod {
  metadata: KubeMetadata;
  status: {
    phase: string;
    reason: string;
    message: string;
    podIP: string;
    hostIP: string;
    containerStatuses: Array<{
      name: string;
      ready: boolean;
      restartCount: number;
    }>;
  };
}
export interface KubeEvent {
  metadata: KubeMetadata;
  type: string;
  reason: string;
  message: string;
  count: number;
  lastTimestamp: string;
  regarding?: { kind: string; namespace: string; name: string };
  involvedObject?: { kind: string; namespace: string; name: string };
}
export interface KubernetesOverview {
  generated_at: string;
  nodes: KubeNode[];
  deployments: KubeDeployment[];
  pods: KubePod[];
  problem_pods: KubePod[];
  events: KubeEvent[];
}
export const getKubernetesOverview = async () =>
  (await api.get<KubernetesOverview>("/v1/kubernetes/overview")).data;
export type KubernetesDeploymentAction = "scale" | "restart" | "delete";
export const runKubernetesDeploymentAction = async (
  namespace: string,
  name: string,
  action: KubernetesDeploymentAction,
  replicas?: number,
  headers?: Record<string, string>,
) =>
  (
    await api.post<{ status: string }>(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/deployments/${encodeURIComponent(name)}/${action}`,
      action === "scale" ? { replicas } : {},
      { headers },
    )
  ).data;
export const getKubernetesPodLogs = async (
  namespace: string,
  pod: string,
  container = "",
) =>
  (
    await api.get<{ output: string }>(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/logs`,
      { params: { container } },
    )
  ).data;
export const execKubernetesPod = async (
  namespace: string,
  pod: string,
  container: string,
  command: string[],
  headers?: Record<string, string>,
) =>
  (
    await api.post<{ output: string }>(
      `/v1/kubernetes/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(pod)}/exec`,
      { container, command },
      { headers, timeout: 45_000 },
    )
  ).data;
