
variable "owner_vpc_id" {
  type = string
}

variable "owner_region_full_name" {
  type = string
}

variable "owner_main_route_table_id" {
  type = string
}

variable "owner_cidr_block" {
  type = string
}

variable "owner_private_route_table_ids" {
  type = list(string)
}

variable "owner_cluster_primary_security_group_id" {
  type = string
}

variable "accepter_vpc_id" {
  type = string
}

variable "accepter_region" {
  type = string
}

variable "accepter_region_full_name" {
  type = string
}

variable "accepter_main_route_table_id" {
  type = string
}

variable "accepter_cidr_block" {
  type = string
}

variable "accepter_private_route_table_ids" {
  type = list(string)
}

variable "accepter_cluster_primary_security_group_id" {
  type = string
}

variable "prefix" {
  type = string
}
