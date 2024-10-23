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
      version = "5.72.0"
    }
  }
}

# Two provider configurations are needed to create resources in two different regions
# It's a limitation by how the AWS provider works
provider "aws" {
  region  = local.useast1.region
  alias   = "useast1"        # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

provider "aws" {
  region  = local.useast2.region
  alias   = "useast2"         # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}

provider "aws" {
  region  = local.cacentral1.region
  alias   = "cacentral1"     # can't use variable as alias is determined beforehand, therefore we need to hardcode it
  profile = var.aws_profile # optional, feel free to remove if you use the default profile
}
