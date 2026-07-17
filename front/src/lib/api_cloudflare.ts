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

export const getCloudflareDNSRecords = async (zoneID: string) =>
  (await api.get<{ items: CloudflareDNSRecord[] }>(`/v1/cloudflare/zones/${encodeURIComponent(zoneID)}/dns-records`)).data.items
