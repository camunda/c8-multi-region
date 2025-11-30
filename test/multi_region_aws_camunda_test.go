package test

import (
	"bytes"
	"fmt"
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

	teleportCluster = "camunda.teleport.sh-camunda-ci-eks"
)

var (
	// TODO: [release-duty] before the release, update this!
	// renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io
	remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "12.7.2")
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

	baseHelmVars    = map[string]string{}
	timeout         = "600s"
)

// AWS EKS Multi-Region Tests

func TestAWSDeployDualRegCamunda(t *testing.T) {
	t.Log("[2 REGION TEST] Deploy Camunda 8 in multi region mode 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
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

// Simplified failover procedure for 8.6+
func TestAWSDualRegFailover_8_6_plus(t *testing.T) {
	t.Log("[2 REGION TEST] Checking Failover procedure for 8.6+ 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
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
		{"TestRemoveSecondaryBrokers", removeSecondaryBrokers},
		{"TestDisableElasticExportersToSecondary", disableElasticExportersToSecondary},
		{"TestCheckTheMathFailover", checkTheMathFailover_8_6_plus},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// Simplified failback procedure for 8.6+
func TestAWSDualRegFailback_8_6_plus(t *testing.T) {
	t.Log("[2 REGION TEST] Running tests for AWS EKS Multi-Region 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		// global.image.tag does not overwrite the image tag for all images
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		// Multi-Region Operational Procedure
		// Failback
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestRecreateCamundaInSecondary", recreateCamundaInSecondary_8_6_plus},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestStopZeebeExporters", stopZeebeExporters},
		{"TestScaleDownWebApps", scaleDownWebApps},
		{"TestCreateElasticBackupRepoPrimary", createElasticBackupRepoPrimary},
		{"TestCreateElasticBackupPrimary", createElasticBackupPrimary},
		{"TestCheckThatElasticBackupIsPresentPrimary", checkThatElasticBackupIsPresentPrimary},
		{"TestCreateElasticBackupRepoSecondary", createElasticBackupRepoSecondary},
		{"TestCheckThatElasticBackupIsPresentSecondary", checkThatElasticBackupIsPresentSecondary},
		{"TestRestoreElasticBackupSecondary", restoreElasticBackupSecondary},
		{"TestEnableElasticExportersToSecondary", enableElasticExportersToSecondary},
		{"TestAddSecondaryBrokers", addSecondaryBrokers},
		{"TestStartZeebeExporters", startZeebeExporters},
		{"TestScaleUpWebApps", scaleUpWebApps},
		{"TestInstallWebAppsSecondary", installWebAppsSecondary_8_6_plus},
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

	if helpers.IsTeleportEnabled() {
		t.Log("[K8S INIT] Initializing Kubernetes helpers with Teleport 🚀")
		primary = helpers.Cluster{
			Region:           "eu-west-2",
			ClusterName:      teleportCluster,
			KubectlNamespace: *k8s.NewKubectlOptions("", "kubeconfig", primaryNamespace),
			KubectlFailover:  *k8s.NewKubectlOptions("", "kubeconfig", primaryNamespaceFailover),
		}
		secondary = helpers.Cluster{
			Region:           "eu-west-3",
			ClusterName:      teleportCluster,
			KubectlNamespace: *k8s.NewKubectlOptions("", "kubeconfig", secondaryNamespace),
			KubectlFailover:  *k8s.NewKubectlOptions("", "kubeconfig", secondaryNamespaceFailover),
		}
	} else {
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
}

func deployC8Helm(t *testing.T) {
	t.Log("[C8 HELM] Deploying Camunda Platform Helm Chart 🚀")

	retries := 30

	if helpers.IsTeleportEnabled() {
		timeout = "1800s"
		retries = 100
		baseHelmVars["zeebe.affinity.podAntiAffinity"] = "null"
	}

	// We have to install both at the same time as otherwise zeebe will not become ready
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 0, false, false, false, baseHelmVars)

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, false, false, false, baseHelmVars)

	// Check that all deployments and Statefulsets are available
	// Terratest has no direct function for Statefulsets, therefore defaulting to pods directly

	// Elastic itself takes already ~2+ minutes to start
	// 30 times with 15 seconds sleep = 7,5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-operate", retries, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-tasklist", retries, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", retries, 15*time.Second)

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-zeebe")

	// 30 times with 15 seconds sleep = 7,5 minutes
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", retries, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", retries, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-gateway", retries, 15*time.Second)

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-zeebe")
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
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "get", "pods")

	t.Log("[DEBUG] Running kubectl describe pods")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "describe", "pods")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "describe", "pods")

	t.Log("[DEBUG] Running kubectl describe configmaps")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "describe", "configmaps")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "describe", "configmaps")

	kubectlHelpers.DumpAllPodLogs(t, &primary.KubectlNamespace)
	kubectlHelpers.DumpAllPodLogs(t, &secondary.KubectlNamespace)
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

func recreateCamundaInSecondary_8_6_plus(t *testing.T) {
	t.Log("[C8 HELM] Recreating Camunda Platform Helm Chart in secondary 🚀")

	setValues := map[string]string{
		"operate.enabled":  "false",
		"tasklist.enabled": "false",
	}

	if helpers.IsTeleportEnabled() {
		timeout = "1800s"
		baseHelmVars["zeebe.affinity.podAntiAffinity"] = "null"
	}

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, false, false, false, helpers.CombineMaps(baseHelmVars, setValues))

	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")
	// We can't wait for Zeebe to become ready as it's not part of the cluster, therefore out of service 503
	// We are using instead elastic to become ready as the next steps depend on it, additionally as direct next step we check that the brokers have joined in again.
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

func installWebAppsSecondary_8_6_plus(t *testing.T) {

	if helpers.IsTeleportEnabled() {
		baseHelmVars["zeebe.affinity.podAntiAffinity"] = "null"
	}

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, primaryNamespaceFailover, secondaryNamespaceFailover, 1, true, false, false, baseHelmVars)

	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-operate", 20, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-tasklist", 20, 15*time.Second)
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

func checkTheMathFailover_8_6_plus(t *testing.T) {
	t.Log("[MATH] Checking the math for Failover 🚀")

	t.Log("[MATH] Checking if the primary deployment has even broker IDs")
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-0")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-1")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-2")))
	require.True(t, helpers.IsEven(kubectlHelpers.GetZeebeBrokerId(t, &primary.KubectlNamespace, "camunda-zeebe-3")))
}

func removeSecondaryBrokers(t *testing.T) {
	t.Log("[FAILOVER] Removing secondary brokers 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 9600)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	// Redistribute to remaining brokers
	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/cluster/brokers?force=true", tunnel.Endpoint()), bytes.NewBuffer([]byte(`["0", "2", "4", "6"]`)))
	if res == nil {
		t.Fatal("[FAILOVER] Failed to create request")
		return
	}

	require.Equal(t, 202, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "PARTITION_FORCE_RECONFIGURE")

	t.Log("[FAILOVER] Give the system some time to redistribute the partitions")
	time.Sleep(5 * time.Second)

	// Check that the removal of obsolete brokers was completed
	for i := 0; i < 3; i++ {
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", tunnel.Endpoint()), nil)
		if res == nil {
			t.Fatal("[FAILOVER] Failed to create request")
			return
		}

		if !strings.Contains(body, "pendingChange") {
			break
		}
		t.Log("[FAILOVER] Broker removal not yet completed, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "COMPLETED")
	require.NotContains(t, body, "pendingChange")
	require.NotContains(t, body, "PARTITION_FORCE_RECONFIGURE")
}

func disableElasticExportersToSecondary(t *testing.T) {
	t.Log("[FAILOVER] Disabling Elasticsearch Exporters to secondary 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 9600)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/exporters/elasticsearchregion1/disable", tunnel.Endpoint()), nil)
	if res == nil {
		t.Fatal("[FAILOVER] Failed to create request")
		return
	}

	require.Equal(t, 202, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "DISABLED")
	require.Contains(t, body, "PARTITION_DISABLE_EXPORTER")

	// Check that the exporter was disabled
	for i := 0; i < 3; i++ {
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/exporters", tunnel.Endpoint()), nil)
		if res == nil {
			t.Fatal("[FAILOVER] Failed to create request")
			return
		}

		if strings.Contains(body, "{\"exporterId\":\"elasticsearchregion1\",\"status\":\"DISABLED\"}") {
			break
		}
		t.Log("[FAILOVER] Exporter not yet disabled, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "{\"exporterId\":\"elasticsearchregion0\",\"status\":\"ENABLED\"}")
	require.Contains(t, body, "{\"exporterId\":\"elasticsearchregion1\",\"status\":\"DISABLED\"}")
}

func enableElasticExportersToSecondary(t *testing.T) {
	t.Log("[FAILBACK] Enabling Elasticsearch Exporters to secondary 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 9600)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/exporters/elasticsearchregion1/enable", tunnel.Endpoint()), bytes.NewBuffer([]byte(`{"initializeFrom":"elasticsearchregion0"}`)))
	if res == nil {
		t.Fatal("[FAILBACK] Failed to create request")
		return
	}

	require.Equal(t, 202, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "ENABLED")
	require.Contains(t, body, "PARTITION_ENABLE_EXPORTER")

	// Check that the exporter was enabled
	for i := 0; i < 3; i++ {
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/exporters", tunnel.Endpoint()), nil)
		if res == nil {
			t.Fatal("[FAILBACK] Failed to create request")
			return
		}

		if strings.Contains(body, "{\"exporterId\":\"elasticsearchregion1\",\"status\":\"ENABLED\"}") {
			break
		}
		t.Log("[FAILBACK] Exporter not yet enabled, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "{\"exporterId\":\"elasticsearchregion0\",\"status\":\"ENABLED\"}")
	require.Contains(t, body, "{\"exporterId\":\"elasticsearchregion1\",\"status\":\"ENABLED\"}")
}

func addSecondaryBrokers(t *testing.T) {
	t.Log("[FAILBACK] Adding secondary brokers 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 9600)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	// Redistribute to new brokers
	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/cluster/brokers?replicationFactor=4", tunnel.Endpoint()), bytes.NewBuffer([]byte(`["0", "1", "2", "3", "4", "5", "6", "7"]`)))
	if res == nil {
		t.Fatal("[FAILBACK] Failed to create request")
		return
	}

	require.Equal(t, 202, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "\"id\":8,\"state\":\"ACTIVE\"")

	// Check that the addition of new brokers was completed
	// This can take a while
	for i := 0; i < 20; i++ {
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", tunnel.Endpoint()), nil)
		if res == nil {
			t.Fatal("[FAILBACK] Failed to create request")
			return
		}

		if !strings.Contains(body, "pendingChange") {
			break
		}
		t.Log("[FAILBACK] Broker addition not yet completed, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "COMPLETED")
	require.NotContains(t, body, "pendingChange")

	// Check that the new brokers have become ready, now that they're integrated in the zeebe cluster again
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout=300s", "statefulset/camunda-zeebe")
}
