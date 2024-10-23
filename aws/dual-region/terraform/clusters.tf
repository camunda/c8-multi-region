################################
# Cluster Creations            #
################################

module "eks_cluster_region_useast1" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.useast1.region
  name               = "${var.cluster_name}-${local.useast1.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.useast1.service_cidr_block
  cluster_node_ipv4_cidr    = local.useast1.vpc_cidr_block

  providers = {
    aws = aws.useast1
  }
}

module "eks_cluster_region_useast2" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.useast2.region
  name               = "${var.cluster_name}-${local.useast2.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.useast2.service_cidr_block
  cluster_node_ipv4_cidr    = local.useast2.vpc_cidr_block

  # Important to reference the correct provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.useast2
  }
}

module "eks_cluster_region_cacentral1" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-tf-eks-module//modules/eks-cluster"

  region             = local.cacentral1.region
  name               = "${var.cluster_name}-${local.cacentral1.region_full_name}"
  kubernetes_version = var.kubernetes_version
  np_instance_types  = var.np_instance_types
  np_capacity_type   = var.np_capacity_type
  np_max_node_count  = var.np_max_node_count

  cluster_service_ipv4_cidr = local.cacentral1.service_cidr_block
  cluster_node_ipv4_cidr    = local.cacentral1.vpc_cidr_block

  # Important to reference the correcet provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.cacentral1
  }
}
