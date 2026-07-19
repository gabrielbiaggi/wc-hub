import { api } from './api'

export interface OCIRegion { name:string; status:string; home:boolean }
export interface OCIAvailabilityDomain { name:string }
export interface OCICompartment { id:string; name:string; description:string; lifecycle_state:string; parent_id?:string }
export interface OCIInstance { id:string; display_name:string; lifecycle_state:string; shape:string; availability_domain:string; fault_domain:string; region:string; compartment_id:string; ocpus:number; memory_gb:number; time_created?:string; tags?:Record<string,string> }
export interface OCIVCN { id:string; display_name:string; cidr_blocks:string[]; lifecycle_state:string; dns_label:string; compartment_id:string; time_created?:string }
export interface OCISubnet { id:string; display_name:string; cidr_block:string; availability_domain?:string; lifecycle_state:string; vcn_id:string; compartment_id:string; prohibit_public_ip_on_vnic:boolean }
export interface OCIOverview { captured_at:string; home_region:string; regions:OCIRegion[]; availability_domains:OCIAvailabilityDomain[]; compartments:OCICompartment[]; instances:OCIInstance[]; vcns:OCIVCN[]; subnets:OCISubnet[] }
export type OCIInstanceAction='start'|'stop'|'shutdown'|'reboot'|'reset'

export const getOCIOverview=async()=>(await api.get<OCIOverview>('/v1/oci/overview',{timeout:60_000})).data
export const runOCIInstanceAction=async(instanceId:string,action:OCIInstanceAction)=>(await api.post<{status:string;action:string}>(`/v1/oci/instances/${action}`,{instance_id:instanceId},{timeout:60_000})).data
