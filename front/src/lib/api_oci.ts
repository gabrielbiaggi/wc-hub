import { api } from "./api";

export interface OCIRegion {
  name: string;
  status: string;
  home: boolean;
}
export interface OCIAvailabilityDomain {
  name: string;
}
export interface OCICompartment {
  id: string;
  name: string;
  description: string;
  lifecycle_state: string;
  parent_id?: string;
}
export interface OCIInstance {
  id: string;
  display_name: string;
  lifecycle_state: string;
  shape: string;
  availability_domain: string;
  fault_domain: string;
  region: string;
  compartment_id: string;
  ocpus: number;
  memory_gb: number;
  public_ip?: string;
  private_ip?: string;
  time_created?: string;
  tags?: Record<string, string>;
}
export interface OCIVCN {
  id: string;
  display_name: string;
  cidr_blocks: string[];
  lifecycle_state: string;
  dns_label: string;
  compartment_id: string;
  time_created?: string;
}
export interface OCISubnet {
  id: string;
  display_name: string;
  cidr_block: string;
  availability_domain?: string;
  lifecycle_state: string;
  vcn_id: string;
  compartment_id: string;
  prohibit_public_ip_on_vnic: boolean;
}
export interface OCIAutonomousDatabase {
  id: string;
  display_name: string;
  db_name: string;
  lifecycle_state: string;
  lifecycle_details?: string;
  compartment_id: string;
  workload: string;
  compute_model: string;
  compute_count: number;
  storage_tb: number;
  free_tier: boolean;
  time_created?: string;
}
export interface OCIDBSystem {
  id: string;
  display_name: string;
  lifecycle_state: string;
  availability_domain: string;
  shape: string;
  compartment_id: string;
  subnet_id: string;
  database_edition: string;
  cpu_core_count: number;
  memory_gb: number;
  time_created?: string;
}
export interface OCIBlockVolume {
  id: string;
  display_name: string;
  lifecycle_state: string;
  availability_domain: string;
  compartment_id: string;
  size_gb: number;
  vpus_per_gb: number;
  time_created?: string;
}
export interface OCIOverview {
  captured_at: string;
  tenancy_id: string;
  tenancy_name: string;
  home_region: string;
  regions: OCIRegion[];
  availability_domains: OCIAvailabilityDomain[];
  compartments: OCICompartment[];
  instances: OCIInstance[];
  vcns: OCIVCN[];
  subnets: OCISubnet[];
  autonomous_databases: OCIAutonomousDatabase[];
  db_systems: OCIDBSystem[];
  block_volumes: OCIBlockVolume[];
}
export type OCIInstanceAction =
  "start" | "stop" | "shutdown" | "reboot" | "reset";

export const getOCIOverview = async () =>
  (await api.get<OCIOverview>("/v1/oci/overview", { timeout: 60_000 })).data;
export const runOCIInstanceAction = async (
  instanceId: string,
  action: OCIInstanceAction,
  region?: string,
) =>
  (
    await api.post<{ status: string; action: string }>(
      `/v1/oci/instances/${action}`,
      { instance_id: instanceId, region },
      { timeout: 60_000 },
    )
  ).data;
export const terminateOCIInstance = async (
  instanceId: string,
  region?: string,
) =>
  (
    await api.delete<{ status: string; instance_id: string }>(
      `/v1/oci/instances/${encodeURIComponent(instanceId)}`,
      { params: { region }, timeout: 60_000 },
    )
  ).data;

export interface OCIImageSummary {
  id: string;
  display_name: string;
  operating_system: string;
  operating_system_version: string;
}
export const getOCIImages = async (
  compartment_id?: string,
  region?: string,
): Promise<OCIImageSummary[]> =>
  (
    await api.get<{ items: OCIImageSummary[] }>("/v1/oci/images", {
      params: { compartment_id, region },
    })
  ).data.items;

export interface OCIShapeSummary {
  name: string;
  ocpus: number;
  memory_gb: number;
}
export const getOCIShapes = async (
  compartment_id?: string,
  region?: string,
): Promise<OCIShapeSummary[]> =>
  (
    await api.get<{ items: OCIShapeSummary[] }>("/v1/oci/shapes", {
      params: { compartment_id, region },
    })
  ).data.items;

export const runOCIAutonomousDatabaseAction = async (
  dbId: string,
  action: "start" | "stop" | "restart" | "delete",
  region?: string,
) =>
  (
    await api.post<{ status: string; database_id: string }>(
      `/v1/oci/autonomous-databases/${encodeURIComponent(dbId)}/${action}`,
      {},
      { params: { region }, timeout: 60_000 },
    )
  ).data;

export const runOCIDBSystemAction = async (
  dbId: string,
  action: "start" | "stop" | "reboot" | "delete",
  region?: string,
) =>
  (
    await api.post<{ status: string; dbsystem_id: string }>(
      `/v1/oci/db-systems/${encodeURIComponent(dbId)}/${action}`,
      {},
      { params: { region }, timeout: 60_000 },
    )
  ).data;

export interface OCILaunchInstanceInput {
  region: string;
  compartment_id: string;
  availability_domain: string;
  display_name: string;
  shape: string;
  image_id: string;
  subnet_id: string;
  ocpus: number;
  memory_gb: number;
  assign_public_ip: boolean;
  ssh_authorized_key: string;
}
export interface OCICreateAutonomousDatabaseInput {
  region: string;
  compartment_id: string;
  display_name: string;
  db_name: string;
  admin_password: string;
  workload: "OLTP" | "DW" | "AJD" | "APEX";
  compute_count: number;
  storage_tb: number;
  free_tier: boolean;
  auto_scaling: boolean;
}
export const launchOCIInstance = async (input: OCILaunchInstanceInput) =>
  (
    await api.post<{ status: string; instance_id: string }>(
      "/v1/oci/instances",
      input,
      { timeout: 60_000 },
    )
  ).data;
export const createOCIAutonomousDatabase = async (
  input: OCICreateAutonomousDatabaseInput,
) =>
  (
    await api.post<{ status: string; database_id: string }>(
      "/v1/oci/autonomous-databases",
      input,
      { timeout: 60_000 },
    )
  ).data;

export interface OCICreateVCNInput {
  region: string;
  compartment_id: string;
  display_name: string;
  cidr_block: string;
  dns_label: string;
}
export const createOCIVCN = async (input: OCICreateVCNInput) =>
  (await api.post<{ status: string; vcn_id: string }>("/v1/oci/vcns", input)).data;

export const deleteOCIVCN = async (vcnId: string, region?: string) =>
  (
    await api.delete<{ status: string; vcn_id: string }>(
      `/v1/oci/vcns/${encodeURIComponent(vcnId)}`,
      { params: { region } },
    )
  ).data;

export interface OCICreateSubnetInput {
  region: string;
  compartment_id: string;
  vcn_id: string;
  display_name: string;
  cidr_block: string;
  private: boolean;
}
export const createOCISubnet = async (input: OCICreateSubnetInput) =>
  (await api.post<{ status: string; subnet_id: string }>("/v1/oci/subnets", input)).data;

export const deleteOCISubnet = async (subnetId: string, region?: string) =>
  (
    await api.delete<{ status: string; subnet_id: string }>(
      `/v1/oci/subnets/${encodeURIComponent(subnetId)}`,
      { params: { region } },
    )
  ).data;

export interface OCICreateVolumeInput {
  region: string;
  compartment_id: string;
  availability_domain: string;
  display_name: string;
  size_gb: number;
}
export const createOCIBlockVolume = async (input: OCICreateVolumeInput) =>
  (await api.post<{ status: string; volume_id: string }>("/v1/oci/volumes", input)).data;

export const deleteOCIBlockVolume = async (volumeId: string, region?: string) =>
  (
    await api.delete<{ status: string; volume_id: string }>(
      `/v1/oci/volumes/${encodeURIComponent(volumeId)}`,
      { params: { region } },
    )
  ).data;
