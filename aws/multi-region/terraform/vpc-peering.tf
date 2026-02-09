################################
# Peering Connection          #
################################
# This is the peering connection between the two VPCs
# You always have a requester and an accepter. The requester is the one who initiates the connection.
# That's why were using the owner and accepter naming convention.
# Auto_accept is only required in the accepter. Otherwise you have to manually accept the connection.
# Auto_accept only works in the "owner" if the VPCs are in the same region

resource "aws_vpc_peering_connection" "owner_accepter" {
  vpc_id      = module.eks_cluster_region_0.vpc_id
  peer_vpc_id = module.eks_cluster_region_1.vpc_id
  peer_region = local.accepter.region
  auto_accept = false

  tags = {
    Name = "${var.cluster_name}-${local.owner.region_full_name}-to-${local.accepter.region_full_name}"
  }
}

resource "aws_vpc_peering_connection" "owner_quorum" {
  vpc_id      = module.eks_cluster_region_0.vpc_id
  peer_vpc_id = module.eks_cluster_region_2.vpc_id
  peer_region = local.quorum.region
  auto_accept = false

  tags = {
    Name = "${var.cluster_name}-${local.owner.region_full_name}-to-${local.quorum.region_full_name}"
  }
}

resource "aws_vpc_peering_connection" "accepter_quorum" {
  provider    = aws.accepter
  vpc_id      = module.eks_cluster_region_1.vpc_id
  peer_vpc_id = module.eks_cluster_region_2.vpc_id
  peer_region = local.quorum.region
  auto_accept = false

  tags = {
    Name = "${var.cluster_name}-${local.accepter.region_full_name}-to-${local.quorum.region_full_name}"
  }
}

resource "aws_vpc_peering_connection_accepter" "accepter_owner" {
  provider = aws.accepter

  vpc_peering_connection_id = aws_vpc_peering_connection.owner_accepter.id
  auto_accept               = true

  tags = {
    Name = "${var.cluster_name}-${local.accepter.region_full_name}-to-${local.owner.region_full_name}"
  }
}

resource "aws_vpc_peering_connection_accepter" "quorum_owner" {
  provider = aws.quorum

  vpc_peering_connection_id = aws_vpc_peering_connection.owner_quorum.id
  auto_accept               = true

  tags = {
    Name = "${var.cluster_name}-${local.quorum.region_full_name}-to-${local.owner.region_full_name}"
  }
}

resource "aws_vpc_peering_connection_accepter" "accepter_quorum" {
  provider = aws.quorum

  vpc_peering_connection_id = aws_vpc_peering_connection.accepter_quorum.id
  auto_accept               = true

  tags = {
    Name = "${var.cluster_name}-${local.quorum.region_full_name}-to-${local.accepter.region_full_name}"
  }
}


################################
# Route Table Updates          #
################################
# These are required to let the VPC know where to route the traffic to
# In this case non local cidr range --> VPC Peering connection.

# FIXED: Changed from aws_vpc_peering_connection.owner.id to .owner_accepter.id
resource "aws_route" "owner_accepter" {
  route_table_id            = module.eks_cluster_region_0.vpc_main_route_table_id
  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_accepter.id
}

resource "aws_route" "owner_quorum" {
  route_table_id            = module.eks_cluster_region_0.vpc_main_route_table_id
  destination_cidr_block    = local.quorum.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_quorum.id
}

# FIXED: Changed from aws_vpc_peering_connection.owner.id to .owner_accepter.id
resource "aws_route" "accepter_to_owner" {
  provider = aws.accepter

  route_table_id            = module.eks_cluster_region_1.vpc_main_route_table_id
  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_accepter.id
}

# FIXED: Changed from acceptor_quorum to accepter_quorum
resource "aws_route" "accepter_to_quorum" {
  provider = aws.accepter

  route_table_id            = module.eks_cluster_region_1.vpc_main_route_table_id
  destination_cidr_block    = local.quorum.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.accepter_quorum.id
}

resource "aws_route" "quorum_to_owner" {
  provider = aws.quorum

  route_table_id            = module.eks_cluster_region_2.vpc_main_route_table_id
  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_quorum.id
}

# FIXED: Changed from service_cidr_block to vpc_cidr_block
# FIXED: Changed from acceptor_quorum to accepter_quorum
resource "aws_route" "quorum_to_accepter" {
  provider = aws.quorum

  route_table_id            = module.eks_cluster_region_2.vpc_main_route_table_id
  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.accepter_quorum.id
}
################################
# Private Route Table Updates  #
################################
# These are required to let the VPC know where to route the traffic to
# In this case non local cidr range --> VPC Peering connection.

resource "aws_route" "owner_accepter_private" {
  count          = length(module.eks_cluster_region_0.private_route_table_ids)
  route_table_id = module.eks_cluster_region_0.private_route_table_ids[count.index]

  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_accepter.id
}

resource "aws_route" "owner_quorum_private" {
  count          = length(module.eks_cluster_region_0.private_route_table_ids)
  route_table_id = module.eks_cluster_region_0.private_route_table_ids[count.index]

  destination_cidr_block    = local.quorum.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_quorum.id
}

resource "aws_route" "accepter_owner_private" {
  provider = aws.accepter

  count          = length(module.eks_cluster_region_1.private_route_table_ids)
  route_table_id = module.eks_cluster_region_1.private_route_table_ids[count.index]

  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_accepter.id
}
resource "aws_route" "accepter_quorum_private" {
  provider = aws.accepter

  count          = length(module.eks_cluster_region_1.private_route_table_ids)
  route_table_id = module.eks_cluster_region_1.private_route_table_ids[count.index]

  destination_cidr_block    = local.quorum.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.accepter_quorum.id
}

resource "aws_route" "quorum_owner_private" {
  provider = aws.quorum

  count          = length(module.eks_cluster_region_2.private_route_table_ids)
  route_table_id = module.eks_cluster_region_2.private_route_table_ids[count.index]

  destination_cidr_block    = local.owner.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner_quorum.id
}
resource "aws_route" "quorum_accepter_private" {
  provider = aws.quorum

  count          = length(module.eks_cluster_region_2.private_route_table_ids)
  route_table_id = module.eks_cluster_region_2.private_route_table_ids[count.index]

  destination_cidr_block    = local.accepter.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.accepter_quorum.id
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

resource "aws_vpc_security_group_ingress_rule" "quorum_eks_primary" {
  provider = aws.quorum

  security_group_id = module.eks_cluster_region_2.cluster_primary_security_group_id

  cidr_ipv4   = local.owner.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

# ADDED: Missing security group rules for complete mesh connectivity
resource "aws_vpc_security_group_ingress_rule" "owner_from_quorum" {
  security_group_id = module.eks_cluster_region_0.cluster_primary_security_group_id

  cidr_ipv4   = local.quorum.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

resource "aws_vpc_security_group_ingress_rule" "accepter_from_quorum" {
  provider = aws.accepter

  security_group_id = module.eks_cluster_region_1.cluster_primary_security_group_id

  cidr_ipv4   = local.quorum.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

resource "aws_vpc_security_group_ingress_rule" "quorum_from_accepter" {
  provider = aws.quorum

  security_group_id = module.eks_cluster_region_2.cluster_primary_security_group_id

  cidr_ipv4   = local.accepter.vpc_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}
