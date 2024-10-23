# Route53 config for internal DNS

resource "aws_route53_zone" "internal" {
  name    = "blobfish.local"
  comment = "Managed by Terraform within the Multi-Region SaaS POC"
  vpc {
    vpc_id     = module.eks_cluster_region_cacentral1.vpc_id
    vpc_region = "ca-central-1"
  }
  vpc {
    vpc_id     = module.eks_cluster_region_useast1.vpc_id
    vpc_region = "us-east-1"
  }
  vpc {
    vpc_id     = module.eks_cluster_region_useast2.vpc_id
    vpc_region = "us-east-2"
  }
}

# Manual Step, can only be enabled after Elasticsearch has been deployed
# We are creating an A record for to move the traffic to the correct region based on the country
# In case of unhealthy target, the traffic will be moved to the next region

### Exclude following block on initial run - Start ###
data "aws_lb" "de_internal_elastic" {
  provider = aws.cacentral1
  tags = {
    "kubernetes.io/service-name" = "camunda-cacentral1/camunda-cacentral1-elasticsearch"
  }
}

data "aws_lb" "gb_internal_elastic" {
  provider = aws.useast1
  tags = {
    "kubernetes.io/service-name" = "camunda-useast1/camunda-useast1-elasticsearch"
  }
}

data "aws_lb" "fr_internal_elastic" {
  provider = aws.useast2
  tags = {
    "kubernetes.io/service-name" = "camunda-useast2/camunda-useast2-elasticsearch"
  }
}

resource "aws_route53_record" "gb_internal_elastic" {
  zone_id        = aws_route53_zone.internal.zone_id
  name           = "elastic.blobfish.local"
  type           = "A"
  set_identifier = "GB"

  alias {
    evaluate_target_health = true
    name                   = data.aws_lb.gb_internal_elastic.dns_name
    zone_id                = data.aws_lb.gb_internal_elastic.zone_id
  }

  geolocation_routing_policy {
    country = "GB"
  }
}

resource "aws_route53_record" "fr_internal_elastic" {
  zone_id        = aws_route53_zone.internal.zone_id
  name           = "elastic.blobfish.local"
  type           = "A"
  set_identifier = "FR"

  alias {
    evaluate_target_health = true
    name                   = data.aws_lb.fr_internal_elastic.dns_name
    zone_id                = data.aws_lb.fr_internal_elastic.zone_id
  }

  geolocation_routing_policy {
    country = "FR"
  }
}

resource "aws_route53_record" "de_internal_elastic" {
  zone_id        = aws_route53_zone.internal.zone_id
  name           = "elastic.blobfish.local"
  type           = "A"
  set_identifier = "DE"

  alias {
    evaluate_target_health = true
    name                   = data.aws_lb.de_internal_elastic.dns_name
    zone_id                = data.aws_lb.de_internal_elastic.zone_id
  }

  geolocation_routing_policy {
    country = "DE"
  }
}
### Exclude following block on initial run - End ###
