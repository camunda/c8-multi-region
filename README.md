# Camunda Platform - Multi-Region

> **⚠️ Note:** This version is no longer maintained. Please refer to the [Camunda release announcements](https://docs.camunda.io/docs/reference/announcements-release-notes/overview/) for current supported versions.

This repository contains configuration files for setting up a multi-region Kubernetes cluster in AWS and running the Camunda Platform in multi-region mode. The current focus is on a dual-region setup.

Additionally, nightly tests are included and executed to ensure that the failover and fallback behaviour don't break.

For greater details and explanations, conduct our [documentation](#documentation).

## Disclaimer

- Customers must develop and test [operational procedures](https://docs.camunda.io/docs/next/self-managed/operational-guides/multi-region/dual-region-operational-procedure/) in non-production environments based on the framework steps outlined by Camunda before applying them in production setups.
- Before advancing to production go-live, validating these procedures with Camunda is strongly recommended.
- Customers are solely responsible for detecting any regional failures and implementing the necessary [operational procedures](https://docs.camunda.io/docs/next/self-managed/operational-guides/multi-region/dual-region-operational-procedure/).

## Requirements

- Terraform
- AWS
- Camunda 8.5+
- Helm

## Limitations

- For an extensive list of limitations, conduct the [limitations section in the docs](https://docs.camunda.io/docs/next/self-managed/concepts/multi-region/dual-region/#limitations)

## Documentation

- [Dual region concept](https://docs.camunda.io/docs/next/self-managed/concepts/multi-region/dual-region/) explaining requirements and limitations.
- [Example AWS implementation](https://docs.camunda.io/docs/next/self-managed/setup/deploy/amazon/amazon-eks/dual-region/).
- [Operational procedure](https://docs.camunda.io/docs/next/self-managed/operational-guides/multi-region/dual-region-operational-procedure/) on how to recover from a total region loss.
