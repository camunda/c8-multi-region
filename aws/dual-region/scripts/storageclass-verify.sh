#!/bin/bash

set -e

SC_NAME="ebs-sc"

DEFAULTS_0=$(kubectl --context "$CLUSTER_0" get storageclass -o json | jq -r '.items[] | select(.metadata.annotations."storageclass.kubernetes.io/is-default-class"=="true") | .metadata.name')

DEFAULTS_1=$(kubectl --context "$CLUSTER_1" get storageclass -o json | jq -r '.items[] | select(.metadata.annotations."storageclass.kubernetes.io/is-default-class"=="true") | .metadata.name')

if [ "$DEFAULTS_0" = "$SC_NAME" ] && [ "$DEFAULTS_1" = "$SC_NAME" ]; then
    echo "OK: Only '$SC_NAME' is the default StorageClass."
    exit 0
fi

echo "FAIL: Default StorageClass is not set correctly."
echo "Current default(s): $DEFAULTS_0 , $DEFAULTS_1"
exit 1
