module "eks_autoscaling_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "5.44.0"

  role_name = "${var.cluster_name}-eks-autoscaling-role"

  oidc_providers = {
    london = {
      provider_arn               = module.eks_cluster_region_london.oidc_provider_arn
      namespace_service_accounts = ["cluster-autoscaler:cluster-autoscaler-aws-cluster-autoscaler"]
    }
    paris = {
      provider_arn               = module.eks_cluster_region_paris.oidc_provider_arn
      namespace_service_accounts = ["cluster-autoscaler:cluster-autoscaler-aws-cluster-autoscaler"]
    }
    frankfurt = {
      provider_arn               = module.eks_cluster_region_frankfurt.oidc_provider_arn
      namespace_service_accounts = ["cluster-autoscaler:cluster-autoscaler-aws-cluster-autoscaler"]
    }
  }

  role_policy_arns = {
    policy = aws_iam_policy.eks_autoscaling_policy.arn
  }
}

resource "aws_iam_policy" "eks_autoscaling_policy" {
  name = "${var.cluster_name}-eks-autoscaling-policy"

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : [
          "autoscaling:DescribeAutoScalingGroups",
          "autoscaling:DescribeAutoScalingInstances",
          "autoscaling:DescribeLaunchConfigurations",
          "autoscaling:DescribeScalingActivities",
          "ec2:DescribeImages",
          "ec2:DescribeInstanceTypes",
          "ec2:DescribeLaunchTemplateVersions",
          "ec2:GetInstanceTypesFromInstanceRequirements",
          "eks:DescribeNodegroup"
        ],
        "Resource" : ["*"]
      },
      {
        "Effect" : "Allow",
        "Action" : [
          "autoscaling:SetDesiredCapacity",
          "autoscaling:TerminateInstanceInAutoScalingGroup"
        ],
        "Resource" : ["*"]
      }
    ]
  })
}
