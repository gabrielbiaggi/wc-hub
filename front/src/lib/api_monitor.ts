import { api } from "./api";
export interface MonitorTarget {
  id: string;
  name: string;
  target: string;
  kind: "http" | "tcp";
  intervalSeconds: number;
  enabled: boolean;
  lastStatus: "unknown" | "up" | "down";
  lastLatencyMS: number;
  lastError: string;
  lastCheckedAt?: string;
}
export const getMonitorTargets = async () =>
  (await api.get<{ items: MonitorTarget[] }>("/v1/monitor/targets")).data.items;
export const createMonitorTarget = async (
  input: Omit<
    MonitorTarget,
    "id" | "lastStatus" | "lastLatencyMS" | "lastError" | "lastCheckedAt"
  >,
) => (await api.post<MonitorTarget>("/v1/monitor/targets", input)).data;
export const deleteMonitorTarget = async (id: string) =>
  api.delete(`/v1/monitor/targets/${id}`);
export const getMonitorWebhook = async () =>
  (await api.get<{ configured: boolean }>("/v1/monitor/webhook")).data;
export const setMonitorWebhook = async (url: string) =>
  (await api.put<{ configured: boolean }>("/v1/monitor/webhook", { url })).data;
