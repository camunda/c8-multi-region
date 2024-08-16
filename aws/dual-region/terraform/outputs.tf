################################################################################
# S3 Outputs
################################################################################

output "s3_aws_access_key" {
  value = aws_iam_access_key.service_account_access_key.id
}

output "s3_aws_secret_access_key" {
  value     = aws_iam_access_key.service_account_access_key.secret
  sensitive = true
}

output "s3_bucket_name" {
  value = aws_s3_bucket.elastic_backup.bucket
}

################################################################################
# IRSA (IAM Roles for Service Accounts) Outputs
################################################################################

# This value is required during the helm installation of the cluster autoscaler
output "eks_autoscaling_role_arn" {
  value       = module.eks_autoscaling_role.iam_role_arn
  description = "The ARN of the IAM role for the EKS autoscaling service account"
}
