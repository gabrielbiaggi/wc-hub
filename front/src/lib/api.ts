import axios from 'axios'

let csrfToken = sessionStorage.getItem('wc_hub_csrf') ?? ''
export const setCSRFToken = (token: string) => { csrfToken = token; token ? sessionStorage.setItem('wc_hub_csrf', token) : sessionStorage.removeItem('wc_hub_csrf') }
export const api = axios.create({ baseURL: import.meta.env.VITE_API_URL || '/api', timeout: 30_000, withCredentials: true, headers: { Accept: 'application/json' } })
api.interceptors.request.use((config) => { if (csrfToken && config.method && !['get','head','options'].includes(config.method)) config.headers.set('X-CSRF-Token', csrfToken); return config })

export interface User { id:string; email:string; display_name:string; totp_enabled:boolean; roles:string[]; permissions:string[] }
export interface AuthResponse { user:User; csrf_token:string; expires_at:string }
export interface Metric { label: string; value: number; unit: string; delta: number; status: string }
export interface Activity { id: string; source: string; message: string; severity: string; at: string }
export interface Overview { generated_at: string; environment: string; self_protected: boolean; metrics: Metric[]; activity: Activity[]; series: number[] }
export interface Integration { id:string; name:string; provider:string; status:string; config:Record<string,unknown>; last_checked_at?:string; created_at:string }
export interface Host { id:string; integration_id?:string; name:string; hostname:string; scope:'local'|'remote'|'cloud'; status:string; self_protected:boolean; labels:Record<string,unknown>; facts:Record<string,unknown>; last_seen_at?:string; created_at:string }
export interface AuditEntry { id:string; actor_email?:string; action:string; scope:string; resource_type:string; resource_id?:string; target_name?:string; risk:string; decision:string; reason?:string; request_id?:string; occurred_at:string; event_hash:string }
export interface AdminUser { id:string; email:string; display_name:string; totp_enabled:boolean; disabled_at?:string; last_login_at?:string; created_at:string; roles:string[] }
export interface Role { id:string; slug:string; name:string; description:string; permissions:string[]; user_count:number }
export interface Permission { id:string; slug:string; description:string; risk:'safe'|'dangerous'|'critical' }
export interface Alert { id:string; resource_type:string; resource_id?:string; severity:string; title:string; description:string; status:'open'|'acknowledged'|'resolved'; acknowledged_at?:string; resolved_at?:string; created_at:string }
export interface ProxmoxSummary { configured:boolean; status:string; last_checked_at?:string; nodes:number; virtual_machines:number; containers:number; storage_pools:number }
export interface ProxmoxNode { node:string;status:string;cpu:number;maxcpu:number;mem:number;maxmem:number;uptime:number;level:string }
export interface ProxmoxGuest { vmid:number;name:string;status:string;cpu:number;cpus:number;mem:number;maxmem:number;maxdisk:number;uptime:number;node:string;type:'qemu'|'lxc';template?:number }
export interface ProxmoxStorage { storage:string;type:string;status:string;active:number;total:number;used:number;avail:number;shared:number;node:string }
export interface ProxmoxInventory { captured_at:string;nodes:ProxmoxNode[];virtual_machines:ProxmoxGuest[];containers:ProxmoxGuest[];storage:ProxmoxStorage[] }
export interface Job { id:string; kind:string; payload:Record<string,unknown>; status:string; priority:number; attempts:number; max_attempts:number; run_after:string; locked_by?:string; last_error?:string; created_at:string; started_at?:string; finished_at?:string }
export interface HostMetric { host_id:string; host_name:string; metric:string; value:number; unit:string; captured_at:string }

export const getOverview = async () => (await api.get<Overview>('/v1/overview')).data
export const getIntegrations = async () => (await api.get<{items:Integration[]}>('/v1/integrations')).data.items
export const createIntegration = async (body:Pick<Integration,'name'|'provider'> & {config?:Record<string,unknown>}) => (await api.post<Integration>('/v1/integrations', body)).data
export const getHosts = async () => (await api.get<{items:Host[]}>('/v1/hosts')).data.items
export const createHost = async (body:Pick<Host,'name'|'hostname'|'scope'|'self_protected'> & Partial<Host>) => (await api.post<Host>('/v1/hosts', body)).data
export const getAudit = async () => (await api.get<{items:AuditEntry[]}>('/v1/audit')).data.items
export const getAdminUsers = async () => (await api.get<{items:AdminUser[]}>('/v1/admin/users')).data.items
export const createAdminUser = async (body:{email:string;display_name:string;password:string;role_ids:string[]}) => (await api.post<AdminUser>('/v1/admin/users',body)).data
export const updateAdminUser = async (id:string,body:{email:string;display_name:string;disabled:boolean;role_ids:string[]}) => (await api.patch<AdminUser>(`/v1/admin/users/${id}`,body)).data
export const disableAdminUser = async (id:string) => api.delete(`/v1/admin/users/${id}`)
export const getRoles = async () => (await api.get<{items:Role[]}>('/v1/admin/roles')).data.items
export const createRole = async (body:{slug:string;name:string;description:string;permission_ids:string[]}) => (await api.post<Role>('/v1/admin/roles',body)).data
export const updateRole = async (id:string,body:{name:string;description:string;permission_ids:string[]}) => (await api.patch<Role>(`/v1/admin/roles/${id}`,body)).data
export const deleteRole = async (id:string) => api.delete(`/v1/admin/roles/${id}`)
export const getPermissions = async () => (await api.get<{items:Permission[]}>('/v1/admin/permissions')).data.items
export const getAlerts = async () => (await api.get<{items:Alert[]}>('/v1/alerts')).data.items
export const updateAlert = async (id:string,status:Alert['status']) => (await api.patch<Alert>(`/v1/alerts/${id}`,{status})).data
export const enrollTOTP = async () => (await api.post<{secret:string;otpauth_uri:string}>('/v1/auth/totp/enroll', {})).data
export const confirmTOTP = async (code:string) => (await api.post<{totp_enabled:boolean}>('/v1/auth/totp/confirm', {code})).data
export const getProxmoxSummary = async () => (await api.get<ProxmoxSummary>('/v1/proxmox/summary')).data
export const syncProxmox = async () => (await api.post<Job>('/v1/proxmox/sync', {})).data
export const getProxmoxInventory = async () => (await api.get<ProxmoxInventory>('/v1/proxmox/inventory')).data
export const runProxmoxPowerAction = async(node:string,kind:'qemu'|'lxc',vmid:number,action:'start'|'stop'|'shutdown'|'reboot'|'reset')=>(await api.post<{status:string}>(`/v1/proxmox/nodes/${encodeURIComponent(node)}/${kind}/${vmid}/${action}`,{})).data
export const getJobs = async () => (await api.get<{items:Job[]}>('/v1/jobs')).data.items
export const enqueueJob = async (kind:string) => (await api.post<Job>('/v1/jobs', {kind,payload:{},priority:100,max_attempts:5})).data
export const getHostTelemetry = async () => (await api.get<{items:HostMetric[]}>('/v1/telemetry/hosts')).data.items
export const createTerminalTicket = async (host_id:string,confirmation:string,totp_code:string) => (await api.post<{ticket:string;session_id:string;expires_in:number}>('/v1/terminal/tickets',{host_id,confirmation,totp_code})).data

export * from './api_docker'
export * from './api_kubernetes'
export * from './api_cloudflare'
export * from './api_github'
export * from './api_terraform'
export * from './api_storage'
export * from './api_oci'
