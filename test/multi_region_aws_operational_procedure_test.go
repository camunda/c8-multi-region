package multiregionawsoperationalprocedure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"

	"github.com/camunda/zeebe/clients/go/v8/pkg/zbc"
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

var remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "8.3.7")
var clusterName = helpers.GetEnv("CLUSTER_NAME", "nightly") // allows supplying random cluster name via GHA
var backupName = helpers.GetEnv("BACKUP_NAME", "nightly")   // allows supplying random backup name via GHA
var awsProfile = helpers.GetEnv("AWS_PROFILE", "infex")

var primary helpers.Cluster
var secondary helpers.Cluster

// Terraform Cluster Setup and TearDown

func TestSetupTerraform(t *testing.T) {
	t.Log("[TF SETUP] Applying Terraform config 👋")

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: terraformDir,
		Vars: map[string]interface{}{
			"cluster_name": clusterName,
			"aws_profile":  awsProfile,
		},
		NoColor: true,
	})

	terraform.InitAndApply(t, terraformOptions)

	t.Log("[TF SETUP] Generating kubeconfig files 📜")

	cmd := exec.Command("aws", "eks", "--region", "eu-west-3", "update-kubeconfig", "--name", fmt.Sprintf("%s-paris", clusterName), "--profile", awsProfile, "--kubeconfig", "kubeconfig-paris")

	_, err := cmd.Output()
	if err != nil {
		t.Fatalf("[TF SETUP] could not run command: %v", err)
		return
	}

	require.FileExists(t, "kubeconfig-paris", "kubeconfig-paris file does not exist")

	cmd2 := exec.Command("aws", "eks", "--region", "eu-west-2", "update-kubeconfig", "--name", fmt.Sprintf("%s-london", clusterName), "--profile", awsProfile, "--kubeconfig", "kubeconfig-london")

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
		Vars: map[string]interface{}{
			"cluster_name": clusterName,
			"aws_profile":  awsProfile,
		},
		NoColor: true,
	})
	terraform.Destroy(t, terraformOptions)

	os.Remove("kubeconfig-paris")
	os.Remove("kubeconfig-london")

	require.NoFileExists(t, "kubeconfig-paris", "kubeconfig-paris file still exists")
	require.NoFileExists(t, "kubeconfig-london", "kubeconfig-london file still exists")
}

// AWS EKS Multi-Region Tests

func TestAWSOperationalProcedure(t *testing.T) {
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
		{"TestCreateElasticAWSSecret", createElasticAWSSecret},
		{"TestDeployC8Helm", deployC8Helm},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", deployC8processAndCheck},
		{"TestCheckTheMath", checkTheMath},
		{"TestDeleteSecondaryRegion", deleteSecondaryRegion},
		{"TestCreateFailoverDeploymentPrimary", createFailoverDeploymentPrimary},
		{"TestCheckTheMathFailover", checkTheMathFailover},
		{"TestPointPrimaryZeebeToFailver", pointPrimaryZeebeToFailver},
		{"TestRecreateCamundaInSecondary", recreateCamundaInSecondary},
		{"TestStopZeebeExporters", stopZeebeExporters},
		{"TestScaleDownWebApps", scaleDownWebApps},
		{"TestCreateElasticBackupRepoPrimary", createElasticBackupRepoPrimary},
		{"TestCreateElasticBackupPrimary", createElasticBackupPrimary},
		{"TestCheckThatElasticBackupIsPresentPrimary", checkThatElasticBackupIsPresentPrimary},
		{"TestCreateElasticBackupRepoSecondary", createElasticBackupRepoSecondary},
		{"TestCheckThatElasticBackupIsPresentSecondary", checkThatElasticBackupIsPresentSecondary},
		{"TestRestoreElasticBackupSecondary", restoreElasticBackupSecondary},
		{"TestPointC8BackToElastic", pointC8BackToElastic},
		{"TestStartZeebeExporters", startZeebeExporters},
		{"TestScaleUpWebApps", scaleUpWebApps},
		{"TestInstallWebAppsSecondary", installWebAppsSecondary},
		{"TestRemoveFailOverRegion", removeFailOverRegion},
		{"TestRemoveFailBackSecondary", removeFailBackSecondary},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", deployC8processAndCheck},
		{"TestCheckTheMath", checkTheMath},
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

	// We have to install both at the same time as otherwise zeebe will not become ready

	helpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, false, false, false, map[string]string{})

	helpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, false, false, false, false, map[string]string{})

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

	t.Log("[C8 PROCESS] Sleeping shortly to let process be propagated")
	time.Sleep(30 * time.Second)

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

	k8s.DeleteNamespace(t, &primary.KubectlFailover, "camunda-primary-failover")
	k8s.DeleteNamespace(t, &secondary.KubectlFailover, "camunda-secondary-failover")

	k8s.RunKubectl(t, &primary.KubectlSystem, "delete", "service", "internal-dns-lb")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "delete", "service", "internal-dns-lb")
}

// New stuff

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

	helpers.RunSensitiveKubectlCommand(t, &primary.KubectlNamespace, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	helpers.RunSensitiveKubectlCommand(t, &secondary.KubectlNamespace, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	helpers.RunSensitiveKubectlCommand(t, &primary.KubectlFailover, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	helpers.RunSensitiveKubectlCommand(t, &secondary.KubectlFailover, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))

	k8s.WaitUntilSecretAvailable(t, &primary.KubectlNamespace, "elasticsearch-env-secret", 5, 15*time.Second)
	k8s.WaitUntilSecretAvailable(t, &secondary.KubectlNamespace, "elasticsearch-env-secret", 5, 15*time.Second)
	k8s.WaitUntilSecretAvailable(t, &primary.KubectlFailover, "elasticsearch-env-secret", 5, 15*time.Second)
	k8s.WaitUntilSecretAvailable(t, &secondary.KubectlFailover, "elasticsearch-env-secret", 5, 15*time.Second)

	secretPrimary := k8s.GetSecret(t, &primary.KubectlNamespace, "elasticsearch-env-secret")
	require.Equal(t, len(secretPrimary.Data), 2)

	secretSecondary := k8s.GetSecret(t, &secondary.KubectlNamespace, "elasticsearch-env-secret")
	require.Equal(t, len(secretSecondary.Data), 2)

	secretPrimaryFailover := k8s.GetSecret(t, &primary.KubectlFailover, "elasticsearch-env-secret")
	require.Equal(t, len(secretPrimaryFailover.Data), 2)

	secretSecondaryFailover := k8s.GetSecret(t, &secondary.KubectlFailover, "elasticsearch-env-secret")
	require.Equal(t, len(secretSecondaryFailover.Data), 2)
}

// ElasticSearch

func createElasticBackupRepoPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating Elasticsearch Backup Repository 🚀")

	helpers.ConfigureElasticBackup(t, primary, clusterName)
}

func createElasticBackupPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Creating Elasticsearch Backup 🚀")

	helpers.CreateElasticBackup(t, primary, backupName)
}

func checkThatElasticBackupIsPresentPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	helpers.CheckThatElasticBackupIsPresent(t, primary, backupName)
}

func createElasticBackupRepoSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating Elasticsearch Backup Repository 🚀")

	helpers.ConfigureElasticBackup(t, secondary, clusterName)
}

func checkThatElasticBackupIsPresentSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	helpers.CheckThatElasticBackupIsPresent(t, secondary, backupName)
}

func restoreElasticBackupSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Restoring Elasticsearch Backup 🚀")

	helpers.RestoreElasticBackup(t, secondary, backupName)
}

func deleteSecondaryRegion(t *testing.T) {
	t.Log("[REGION REMOVAL] Deleting secondary region 🚀")

	helpers.TeardownC8Helm(t, &secondary.KubectlNamespace)
}

func createFailoverDeploymentPrimary(t *testing.T) {
	t.Log("[FAILOVER] Creating failover deployment 🚀")

	helpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, false, true, false, map[string]string{})

	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 20, 15*time.Second)

	// no functions for Statefulsets yet, fallback to pods
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-elasticsearch-master-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-elasticsearch-master-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-1", 20, 15*time.Second)

}

func pointPrimaryZeebeToFailver(t *testing.T) {
	t.Log("[FAILOVER] Pointing primary Zeebe to failover Elastic 🚀")

	helpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, true, true, false, true, map[string]string{})

	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)

}

func recreateCamundaInSecondary(t *testing.T) {
	t.Log("[C8 HELM] Recreating Camunda Platform Helm Chart in secondary 🚀")

	setValues := map[string]string{
		"global.multiregion.installationType": "failBack",
		"operate.enabled":                     "false",
		"tasklist.enabled":                    "false",
	}

	helpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, false, false, false, true, setValues)

	// expected that only 1 and 3 comes on
	// 0 and 4 should be in not ready state
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
}

func stopZeebeExporters(t *testing.T) {
	t.Log("[ZEEBE EXPORTERS] Stopping Zeebe Exporters 🚀")

	var output string
	var err error

	// Partition distribution may take a while and results in a 500 error
	for i := 0; i < 10; i++ {
		output, err = k8s.RunKubectlAndGetOutputE(t, &primary.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-i", "camunda-zeebe-gateway:9600/actuator/exporting/pause", "-XPOST")
		if err != nil {
			t.Fatalf("[ZEEBE EXPORTERS] Failed to pause exporters: %v", err)
			return
		}

		if strings.Contains(output, "HTTP/1.1 20") {
			t.Log("[ZEEBE EXPORTERS] Output contains a 20x response")
			break
		}
		t.Log("[ZEEBE EXPORTERS] Pausing exporters failed, retrying...")
		time.Sleep(30 * time.Second)
	}

	require.Contains(t, output, "HTTP/1.1 20")
	t.Logf("[ZEEBE EXPORTERS] Paused exporters: %s", output)
}

func startZeebeExporters(t *testing.T) {
	t.Log("[ZEEBE EXPORTERS] Starting Zeebe Exporters 🚀")

	var output string
	var err error

	// Partition distribution may take a while and results in a 500 error
	for i := 0; i < 10; i++ {
		output, err = k8s.RunKubectlAndGetOutputE(t, &primary.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-i", "camunda-zeebe-gateway:9600/actuator/exporting/resume", "-XPOST")
		if err != nil {
			t.Fatalf("[ZEEBE EXPORTERS] Failed to resume exporters: %v", err)
			return
		}

		if strings.Contains(output, "HTTP/1.1 20") {
			t.Log("[ZEEBE EXPORTERS] Output contains a 20x response")
			break
		}
		t.Log("[ZEEBE EXPORTERS] Resuming exporters failed, retrying...")
		time.Sleep(30 * time.Second)
	}

	require.Contains(t, output, "HTTP/1.1 20")
	t.Logf("[ZEEBE EXPORTERS] Resumed exporters: %s", output)
}

func scaleDownWebApps(t *testing.T) {
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-operate", "--replicas=0")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-tasklist", "--replicas=0")

	deploymentOperate := k8s.GetDeployment(t, &primary.KubectlNamespace, "camunda-operate")
	deploymentTasklist := k8s.GetDeployment(t, &primary.KubectlNamespace, "camunda-tasklist")

	require.Equal(t, int32(0), *deploymentOperate.Spec.Replicas)
	require.Equal(t, int32(0), *deploymentTasklist.Spec.Replicas)
}

func scaleUpWebApps(t *testing.T) {
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-operate", "--replicas=1")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-tasklist", "--replicas=1")

	deploymentOperate := k8s.GetDeployment(t, &primary.KubectlNamespace, "camunda-operate")
	deploymentTasklist := k8s.GetDeployment(t, &primary.KubectlNamespace, "camunda-tasklist")

	require.Equal(t, int32(1), *deploymentOperate.Spec.Replicas)
	require.Equal(t, int32(1), *deploymentTasklist.Spec.Replicas)
}

func installWebAppsSecondary(t *testing.T) {
	setValues := map[string]string{
		"global.multiregion.installationType": "failBack",
	}

	helpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, setValues)

	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", 20, 15*time.Second)
}

func pointC8BackToElastic(t *testing.T) {
	setValuesSecondary := map[string]string{
		"global.multiregion.installationType": "failBack",
		"operate.enabled":                     "false",
		"tasklist.enabled":                    "false",
	}

	helpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, true, true, false, false, map[string]string{})

	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-operate", "--replicas=0")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-tasklist", "--replicas=0")

	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)

	require.True(t, helpers.StatefulSetContains(t, &primary.KubectlNamespace, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

	helpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, true, true, true, map[string]string{})

	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-0", 20, 15*time.Second)

	require.True(t, helpers.StatefulSetContains(t, &primary.KubectlFailover, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

	helpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, setValuesSecondary)

	// 2 pods are sleeping indefinitely and block rollout
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "delete", "pod", "camunda-zeebe-0", "camunda-zeebe-1", "camunda-zeebe-2", "camunda-zeebe-3")

	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)

	require.True(t, helpers.StatefulSetContains(t, &secondary.KubectlNamespace, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

}

func removeFailOverRegion(t *testing.T) {
	t.Log("[FAILOVER] Removing failover region 🚀")

	helpers.TeardownC8Helm(t, &primary.KubectlFailover)
}

func removeFailBackSecondary(t *testing.T) {
	t.Log("[FAILOVER] Removing failback flag 🚀")

	helpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, map[string]string{})

	// 2 pods are sleeping indefinitely and block rollout
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "delete", "pod", "camunda-zeebe-0", "camunda-zeebe-1", "camunda-zeebe-2", "camunda-zeebe-3")

	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)
}

func checkTheMath(t *testing.T) {
	t.Log("[MATH] Checking the math 🚀")

	t.Log("[MATH] Checking if the primary deployment has even broker IDs")
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-3")))

	t.Log("[MATH] Checking if the secondary deployment has odd broker IDs")
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-3")))
}

func checkTheMathFailover(t *testing.T) {
	t.Log("[MATH] Checking the math for Failover 🚀")

	t.Log("[MATH] Checking if the primary deployment has even broker IDs")
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsEven(helpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-3")))

	t.Log("[MATH] Checking if the failover deployment has odd broker IDs")
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &primary.KubectlFailover, "camunda-zeebe-0")))
	require.True(t, helpers.IsOdd(helpers.GetZeebeBrokerId(t, &primary.KubectlFailover, "camunda-zeebe-1")))
}
