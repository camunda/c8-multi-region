#!/bin/bash

set -e

generate_initial_contact() {
    # Function to generate the initial contact string for Zeebe clusters
    local ns_0=$1
    local ns_1=$2
    local ns_2=$3
    local release=$4
    local count=$5
    local regions=$6
    local port_number=26502
    local result=""

    if [ "$regions" == "3" ]; then
        for ((i=0; i<count/3; i++)); do
            result+="${release}-zeebe-${i}.${release}-zeebe.${ns_0}.svc.cluster.local:${port_number},"
            result+="${release}-zeebe-${i}.${release}-zeebe.${ns_1}.svc.cluster.local:${port_number},"
            result+="${release}-zeebe-${i}.${release}-zeebe.${ns_2}.svc.cluster.local:${port_number},"
        done
        echo "${result%,}"  # Remove the trailing comma
    else
        for ((i=0; i<count/2; i++)); do
            result+="${release}-zeebe-${i}.${release}-zeebe.${ns_0}.svc.cluster.local:${port_number},"
            result+="${release}-zeebe-${i}.${release}-zeebe.${ns_1}.svc.cluster.local:${port_number},"
        done
        echo "${result%,}"  # Remove the trailing comma
    fi
}

namespace_0=${CAMUNDA_NAMESPACE_0:-""}
namespace_1=${CAMUNDA_NAMESPACE_1:-""}
namespace_2=${CAMUNDA_NAMESPACE_2:-""}
helm_release_name=${HELM_RELEASE_NAME:-""}
regions=${REGIONS:-""}
cluster_size=${CLUSTER_SIZE:-""}

# Taking inputs from the user
if [ -z "$regions" ]; then
    read -r -p "Enter amount of Regions you will deploy Camunda to (2 || 3) " regions
fi
if [ -z "$namespace_0" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 0: " namespace_0
fi
if [ -z "$namespace_1" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 1: " namespace_1
fi
if [ -z "$namespace_2" ] && [ "$regions" == "3" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 3: " namespace_2
fi
if [ -z "$helm_release_name" ]; then
    read -r -p "Enter Helm release name used for installing Camunda 8 in both Kubernetes clusters: " helm_release_name
fi

if [ -z "$cluster_size" ]; then
    read -r -p "Enter Zeebe cluster size (total number of Zeebe brokers in both Kubernetes clusters): " cluster_size
fi

if ((cluster_size % 2 != 0)); then
    echo "Cluster size $cluster_size is an odd number and not supported in a multi-region setup (must be an even number)"
    exit 1
fi

if ((cluster_size < 4)); then
    echo "Cluster size $cluster_size is too small and should be at least 4. A multi-region setup is not recommended for a small cluster size."
    exit 1
fi

if ((cluster_size % regions != 0)); then
    echo "Cluster size $cluster_size is not divisible by the number of regions $regions"
    exit 1
fi

if [[ "$namespace_0" == "$namespace_1" ]] || [[ "$namespace_0" == "$namespace_2" ]] || [[ "$namespace_1" == "$namespace_2" ]]; then
    echo "Kubernetes namespaces for Camunda installations must be called differently"
    exit 1
fi

# Generating and printing the string
initial_contact=$(generate_initial_contact "$namespace_0" "$namespace_1" "$namespace_2" "$helm_release_name" "$cluster_size" "$regions")
echo "$initial_contact"
