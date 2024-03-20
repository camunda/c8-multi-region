package multiregionawsoperationalprocedure

import (
	"fmt"
	"strings"
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

	resourceDir         = "../aws/dual-region"
	terraformDir        = "../aws/dual-region/terraform"
	kubeConfigPrimary   = "./kubeconfig-london"
	kubeConfigSecondary = "./kubeconfig-paris"
	k8sManifests        = "../aws/dual-region/kubernetes"
)

var remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "8.3.10")
var clusterName = helpers.GetEnv("CLUSTER_NAME", "nightly") // allows supplying random cluster name via GHA
var backupName = helpers.GetEnv("BACKUP_NAME", "nightly")   // allows supplying random backup name via GHA
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

func TestAWSOperationalProcedure(t *testing.T) {
	t.Log("[2 REGION TEST] Running tests for AWS EKS Multi-Region 🚀")

	// For CI run it separately
	// go test --count=1 -v -timeout 120m ./multi_region_aws_operational_procedure_test.go -run TestSetupTerraform
	// go test --count=1 -v -timeout 120m ./multi_region_aws_operational_procedure_test.go -run TestAWSOperationalProcedure
	// go test --count=1 -v -timeout 120m ./multi_region_aws_operational_procedure_test.go -run TestTeardownTerraform

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
		// Multi-Region Operational Procedure
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

	k8s.RunKubectl(t, &primary.KubectlSystem, "wait", "--for=delete", "namespace/camunda-primary", "--timeout=300s")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "wait", "--for=delete", "namespace/camunda-secondary", "--timeout=300s")

	k8s.RunKubectl(t, &primary.KubectlSystem, "wait", "--for=delete", "namespace/camunda-primary-failover", "--timeout=300s")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "wait", "--for=delete", "namespace/camunda-secondary-failover", "--timeout=300s")

	k8s.RunKubectl(t, &primary.KubectlSystem, "delete", "service", "internal-dns-lb")
	k8s.RunKubectl(t, &secondary.KubectlSystem, "delete", "service", "internal-dns-lb")
}

// Multi-Region Operational Procedure Additions

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
	kubectlHelpers.RunSensitiveKubectlCommand(t, &primary.KubectlFailover, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	kubectlHelpers.RunSensitiveKubectlCommand(t, &secondary.KubectlFailover, "create", "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))

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

	kubectlHelpers.ConfigureElasticBackup(t, primary, clusterName)
}

func createElasticBackupPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Creating Elasticsearch Backup 🚀")

	kubectlHelpers.CreateElasticBackup(t, primary, backupName)
}

func checkThatElasticBackupIsPresentPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	kubectlHelpers.CheckThatElasticBackupIsPresent(t, primary, backupName)
}

func createElasticBackupRepoSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating Elasticsearch Backup Repository 🚀")

	kubectlHelpers.ConfigureElasticBackup(t, secondary, clusterName)
}

func checkThatElasticBackupIsPresentSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	kubectlHelpers.CheckThatElasticBackupIsPresent(t, secondary, backupName)
}

func restoreElasticBackupSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Restoring Elasticsearch Backup 🚀")

	kubectlHelpers.RestoreElasticBackup(t, secondary, backupName)
}

func deleteSecondaryRegion(t *testing.T) {
	t.Log("[REGION REMOVAL] Deleting secondary region 🚀")

	kubectlHelpers.TeardownC8Helm(t, &secondary.KubectlNamespace)
}

func createFailoverDeploymentPrimary(t *testing.T) {
	t.Log("[FAILOVER] Creating failover deployment 🚀")

	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, false, true, false, map[string]string{})

	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 20, 15*time.Second)

	// no functions for Statefulsets yet, fallback to pods
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-elasticsearch-master-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-elasticsearch-master-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-0", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-1", 20, 15*time.Second)

}

func pointPrimaryZeebeToFailver(t *testing.T) {
	t.Log("[FAILOVER] Pointing primary Zeebe to failover Elastic 🚀")

	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, true, true, false, true, map[string]string{})

	// Give it a short time start doing the rollout, otherwise it will send data to the recreated elasticsearch
	time.Sleep(15 * time.Second)

	// waiting explicitly for the rollout as the waitUntil can be flaky
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")
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

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, false, false, false, true, setValues)

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

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, setValues)

	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", 20, 15*time.Second)
}

func pointC8BackToElastic(t *testing.T) {
	setValuesSecondary := map[string]string{
		"global.multiregion.installationType": "failBack",
		"operate.enabled":                     "false",
		"tasklist.enabled":                    "false",
	}

	// primary region pointing back to secondary
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 0, true, true, false, false, map[string]string{})

	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-operate", "--replicas=0")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-tasklist", "--replicas=0")

	// waiting explicitly for the rollout as the waitUntil can be flaky
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")

	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-2", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-0", 20, 15*time.Second)

	require.True(t, kubectlHelpers.StatefulSetContains(t, &primary.KubectlNamespace, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

	// failover pointing back to secondary
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, 0, false, true, true, true, map[string]string{})

	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-1", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlFailover, "camunda-zeebe-0", 20, 15*time.Second)

	require.True(t, kubectlHelpers.StatefulSetContains(t, &primary.KubectlFailover, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

	// secondary pointint back to secondary
	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, setValuesSecondary)

	// 2 pods are sleeping indefinitely and block rollout
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "delete", "pod", "camunda-zeebe-0", "camunda-zeebe-1", "camunda-zeebe-2", "camunda-zeebe-3")

	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)

	require.True(t, kubectlHelpers.StatefulSetContains(t, &secondary.KubectlNamespace, "camunda-zeebe", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200"))

}

func removeFailOverRegion(t *testing.T) {
	t.Log("[FAILOVER] Removing failover region 🚀")

	kubectlHelpers.TeardownC8Helm(t, &primary.KubectlFailover)
}

func removeFailBackSecondary(t *testing.T) {
	t.Log("[FAILOVER] Removing failback flag 🚀")

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, 1, true, true, false, false, map[string]string{})

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
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-3")))

	t.Log("[MATH] Checking if the secondary deployment has odd broker IDs")
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &secondary.KubectlNamespace, "camunda-zeebe-3")))
}

func checkTheMathFailover(t *testing.T) {
	t.Log("[MATH] Checking the math for Failover 🚀")

	t.Log("[MATH] Checking if the primary deployment has even broker IDs")
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-3")))

	t.Log("[MATH] Checking if the failover deployment has odd broker IDs")
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlFailover, "camunda-zeebe-0")))
	require.True(t, helpers.IsOdd(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlFailover, "camunda-zeebe-1")))
}
