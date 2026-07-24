import { api } from '@/lib/api'

export type CloudflareTunnelStatus = 'healthy' | 'degraded' | 'down' | 'inactive' | 'unknown'
export type CloudflareProviderStatus = 'healthy' | 'degraded' | 'unavailable'

export interface CloudflareTunnelConnection {
  colocation: string
  connection_id: string
  client_version: string
  opened_at: string
  pending: boolean
}

export interface CloudflareTunnel {
  id: string
  account_id: string
  name: string
  status: CloudflareTunnelStatus
  created_at: string
  connections: CloudflareTunnelConnection[]
}

export interface CloudflareDNSRecord {
  id: string
  zone_id: string
  type: string
  name: string
  content: string
  proxied: boolean
  proxiable: boolean
  ttl: number
  comment?: string
  modified_at: string
}

export interface CloudflareTargetHealth {
  kind: 'account' | 'zone'
  id: string
  status: 'healthy' | 'unavailable'
  message: string
  item_count: number
}

export interface CloudflareOverview {
  generated_at: string
  status: CloudflareProviderStatus
  tunnels: CloudflareTunnel[]
  dns_records: CloudflareDNSRecord[]
  targets: CloudflareTargetHealth[]
  summary: {
    accounts: number
    zones: number
    tunnels: number
    healthy_tunnels: number
    dns_records: number
    proxied_records: number
  }
}

export const getCloudflareOverview = async () =>
  (await api.get<CloudflareOverview>('/v1/cloudflare/overview')).data

export const getCloudflareTunnels = async (accountID: string) =>
  (await api.get<{ items: CloudflareTunnel[] }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels`)).data.items
export const createCloudflareTunnel=async(accountID:string,name:string)=>(await api.post<CloudflareTunnel>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels`,{name})).data
export const updateCloudflareTunnel=async(accountID:string,tunnelID:string,name:string)=>(await api.patch<CloudflareTunnel>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels/${encodeURIComponent(tunnelID)}`,{name})).data
export const deleteCloudflareTunnel=async(accountID:string,tunnelID:string)=>api.delete(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels/${encodeURIComponent(tunnelID)}`)
export interface CloudflareTunnelIngressRule { hostname?: string; service: string; path?: string; originRequest?: Record<string, unknown> }
export interface CloudflareTunnelConfiguration { account_id:string; tunnel_id:string; version:number; config:{ ingress:CloudflareTunnelIngressRule[]; originRequest?:Record<string,unknown> } }
export const getCloudflareTunnelConfiguration=async(accountID:string,tunnelID:string)=>(await api.get<CloudflareTunnelConfiguration>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels/${encodeURIComponent(tunnelID)}/configuration`)).data
export const updateCloudflareTunnelConfiguration=async(accountID:string,tunnelID:string,ingress:CloudflareTunnelIngressRule[])=>(await api.put<CloudflareTunnelConfiguration>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/tunnels/${encodeURIComponent(tunnelID)}/configuration`,{ingress})).data
export interface CloudflarePrivateRoute{id:string;network:string;tunnel_id:string;comment?:string}
export const getCloudflarePrivateRoutes=async(accountID:string)=>(await api.get<{items:CloudflarePrivateRoute[]}>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/private-routes`)).data.items
export const createCloudflarePrivateRoute=async(accountID:string,input:{network:string;tunnel_id:string;comment?:string})=>(await api.post<CloudflarePrivateRoute>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/private-routes`,input)).data

export const getCloudflareDNSRecords = async (zoneID: string) =>
  (await api.get<{ items: CloudflareDNSRecord[] }>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/dns-records`)).data.items

export interface CloudflareDNSInput { type:string; name:string; content:string; proxied:boolean; ttl:number; comment?:string }
export const createCloudflareDNSRecord = async(zoneID:string,input:CloudflareDNSInput)=>(await api.post<CloudflareDNSRecord>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/dns-records`,input)).data
export const updateCloudflareDNSRecord = async(zoneID:string,recordID:string,input:CloudflareDNSInput)=>(await api.put<CloudflareDNSRecord>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/dns-records/${encodeURIComponent(recordID)}`,input)).data
export const deleteCloudflareDNSRecord = async(zoneID:string,recordID:string)=>api.delete(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/dns-records/${encodeURIComponent(recordID)}`)
export interface CloudflareZoneSetting{id:string;value:unknown;editable:boolean;modified_on:string}
export interface CloudflareRuleset{id:string;name:string;description:string;kind:string;phase:string;version:string;last_updated:string}
export const getCloudflareZoneSettings=async(zoneID:string)=>(await api.get<{items:CloudflareZoneSetting[]}>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/settings`)).data.items
export const updateCloudflareZoneSetting=async(zoneID:string,setting:string,value:unknown)=>(await api.patch<CloudflareZoneSetting>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/settings/${encodeURIComponent(setting)}`,{value})).data
export const purgeCloudflareCache=async(zoneID:string)=>(await api.post<{status:string}>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/purge-cache`,{})).data
export const getCloudflareRulesets=async(zoneID:string)=>(await api.get<{items:CloudflareRuleset[]}>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/rulesets`)).data.items

export interface CloudflareWorker {
  id: string
  etag: string
  modified_on: string
  usage_model: string
}
export interface CloudflareKVNamespace {
  id: string
  title: string
}
export interface CloudflareWAFRule {
  id: string
  action: string
  description: string
  filter: {
    id: string
    expression: string
  }
}
export interface CloudflareZoneAnalytics {
  total_requests: number
  cached_requests: number
  uncached_requests: number
  total_bytes: number
  cached_bytes: number
  threats_blocked: number
}

export const getCloudflareWorkers = async (accountID: string) =>
  (await api.get<{ items: CloudflareWorker[] }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/workers`)).data.items

export const uploadCloudflareWorker = async (accountID: string, name: string, code: string) =>
  (await api.put<{ status: string; name: string }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/workers/${encodeURIComponent(name)}`, { code })).data

export const deleteCloudflareWorker = async (accountID: string, name: string) =>
  (await api.delete<{ status: string; name: string }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/workers/${encodeURIComponent(name)}`)).data

export const getCloudflareKVNamespaces = async (accountID: string) =>
  (await api.get<{ items: CloudflareKVNamespace[] }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/kv`)).data.items

export const createCloudflareKVNamespace = async (accountID: string, title: string) =>
  (await api.post<{ status: string; id: string; title: string }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/kv`, { title })).data

export const deleteCloudflareKVNamespace = async (accountID: string, id: string) =>
  (await api.delete<{ status: string; id: string }>(`/v1/cloudflare/accounts/${encodeURIComponent(accountID)}/kv/${encodeURIComponent(id)}`)).data

export const getCloudflareWAFRules = async (zoneID: string) =>
  (await api.get<{ items: CloudflareWAFRule[] }>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/waf`)).data.items

export const createCloudflareWAFRule = async (zoneID: string, input: { action: string; expression: string; description: string }) =>
  (await api.post<{ status: string; id: string }>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/waf`, input)).data

export const deleteCloudflareWAFRule = async (zoneID: string, id: string) =>
  (await api.delete<{ status: string; id: string }>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/waf/${encodeURIComponent(id)}`)).data

export const getCloudflareZoneAnalytics = async (zoneID: string) =>
  (await api.get<CloudflareZoneAnalytics>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/analytics`)).data
