#!/bin/bash
set -euxo pipefail

# list of the folders that we want to parse, only if a README.md exists and no .trivy_ignore
echo "Scanning terraform configuration with trivy: aws/dual-region/terraform"
trivy config --config .lint/trivy/trivy.yaml --ignorefile .trivyignore aws/dual-region/terraform
