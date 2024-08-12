terraform {
  required_version = ">= 0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 0"
      # Peering has to be initiated from own side - owner and accepted from the other - accepter.
      configuration_aliases = [aws.owner, aws.accepter]
    }
  }
}
