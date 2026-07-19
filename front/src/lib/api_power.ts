import { api } from "./api";

export interface PowerStatus {
  configured: boolean;
  online: boolean;
  batteryPercent?: number;
  loadPercent?: number;
  runtimeSeconds?: number;
  upsStatus?: string;
  checkedAt: string;
  error?: string;
}
export interface WakeTarget {
  id: string;
  mac: string;
}

export const getPowerStatus = async () =>
  (await api.get<PowerStatus>("/v1/power/status")).data;
export const getWakeTargets = async () =>
  (await api.get<{ items: WakeTarget[] }>("/v1/power/targets")).data.items;
export const wakeTarget = async (target: string) =>
  (
    await api.post<{ target: string; status: string }>(
      `/v1/power/wake/${encodeURIComponent(target)}`,
    )
  ).data;
