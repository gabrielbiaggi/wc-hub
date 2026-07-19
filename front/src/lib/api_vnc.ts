import { api } from './api'
export interface VncTarget {
  id: string
  address: string
  name?: string
  kind?: string
  status?: string
  ws_path: string
}
export const getVncTargets=async()=>(await api.get<{items:VncTarget[]}>('/v1/vnc/targets')).data.items
