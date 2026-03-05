#!/bin/bash
set -euxo pipefail

# Trivy is run inside a Docker container to limit supply chain exposure.
# The binary was removed from asdf/.tool-versions after corruption concerns.

# renovate: datasource=docker depName=ghcr.io/aquasecurity/trivy versioning=semver
TRIVY_VERSION="0.69.3"

REPO_ROOT="$(git rev-parse --show-toplevel)"

# list of the folders that we want to parse, only if a README.md exists and no .trivy_ignore
echo "Scanning terraform configuration with trivy: aws/dual-region/terraform"
docker run --rm \
  -v "${REPO_ROOT}:/workspace:ro" \
  -w /workspace \
  "ghcr.io/aquasecurity/trivy:${TRIVY_VERSION}" \
  config --config /workspace/.lint/trivy/trivy.yaml --ignorefile /workspace/.trivyignore aws/dual-region/terraform
