################################
# Cluster Creations            #
################################

module "eks_cluster_region_0" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-deployment-references//aws/modules/eks-cluster"

  region                = local.owner.region
  name                  = "${var.cluster_name}-${local.owner.region_full_name}"
  kubernetes_version    = var.kubernetes_version
  np_instance_types     = var.np_instance_types
  np_capacity_type      = var.np_capacity_type
  np_max_node_count     = var.np_max_node_count
  np_desired_node_count = var.np_desired_node_count
  single_nat_gateway    = var.single_nat_gateway

  cluster_service_ipv4_cidr = local.owner.service_cidr_block
  cluster_node_ipv4_cidr    = local.owner.vpc_cidr_block
}

module "eks_cluster_region_1" {
  # tflint-ignore: terraform_module_pinned_source
  source = "github.com/camunda/camunda-deployment-references//aws/modules/eks-cluster"

  region                = local.accepter.region
  name                  = "${var.cluster_name}-${local.accepter.region_full_name}"
  kubernetes_version    = var.kubernetes_version
  np_instance_types     = var.np_instance_types
  np_capacity_type      = var.np_capacity_type
  np_max_node_count     = var.np_max_node_count
  np_desired_node_count = var.np_desired_node_count
  single_nat_gateway    = var.single_nat_gateway

  cluster_service_ipv4_cidr = local.accepter.service_cidr_block
  cluster_node_ipv4_cidr    = local.accepter.vpc_cidr_block

  # Important to reference the correcet provider for the "foreign" region
  # Otherwise the resources will be created in the default region
  # Also important for all other resources that need to be created in the "foreign" region
  providers = {
    aws = aws.accepter
  }
}
