#!/bin/bash

###############################################################################
# Important: Adjust the following environment variables to your setup         #
###############################################################################

# The script must be executed with
# . ./export_environment_prerequisites.sh
# to export the environment variables to the current shell

# The AWS regions of your Kubernetes cluster 0 and 1
export REGION_0=ap-northeast-1
export REGION_1=ap-northeast-2
export REGION_2=ap-northeast-3

# The names of your Kubernetes clusters in regions 0 and 1
# default based on the tutorial is the following
export CLUSTER_0=<prefix_name>-tokyo
export CLUSTER_1=<prefix_name>-seoul
export CLUSTER_2=<prefix_name>-osaka

# The Kubernetes namespaces for each region where Camunda 8 should be running
# Namespace names must be unique to route the traffic
export CAMUNDA_NAMESPACE_0=camunda-tokyo
export CAMUNDA_NAMESPACE_1=camunda-seoul
export CAMUNDA_NAMESPACE_2=camunda-osaka

# The Helm release name used for installing Camunda 8 in both Kubernetes clusters
export CAMUNDA_RELEASE_NAME=camunda

# renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io versioning=regex:^13(\.(?<minor>\d+))?(\.(?<patch>\d+))?$
export HELM_CHART_VERSION=13.3.2
# TODO: [release-duty] before the release, update this!

aws eks --region $REGION_0 update-kubeconfig --name $CLUSTER_0 --alias $CLUSTER_0
aws eks --region $REGION_1 update-kubeconfig --name $CLUSTER_1 --alias $CLUSTER_1
aws eks --region $REGION_2 update-kubeconfig --name $CLUSTER_2 --alias $CLUSTER_2


kubectl --context $CLUSTER_0 apply -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/aws/dual-region/kubernetes/internal-dns-lb.yml
kubectl --context $CLUSTER_1 apply -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/aws/dual-region/kubernetes/internal-dns-lb.yml
kubectl --context $CLUSTER_2 apply -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/aws/dual-region/kubernetes/internal-dns-lb.yml