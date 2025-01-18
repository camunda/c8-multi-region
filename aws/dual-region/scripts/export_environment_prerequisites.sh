#!/bin/bash

###############################################################################
# Important: Adjust the following environment variables to your setup         #
###############################################################################

# The script must be executed with
# . ./export_environment_prerequisites.sh
# to export the environment variables to the current shell

# The AWS regions of your Kubernetes cluster 0 and 1
export REGION_0=eu-west-2
export REGION_1=eu-west-3

# The names of your Kubernetes clusters in regions 0 and 1
# default based on the tutorial is the following
export CLUSTER_0=cluster-london
export CLUSTER_1=cluster-paris

# The Kubernetes namespaces for each region where Camunda 8 should be running and the failover namespaces
# Namespace names must be unique to route the traffic
export CAMUNDA_NAMESPACE_0=camunda-london
export CAMUNDA_NAMESPACE_0_FAILOVER=camunda-london-failover
export CAMUNDA_NAMESPACE_1=camunda-paris
export CAMUNDA_NAMESPACE_1_FAILOVER=camunda-paris-failover

# The Helm release name used for installing Camunda 8 in both Kubernetes clusters
export HELM_RELEASE_NAME=camunda
# renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io versioning=regex:^10(\.(?<minor>\d+))?(\.(?<patch>\d+))?$
export HELM_CHART_VERSION=10.4.7
