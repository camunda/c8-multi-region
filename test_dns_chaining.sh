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

ping_instance() {
    local context=$1
    local source_namespace=$2
    local target_namespace=$3
    for ((i=1; i<=5; i++))
    do
        echo "Iteration $i - $source_namespace -> $target_namespace"
        output=$(kubectl --context "$context" exec -n "$source_namespace" -it sample-nginx -- curl "http://sample-nginx.sample-nginx-peer.$target_namespace.svc.cluster.local")
        if output=$(echo "$output" | grep "Welcome to nginx!"); then
            echo "Success: $output"
            return
        else
            echo "Try again in 15 seconds..."
            sleep 15
        fi
    done
    echo "Failed to reach the target instance - CoreDNS might not be reloaded yet or wrongly configured"
}

create_namespace "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0"
create_namespace "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1"

kubectl --context "$CLUSTER_0" apply -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/test/resources/aws/2-region/kubernetes/nginx.yml -n "$CAMUNDA_NAMESPACE_0"
kubectl --context "$CLUSTER_1" apply -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/test/resources/aws/2-region/kubernetes/nginx.yml -n "$CAMUNDA_NAMESPACE_1"


kubectl --context "$CLUSTER_0" wait --for=condition=Ready pod/sample-nginx -n "$CAMUNDA_NAMESPACE_0" --timeout=300s
kubectl --context "$CLUSTER_1" wait --for=condition=Ready pod/sample-nginx -n "$CAMUNDA_NAMESPACE_1" --timeout=300s

ping_instance "$CLUSTER_0" "$CAMUNDA_NAMESPACE_0" "$CAMUNDA_NAMESPACE_1"
ping_instance "$CLUSTER_1" "$CAMUNDA_NAMESPACE_1" "$CAMUNDA_NAMESPACE_0"

kubectl --context "$CLUSTER_0" delete -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/test/resources/aws/2-region/kubernetes/nginx.yml -n "$CAMUNDA_NAMESPACE_0"
kubectl --context "$CLUSTER_1" delete -f https://raw.githubusercontent.com/camunda/c8-multi-region/main/test/resources/aws/2-region/kubernetes/nginx.yml -n "$CAMUNDA_NAMESPACE_1"
