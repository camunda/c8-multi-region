################################
# Magic Variables             #
################################

locals {
  # For dual-region we used owner and accepter to distinguish between the two regions but with three regions we default on the region itself
  # We will need multiple peerings, so the naming doesn't make sense anymore
  useast1 = {
    region             = "us-east-1"     # London
    vpc_cidr_block     = "10.192.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.190.0.0/16" # internal network of the cluster
    region_full_name   = "useast1"
  }
  useast2 = {
    region             = "us-east-2"     # Paris
    vpc_cidr_block     = "10.202.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.200.0.0/16" # internal network of the cluster
    region_full_name   = "useast2"
  }
  cacentral1 = {
    region             = "ca-central-1"  # Frankfurt
    vpc_cidr_block     = "10.212.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.210.0.0/16" # internal network of the cluster
    region_full_name   = "cacentral1"
  }
}

################################
# Variables                    #
################################

variable "cluster_name" {
  type        = string
  description = "Name of the cluster to prefix resources"
  default     = "dave-poc"
}

variable "aws_profile" {
  type        = string
  description = "AWS Profile to use"
  default     = "infex"
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes version to use"
  default     = "1.30"
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
  default     = 15
  description = "Maximum number of nodes in the node pool"
}
