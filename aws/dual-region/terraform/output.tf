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
