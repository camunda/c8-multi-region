#!/bin/bash

# Resolve path to the storage class YAML relative to this script, so it works from any CWD.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SC_YAML="${SCRIPT_DIR}/../kubernetes/storage-class.yml"

if [[ ! -f "$SC_YAML" ]]; then
  echo "ERROR: StorageClass manifest not found at '$SC_YAML'" >&2
  exit 1
fi

kubectl --context "$CLUSTER_0" patch storageclass gp2 \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
kubectl --context "$CLUSTER_1" patch storageclass gp2 \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'

kubectl --context "$CLUSTER_0" apply -f "$SC_YAML"
kubectl --context "$CLUSTER_1" apply -f "$SC_YAML"
