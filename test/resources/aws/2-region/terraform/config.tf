################################
# Magic Variables             # 
################################

locals {
  name = "nightly" # just some abbotrary name to prefix resources
  # For demenstration purposes, we will use owner and acceptor as separation. Naming choice will become clearer when seeing the peering setup
  owner = {
    region           = "eu-west-2"     # London
    vpc_cidr_block   = "10.192.0.0/16" # vpc for the cluster and pod range
    region_full_name = "london"
  }
  accepter = {
    region           = "eu-west-3"     # Paris
    vpc_cidr_block   = "10.202.0.0/16" # vpc for the cluster and pod range
    region_full_name = "paris"
  }
}

################################
# Backend & Provider Setup    # 
################################

terraform {
  backend "local" {
    path = "terraform.tfstate"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.22.0"
    }
  }
}

provider "aws" {
  region     = local.owner.region
  profile = "infex"
}

provider "aws" {
  region     = local.accepter.region
  alias      = "accepter"
  profile = "infex"
}

################################
# Cluster Creations            # 
################################

module "eks_cluster" {
  source = "github.com/camunda/camunda-tf-eks-module/modules/eks-cluster"

  region             = local.owner.region
  name               = "${local.name}-${local.owner.region_full_name}"
  kubernetes_version = "1.28"
  np_instance_types  = ["m6i.xlarge"]

  cluster_service_ipv4_cidr = "10.190.0.0/16" # internal network of the cluster
  cluster_node_ipv4_cidr    = local.owner.vpc_cidr_block

  # InfEx Specific
  aws_auth_roles = [{
    rolearn  = "arn:aws:iam::444804106854:role/AWSReservedSSO_SystemAdministrator_555f3db864dcee7e"
    username = "AWSReservedSSO_SystemAdministrator_555f3db864dcee7e"
    groups   = ["system:masters"]
  }]
}

module "eks_cluster_region_b" {
  source = "github.com/camunda/camunda-tf-eks-module/modules/eks-cluster"

  region             = local.accepter.region
  name               = "${local.name}-${local.accepter.region_full_name}"
  kubernetes_version = "1.28"
  np_instance_types  = ["m6i.xlarge"]

  cluster_service_ipv4_cidr = "10.200.0.0/16" # internal network of the cluster
  cluster_node_ipv4_cidr    = local.accepter.vpc_cidr_block

  # InfEx Specific
  aws_auth_roles = [{
    rolearn  = "arn:aws:iam::444804106854:role/AWSReservedSSO_SystemAdministrator_555f3db864dcee7e"
    username = "AWSReservedSSO_SystemAdministrator_555f3db864dcee7e"
    groups   = ["system:masters"]
  }]

  # Important to reference the correcet provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.accepter
  }
}
