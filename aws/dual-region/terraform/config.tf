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
      version = "5.61.0"
    }
  }
}

# Two provider configurations are needed to create resources in two different regions
# It's a limitation by how the AWS provider works
provider "aws" {
  region  = local.london.region
  alias   = "london"        # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

provider "aws" {
  region  = local.paris.region
  alias   = "paris"         # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

provider "aws" {
  region  = local.frankfurt.region
  alias   = "frankfurt"     # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}
