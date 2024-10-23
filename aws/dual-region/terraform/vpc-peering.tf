
module "vpc_peering_useast1_useast2" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_useast1.vpc_id
  owner_region_full_name                  = local.useast1.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_useast1.vpc_main_route_table_id
  owner_cidr_block                        = local.useast1.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_useast1.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_useast1.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_useast2.vpc_id
  accepter_region                            = local.useast2.region
  accepter_region_full_name                  = local.useast2.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_useast2.vpc_main_route_table_id
  accepter_cidr_block                        = local.useast2.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_useast2.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_useast2.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_useast1,
    module.eks_cluster_region_useast2
  ]

  providers = {
    aws.owner    = aws.useast1
    aws.accepter = aws.useast2
  }
}

module "vpc_peering_useast1_cacentral1" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_useast1.vpc_id
  owner_region_full_name                  = local.useast1.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_useast1.vpc_main_route_table_id
  owner_cidr_block                        = local.useast1.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_useast1.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_useast1.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_cacentral1.vpc_id
  accepter_region                            = local.cacentral1.region
  accepter_region_full_name                  = local.cacentral1.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_cacentral1.vpc_main_route_table_id
  accepter_cidr_block                        = local.cacentral1.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_cacentral1.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_cacentral1.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_useast1,
    module.eks_cluster_region_cacentral1
  ]

  providers = {
    aws.owner    = aws.useast1
    aws.accepter = aws.cacentral1
  }
}

module "vpc_peering_cacentral1_useast2" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_cacentral1.vpc_id
  owner_region_full_name                  = local.cacentral1.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_cacentral1.vpc_main_route_table_id
  owner_cidr_block                        = local.cacentral1.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_cacentral1.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_cacentral1.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_useast2.vpc_id
  accepter_region                            = local.useast2.region
  accepter_region_full_name                  = local.useast2.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_useast2.vpc_main_route_table_id
  accepter_cidr_block                        = local.useast2.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_useast2.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_useast2.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_cacentral1,
    module.eks_cluster_region_useast2
  ]

  providers = {
    aws.owner    = aws.cacentral1
    aws.accepter = aws.useast2
  }
}
