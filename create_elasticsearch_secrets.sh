#!/bin/bash

create_namespace() {
    local context=$1
    local namespace=$2
    if kubectl --context "$context" get namespace "$namespace" &> /dev/null; then
        echo "Namespace $namespace already exists."
    else
        # Create the namespace
        kubectl --context "$context" create namespace "$namespace"
    fi
}

create_secret() {
    local context=$1
    local namespace=$2
    local secret_name=$3
    local access_key=$4
    local secret_access_key=$5
    kubectl --context "$context" -n "$namespace" create secret generic "$secret_name" \
        --from-literal=S3_ACCESS_KEY="$access_key" \
        --from-literal=S3_SECRET_KEY="$secret_access_key"
}

if [ -z "$ACCESS_KEY" ]; then
    echo "Error: ACCESS_KEY environment variable is not set."
    exit 1
fi

if [ -z "$SECRET_ACCESS_KEY" ]; then
    echo "Error: SECRET_ACCESS_KEY environment variable is not set."
    exit 1
fi

create_namespace "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0"
create_namespace "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0_FAILOVER"
create_namespace "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1"
create_namespace "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1_FAILOVER"

create_secret "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0" "elasticsearch-env-secret" "$ACCESS_KEY" "$SECRET_ACCESS_KEY"
create_secret "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0_FAILOVER" "elasticsearch-env-secret" "$ACCESS_KEY" "$SECRET_ACCESS_KEY"
create_secret "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1" "elasticsearch-env-secret" "$ACCESS_KEY" "$SECRET_ACCESS_KEY"
create_secret "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1_FAILOVER" "elasticsearch-env-secret" "$ACCESS_KEY" "$SECRET_ACCESS_KEY"
