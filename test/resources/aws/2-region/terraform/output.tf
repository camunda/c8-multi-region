output "cert_manager_arn" {
  value       = module.eks_cluster.cert_manager_arn
  description = "The Amazon Resource Name (ARN) of the IAM Roles for Service Account mapping for the cert-manager"
}

output "external_dns_arn" {
  value       = module.eks_cluster.external_dns_arn
  description = "The Amazon Resource Name (ARN) of the IAM Roles for Service Account mapping for the external-dns"
}

output "s3_aws_access_key" {
  value = aws_iam_access_key.service_account_access_key.id
}

output "s3_aws_secret_access_key" {
  value = aws_iam_access_key.service_account_access_key.secret
  sensitive = true
}
