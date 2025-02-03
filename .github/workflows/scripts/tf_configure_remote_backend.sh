#!/bin/bash

# This file is used to configure the remote backend for Terraform
# We keep local backend to align with the docs and for easy try out
# This script will replace the local backend with the S3 backend
# lastly export a variable to be used by Terraform to confgire the remote backend

# Check if a file path is provided as an argument
if [ $# -eq 0 ]; then
    echo "Usage: $0 <file>"
    exit 1
fi

# Check if the file exists
if [ ! -f "$1" ]; then
    echo "File $1 not found."
    exit 1
fi

# if running on mac requires extra sed -i '' '...' $f1
# Remove path argument, doesn't exist on S3
sed -i '/path/d' "$1"

# Replace local with s3 backend
sed -i 's/\"local\"/\"s3\"/g' "$1"

cat <<EOF > ~/.terraformrc
cli {
  args = [
    "-backend-config=bucket=tf-state-multi-reg",
    "-backend-config=key=state/\${CLUSTER_NAME}/terraform.tfstate",
    "-backend-config=region=eu-central-1",
    "-backend-config=encrypt=true"
  ]
}
EOF
