resource "aws_s3_bucket" "elastic_backup" {
  bucket = "${var.cluster_name}-elastic-backup"

  tags = {
    Name = var.cluster_name
  }

  force_destroy = true
}

resource "aws_iam_user" "service_account" {
  name = "${var.cluster_name}-s3-service-account"
}

resource "aws_iam_access_key" "service_account_access_key" {
  user = aws_iam_user.service_account.name
}

resource "aws_iam_policy" "s3_access_policy" {
  name        = "${var.cluster_name}-s3-access-policy"
  description = "Policy for accessing S3 bucket"

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = [
          "s3:*"
        ],
        Resource = [
          aws_s3_bucket.elastic_backup.arn,
          "${aws_s3_bucket.elastic_backup.arn}/*"
        ]
      }
    ]
  })
}

resource "aws_iam_user_policy_attachment" "s3_access_attachment" {
  user       = aws_iam_user.service_account.name
  policy_arn = aws_iam_policy.s3_access_policy.arn
}
