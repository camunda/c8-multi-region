#!/bin/bash

###############################################################################
# Important: Adjust the following environment variables to your setup         #
###############################################################################

# The script must be executed with
# . ./export_environment_prerequisites.sh
# to export the environment variables to the current shell

# The AWS regions of your Kubernetes cluster 0, 1, and 2
export REGION_0=us-east-1
export REGION_1=us-east-2
export REGION_2=ca-central-1

# The names of your Kubernetes clusters in regions 0, 1, and 2
# default based on the tutorial is the following
export CLUSTER_0=cluster-us-east-1
export CLUSTER_1=cluster-us-east-2
export CLUSTER_2=cluster-ca-central-1

# The Kubernetes namespaces for each region where Camunda 8 should be running and the failover namespaces
# Namespace names must be unique to route the traffic
export CAMUNDA_NAMESPACE_0=camunda-us-east-1
export CAMUNDA_NAMESPACE_0_FAILOVER=camunda-us-east-1-failover
export CAMUNDA_NAMESPACE_1=camunda-us-east-2
export CAMUNDA_NAMESPACE_1_FAILOVER=camunda-us-east-2-failover
export CAMUNDA_NAMESPACE_2=camunda-ca-central-1
export CAMUNDA_NAMESPACE_2_FAILOVER=camunda-ca-central-1-failover

# The Helm release name used for installing Camunda 8 in both Kubernetes clusters
export HELM_RELEASE_NAME=camunda
# renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io
export HELM_CHART_VERSION=11.0.1
