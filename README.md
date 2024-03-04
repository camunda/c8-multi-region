# Camunda Platform - Multi-Region

This repository contains configuration files for setting up a multi-region Kubernetes cluster in AWS and running the Camunda Platform in multi-region mode. The current focus is on a dual-region setup.

Additionally, nightly tests are included and executed to ensure that the failover and fallback behaviour don't break.

For greater details and explanations, conduct our [documentation](#documentation).

## Disclaimer

- Customers must develop and test operational procedures in non-production environments based on the framework steps outlined by Camunda concerning the dual-region setup.
- Before advancing to production go-live with a dual-region setup, it is essential for customers to validate these procedures with Camunda.

## Requirements

- Terraform
- AWS
- Camunda 8.5+
- Helm

## Limitations
- Camunda 8 must be installed with the [Camunda Helm chart](https://github.com/camunda/camunda-platform-helm)
  - e.g., plain docker installation is not supported
- Looking at the whole Camunda Platform, it's active-passive, while some key components are active-active
  - meaning there's always one primary and one secondary region
- The user is responsible for detecting a regional failure and executing the operational procedure
- Currently, there is no support for Identity and Optimize
  - Multi-tenancy does not work
  - Role Based Access Control (RBAC) does not work
- Only a maximum latency of 100 ms between the regions is supported
- During the fallback there’s a small chance that some data will be lost for the WebApps
  - Due to the difference in sequence position tracking for exporters to different ElasticSearch locations

## Documentation
- TODO: link to docs.camunda.io
