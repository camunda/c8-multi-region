package multiregionaws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"

	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"
)

const (
	remoteChartSource = "https://helm.camunda.io"
	remoteChartName   = "camunda/camunda-platform"

	resourceDir         = "./resources/aws/2-region"
	terraformDir        = "./resources/aws/2-region/terraform"
	kubeConfigPrimary   = "./kubeconfig-london"
	kubeConfigSecondary = "./kubeconfig-paris"
	k8sManifests        = "./resources/aws/2-region/kubernetes"
)

var remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "8.3.5")

var primary helpers.Cluster
var secondary helpers.Cluster

// Terraform Cluster Setup and TearDown

func TestSetupTerraform(t *testing.T) {
	t.Log("[TF SETUP] Applying Terraform config 👋")

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
	})

	terraform.InitAndApply(t, terraformOptions)

	t.Log("[TF SETUP] Generating kubeconfig files 📜")

	cmd := exec.Command("aws", "eks", "--region", "eu-west-3", "update-kubeconfig", "--name", "nightly-paris", "--profile", "infex", "--kubeconfig", "kubeconfig-paris")

	_, err := cmd.Output()
	if err != nil {
		t.Fatalf("[TF SETUP] could not run command: %v", err)
		return
	}

	require.FileExists(t, "kubeconfig-paris", "kubeconfig-paris file does not exist")

	cmd2 := exec.Command("aws", "eks", "--region", "eu-west-2", "update-kubeconfig", "--name", "nightly-london", "--profile", "infex", "--kubeconfig", "kubeconfig-london")

	_, err2 := cmd2.Output()
	if err2 != nil {
		t.Fatalf("[TF SETUP] could not run command: %v", err2)
		return
	}

	require.FileExists(t, "kubeconfig-london", "kubeconfig-london file does not exist")
}

func TestTeardownTerraform(t *testing.T) {
	t.Log("[TF TEARDOWN] Destroying workspace 🖖")

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: terraformDir,
		NoColor:      true,
	})
	terraform.Destroy(t, terraformOptions)

	os.Remove("kubeconfig-paris")
	os.Remove("kubeconfig-london")

	require.NoFileExists(t, "kubeconfig-paris", "kubeconfig-paris file still exists")
	require.NoFileExists(t, "kubeconfig-london", "kubeconfig-london file still exists")
}

// AWS EKS Multi-Region Tests

func Test2RegionAWSEKS(t *testing.T) {
	t.Log("[2 REGION TEST] Running tests for AWS EKS Multi-Region 🚀")

	// For CI run it separately
	// go test --count=1 -v -timeout 120m ../test -run TestSetupTerraform
	// go test --count=1 -v -timeout 120m ../test -run Test2RegionAWSEKS
	// go test --count=1 -v -timeout 120m ../test -run TestTeardownTerraform

	// Pre and Post steps - deactivated for CI
	// setupTerraform(t)
	// defer teardownTerraform(t)

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestClusterReadyCheck", clusterReadyCheck},
		{"TestCrossClusterCommunication", testCrossClusterCommunication},
		{"TestApplyDnsChaining", applyDnsChaining},
		{"TestCoreDNSReload", testCoreDNSReload},
		{"TestCrossClusterCommunicationWithDNS", testCrossClusterCommunicationWithDNS},
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
		ClusterName:      "nightly-london",
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigPrimary, "camunda-primary"),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigPrimary, "kube-system"),
	}
	secondary = helpers.Cluster{
		Region:           "eu-west-3",
		ClusterName:      "nightly-paris",
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigSecondary, "camunda-secondary"),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigSecondary, "kube-system"),
	}

	k8s.CreateNamespace(t, &primary.KubectlNamespace, "camunda-primary")
	k8s.CreateNamespace(t, &secondary.KubectlNamespace, "camunda-secondary")
}

func clusterReadyCheck(t *testing.T) {
	t.Log("[CLUSTER CHECK] Checking if clusters are ready 🚦")
	clusterStatusPrimary := helpers.WaitForCluster(primary.Region, primary.ClusterName)
	clusterStatusSecondary := helpers.WaitForCluster(secondary.Region, secondary.ClusterName)

	require.Equal(t, "ACTIVE", clusterStatusPrimary)
	require.Equal(t, "ACTIVE", clusterStatusSecondary)

	nodeGroupStatusPrimary := helpers.WaitForNodeGroup(primary.Region, primary.ClusterName, "services")
	nodeGroupStatusSecondary := helpers.WaitForNodeGroup(secondary.Region, secondary.ClusterName, "services")

	require.Equal(t, "ACTIVE", nodeGroupStatusPrimary)
	require.Equal(t, "ACTIVE", nodeGroupStatusSecondary)
}

func testCrossClusterCommunication(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with IPs 📡")
	helpers.CrossClusterCommunication(t, false, k8sManifests, primary, secondary)
}

func applyDnsChaining(t *testing.T) {
	t.Log("[DNS CHAINING] Applying DNS chaining 📡")
	helpers.DNSChaining(t, primary, secondary, k8sManifests)
	helpers.DNSChaining(t, secondary, primary, k8sManifests)
}

func testCoreDNSReload(t *testing.T) {
	t.Logf("[COREDNS RELOAD] Checking for CoreDNS reload 🔄")
	helpers.CheckCoreDNSReload(t, &primary.KubectlSystem)
	helpers.CheckCoreDNSReload(t, &secondary.KubectlSystem)
}

func testCrossClusterCommunicationWithDNS(t *testing.T) {
	t.Log("[CROSS CLUSTER] Testing cross-cluster communication with DNS 📡")
	helpers.CrossClusterCommunication(t, true, k8sManifests, primary, secondary)
}

func deployC8Helm(t *testing.T) {
	t.Log("[C8 HELM] Deploying Camunda Platform Helm Chart 🚀")
	zeebeContactPoints := ""

	for i := 0; i < 4; i++ {
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.camunda-primary.svc.cluster.local:26502,", strconv.Itoa((i)))
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.camunda-secondary.svc.cluster.local:26502,", strconv.Itoa((i)))
	}

	// Cut the last character "," from the string
	zeebeContactPoints = zeebeContactPoints[:len(zeebeContactPoints)-1]

	filePath := "./resources/aws/2-region/kubernetes/camunda-values.yml"
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("[C8 HELM] Error reading file: %v\n", err)
		return
	}

	// Convert byte slice to string
	fileContent := string(content)

	// Define the template and replacement string
	template := "PLACEHOLDER"

	// Replace the template with the replacement string
	modifiedContent := strings.Replace(fileContent, template, zeebeContactPoints, -1)

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("[C8 HELM] Error writing file: %v\n", err)
		return
	}

	helmOptionsPrimary := &helm.Options{
		KubectlOptions: &primary.KubectlNamespace,
		Version:        remoteChartVersion,
		ValuesFiles:    []string{"./resources/aws/2-region/kubernetes/camunda-values.yml", "./resources/aws/2-region/kubernetes/region0/camunda-values.yml"},
	}

	helmOptionsSecondary := &helm.Options{
		KubectlOptions: &secondary.KubectlNamespace,
		Version:        remoteChartVersion,
		ValuesFiles:    []string{"./resources/aws/2-region/kubernetes/camunda-values.yml", "./resources/aws/2-region/kubernetes/region1/camunda-values.yml"},
	}

	helm.AddRepo(t, helmOptionsPrimary, "camunda", remoteChartSource)
	helm.AddRepo(t, helmOptionsSecondary, "camunda", remoteChartSource)

	helmChart := remoteChartName

	releaseName := "camunda"

	helm.Install(t, helmOptionsPrimary, helmChart, releaseName)
	helm.Install(t, helmOptionsSecondary, helmChart, releaseName)

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

	// Write the old file back to the file - mostly for local development
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		t.Fatalf("[C8 HELM] Error writing file: %v\n", err)
		return
	}
}

func checkC8RunningProperly(t *testing.T) {
	t.Log("[C8 CHECK] Checking if Camunda Platform is running properly 🚦")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 26500)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         tunnel.Endpoint(),
		UsePlaintextConnection: true,
	})
	if err != nil {
		t.Fatalf("[C8 CHECK] Failed to create client: %v", err)
		return
	}

	defer client.Close()

	// Get the topology of the Zeebe cluster
	topology, err := client.NewTopologyCommand().Send(context.Background())
	if err != nil {
		t.Fatalf("[C8 CHECK] Failed to get topology: %v", err)
		return
	}

	require.Equal(t, 8, len(topology.Brokers))

	primaryCount := 0
	secondaryCount := 0

	t.Log("[C8 CHECK] Cluster status:")
	for _, broker := range topology.Brokers {
		if strings.Contains(broker.Host, "camunda-primary") {
			primaryCount++
		} else if strings.Contains(broker.Host, "camunda-secondary") {
			secondaryCount++
		}
		t.Logf("[C8 CHECK] Broker ID: %d, Address: %s, Partitions: %v\n", broker.NodeId, broker.Host, broker.Partitions)
	}

	require.Equal(t, 4, primaryCount)
	require.Equal(t, 4, secondaryCount)
}

func deployC8processAndCheck(t *testing.T) {
	t.Log("[C8 PROCESS] Deploying a process and checking if it's running 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 26500)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	client, err := zbc.NewClient(&zbc.ClientConfig{
		GatewayAddress:         tunnel.Endpoint(),
		UsePlaintextConnection: true,
	})
	if err != nil {
		t.Fatalf("[C8 PROCESS] Failed to create client: %v", err)
		return
	}

	defer client.Close()

	ctx := context.Background()
	response, err := client.NewDeployResourceCommand().AddResourceFile(fmt.Sprintf("%s/single-task.bpmn", resourceDir)).Send(ctx)
	if err != nil {
		t.Fatalf("[C8 PROCESS] %s", err)
		return
	}
	t.Logf("[C8 PROCESS] Created process: %s", response.String())
	require.NotEmpty(t, response.String())
	require.Contains(t, response.String(), "bigVarProcess")

	t.Log("[C8 PROCESS] Starting another Process instance 🚀")
	msg, err := client.NewCreateInstanceCommand().BPMNProcessId("bigVarProcess").LatestVersion().Send(ctx)
	if err != nil {
		t.Fatalf("[C8 PROCESS] %s", err)
		return
	}
	t.Logf("[C8 PROCESS] Created process: %s", msg.String())
	require.NotEmpty(t, msg.String())
	require.Contains(t, msg.String(), "bigVarProcess")

	// check that was exported to ElasticSearch and available via Operate
	helpers.CheckOperateForProcesses(t, primary)
	helpers.CheckOperateForProcesses(t, secondary)
}

func teardownAllC8Helm(t *testing.T) {
	t.Log("[C8 HELM TEARDOWN] Tearing down Camunda Platform Helm Chart 🚀")
	helpers.TeardownC8Helm(t, &primary.KubectlNamespace)
	helpers.TeardownC8Helm(t, &secondary.KubectlNamespace)
}

func cleanupKubernetes(t *testing.T) {
	t.Log("[K8S CLEANUP] Cleaning up Kubernetes resources 🧹")
	k8s.DeleteNamespace(t, &primary.KubectlNamespace, "camunda-primary")
	k8s.DeleteNamespace(t, &secondary.KubectlNamespace, "camunda-secondary")

	k8s.RunKubectl(t, &primary.KubectlSystem, "delete", "service", "internal-dns-lb")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "delete", "service", "internal-dns-lb")
}
