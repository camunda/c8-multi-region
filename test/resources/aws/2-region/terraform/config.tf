################################
# Magic Variables             #
################################

locals {
  # For demenstration purposes, we will use owner and acceptor as separation.
  # Naming choice will become clearer when seeing the peering setup, since you always have an initiator and an acceptor.
  owner = {
    region             = "eu-west-2" # London
    region_full_name   = "london"
    vpc_cidr_block     = "10.192.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.190.0.0/16" # internal network of the cluster
  }
  accepter = {
    region             = "eu-west-3" # Paris
    region_full_name   = "paris"
    vpc_cidr_block     = "10.202.0.0/16" # vpc for the cluster and pod range
    service_cidr_block = "10.200.0.0/16" # internal network of the cluster
  }
}

################################
# Backend & Provider Setup    #
################################

terraform {
  required_version = ">= 1.6.0"
  backend "local" {
    path = "terraform.tfstate"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.37.0"
    }
  }
}

# Two provider configurations are needed to create resources in two different regions
# It's a limitation by how the AWS provider works
provider "aws" {
  region  = local.owner.region
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

provider "aws" {
  region  = local.accepter.region
  alias   = "accepter"
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

################################
# Cluster Creations            #
################################

module "eks_cluster" {
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster?ref=1.0.2"

  region             = local.owner.region
  name               = "${var.cluster_name}-${local.owner.region_full_name}"
  kubernetes_version = "1.28"
  np_instance_types  = ["m6i.xlarge"]

  cluster_service_ipv4_cidr = local.owner.service_cidr_block
  cluster_node_ipv4_cidr    = local.owner.vpc_cidr_block
}

module "eks_cluster_region_b" {
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster?ref=1.0.2"

  region             = local.accepter.region
  name               = "${var.cluster_name}-${local.accepter.region_full_name}"
  kubernetes_version = "1.28"
  np_instance_types  = ["m6i.xlarge"]

  cluster_service_ipv4_cidr = local.accepter.service_cidr_block
  cluster_node_ipv4_cidr    = local.accepter.vpc_cidr_block

  # Important to reference the correcet provider for the "remote" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "remote" region
  providers = {
    aws = aws.accepter
  }
}
