#!/bin/bash

set -euo pipefail

SC_NAME="ebs-sc"

# verify_storageclass <kube_context>
# Returns 0 if only $SC_NAME is the default StorageClass in the given context, otherwise 1.
verify_storageclass() {
    local context="$1"
    echo "Checking default StorageClass in '$context'..."

    # Get names of default storageclasses (if any) using jsonpath (no jq dependency)
    local defaults
    defaults=$(kubectl --context "$context" get storageclass -o jsonpath='{range .items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{" "}{end}')
    defaults=${defaults%% } # trim trailing space if present

    if [[ "$defaults" == "$SC_NAME" ]]; then
        echo "OK: '$SC_NAME' is the default StorageClass in '$context'."
        return 0
    fi

    echo "FAIL: Default StorageClass is not set correctly in '$context'."
    echo "Current default(s): ${defaults:-<none>}"
    return 1
}

# Verify both cluster contexts if provided via env vars
rc=0
if [[ -n "${CLUSTER_0:-}" ]]; then
    if ! verify_storageclass "$CLUSTER_0"; then rc=1; fi
else
    echo "WARN: CLUSTER_0 is not set; skipping."
fi

if [[ -n "${CLUSTER_1:-}" ]]; then
    if ! verify_storageclass "$CLUSTER_1"; then rc=1; fi
else
    echo "WARN: CLUSTER_1 is not set; skipping."
fi

exit $rc
