################################
# Magic Variables             #
################################

locals {
  # For demenstration purposes, we will use owner and acceptor as separation. Naming choice will become clearer when seeing the peering setup
  owner = {
    region             = "eu-west-2"     # London
    vpc_cidr_block     = "10.192.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.190.0.0/16" # internal network of the cluster
    region_full_name   = "london"
  }
  accepter = {
    region             = "eu-west-3"     # Paris
    vpc_cidr_block     = "10.202.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.200.0.0/16" # internal network of the cluster
    region_full_name   = "paris"
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
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes version to use"
  # renovate: datasource=endoflife-date depName=amazon-eks versioning=loose
  default = "1.32"
}

variable "np_instance_types" {
  type        = list(string)
  description = "Instance types for the node pool"
  default     = ["m6i.xlarge"]
}

variable "np_capacity_type" {
  type        = string
  default     = "ON_DEMAND"
  description = "Allows setting the capacity type to ON_DEMAND or SPOT to determine stable nodes"
}

variable "np_max_node_count" {
  type        = number
  default     = 10
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
