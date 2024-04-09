package test

import (
	"testing"

	"multiregiontests/internal/helpers"
	awsHelpers "multiregiontests/internal/helpers/aws"

	"github.com/gruntwork-io/terratest/modules/k8s"
)

var tfBinary = helpers.GetEnv("TESTS_TF_BINARY_NAME", "tofu")

// Terraform Cluster Setup and TearDown

func TestSetupTerraform(t *testing.T) {
	t.Log("[TF SETUP] Applying Terraform config 👋")
	awsHelpers.TestSetupTerraform(t, terraformDir, clusterName, awsProfile, tfBinary)
}

func TestAWSKubeConfigCreation(t *testing.T) {
	t.Log("[KUBECONFIG] Creating kubeconfig files 🚀")
	awsHelpers.GenerateAWSKubeConfig(t, clusterName, awsProfile, "eu-west-2", "london")
	awsHelpers.GenerateAWSKubeConfig(t, clusterName, awsProfile, "eu-west-3", "paris")
}

func TestTeardownTerraform(t *testing.T) {
	t.Log("[TF TEARDOWN] Destroying workspace 🖖")
	awsHelpers.TestTeardownTerraform(t, terraformDir, clusterName, awsProfile, tfBinary)
}

func TestClusterCleanup(t *testing.T) {
	t.Log("[CLEANUP] Cleaning up resources 🧹")

	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestCleanupKubernetes", cleanupKubernetes},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func cleanupKubernetes(t *testing.T) {
	t.Log("[K8S CLEANUP] Cleaning up Kubernetes resources 🧹")

	k8s.RunKubectl(t, &primary.KubectlSystem, "delete", "service", "internal-dns-lb")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "delete", "service", "internal-dns-lb")
}
