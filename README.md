# Camunda Platform - Multi-Region

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
- [Cluster scaling](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/) on how to scale Zeebe brokers and partitions.

## Testing

This repository includes comprehensive tests for multi-region operations:

### Deployment and Operational Procedures

- **Deployment Tests**: `TestAWSDeployDualRegCamunda` - Deploy Camunda 8 in dual-region mode
- **Failover Tests**: `TestAWSDualRegFailover_8_6_plus` - Validate failover procedure (8.6+)
- **Failback Tests**: `TestAWSDualRegFailback_8_6_plus` - Validate failback procedure (8.6+)
- **Migration Tests**: `TestMigrationDualReg` - Test Camunda version migration
- **Multi-Tenancy Tests**: `TestMultiTenancyDualReg` - Validate multi-tenant deployments

### Cluster Scaling Tests

Three comprehensive test scenarios validate Zeebe cluster scaling operations in multi-region setups:

- **Scale Brokers**: `TestZeebeClusterScaleUpBrokers` - Scale from 8 to 12 brokers (4→6 per region) and back
- **Scale Partitions**: `TestZeebeClusterScaleUpPartitions` - Scale from 8 to 12 partitions across existing brokers
- **Scale Both**: `TestZeebeClusterScaleUpBothBrokersAndPartitions` - Simultaneously scale brokers to 16 and partitions to 16
