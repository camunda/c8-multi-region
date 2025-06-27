################################
# Backend & Provider Setup    #
################################

terraform {
  required_version = ">= 1.6.0"

  backend "local" {
    path = "terraform.tfstate"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.80.0"
    }
  }
}

# Two provider configurations are needed to create resources in two different regions
# It's a limitation by how the AWS provider works
provider "aws" {
  region  = local.owner.region
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
  default_tags {
    tags = var.default_tags
  }
}

provider "aws" {
  region  = local.accepter.region
  alias   = "accepter"
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
  default_tags {
    tags = var.default_tags
  }
}
