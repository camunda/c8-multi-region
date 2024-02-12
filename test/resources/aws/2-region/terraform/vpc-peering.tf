
################################
# Peering Connection          #
################################
# This is the peering connection between the two VPCs
# You always have a requester and an accepter. The requester is the one who initiates the connection.
# That's why were using the owner and accepter naming convention.
# Auto_accept is only required in the accepter. Otherwise you have to manually accept the connection.
# Auto_accept only works in the "owner" if the VPCs are in the same region

resource "aws_vpc_peering_connection" "owner" {
  vpc_id      = module.eks_cluster.vpc_id
  peer_vpc_id = module.eks_cluster_region_b.vpc_id
  peer_region = local.accepter.region
  auto_accept = false

  tags = {
    Name = "${local.name}-${local.owner.region_full_name}-to-${local.accepter.region_full_name}"
  }
}

# Important: Breaks on first apply. Not sure why.
# Not required for the PoC.
# resource "aws_vpc_peering_connection_options" "owner" {
#   vpc_peering_connection_id = aws_vpc_peering_connection.owner.id

#   requester {
#     allow_remote_vpc_dns_resolution = true
#   }

#   depends_on = [aws_vpc_peering_connection.owner]
# }

resource "aws_vpc_peering_connection_accepter" "accepter" {
  provider = aws.accepter

  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
  auto_accept               = true

  tags = {
    Name = "${local.name}-${local.accepter.region_full_name}-to-${local.owner.region_full_name}"
  }
}

# Important: Breaks on first apply. Not sure why.
# Not required for the PoC.
# resource "aws_vpc_peering_connection_options" "accepter" {
#   provider                  = aws.accepter
#   vpc_peering_connection_id = aws_vpc_peering_connection_accepter.accepter.id


#   accepter {
#     allow_remote_vpc_dns_resolution = true
#   }

#   depends_on = [aws_vpc_peering_connection_accepter.accepter]
# }

################################
# Route Table Updates          #
################################
# These are required to let the VPC know where to route the traffic to
# In this case non local cidr range --> VPC Peering connection.
# Maybe there's a better way to add the info the the required route tables. Kinda hacky but works for first iteration.

resource "aws_route" "owner" {
  route_table_id            = module.eks_cluster.vpc_main_route_table_id
  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "owner_private" {
  count          = length(module.eks_cluster.private_route_table_ids)
  route_table_id = module.eks_cluster.private_route_table_ids[count.index]

  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter" {
  provider = aws.accepter

  route_table_id            = module.eks_cluster_region_b.vpc_main_route_table_id
  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter_private" {
  provider = aws.accepter

  count          = length(module.eks_cluster_region_b.private_route_table_ids)
  route_table_id = module.eks_cluster_region_b.private_route_table_ids[count.index]

  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

################################
# Security Groups Updates      # 
################################
# These changes are required to actually allow inbound traffic from the other VPC.
# Maybe there's a better way to add the info the the required security groups. Kinda hacky but works for first iteration.

resource "aws_vpc_security_group_ingress_rule" "owner_eks_primary" {
  security_group_id = module.eks_cluster.cluster_primary_security_group_id

  cidr_ipv4   = local.accepter.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

resource "aws_vpc_security_group_ingress_rule" "accepter_eks_primary" {
  provider = aws.accepter

  security_group_id = module.eks_cluster_region_b.cluster_primary_security_group_id

  cidr_ipv4   = local.owner.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}
