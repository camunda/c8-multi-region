package test

import (
	"os"
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
	t.Log("[DNS CHAINING] Running tests for AWS EKS Multi-Region 🚀")

	t.Run("TestInitKubernetesHelpers", initKubernetesHelpers)

	t.Run("TestCreateAllNamespacesAndSecrets", func(t *testing.T) {
		t.Log("[K8S] Creating all namespaces and secrets 🚀")

		// Combine primary and failover namespaces
		allPrimaryNamespaces := append(strings.Split(primaryNamespaceArr, ","), strings.Split(primaryNamespaceFailoverArr, ",")...)
		allSecondaryNamespaces := append(strings.Split(secondaryNamespaceArr, ","), strings.Split(secondaryNamespaceFailoverArr, ",")...)

		// Ensure both arrays have the same length
		if len(allPrimaryNamespaces) != len(allSecondaryNamespaces) {
			t.Fatal("Primary and secondary namespace arrays must have the same length")
		}

		// Iterate over namespaces
		for i := range allPrimaryNamespaces {
			os.Setenv("CLUSTER_0", "arn:aws:eks:"+primary.Region+":444804106854:cluster/"+primary.ClusterName)
			os.Setenv("CAMUNDA_NAMESPACE_0", allPrimaryNamespaces[i])
			os.Setenv("CLUSTER_1", "arn:aws:eks:"+secondary.Region+":444804106854:cluster/"+primary.ClusterName)
			os.Setenv("CAMUNDA_NAMESPACE_1", allSecondaryNamespaces[i])
			os.Setenv("KUBECONFIG", kubeConfigPrimary+":"+kubeConfigSecondary)

			shell.RunCommand(t, shell.Command{
				Command: "sh",
				Args: []string{
					"../aws/dual-region/scripts/create_elasticsearch_secrets.sh",
				},
			})
		}
	})
}

func clusterReadyCheck(t *testing.T) {
	t.Log("[CLUSTER CHECK] Checking if clusters are ready 🚦")
	awsHelpers.ClusterReadyCheck(t, primary)
	awsHelpers.ClusterReadyCheck(t, secondary)
}

func testCrossClusterCommunication(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with IPs 📡")
	kubectlHelpers.CrossClusterCommunication(t, false, k8sManifests, primary, secondary)
}

func applyDnsChaining(t *testing.T) {
	t.Log("[DNS CHAINING] Applying DNS chaining 📡")
	awsHelpers.CreateLoadBalancers(t, primary, k8sManifests)
	awsHelpers.CreateLoadBalancers(t, secondary, k8sManifests)
	awsHelpers.DNSChaining(t, primary, secondary, k8sManifests, primaryNamespaceArr, secondaryNamespaceArr)
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
