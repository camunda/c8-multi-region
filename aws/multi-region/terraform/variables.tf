################################
# Magic Variables             #
################################

locals {
  # For demenstration purposes, we will use owner and acceptor as separation. Naming choice will become clearer when seeing the peering setup
  owner = {
    region             = "ap-northeast-1" # Tokyo
    vpc_cidr_block     = "10.200.0.0/16"  # vpc for the cluster and pod range
    service_cidr_block = "10.201.0.0/16"  # internal network of the cluster
    region_full_name   = "tokyo"
    availability_zones = ["ap-northeast-1a", "ap-northeast-1c"]
  }
  accepter = {
    region             = "ap-northeast-2" # Seoul
    vpc_cidr_block     = "10.150.0.0/16"  # vpc for the cluster and pod range
    service_cidr_block = "10.151.0.0/16"  # internal network of the cluster
    region_full_name   = "seoul"
    availability_zones = ["ap-northeast-2a", "ap-northeast-2c"]
  }
  quorum = {
    region             = "ap-northeast-3" # Osaka
    vpc_cidr_block     = "10.120.0.0/16"  # vpc for the cluster and pod range
    service_cidr_block = "10.121.0.0/16"  # internal network of the cluster
    region_full_name   = "osaka"
    availability_zones = ["ap-northeast-3a", "ap-northeast-3c"]
  }
}

################################
# Variables                    #
################################

variable "cluster_name" {
  type        = string
  description = "Name of the cluster to prefix resources"
}

variable "aws_profile" {
  type        = string
  description = "AWS Profile to use"
  default     = "default"
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes version to use"
  # renovate: datasource=endoflife-date depName=amazon-eks versioning=loose
  default = "1.34"
}

variable "np_instance_types" {
  type        = list(string)
  description = "Instance types for the node pool"
  default     = ["m6i.xlarge"]
}

variable "np_capacity_type" {
  type        = string
  default     = "SPOT"
  description = "Allows setting the capacity type to ON_DEMAND or SPOT to determine stable nodes"
}

variable "np_max_node_count" {
  type        = number
  default     = 5
  description = "Maximum number of nodes in the node pool"
}

variable "np_desired_node_count" {
  type        = number
  default     = 4
  description = "Desired number of nodes in the node pool"
}

variable "single_nat_gateway" {
  type        = bool
  default     = false
  description = "If true, only one NAT gateway will be created to save on e.g. IPs, not good for HA"
}
variable "default_tags" {
  type        = map(string)
  default     = {}
  description = "Default tags to apply to all resources"
}
