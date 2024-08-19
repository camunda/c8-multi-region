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

output "paris_external_dns_role_arn" {
  value       = module.eks_cluster_region_paris.external_dns_arn
  description = "The ARN of the IAM role for the EKS external-dns service account in the Paris region"
}

output "paris_cert_manager_role_arn" {
  value       = module.eks_cluster_region_paris.cert_manager_arn
  description = "The ARN of the IAM role for the EKS cert-manager service account in the Paris region"
}
