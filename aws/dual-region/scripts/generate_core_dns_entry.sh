#!/bin/bash

set -e

# This script generates the CoreDNS config addition for a dual-region setup.
# This config addition configures CoreDNS to forward DNS requests for specific
# namespaces to a remote cluster.
# The resulting contents, as described in the output, should be copied and added
# to the configmap of CoreDNS and their respective clusters.
# It does so by using the Kubectl and the AWS CLI to retrieve the hostnames and IPs
# for the internal load balancers.

generate_string() {
    ns=$1
    ips=$2

    echo "\
    ${ns}.svc.cluster.local:53 {
        errors
        cache 30
        forward . ${ips} {
            force_tcp
        }
    }"
}

namespace_0=${CAMUNDA_NAMESPACE_0:-""}
namespace_1=${CAMUNDA_NAMESPACE_1:-""}


if [ -z "$namespace_0" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 0: " namespace_0
fi

if [ -z "$namespace_1" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 1: " namespace_1
fi

if [ "$namespace_0" = "$namespace_1" ]; then
    echo "Kubernetes namespaces for Camunda installations must be called differently"
    exit 1
fi

HOST_0=$(kubectl --context "$CLUSTER_0" -n kube-system get svc internal-dns-lb -o jsonpath="{.status.loadBalancer.ingress[0].hostname}" | cut -d - -f 1)
HOST_1=$(kubectl --context "$CLUSTER_1" -n kube-system get svc internal-dns-lb -o jsonpath="{.status.loadBalancer.ingress[0].hostname}" | cut -d - -f 1)

IPS_0=$(aws ec2 describe-network-interfaces --region "$REGION_0" --filters Name=description,Values="ELB net/${HOST_0}*" --query  'NetworkInterfaces[*].PrivateIpAddress' --output text --no-cli-pager)
IPS_1=$(aws ec2 describe-network-interfaces --region "$REGION_1" --filters Name=description,Values="ELB net/${HOST_1}*" --query  'NetworkInterfaces[*].PrivateIpAddress' --output text --no-cli-pager)

if [ -z "$IPS_0" ] || [ -z "$IPS_1" ]; then
    echo "Could not retrieve the internal load balancer IPs. Please try again, it may take a while for the IPs to be available. Alternatively check that the correct AWS CLI credentials are set."
    exit 1
fi

# String sanitization
# turn tabs into whitespaces
internal_lb_0=$(echo "$IPS_0" | tr '\t' ' ')
internal_lb_1=$(echo "$IPS_1" | tr '\t' ' ')

config_for_cluster_0=$(generate_string "$namespace_1" "$internal_lb_1" )
config_for_cluster_1=$(generate_string "$namespace_0" "$internal_lb_0")

cat <<EOF
Please copy the following between
### Cluster 0 - Start ### and ### Cluster 0 - End ###
and insert it at the end of your CoreDNS configmap in Cluster 0

kubectl --context $CLUSTER_0 -n kube-system edit configmap coredns

### Cluster 0 - Start ###
$config_for_cluster_0
### Cluster 0 - End ###

Please copy the following between
### Cluster 1 - Start ### and ### Cluster 1 - End ###
and insert it at the end of your CoreDNS configmap in Cluster 1

kubectl --context $CLUSTER_1 -n kube-system edit configmap coredns

### Cluster 1 - Start ###
$config_for_cluster_1
### Cluster 1 - End ###
EOF
