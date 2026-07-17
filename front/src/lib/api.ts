import axios from 'axios'

let csrfToken = sessionStorage.getItem('wc_hub_csrf') ?? ''
export const setCSRFToken = (token: string) => { csrfToken = token; token ? sessionStorage.setItem('wc_hub_csrf', token) : sessionStorage.removeItem('wc_hub_csrf') }
export const api = axios.create({ baseURL: import.meta.env.VITE_API_URL || '/api', timeout: 10_000, withCredentials: true, headers: { Accept: 'application/json' } })
api.interceptors.request.use((config) => { if (csrfToken && config.method && !['get','head','options'].includes(config.method)) config.headers.set('X-CSRF-Token', csrfToken); return config })

export interface User { id:string; email:string; display_name:string; totp_enabled:boolean; roles:string[]; permissions:string[] }
export interface AuthResponse { user:User; csrf_token:string; expires_at:string }
export interface Metric { label: string; value: number; unit: string; delta: number; status: string }
export interface Activity { id: string; source: string; message: string; severity: string; at: string }
export interface Overview { generated_at: string; environment: string; self_protected: boolean; metrics: Metric[]; activity: Activity[]; series: number[] }
export interface Integration { id:string; name:string; provider:string; status:string; config:Record<string,unknown>; last_checked_at?:string; created_at:string }
export interface Host { id:string; integration_id?:string; name:string; hostname:string; scope:'local'|'remote'|'cloud'; status:string; self_protected:boolean; labels:Record<string,unknown>; facts:Record<string,unknown>; last_seen_at?:string; created_at:string }
export interface AuditEntry { id:string; actor_email?:string; action:string; scope:string; resource_type:string; resource_id?:string; target_name?:string; risk:string; decision:string; reason?:string; request_id?:string; occurred_at:string; event_hash:string }

export const getOverview = async () => (await api.get<Overview>('/v1/overview')).data
export const getIntegrations = async () => (await api.get<{items:Integration[]}>('/v1/integrations')).data.items
export const createIntegration = async (body:Pick<Integration,'name'|'provider'> & {config?:Record<string,unknown>}) => (await api.post<Integration>('/v1/integrations', body)).data
export const getHosts = async () => (await api.get<{items:Host[]}>('/v1/hosts')).data.items
export const createHost = async (body:Pick<Host,'name'|'hostname'|'scope'|'self_protected'> & Partial<Host>) => (await api.post<Host>('/v1/hosts', body)).data
export const getAudit = async () => (await api.get<{items:AuditEntry[]}>('/v1/audit')).data.items
export const enrollTOTP = async () => (await api.post<{secret:string;otpauth_uri:string}>('/v1/auth/totp/enroll', {})).data
export const confirmTOTP = async (code:string) => (await api.post<{totp_enabled:boolean}>('/v1/auth/totp/confirm', {code})).data
