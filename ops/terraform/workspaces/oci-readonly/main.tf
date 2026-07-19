terraform {
  required_version = ">= 1.10.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 7.0"
    }
  }
}

provider "oci" {
  config_file_profile = "DEFAULT"
}

variable "tenancy_ocid" {
  type      = string
  sensitive = true
}

data "oci_identity_availability_domains" "current" {
  compartment_id = var.tenancy_ocid
}

output "availability_domain_count" {
  value = length(data.oci_identity_availability_domains.current.availability_domains)
}
