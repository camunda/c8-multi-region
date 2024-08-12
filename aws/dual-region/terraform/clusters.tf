################################
# Cluster Creations            #
################################

module "eks_cluster_region_london" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.london.region
  name               = "${var.cluster_name}-${local.london.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.london.service_cidr_block
  cluster_node_ipv4_cidr    = local.london.vpc_cidr_block

  providers = {
    aws = aws.london
  }
}

module "eks_cluster_region_paris" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.paris.region
  name               = "${var.cluster_name}-${local.paris.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.paris.service_cidr_block
  cluster_node_ipv4_cidr    = local.paris.vpc_cidr_block

  # Important to reference the correcet provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.paris
  }
}

module "eks_cluster_region_frankfurt" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.frankfurt.region
  name               = "${var.cluster_name}-${local.frankfurt.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.frankfurt.service_cidr_block
  cluster_node_ipv4_cidr    = local.frankfurt.vpc_cidr_block

  # Important to reference the correcet provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.frankfurt
  }
}
