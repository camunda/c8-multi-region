
################################
# Peering Connection          #
################################
# This is the peering connection between the two VPCs
# You always have a requester and an accepter. The requester is the one who initiates the connection.
# That's why were using the owner and accepter naming convention.
# Auto_accept is only required in the accepter. Otherwise you have to manually accept the connection.
# Auto_accept only works in the "owner" if the VPCs are in the same region

resource "aws_vpc_peering_connection" "owner" {
  vpc_id      = module.eks_cluster_region_0.vpc_id
  peer_vpc_id = module.eks_cluster_region_1.vpc_id
  peer_region = local.accepter.region
  auto_accept = false

  tags = {
    Name = "${var.cluster_name}-${local.owner.region_full_name}-to-${local.accepter.region_full_name}"
  }
}

resource "aws_vpc_peering_connection_accepter" "accepter" {
  provider = aws.accepter

  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
  auto_accept               = true

  tags = {
    Name = "${var.cluster_name}-${local.accepter.region_full_name}-to-${local.owner.region_full_name}"
  }
}

################################
# Route Table Updates          #
################################
# These are required to let the VPC know where to route the traffic to
# In this case non local cidr range --> VPC Peering connection.

resource "aws_route" "owner" {
  route_table_id            = module.eks_cluster_region_0.vpc_main_route_table_id
  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "owner_private" {
  count          = length(module.eks_cluster_region_0.private_route_table_ids)
  route_table_id = module.eks_cluster_region_0.private_route_table_ids[count.index]

  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter" {
  provider = aws.accepter

  route_table_id            = module.eks_cluster_region_1.vpc_main_route_table_id
  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter_private" {
  provider = aws.accepter

  count          = length(module.eks_cluster_region_1.private_route_table_ids)
  route_table_id = module.eks_cluster_region_1.private_route_table_ids[count.index]

  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

################################
# Security Groups Updates      #
################################
# These changes are required to actually allow inbound traffic from the other VPC.

resource "aws_vpc_security_group_ingress_rule" "owner_eks_primary" {
  security_group_id = module.eks_cluster_region_0.cluster_primary_security_group_id

  cidr_ipv4   = local.accepter.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

resource "aws_vpc_security_group_ingress_rule" "accepter_eks_primary" {
  provider = aws.accepter

  security_group_id = module.eks_cluster_region_1.cluster_primary_security_group_id

  cidr_ipv4   = local.owner.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}
