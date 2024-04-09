package test

import (
	"testing"

	"multiregiontests/internal/helpers"
	awsHelpers "multiregiontests/internal/helpers/aws"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"
)

// Used for creating the global core dns configmap for all versions
// each is a comma separated string "namespace1,namespace2" value originates from "c8_namespace_parser.sh"
var primaryNamespaceArr = helpers.GetEnv("CLUSTER_0_NAMESPACE_ARR", "")
var primaryNamespaceFailoverArr = helpers.GetEnv("CLUSTER_0_NAMESPACE_FAILOVER_ARR", "")
var secondaryNamespaceArr = helpers.GetEnv("CLUSTER_1_NAMESPACE_ARR", "")
var secondaryNamespaceFailoverArr = helpers.GetEnv("CLUSTER_1_NAMESPACE_FAILOVER_ARR", "")

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
	t.Log("[DNS CHAINING] Running tests for AWS EKS Multi-Region 🚀")

	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestCreateAllNamespaces", testCreateAllNamespaces},
		{"TestCreateAllRequiredSecrets", testCreateAllRequiredSecrets},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func clusterReadyCheck(t *testing.T) {
	t.Log("[CLUSTER CHECK] Checking if clusters are ready 🚦")
	awsHelpers.ClusterReadyCheck(t, primary, secondary)
}

func testCrossClusterCommunication(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with IPs 📡")
	kubectlHelpers.CrossClusterCommunication(t, false, k8sManifests, primary, secondary)
}

func applyDnsChaining(t *testing.T) {
	t.Log("[DNS CHAINING] Applying DNS chaining 📡")
	awsHelpers.DNSChaining(t, primary, secondary, k8sManifests, primaryNamespaceArr, primaryNamespaceFailoverArr)
	awsHelpers.DNSChaining(t, secondary, primary, k8sManifests, secondaryNamespaceArr, secondaryNamespaceFailoverArr)
}

func testCoreDNSReload(t *testing.T) {
	t.Logf("[COREDNS RELOAD] Checking for CoreDNS reload 🔄")
	kubectlHelpers.CheckCoreDNSReload(t, &primary.KubectlSystem)
	kubectlHelpers.CheckCoreDNSReload(t, &secondary.KubectlSystem)
}

func testCrossClusterCommunicationWithDNS(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with DNS 📡")
	kubectlHelpers.CrossClusterCommunication(t, true, k8sManifests, primary, secondary)
}

func testCreateAllNamespaces(t *testing.T) {
	t.Log("[K8S] Creating all namespaces 🚀")
	kubectlHelpers.CreateAllNamespaces(t, primary, primaryNamespaceArr, primaryNamespaceFailoverArr)
	kubectlHelpers.CreateAllNamespaces(t, secondary, secondaryNamespaceArr, secondaryNamespaceFailoverArr)
}

func testCreateAllRequiredSecrets(t *testing.T) {
	t.Log("[K8S] Creating all required secrets 🚀")
	kubectlHelpers.CreateAllRequiredSecrets(t, primary, primaryNamespaceArr, primaryNamespaceFailoverArr)
	kubectlHelpers.CreateAllRequiredSecrets(t, secondary, secondaryNamespaceArr, secondaryNamespaceFailoverArr)
}
