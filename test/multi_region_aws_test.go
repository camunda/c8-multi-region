package multiregionaws

import (
	"fmt"
	"testing"
	"time"

	"multiregiontests/internal/helpers"
	awsHelpers "multiregiontests/internal/helpers/aws"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"
)

const (
	remoteChartSource = "https://helm.camunda.io"
	remoteChartName   = "camunda/camunda-platform"

	resourceDir         = "../aws/2-region"
	terraformDir        = "../aws/2-region/terraform"
	kubeConfigPrimary   = "./kubeconfig-london"
	kubeConfigSecondary = "./kubeconfig-paris"
	k8sManifests        = "../aws/2-region/kubernetes"
)

var remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "8.3.10")
var clusterName = helpers.GetEnv("CLUSTER_NAME", "nightly") // allows supplying random cluster name via GHA
var awsProfile = helpers.GetEnv("AWS_PROFILE", "infex")

var primary helpers.Cluster
var secondary helpers.Cluster

// Terraform Cluster Setup and TearDown

func TestSetupTerraform(t *testing.T) {
	t.Log("[TF SETUP] Applying Terraform config 👋")
	awsHelpers.TestSetupTerraform(t, terraformDir, clusterName, awsProfile)
}

func TestTeardownTerraform(t *testing.T) {
	t.Log("[TF TEARDOWN] Destroying workspace 🖖")
	awsHelpers.TestTeardownTerraform(t, terraformDir, clusterName, awsProfile)
}

// AWS EKS Multi-Region Tests

func Test2RegionAWSEKS(t *testing.T) {
	t.Log("[2 REGION TEST] Running tests for AWS EKS Multi-Region 🚀")

	// For CI run it separately
	// go test --count=1 -v -timeout 120m ./multi_region_aws_test.go -run TestSetupTerraform
	// go test --count=1 -v -timeout 120m ./multi_region_aws_test.go -run Test2RegionAWSEKS
	// go test --count=1 -v -timeout 120m ./multi_region_aws_test.go -run TestTeardownTerraform

	// Pre and Post steps - deactivated for CI
	// setupTerraform(t)
	// defer teardownTerraform(t)

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		// AWS Multi Region Setup
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestClusterReadyCheck", clusterReadyCheck},
		{"TestCrossClusterCommunication", testCrossClusterCommunication},
		{"TestApplyDnsChaining", applyDnsChaining},
		{"TestCoreDNSReload", testCoreDNSReload},
		{"TestCrossClusterCommunicationWithDNS", testCrossClusterCommunicationWithDNS},
		{"TestCreateElasticAWSSecret", createElasticAWSSecret},
		{"TestDeployC8Helm", deployC8Helm},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", deployC8processAndCheck},
		{"TestTeardownAllC8Helm", teardownAllC8Helm},
		{"TestCleanupKubernetes", cleanupKubernetes},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// Single Test functions

func initKubernetesHelpers(t *testing.T) {
	t.Log("[K8S INIT] Initializing Kubernetes helpers 🚀")
	primary = helpers.Cluster{
		Region:           "eu-west-2",
		ClusterName:      fmt.Sprintf("%s-london", clusterName),
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigPrimary, "camunda-primary"),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigPrimary, "kube-system"),
		KubectlFailover:  *k8s.NewKubectlOptions("", kubeConfigPrimary, "camunda-primary-failover"),
	}
	secondary = helpers.Cluster{
		Region:           "eu-west-3",
		ClusterName:      fmt.Sprintf("%s-paris", clusterName),
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigSecondary, "camunda-secondary"),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigSecondary, "kube-system"),
		KubectlFailover:  *k8s.NewKubectlOptions("", kubeConfigSecondary, "camunda-secondary-failover"),
	}

	k8s.CreateNamespace(t, &primary.KubectlNamespace, "camunda-primary")
	k8s.CreateNamespace(t, &primary.KubectlFailover, "camunda-primary-failover")
	k8s.CreateNamespace(t, &secondary.KubectlNamespace, "camunda-secondary")
	k8s.CreateNamespace(t, &secondary.KubectlFailover, "camunda-secondary-failover")
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
	awsHelpers.DNSChaining(t, primary, secondary, k8sManifests)
	awsHelpers.DNSChaining(t, secondary, primary, k8sManifests)
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

func createElasticAWSSecret(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating AWS Secret for Elasticsearch 🚀")

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: terraformDir,
		Vars: map[string]interface{}{
			"cluster_name": clusterName,
			"aws_profile":  awsProfile,
		},
		NoColor: true,
	})

	S3AWSAccessKey := helpers.FetchSensitiveTerraformOutput(t, terraformOptions, "s3_aws_access_key")
	S3AWSSecretAccessKey := helpers.FetchSensitiveTerraformOutput(t, terraformOptions, "s3_aws_secret_access_key")

	kubectlHelpers.RunSensitiveKubectlCommand(t, &primary.KubectlNamespace, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	kubectlHelpers.RunSensitiveKubectlCommand(t, &secondary.KubectlNamespace, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))

	k8s.WaitUntilSecretAvailable(t, &primary.KubectlNamespace, "elasticsearch-env-secret", 5, 15*time.Second)
	k8s.WaitUntilSecretAvailable(t, &secondary.KubectlNamespace, "elasticsearch-env-secret", 5, 15*time.Second)

	secretPrimary := k8s.GetSecret(t, &primary.KubectlNamespace, "elasticsearch-env-secret")
	require.Equal(t, len(secretPrimary.Data), 2)

	secretSecondary := k8s.GetSecret(t, &secondary.KubectlNamespace, "elasticsearch-env-secret")
	require.Equal(t, len(secretSecondary.Data), 2)
}

func deployC8Helm(t *testing.T) {
	t.Log("[C8 HELM] Deploying Camunda Platform Helm Chart 🚀")

	// We have to install both at the same time as otherwise zeebe will not become ready

	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, false, false, false, map[string]string{})

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, false, false, false, false, map[string]string{})

	// Check that all deployments and Statefulsets are available
	// Terratest has no direct function for Statefulsets, therefore defaulting to pods directly

	// 20 times with 15 seconds sleep = 5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-connectors", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-operate", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-tasklist", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 20, 15*time.Second)

	// no functions for Statefulsets yet, fallback to pods
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-elasticsearch-master-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-elasticsearch-master-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)

	// 20 times with 15 seconds sleep = 5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-connectors", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-gateway", 20, 15*time.Second)

	// no functions for Statefulsets yet, fallback to pods
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-elasticsearch-master-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-elasticsearch-master-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
}

func checkC8RunningProperly(t *testing.T) {
	t.Log("[C8 CHECK] Checking if Camunda Platform is running properly 🚦")
	kubectlHelpers.CheckC8RunningProperly(t, primary)
}

func deployC8processAndCheck(t *testing.T) {
	t.Log("[C8 PROCESS] Deploying a process and checking if it's running 🚀")
	kubectlHelpers.DeployC8processAndCheck(t, primary, secondary, resourceDir)
}

func teardownAllC8Helm(t *testing.T) {
	t.Log("[C8 HELM TEARDOWN] Tearing down Camunda Platform Helm Chart 🚀")
	kubectlHelpers.TeardownC8Helm(t, &primary.KubectlNamespace)
	kubectlHelpers.TeardownC8Helm(t, &secondary.KubectlNamespace)
}

func cleanupKubernetes(t *testing.T) {
	t.Log("[K8S CLEANUP] Cleaning up Kubernetes resources 🧹")
	k8s.DeleteNamespace(t, &primary.KubectlNamespace, "camunda-primary")
	k8s.DeleteNamespace(t, &secondary.KubectlNamespace, "camunda-secondary")
	k8s.DeleteNamespace(t, &primary.KubectlFailover, "camunda-primary-failover")
	k8s.DeleteNamespace(t, &secondary.KubectlFailover, "camunda-secondary-failover")

	k8s.RunKubectl(t, &primary.KubectlSystem, "delete", "service", "internal-dns-lb")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "delete", "service", "internal-dns-lb")
}
