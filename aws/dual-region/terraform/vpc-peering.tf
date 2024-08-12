
module "vpc_peering_london_paris" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_london.vpc_id
  owner_region_full_name                  = local.london.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_london.vpc_main_route_table_id
  owner_cidr_block                        = local.london.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_london.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_london.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_paris.vpc_id
  accepter_region                            = local.paris.region
  accepter_region_full_name                  = local.paris.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_paris.vpc_main_route_table_id
  accepter_cidr_block                        = local.paris.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_paris.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_paris.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_london,
    module.eks_cluster_region_paris
  ]

  providers = {
    aws.owner    = aws.london
    aws.accepter = aws.paris
  }
}

module "vpc_peering_london_frankfurt" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_london.vpc_id
  owner_region_full_name                  = local.london.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_london.vpc_main_route_table_id
  owner_cidr_block                        = local.london.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_london.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_london.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_frankfurt.vpc_id
  accepter_region                            = local.frankfurt.region
  accepter_region_full_name                  = local.frankfurt.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_frankfurt.vpc_main_route_table_id
  accepter_cidr_block                        = local.frankfurt.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_frankfurt.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_frankfurt.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_london,
    module.eks_cluster_region_frankfurt
  ]

  providers = {
    aws.owner    = aws.london
    aws.accepter = aws.frankfurt
  }
}

module "vpc_peering_frankfurt_paris" {
  source = "./modules/vpc-peering"

  owner_vpc_id                            = module.eks_cluster_region_frankfurt.vpc_id
  owner_region_full_name                  = local.frankfurt.region_full_name
  owner_main_route_table_id               = module.eks_cluster_region_frankfurt.vpc_main_route_table_id
  owner_cidr_block                        = local.frankfurt.vpc_cidr_block
  owner_private_route_table_ids           = module.eks_cluster_region_frankfurt.private_route_table_ids
  owner_cluster_primary_security_group_id = module.eks_cluster_region_frankfurt.cluster_primary_security_group_id

  accepter_vpc_id                            = module.eks_cluster_region_paris.vpc_id
  accepter_region                            = local.paris.region
  accepter_region_full_name                  = local.paris.region_full_name
  accepter_main_route_table_id               = module.eks_cluster_region_paris.vpc_main_route_table_id
  accepter_cidr_block                        = local.paris.vpc_cidr_block
  accepter_private_route_table_ids           = module.eks_cluster_region_paris.private_route_table_ids
  accepter_cluster_primary_security_group_id = module.eks_cluster_region_paris.cluster_primary_security_group_id

  prefix = var.cluster_name

  depends_on = [
    module.eks_cluster_region_frankfurt,
    module.eks_cluster_region_paris
  ]

  providers = {
    aws.owner    = aws.frankfurt
    aws.accepter = aws.paris
  }
}
