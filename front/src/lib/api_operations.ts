import { api } from './api'

export interface OperationCatalogEntry {
  id:string; provider:string; resource:string; name:string; permission:string
  risk:'safe'|'dangerous'|'critical'; confirmation:string; execution:'direct'|'job'; status:'available'|'planned'; route?:string
}
export const getOperationCatalog = async () => (await api.get<{items:OperationCatalogEntry[]}>('/v1/operations/catalog')).data.items
