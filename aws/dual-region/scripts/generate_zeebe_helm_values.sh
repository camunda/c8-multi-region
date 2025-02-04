#!/bin/bash

set -ex

generate_initial_contact() {
    # Function to generate the initial contact string for Zeebe clusters
    local ns_0=$1
    local ns_1=$2
    local release=$3
    local count=$4
    local port_number=26502
    local result=""

    # Trim spaces and validate
    count=$(echo "$count" | tr -d '[:space:]')

    if ! [[ "$count" =~ ^[0-9]+$ ]]; then
        echo "Error: count must be a valid integer" >&2
        exit 1
    fi

    count=$((count))  # Force integer conversion
    local half_count=$((count / 2))

    echo "DEBUG: count='$count'"
    echo "DEBUG: half_count='$half_count'"

    # Alternative loop to avoid "Bad for loop variable" issues
    i=0
    while [[ $i -lt $half_count ]]; do
        result+="${release}-zeebe-${i}.${release}-zeebe.${ns_0}.svc.cluster.local:${port_number},"
        result+="${release}-zeebe-${i}.${release}-zeebe.${ns_1}.svc.cluster.local:${port_number},"
        ((i++))
    done
    echo "${result%,}"  # Remove the trailing comma
}

generate_exporter_elasticsearch_url() {
    local ns=$1
    local release=$2
    local port_number=9200
    echo "http://${release}-elasticsearch-master-hl.${ns}.svc.cluster.local:${port_number}"
}

namespace_0=${CAMUNDA_NAMESPACE_0:-""}
namespace_1=${CAMUNDA_NAMESPACE_1:-""}
helm_release_name=${HELM_RELEASE_NAME:-""}
cluster_size=${ZEEBE_CLUSTER_SIZE:-""}

echo "namespace_0: $namespace_0"
echo "namespace_1: $namespace_1"
echo "helm_release_name: $helm_release_name"
echo "cluster_size: $cluster_size"

target_text="in the base Camunda Helm chart values file 'camunda-values.yml'"

# Taking inputs from the user
if [ -z "$namespace_0" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 0: " namespace_0
fi
if [ -z "$namespace_1" ]; then
    read -r -p "Enter the Kubernetes cluster namespace where Camunda 8 is installed, in region 1: " namespace_1
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

if [[ "$namespace_0" == "$namespace_1" ]]; then
    echo "Kubernetes namespaces for Camunda installations must be called differently"
    exit 1
fi

# Generating and printing the string
initial_contact=$(generate_initial_contact "$namespace_0" "$namespace_1" "$helm_release_name" "$cluster_size")
elastic0=$(generate_exporter_elasticsearch_url "$namespace_0" "$helm_release_name")
elastic1=$(generate_exporter_elasticsearch_url "$namespace_1" "$helm_release_name")

echo
echo "Please use the following to change the existing environment variable ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS $target_text. It's part of the 'zeebe.env' path."
echo
echo "- name: ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS"
echo "  value: $initial_contact"
echo
echo "Please use the following to change the existing environment variable ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION0_ARGS_URL $target_text. It's part of the 'zeebe.env' path."
echo
echo "- name: ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION0_ARGS_URL"
echo "  value: $elastic0"
echo
echo "Please use the following to change the existing environment variable ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION1_ARGS_URL $target_text. It's part of the 'zeebe.env' path."
echo
echo "- name: ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION1_ARGS_URL"
echo "  value: $elastic1"
