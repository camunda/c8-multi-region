""""This script generates the initial contact string for Zeebe clusters in a multi-region setup."""

# For now the script is focused on dual-region.
# It will be extended if anything specific changes for more than 2 regions.

def generate_string(ns_0, ns_1, release, count):
    """Generates the initial contact string for Zeebe clusters in a multi-region setup."""
    result = ""
    for i in range(count // 2):
        result += f"{release}-zeebe-{i}.{release}-zeebe.{ns_0}.svc.cluster.local:26502,"
        result += f"{release}-zeebe-{i}.{release}-zeebe.{ns_1}.svc.cluster.local:26502,"
    return result

# Taking inputs from the user
namespace_0 = input("Enter a namespace for cluster in region 0: ")
namespace_1 = input("Enter a namespace for cluster in region 1: ")
helm_release_name = input("Enter Helm release name: ")
cluster_size = int(input("Enter Zeebe cluster size: "))

if cluster_size % 2 != 0:
    raise ValueError(f"Number {cluster_size} is odd and not supported in a multi-region setup.")

if namespace_0 == namespace_1:
    raise ValueError("Namespace for both clusters cannot be the same.")

# Generating and printing the string
output_string = generate_string(namespace_0, namespace_1, helm_release_name, cluster_size)
print(output_string[:-1])
