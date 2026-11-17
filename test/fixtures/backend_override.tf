# tflint-ignore-file: all # Terraform merges override files into the module, so this file holds only a subset of the configuration

# For tests we're overwriting the backend to use S3
# The S3 backend is configured via the environment variable TF_CLI_ARGS_init
terraform {
  backend "s3" {}
}
