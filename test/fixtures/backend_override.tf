# tflint-ignore-file: all

# For tests we're overwriting the backend to use S3
# The S3 backend is configured via the environment variable TF_CLI_ARGS_init
# Ignoring TF Lint, overrides make use of merges, therefore only contain the required subset of the configuration
terraform {
  backend "s3" {}
}
