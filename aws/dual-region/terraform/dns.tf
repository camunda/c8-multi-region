# Route53 config for internal DNS

resource "aws_route53_zone" "internal" {
  name    = "blobfish.local"
  comment = "Managed by Terraform within the Multi-Region SaaS POC"
  vpc {
    vpc_id     = module.eks_cluster_region_frankfurt.vpc_id
    vpc_region = "eu-central-1"
  }
  vpc {
    vpc_id     = module.eks_cluster_region_london.vpc_id
    vpc_region = "eu-west-2"
  }
  vpc {
    vpc_id     = module.eks_cluster_region_paris.vpc_id
    vpc_region = "eu-west-3"
  }
}
