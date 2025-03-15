package test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
)

const (
	remoteChartSource = "https://helm.camunda.io"

	resourceDir         = "../aws/dual-region"
	terraformDir        = "../aws/dual-region/terraform"
	kubeConfigPrimary   = "./kubeconfig-london"
	kubeConfigSecondary = "./kubeconfig-paris"
	k8sManifests        = "../aws/dual-region/kubernetes"
)

var (
	// renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io versioning=regex:^9(\.(?<minor>\d+))?(\.(?<patch>\d+))?$
	remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "9.4.8")
	remoteChartName    = helpers.GetEnv("HELM_CHART_NAME", "camunda/camunda-platform") // allows using OCI registries
	globalImageTag     = helpers.GetEnv("GLOBAL_IMAGE_TAG", "")                        // allows overwriting the image tag via GHA of every Camunda image
	clusterName        = helpers.GetEnv("CLUSTER_NAME", "nightly")                     // allows supplying random cluster name via GHA
	backupName         = helpers.GetEnv("BACKUP_NAME", "nightly")                      // allows supplying random backup name via GHA
	awsProfile         = helpers.GetEnv("AWS_PROFILE", "infex")

	primary   helpers.Cluster
	secondary helpers.Cluster

	// Allows setting namespaces via GHA
	primaryNamespace           = helpers.GetEnv("CLUSTER_0_NAMESPACE", "c8-snap-cluster-0")
	primaryNamespaceFailover   = helpers.GetEnv("CLUSTER_0_NAMESPACE_FAILOVER", "c8-snap-cluster-0-failover")
	secondaryNamespace         = helpers.GetEnv("CLUSTER_1_NAMESPACE", "c8-snap-cluster-1")
	secondaryNamespaceFailover = helpers.GetEnv("CLUSTER_1_NAMESPACE_FAILOVER", "c8-snap-cluster-1-failover")

	baseHelmVars = map[string]string{}
)

// AWS EKS Multi-Region Tests

func TestAWSDeployDualRegCamunda(t *testing.T) {
	t.Log("[2 REGION TEST] Deploy Camunda 8 in multi region mode 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	parts := strings.Split(remoteChartVersion, ".")

	// Convert the first part to integer
	firstValue, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Println("Error parsing first value:", err)
		return
	}

	// Check if the first value is smaller than 10 or not snapshot (0)
	if firstValue < 10 && firstValue != 0 {
		t.Log("[C8 VERSION] Detected <10 release, requiring Bitnami adjustment")
		baseHelmVars["elasticsearch.extraVolumes[0].name"] = "empty-dir"
		baseHelmVars["elasticsearch.extraVolumes[0].emptyDir.medium"] = "Memory"
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		// Camunda 8 Deployment
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestDeployC8Helm", deployC8Helm},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", deployC8processAndCheck},
		{"TestCheckTheMath", checkTheMath},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestAWSDualRegFailover_8_6_below(t *testing.T) {
	t.Log("[2 REGION TEST] Checking Failover procedure 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	parts := strings.Split(remoteChartVersion, ".")

	// Convert the first part to integer
	firstValue, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Println("Error parsing first value:", err)
		return
	}

	// Check if the first value is smaller than 10 or not snapshot (0)
	if firstValue < 10 && firstValue != 0 {
		t.Log("[C8 VERSION] Detected <10 release, requiring Bitnami adjustment")
		baseHelmVars["elasticsearch.extraVolumes[0].name"] = "empty-dir"
		baseHelmVars["elasticsearch.extraVolumes[0].emptyDir.medium"] = "Memory"
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		// Multi-Region Operational Procedure
		// Failover
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestDeleteSecondaryRegion", deleteSecondaryRegion},
		{"TestCreateFailoverDeploymentPrimary", createFailoverDeploymentPrimary},
		{"TestCheckTheMathFailover", checkTheMathFailover},
		{"TestPointPrimaryZeebeToFailver", pointPrimaryZeebeToFailver},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestAWSDualRegFailback_8_6_below(t *testing.T) {
	t.Log("[2 REGION TEST] Running tests for AWS EKS Multi-Region 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	parts := strings.Split(remoteChartVersion, ".")

	// Convert the first part to integer
	firstValue, err := strconv.Atoi(parts[0])
	if err != nil {
		fmt.Println("Error parsing first value:", err)
		return
	}

	// Check if the first value is smaller than 10 or not snapshot (0)
	if firstValue < 10 && firstValue != 0 {
		t.Log("[C8 VERSION] Detected <10 release, requiring Bitnami adjustment")
		baseHelmVars["elasticsearch.extraVolumes[0].name"] = "empty-dir"
		baseHelmVars["elasticsearch.extraVolumes[0].emptyDir.medium"] = "Memory"
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		// Multi-Region Operational Procedure
		// Failback
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
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
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestDebugStep(t *testing.T) {
	t.Log("[DEBUG] Debugging step 🚀")

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestDebugStep", debugStep},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestAWSDualRegCleanup(t *testing.T) {
	t.Log("[2 REGION TEST] Cleaning up the environment 🚀")

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestTeardownAllC8Helm", teardownAllC8Helm},
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
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigPrimary, primaryNamespace),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigPrimary, "kube-system"),
		KubectlFailover:  *k8s.NewKubectlOptions("", kubeConfigPrimary, primaryNamespaceFailover),
	}
	secondary = helpers.Cluster{
		Region:           "eu-west-3",
		ClusterName:      fmt.Sprintf("%s-paris", clusterName),
		KubectlNamespace: *k8s.NewKubectlOptions("", kubeConfigSecondary, secondaryNamespace),
		KubectlSystem:    *k8s.NewKubectlOptions("", kubeConfigSecondary, "kube-system"),
		KubectlFailover:  *k8s.NewKubectlOptions("", kubeConfigSecondary, secondaryNamespaceFailover),
	}
}

func deployC8Helm(t *testing.T) {
	t.Log("[C8 HELM] Deploying Camunda Platform Helm Chart 🚀")

	// We have to install both at the same time as otherwise zeebe will not become ready
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, false, false, false, baseHelmVars)

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, false, false, false, baseHelmVars)

	// Check that all deployments and Statefulsets are available
	// Terratest has no direct function for Statefulsets, therefore defaulting to pods directly

	// Elastic itself takes already ~2+ minutes to start
	// 30 times with 15 seconds sleep = 7,5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-operate", 30, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-tasklist", 30, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 30, 15*time.Second)

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")

	// 30 times with 15 seconds sleep = 7,5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", 30, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", 30, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-gateway", 30, 15*time.Second)

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")
}

func checkC8RunningProperly(t *testing.T) {
	t.Log("[C8 CHECK] Checking if Camunda Platform is running properly 🚦")
	kubectlHelpers.CheckC8RunningProperly(t, primary, primaryNamespace, secondaryNamespace)
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

func debugStep(t *testing.T) {
	t.Log("[DEBUG] Debugging step 🚀")

	t.Log("[DEBUG] Running kubectl get pods")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "get", "pods")
	k8s.RunKubectl(t, &primary.KubectlFailover, "get", "pods")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "get", "pods")
	k8s.RunKubectl(t, &secondary.KubectlFailover, "get", "pods")

	t.Log("[DEBUG] Running kubectl describe pods")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "describe", "pods")
	k8s.RunKubectl(t, &primary.KubectlFailover, "describe", "pods")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "describe", "pods")
	k8s.RunKubectl(t, &secondary.KubectlFailover, "describe", "pods")

	t.Log("[DEBUG] Running kubectl describe configmaps")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "describe", "configmaps")
	k8s.RunKubectl(t, &primary.KubectlFailover, "describe", "configmaps")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "describe", "configmaps")
	k8s.RunKubectl(t, &secondary.KubectlFailover, "describe", "configmaps")

	kubectlHelpers.DumpAllPodLogs(t, &primary.KubectlNamespace)
	kubectlHelpers.DumpAllPodLogs(t, &primary.KubectlFailover)
	kubectlHelpers.DumpAllPodLogs(t, &secondary.KubectlNamespace)
	kubectlHelpers.DumpAllPodLogs(t, &secondary.KubectlFailover)
}

// Multi-Region Operational Procedure Additions

// ElasticSearch

func createElasticBackupRepoPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating Elasticsearch Backup Repository 🚀")

	kubectlHelpers.ConfigureElasticBackup(t, primary, clusterName, remoteChartVersion)
}

func createElasticBackupPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Creating Elasticsearch Backup 🚀")

	kubectlHelpers.CreateElasticBackup(t, primary, backupName)
}

func checkThatElasticBackupIsPresentPrimary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	kubectlHelpers.CheckThatElasticBackupIsPresent(t, primary, backupName, clusterName, remoteChartVersion)
}

func createElasticBackupRepoSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH] Creating Elasticsearch Backup Repository 🚀")

	kubectlHelpers.ConfigureElasticBackup(t, secondary, clusterName, remoteChartVersion)
}

func checkThatElasticBackupIsPresentSecondary(t *testing.T) {
	t.Log("[ELASTICSEARCH BACKUP] Checking if Elasticsearch Backup is present 🚀")

	kubectlHelpers.CheckThatElasticBackupIsPresent(t, secondary, backupName, clusterName, remoteChartVersion)
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

	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, false, true, false, baseHelmVars)

	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlFailover, "camunda-zeebe-gateway", 20, 15*time.Second)

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &primary.KubectlFailover, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &primary.KubectlFailover, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")
}

func pointPrimaryZeebeToFailver(t *testing.T) {
	t.Log("[FAILOVER] Pointing primary Zeebe to failover Elastic 🚀")

	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, true, false, true, baseHelmVars)

	// Give it a short time start doing the rollout, otherwise it will send data to the recreated elasticsearch
	time.Sleep(15 * time.Second)

	// waiting explicitly for the rollout as the waitUntil can be flaky
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")
}

func recreateCamundaInSecondary(t *testing.T) {
	t.Log("[C8 HELM] Recreating Camunda Platform Helm Chart in secondary 🚀")

	setValues := map[string]string{
		"global.multiregion.installationType": "failBack",
		"operate.enabled":                     "false",
		"tasklist.enabled":                    "false",
	}

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, false, false, true, helpers.CombineMaps(baseHelmVars, setValues))

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

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, true, false, false, helpers.CombineMaps(baseHelmVars, setValues))

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
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, true, false, false, baseHelmVars)

	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-operate", "--replicas=0")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "deployment", "camunda-tasklist", "--replicas=0")

	// waiting explicitly for the rollout as the waitUntil can be flaky
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")

	require.True(t, kubectlHelpers.StatefulSetContains(t, &primary.KubectlNamespace, "camunda-zeebe", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", secondaryNamespace)))

	// failover pointing back to secondary
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlFailover, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, true, true, true, baseHelmVars)

	k8s.RunKubectl(t, &primary.KubectlFailover, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")

	require.True(t, kubectlHelpers.StatefulSetContains(t, &primary.KubectlFailover, "camunda-zeebe", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", secondaryNamespace)))

	// secondary pointing back to secondary
	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, true, false, false, helpers.CombineMaps(baseHelmVars, setValuesSecondary))

	// Allow the pods to start rollout, otherwise might be unavailable when trying to delete
	time.Sleep(10 * time.Second)

	// 2 pods are sleeping indefinitely and block rollout
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "delete", "pod", "camunda-zeebe-0", "camunda-zeebe-1", "camunda-zeebe-2", "camunda-zeebe-3", "--force", "--grace-period=0")

	time.Sleep(15 * time.Second)

	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-3", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-1", 20, 15*time.Second)

	require.True(t, kubectlHelpers.StatefulSetContains(t, &secondary.KubectlNamespace, "camunda-zeebe", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", secondaryNamespace)))

}

func removeFailOverRegion(t *testing.T) {
	t.Log("[FAILOVER] Removing failover region 🚀")

	kubectlHelpers.TeardownC8Helm(t, &primary.KubectlFailover)
}

func removeFailBackSecondary(t *testing.T) {
	t.Log("[FAILOVER] Removing failback flag 🚀")

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, true, false, false, baseHelmVars)

	// Allow the pods to start rollout, otherwise might be unavailable when trying to delete
	time.Sleep(10 * time.Second)

	// 2 pods are sleeping indefinitely and block rollout
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "delete", "pod", "camunda-zeebe-0", "camunda-zeebe-1", "camunda-zeebe-2", "camunda-zeebe-3", "--force", "--grace-period=0")

	time.Sleep(15 * time.Second)

	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=600s", "statefulset/camunda-zeebe")
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
