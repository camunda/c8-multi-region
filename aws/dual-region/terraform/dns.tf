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

# Manual Step, can only be enabled after Elasticsearch has been deployed
# We are creating an A record for to move the traffic to the correct region based on the country
# In case of unhealthy target, the traffic will be moved to the next region

### Exclude following block on initial run - Start ###
data "aws_lb" "de_internal_elastic" {
  provider = aws.frankfurt
  tags = {
    "kubernetes.io/service-name" = "camunda-frankfurt/camunda-frankfurt-elasticsearch"
  }
}

data "aws_lb" "gb_internal_elastic" {
  provider = aws.london
  tags = {
    "kubernetes.io/service-name" = "camunda-london/camunda-london-elasticsearch"
  }
}

data "aws_lb" "fr_internal_elastic" {
  provider = aws.paris
  tags = {
    "kubernetes.io/service-name" = "camunda-paris/camunda-paris-elasticsearch"
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
