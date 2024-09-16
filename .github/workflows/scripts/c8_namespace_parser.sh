#!/bin/bash

# Check if a file path is provided as an argument
if [ $# -eq 0 ]; then
    echo "Usage: $0 <file>"
    exit 1
fi

# Check if the file exists
if [ ! -f "$1" ]; then
    echo "File $1 not found."
    exit 1
fi

# For new versions bump -A argument by 1
# It greps the c8-version and the next x lines
versions=$(grep 'c8-version:' -A 5 "$1" | awk '/c8-version:/ {flag=1; next} flag {print $2}')

variables=("CLUSTER_0_NAMESPACE" "CLUSTER_1_NAMESPACE" "CLUSTER_0_NAMESPACE_FAILOVER" "CLUSTER_1_NAMESPACE_FAILOVER")

# Loop through each variable and print its name and all values found
for var in "${variables[@]}"; do
    echo "Variable: $var"

    namespace_suffix=""

    if [ "$var" == "CLUSTER_0_NAMESPACE" ]; then
        namespace_suffix="-cluster-0"
    elif [ "$var" == "CLUSTER_1_NAMESPACE" ]; then
        namespace_suffix="-cluster-1"
    elif [ "$var" == "CLUSTER_0_NAMESPACE_FAILOVER" ]; then
        namespace_suffix="-cluster-0-failover"
    elif [ "$var" == "CLUSTER_1_NAMESPACE_FAILOVER" ]; then
        namespace_suffix="-cluster-1-failover"
    fi

    namespaces=""
    version_regex="[0-9]+\.[0-9]+\.[0-9]+|SNAPSHOT"

    while read -r version; do
        # Ignore strings that do not match the version regex
        if ! [[ "$version" =~ $version_regex ]]; then
            continue
        fi

        if [ "$version" == "SNAPSHOT" ]; then
            version="snapshot"
        fi

        if [ "$version" == "SNAPSHOT-NEW" ]; then
            version="snapshot-new"
        fi

        version_with_hyphens="${version//./-}"
        namespaces+="${version_with_hyphens}${namespace_suffix},"
    done <<< "$versions"

    namespaces="${namespaces%?}"
    echo "${var}_ARR=$namespaces" >> "$GITHUB_ENV"

done
