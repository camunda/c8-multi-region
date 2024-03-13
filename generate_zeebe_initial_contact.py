""""
    This script generates the initial contact string for Zeebe clusters in a
    multi-region setup. The initial contact string is used to configure all
    Zeebe brokers in a cluster to know about each other for the initial cluster
    formation. The output of this script should be used to define the
    ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS environment variable in the base
    Camunda Helm chart values file.
"""

# For now the script is focused on dual-region.
# It will be extended if anything specific changes for more than 2 regions.

def generate_string(ns_0: str, ns_1: str, release: str, count: int):
    """Generates the initial contact string for Zeebe clusters in a multi-region setup."""
    port_number = 26502
    result = ""
    for i in range(count // 2):
        result += f"{release}-zeebe-{i}.{release}-zeebe.{ns_0}.svc.cluster.local:{port_number},"
        result += f"{release}-zeebe-{i}.{release}-zeebe.{ns_1}.svc.cluster.local:{port_number},"
    return result[:-1]

# Taking inputs from the user
namespace_0 = input("Enter the Kubernetes cluster namespace " +
                    "where Camunda 8 is installed, in region 0: ")
namespace_1 = input("Enter the Kubernetes cluster namespace " +
                    "where Camunda 8 is installed, in region 1: ")
helm_release_name = input("Enter Helm release name used for " +
                          "installing Camunda 8 in both Kubernetes clusters: ")
cluster_size = int(input("Enter Zeebe cluster size (total number of " +
                         "Zeebe brokers in both Kubernetes clusters): "))

if cluster_size % 2 != 0:
    raise ValueError(f"Cluster size {cluster_size} is an odd number " +
                     "and not supported in a multi-region setup (must be an even number)")

if cluster_size < 4:
    raise ValueError(f"Cluster size {cluster_size} is too small and should be at least 4. " +
                    "A multi-region setup is not recommended for a small cluster size.")

if namespace_0 == namespace_1:
    raise ValueError("Kubernetes namespaces for Camunda installations must be called differently")

# Generating and printing the string
output_string = generate_string(namespace_0, namespace_1, helm_release_name, cluster_size)

print()
print("Please use the following to set the environment variable " +
      "ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS in the base Camunda Helm chart values file.")
print()
print("- name: ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS")
print(f"  value: {output_string}")
