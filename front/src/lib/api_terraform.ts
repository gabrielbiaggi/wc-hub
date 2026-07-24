import { api } from '@/lib/api'

export type TerraformOperation = 'validate' | 'plan' | 'apply' | 'destroy' | 'output'
export interface TerraformRun { id:string; workspace:string; operation:TerraformOperation; status:string; output:string; summary:{add:number;change:number;destroy:number}; created_at:string; finished_at?:string }
export const getTerraformRuns = async () => (await api.get<{items:TerraformRun[];workspaces:string[]}>('/v1/terraform/runs')).data
export const startTerraformRun = async (
  operation: TerraformOperation,
  workspace: string,
  vars?: Record<string, string>,
  headers?: Record<string, string>,
) =>
  (
    await api.post<TerraformRun>(
      `/v1/terraform/${operation}`,
      { workspace, vars },
      { timeout: 660_000, headers },
    )
  ).data

export const getTerraformState = async (workspace: string) =>
  (await api.get<Record<string, unknown>>('/v1/terraform/state', { params: { workspace } })).data
