
################################
# Peering Connection          #
################################
# You always have a requester and an accepter. The requester is the one who initiates the connection.
# That's why were using the owner and accepter naming convention.
# Auto_accept is only required in the accepter. Otherwise you have to manually accept the connection.
# Auto_accept only works in the "owner" if the VPCs are in the same region

resource "aws_vpc_peering_connection" "owner" {
  provider = aws.owner

  vpc_id      = var.owner_vpc_id
  peer_vpc_id = var.accepter_vpc_id
  peer_region = var.accepter_region
  auto_accept = false

  tags = {
    Name = "${var.prefix}-${var.owner_region_full_name}-to-${var.accepter_region_full_name}"
  }
}

resource "aws_vpc_peering_connection_accepter" "accepter" {
  provider = aws.accepter

  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
  auto_accept               = true

  tags = {
    Name = "${var.prefix}-${var.accepter_region_full_name}-to-${var.owner_region_full_name}"
  }
}

################################
# Route Table Updates          #
################################
# These are required to let the VPC know where to route the traffic to
# In this case non local cidr range --> VPC Peering connection.

resource "aws_route" "owner" {
  provider = aws.owner

  route_table_id            = var.owner_main_route_table_id
  destination_cidr_block    = var.accepter_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "owner_private" {
  provider = aws.owner

  count          = length(var.owner_private_route_table_ids)
  route_table_id = var.owner_private_route_table_ids[count.index]

  destination_cidr_block    = var.accepter_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter" {
  provider = aws.accepter

  route_table_id            = var.accepter_main_route_table_id
  destination_cidr_block    = var.owner_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

resource "aws_route" "accepter_private" {
  provider = aws.accepter

  count          = length(var.accepter_private_route_table_ids)
  route_table_id = var.accepter_private_route_table_ids[count.index]

  destination_cidr_block    = var.owner_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.owner.id
}

################################
# Security Groups Updates      #
################################
# These changes are required to actually allow inbound traffic from the other VPC.

resource "aws_vpc_security_group_ingress_rule" "owner_eks_primary" {
  provider = aws.owner

  security_group_id = var.owner_cluster_primary_security_group_id

  cidr_ipv4   = var.accepter_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}

resource "aws_vpc_security_group_ingress_rule" "accepter_eks_primary" {
  provider = aws.accepter

  security_group_id = var.accepter_cluster_primary_security_group_id

  cidr_ipv4   = var.owner_cidr_block
  from_port   = -1
  ip_protocol = -1
  to_port     = -1
}
