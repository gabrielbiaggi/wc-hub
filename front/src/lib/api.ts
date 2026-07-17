import axios from 'axios'
export const api = axios.create({ baseURL: import.meta.env.VITE_API_URL || '/api', timeout: 10_000, headers: { Accept: 'application/json' } })

export interface Metric { label: string; value: number; unit: string; delta: number; status: string }
export interface Activity { id: string; source: string; message: string; severity: string; at: string }
export interface Overview { generated_at: string; environment: string; self_protected: boolean; metrics: Metric[]; activity: Activity[]; series: number[] }
export const getOverview = async () => (await api.get<Overview>('/v1/overview')).data

