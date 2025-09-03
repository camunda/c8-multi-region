package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"multiregiontests/internal/helpers"
	awsHelpers "multiregiontests/internal/helpers/aws"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"

	"github.com/gruntwork-io/terratest/modules/shell"
)

// Used for creating the global core dns configmap for all versions
// each is a comma separated string "namespace1,namespace2" value originates from "c8_namespace_parser.sh"
var (
	primaryNamespaceArr           = helpers.GetEnv("CLUSTER_0_NAMESPACE_ARR", "")
	primaryNamespaceFailoverArr   = helpers.GetEnv("CLUSTER_0_NAMESPACE_FAILOVER_ARR", "")
	secondaryNamespaceArr         = helpers.GetEnv("CLUSTER_1_NAMESPACE_ARR", "")
	secondaryNamespaceFailoverArr = helpers.GetEnv("CLUSTER_1_NAMESPACE_FAILOVER_ARR", "")
)

func TestAWSDNSChaining(t *testing.T) {
	t.Log("[DNS CHAINING] Running tests for AWS EKS Multi-Region 🚀")

	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestClusterReadyCheck", clusterReadyCheck},
		// AWS DNS Chaining and cross cluster communication
		{"TestCrossClusterCommunication", testCrossClusterCommunication},
		{"TestApplyDnsChaining", applyDnsChaining},
		{"TestCoreDNSReload", testCoreDNSReload},
		{"TestCrossClusterCommunicationWithDNS", testCrossClusterCommunicationWithDNS},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestClusterPrerequisites(t *testing.T) {
	// Log the appropriate test banner.
	if helpers.IsTeleportEnabled() {
		t.Log("[DNS CHAINING] Running tests for AWS EKS Multi-Region through Teleport access 🚀")
	} else {
		t.Log("[DNS CHAINING] Running tests for AWS EKS Multi-Region 🚀")
	}

	// Initialize Kubernetes helpers.
	t.Run("TestInitKubernetesHelpers", initKubernetesHelpers)

	// Create namespaces and secrets.
	t.Run("TestCreateAllNamespacesAndSecrets", func(t *testing.T) {
		t.Log("[K8S] Creating all namespaces and secrets 🚀")

		// Combine primary and failover namespaces.
		allPrimaryNamespaces := append(
			strings.Split(primaryNamespaceArr, ","),
			strings.Split(primaryNamespaceFailoverArr, ",")...,
		)
		allSecondaryNamespaces := append(
			strings.Split(secondaryNamespaceArr, ","),
			strings.Split(secondaryNamespaceFailoverArr, ",")...,
		)

		// Ensure both arrays have the same length.
		if len(allPrimaryNamespaces) != len(allSecondaryNamespaces) {
			t.Fatal("Primary and secondary namespace arrays must have the same length")
		}

		// Iterate over namespaces and set environment variables appropriately.
		for i := range allPrimaryNamespaces {
			if helpers.IsTeleportEnabled() {
				os.Setenv("KUBECONFIG", "./kubeconfig")
				t.Logf("Primary Namespace: %s, Secondary Namespace: %s", allPrimaryNamespaces[i], allSecondaryNamespaces[i])
			} else {
				os.Setenv("KUBECONFIG", kubeConfigPrimary+":"+kubeConfigSecondary)
				os.Setenv("CLUSTER_0", primary.ClusterName)
				os.Setenv("CAMUNDA_NAMESPACE_0", allPrimaryNamespaces[i])
				os.Setenv("CLUSTER_1", secondary.ClusterName)
				os.Setenv("CAMUNDA_NAMESPACE_1", allSecondaryNamespaces[i])
			}

			shell.RunCommand(t, shell.Command{
				Command: "sh",
				Args: []string{
					"../aws/dual-region/scripts/create_elasticsearch_secrets.sh",
				},
			})
		}
	})

	// Create Storage Class.
	t.Run("TestCreateStorageClass", createStorageClass)
}

func createStorageClass(t *testing.T) {
	t.Log("[STORAGE CLASS] Creating Storage Class for both clusters 🚀")

	if helpers.IsTeleportEnabled() {
		t.Logf("Skipping Storage Class creation when Teleport is enabled")
		return
	}

	wd, _ := os.Getwd()
	os.Setenv("KUBECONFIG",
		filepath.Join(wd, kubeConfigPrimary)+string(os.PathListSeparator)+filepath.Join(wd, kubeConfigSecondary))
	os.Setenv("CLUSTER_0", primary.ClusterName)
	os.Setenv("CLUSTER_1", secondary.ClusterName)

	shell.RunCommand(t, shell.Command{
		Command:    "sh",
		WorkingDir: "../aws/dual-region/scripts",
		Args: []string{
			"./storageclass-configure.sh",
		},
	})

	shell.RunCommand(t, shell.Command{
		Command: "sh",
		Args: []string{
			"../aws/dual-region/scripts/storageclass-verify.sh",
		},
	})
}

func clusterReadyCheck(t *testing.T) {
	t.Log("[CLUSTER CHECK] Checking if clusters are ready 🚦")
	awsHelpers.ClusterReadyCheck(t, primary)
	awsHelpers.ClusterReadyCheck(t, secondary)
}

func testCrossClusterCommunication(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with IPs 📡")
	t.Run("TestInitKubernetesHelpers", initKubernetesHelpers)

	kubectlHelpers.CrossClusterCommunication(t, false, k8sManifests, primary, secondary, kubeConfigPrimary, kubeConfigSecondary)
}

func applyDnsChaining(t *testing.T) {
	t.Log("[DNS CHAINING] Applying DNS chaining 📡")
	awsHelpers.CreateLoadBalancers(t, primary, k8sManifests)
	awsHelpers.CreateLoadBalancers(t, secondary, k8sManifests)
	allPrimaryNamespaces := primaryNamespaceArr + "," + primaryNamespaceFailoverArr
	allSecondaryNamespaces := secondaryNamespaceArr + "," + secondaryNamespaceFailoverArr
	awsHelpers.DNSChaining(t, primary, secondary, k8sManifests, allPrimaryNamespaces, allSecondaryNamespaces)
}

func testCoreDNSReload(t *testing.T) {
	t.Logf("[COREDNS RELOAD] Checking for CoreDNS reload 🔄")
	kubectlHelpers.CheckCoreDNSReload(t, &primary.KubectlSystem)
	kubectlHelpers.CheckCoreDNSReload(t, &secondary.KubectlSystem)
}

func testCrossClusterCommunicationWithDNS(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with DNS 📡")
	t.Run("TestInitKubernetesHelpers", initKubernetesHelpers)
	kubectlHelpers.CrossClusterCommunication(t, false, k8sManifests, primary, secondary, kubeConfigPrimary, kubeConfigSecondary)
}
