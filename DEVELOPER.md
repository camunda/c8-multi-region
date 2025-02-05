# Developer's Guide

Welcome to the development reference for Camunda's C8 Multi Region! This document provides guidance on setting up a testing environment, running tests, and managing releases.

## Setting up Development Environment

TODO: this part is not yet documented outside of the official documentation: https://docs.camunda.io/docs/next/self-managed/setup/deploy/amazon/amazon-eks/dual-region/

## Releasing a New Version

We follow Semantic Versioning (SemVer) guidelines for versioning. Follow these steps to release a new version:

1. **Commit History:**
   - Maintain a clear commit history with explicit messages detailing additions and deletions.

2. **Versioning:**
   - Determine the appropriate version number based on the changes made since the last release.
   - Follow the format `MAJOR.MINOR.PATCH` as per Semantic Versioning guidelines.

3. **GitHub Releases:**
   - Publish the new version on GitHub Releases.
   - Tag the release with the version number and include release notes summarizing changes.

## Adding new GH actions

Please pin GitHub action, if you need you can use [pin-github-action](https://github.com/mheap/pin-github-action) cli tool.

## Local Development

### Cluster Setup

1. Ensure AWS is setup with the profile `infex`. Otherwise overwrite with `AWS_PROFILE` to e.g. `default`.
2. Export `TESTS_TF_BINARY_NAME` to `terraform` if you don't want to use Tofu.
3. Adjust the AWS regions in `aws/dual-region/terraform/variables.tf`. The defaults are cleaned up nightly in InfraEx.
4. Ensure to export `CLUSTER_NAME` and `BACKUP_NAME` with custom values as the default `nightly` is cleaned up in InfraEx.
5. Ensure to remove any temporary terraform files from `aws/dual-region/terraform` alternatively run `terraform init -upgrade`
6. Go into `test` and run the following to execute terraform, alternatively, you can also run it manually from within terraform:

```bash
go test --count=1 -v -timeout 120m -run TestSetupTerraform
```

7. Export following environment variables, those will be used to setup the DNS chaining on each cluster for the opposing namespaces:

```bash
export CLUSTER_1_NAMESPACE_ARR=c8-3-cluster-1,c8-4-cluster-1,c8-5-cluster-1,c8-snap-cluster-1
export CLUSTER_1_NAMESPACE_FAILOVER_ARR=c8-3-cluster-1-failover,c8-4-cluster-1-failover,c8-5-cluster-1-failover,c8-snap-cluster-1-failover
export CLUSTER_0_NAMESPACE_ARR=c8-3-cluster-0,c8-4-cluster-0,c8-5-cluster-0,c8-snap-cluster-0
export CLUSTER_0_NAMESPACE_FAILOVER_ARR=c8-3-cluster-0-failover,c8-4-cluster-0-failover,c8-5-cluster-0-failover,c8-snap-cluster-0-failover
```

8. Adjust AWS regions in `TestAWSKubeConfigCreation` and `initKubernetesHelpers` function based on the ones chosen in Terraform.
9. Run in `test` the command to create the KubeConfig for each cluster:

```bash
go test --count=1 -v -timeout 120m -run TestAWSKubeConfigCreation
```

10. Export `AWS_ACCESS_KEY_ES` and `AWS_SECRET_ACCESS_KEY_ES` based on the terraform output.
   E.g. from within the `aws/dual-region/terraform` folder:

   ```bash
   export AWS_ACCESS_KEY_ES=$(terraform output -raw s3_aws_access_key)
   export AWS_SECRET_ACCESS_KEY_ES=$(terraform output -raw s3_aws_secret_access_key)
   ```

11. The following command will setup all namespaces and adds the elastic secret for backups. Run in `test` the command:

```bash
go test --count=1 -v -timeout 120m -run TestClusterPrerequisites
```

12. This will do the DNS chaining by deploying internal loadbalancers and configuring CoreDNS. Run in `test` the command:

```bash
go test --count=1 -v -timeout 120m -run TestAWSDNSChaining
```

### Running Tests

(Optional) Allows overwriting the version to use for Camunda 8, e.g. snapshot.
Otherwise defaults to published Helm versions and the latest stable release.

```bash
# Overwriting to snapshot image + snapshot helm chart
export HELM_CHART_VERSION=0.0.0-snapshot-alpha
export HELM_CHART_NAME=oci://ghcr.io/camunda/helm/camunda-platform
export GLOBAL_IMAGE_TAG=SNAPSHOT

# Otherwise it's sufficient to set the helm chart version or rely on the default.
export HELM_CHART_VERSION=9.3.8
```

- Deploy the dual-region setup

```bash
go test --count=1 -v -timeout 120m -run TestAWSDeployDualRegCamunda
```

- If checking against >= 8.6 with the new procedure

```bash
go test --count=1 -v -timeout 120m -run TestAWSDualRegFailover_8_6_plus
go test --count=1 -v -timeout 120m -run TestAWSDualRegFailback_8_6_plus
```

### Cleanup

```bash
# removes both C8 installations in both clusters
go test --count=1 -v -timeout 120m -run TestAWSDualRegCleanup

# removes internal loadbalancers to unblock TF destruction
go test --count=1 -v -timeout 120m -run TestClusterCleanup

# runs tf destroy, could also be done manually
go test --count=1 -v -timeout 120m -run TestTeardownTerraform
```

---

By following these guidelines, we ensure smooth development iterations, robust testing practices, and clear version management for the Terraform EKS module. Happy coding!
