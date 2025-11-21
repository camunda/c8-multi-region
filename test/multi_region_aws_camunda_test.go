package test

import (
	"bytes"
	"fmt"
	"net/http"
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
	defaultValuesYaml   = "camunda-values.yml"
	migrationValuesYaml = "camunda-values-migration.yml"

	teleportCluster = "camunda.teleport.sh-camunda-ci-eks"
)

var (
	// renovate: datasource=helm depName=camunda-platform registryUrl=https://helm.camunda.io versioning=regex:^13(\.(?<minor>\d+))?(\.(?<patch>\d+))?$
	remoteChartVersion = helpers.GetEnv("HELM_CHART_VERSION", "13.1.2")
	remoteChartName    = helpers.GetEnv("HELM_CHART_NAME", "camunda/camunda-platform") // allows using OCI registries
	globalImageTag     = helpers.GetEnv("GLOBAL_IMAGE_TAG", "")                        // allows overwriting the image tag via GHA of every Camunda image
	clusterName        = helpers.GetEnv("CLUSTER_NAME", "nightly")                     // allows supplying random cluster name via GHA
	backupName         = helpers.GetEnv("BACKUP_NAME", "nightly")                      // allows supplying random backup name via GHA
	awsProfile         = helpers.GetEnv("AWS_PROFILE", "infex")
	migrationOffset, _ = strconv.Atoi(helpers.GetEnv("MIGRATION_OFFSET", "0")) // Offset for process instances started before migration

	primary   helpers.Cluster
	secondary helpers.Cluster

	// Allows setting namespaces via GHA
	primaryNamespace           = helpers.GetEnv("CLUSTER_0_NAMESPACE", "c8-snap-cluster-0")
	primaryNamespaceFailover   = helpers.GetEnv("CLUSTER_0_NAMESPACE_FAILOVER", "c8-snap-cluster-0-failover")
	secondaryNamespace         = helpers.GetEnv("CLUSTER_1_NAMESPACE", "c8-snap-cluster-1")
	secondaryNamespaceFailover = helpers.GetEnv("CLUSTER_1_NAMESPACE_FAILOVER", "c8-snap-cluster-1-failover")

	baseHelmVars = map[string]string{}
	timeout      = "600s"
	retries      = 20
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
		{"TestDeployC8Helm", func(t *testing.T) { deployC8Helm(t, defaultValuesYaml) }},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", func(t *testing.T) { deployC8processAndCheck(t, 6, "default") }},
		{"TestCheckElasticsearchClusterHealth", checkElasticsearchClusterHealth},
		{"TestCheckTheMath", checkTheMath},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

func TestMigrationDualReg(t *testing.T) {
	t.Log("[2 REGION TEST] Migrate Camunda 8 in multi region mode 🚀")

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
		{"TestDeployC8Helm", func(t *testing.T) { deployC8Helm(t, migrationValuesYaml) }},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestCheckMigrationSucceed", checkMigrationSucceed},
		{"TestPostMigrationCleanup", postMigrationCleanup},
		{"TestDeployC8processAndCheck", func(t *testing.T) { deployC8processAndCheck(t, 7, "migration") }},
		{"TestCheckElasticsearchClusterHealth", checkElasticsearchClusterHealth},
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
		{"TestDeployC8processAndCheck", func(t *testing.T) { deployC8processAndCheck(t, 12, "failover") }},
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
		{"TestRecreateCamundaInSecondary", func(t *testing.T) { redeployWithoutOperateTasklist(t, secondary, true) }},
		{"TestRedeployCamundaInPrimary", func(t *testing.T) { redeployWithoutOperateTasklist(t, primary, false) }},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestStopZeebeExporters", stopZeebeExporters},
		{"TestCreateElasticBackupRepoPrimary", createElasticBackupRepoPrimary},
		{"TestCreateElasticBackupPrimary", createElasticBackupPrimary},
		{"TestCheckThatElasticBackupIsPresentPrimary", checkThatElasticBackupIsPresentPrimary},
		{"TestCreateElasticBackupRepoSecondary", createElasticBackupRepoSecondary},
		{"TestCheckThatElasticBackupIsPresentSecondary", checkThatElasticBackupIsPresentSecondary},
		{"TestRestoreElasticBackupSecondary", restoreElasticBackupSecondary},
		{"TestCheckElasticsearchClusterHealthAfterRestore", checkElasticsearchClusterHealth},
		{"TestEnableElasticExportersToSecondary", enableElasticExportersToSecondary},
		{"TestStartZeebeExporters", startZeebeExporters},
		{"TestAddSecondaryBrokers", addSecondaryBrokers},
		{"TestRedeployC8ToEnableOperateTasklist", func(t *testing.T) { deployC8Helm(t, defaultValuesYaml) }},
		{"TestCheckC8RunningProperly", checkC8RunningProperly},
		{"TestDeployC8processAndCheck", func(t *testing.T) { deployC8processAndCheck(t, 18, "default") }},
		{"TestCheckElasticsearchClusterHealthAfterProcessDeploy", checkElasticsearchClusterHealth},
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

func deployC8Helm(t *testing.T, valuesYaml string) {
	t.Log("[C8 HELM] Deploying Camunda Platform Helm Chart 🚀")

	setStringValues := map[string]string{}

	if helpers.IsTeleportEnabled() {
		timeout = "1800s"
		retries = 100
		baseHelmVars["orchestration.affinity.podAntiAffinity"] = "null"

		if valuesYaml == migrationValuesYaml {
			baseHelmVars["orchestration.migration.affinity.podAntiAffinity"] = "null"
		}
	}

	// We have to install both at the same time as otherwise zeebe will not become ready
	kubectlHelpers.InstallUpgradeC8Helm(t, &primary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, valuesYaml, 0, baseHelmVars, setStringValues)

	kubectlHelpers.InstallUpgradeC8Helm(t, &secondary.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, valuesYaml, 1, baseHelmVars, setStringValues)

	// Check that all deployments and Statefulsets are available
	// Terratest has no direct function for Statefulsets, therefore defaulting to pods directly

	k8s.RunKubectl(t, &primary.KubectlNamespace, "get", "pods")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "get", "pods")

	// Elastic itself takes already ~2+ minutes to start
	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-zeebe")

	// no functions for Statefulsets yet
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-zeebe")

	// connectors last as they depend on the Orchestration Cluster
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-connectors", retries, 15*time.Second)
}

func checkC8RunningProperly(t *testing.T) {
	t.Log("[C8 CHECK] Checking if Camunda Platform is running properly 🚦")
	kubectlHelpers.CheckC8RunningProperly(t, primary, primaryNamespace, secondaryNamespace)
}

func deployC8processAndCheck(t *testing.T, expectedProcesses int, mode string) {
	t.Log("[C8 PROCESS] Deploying a process and checking if it's running 🚀")

	tmpExpectedProcesses := expectedProcesses + migrationOffset

	kubectlHelpers.DeployC8processAndCheck(t, primary, resourceDir)

	kubectlHelpers.CheckOperateForProcesses(t, primary)

	if mode != "failover" {
		kubectlHelpers.CheckOperateForProcesses(t, secondary)
	}

	kubectlHelpers.CheckOperateForProcessInstances(t, primary, tmpExpectedProcesses)

	if mode != "failover" {
		kubectlHelpers.CheckOperateForProcessInstances(t, secondary, tmpExpectedProcesses)
	}
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

func checkElasticsearchClusterHealth(t *testing.T) {
	t.Log("[ELASTICSEARCH HEALTH] Checking cluster health in both regions 🚀")

	kubectlHelpers.CheckElasticsearchClusterHealth(t, primary)
	kubectlHelpers.CheckElasticsearchClusterHealth(t, secondary)
}

func deleteSecondaryRegion(t *testing.T) {
	t.Log("[REGION REMOVAL] Deleting secondary region 🚀")

	kubectlHelpers.TeardownC8Helm(t, &secondary.KubectlNamespace)
}

// redeployWithoutOperateTasklist redeploys Camunda in the specified cluster with Operate and Tasklist disabled.
// For secondary cluster, it also disables schema creation to prevent conflicts during DB restore.
func redeployWithoutOperateTasklist(t *testing.T, cluster helpers.Cluster, disableSchemaCreation bool) {
	t.Logf("[C8 HELM] Redeploying Camunda Platform Helm Chart in %s 🚀", cluster.ClusterName)

	region := 0

	// assumption: eu-west-2 = 0 and eu-west-3 = 1
	if cluster.Region == "eu-west-3" {
		region = 1
	}

	setValues := map[string]string{}
	setStringValues := map[string]string{}

	if helpers.IsTeleportEnabled() {
		timeout = "1800s"
		baseHelmVars["orchestration.affinity.podAntiAffinity"] = "null"
	}

	// We have to disable Operate and Tasklist due to better UX + risk of data loss in case of local actions
	setValues["orchestration.profiles.operate"] = "false"
	setValues["orchestration.profiles.tasklist"] = "false"

	// Disable schema creation if requested (needed for secondary during DB restore)
	if disableSchemaCreation {
		setStringValues["orchestration.env[15].name"] = "CAMUNDA_DATABASE_SCHEMAMANAGER_CREATESCHEMA"
		setStringValues["orchestration.env[15].value"] = "false"
	}

	kubectlHelpers.InstallUpgradeC8Helm(t, &cluster.KubectlNamespace, remoteChartVersion, remoteChartName, remoteChartSource, primaryNamespace, secondaryNamespace, defaultValuesYaml, region, helpers.CombineMaps(baseHelmVars, setValues), setStringValues)

	k8s.RunKubectl(t, &cluster.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-elasticsearch-master")

	// We can't wait for Zeebe to become ready as it's not part of the cluster, therefore out of service 503
	// We are using instead elastic to become ready as the next steps depend on it, additionally as direct next step we check that the brokers have joined in again.
	// We skip this for region 1 since only region 0 is part of the cluster at this point.
	if region == 0 {
		k8s.RunKubectl(t, &cluster.KubectlNamespace, "rollout", "status", "--watch", "--timeout="+timeout, "statefulset/camunda-zeebe")
	}
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

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Redistribute to remaining brokers
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?force=true", endpoint),
		bytes.NewBuffer([]byte(`{"brokers":{"remove":[1,3,5,7]}}`)),
	)
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
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", endpoint), nil)
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

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/exporters/camundaregion1/disable", endpoint), nil)
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
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/exporters", endpoint), nil)
		if res == nil {
			t.Fatal("[FAILOVER] Failed to create request")
			return
		}

		if strings.Contains(body, "{\"exporterId\":\"camundaregion1\",\"status\":\"DISABLED\"}") {
			break
		}
		t.Log("[FAILOVER] Exporter not yet disabled, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "{\"exporterId\":\"camundaregion0\",\"status\":\"ENABLED\"}")
	require.Contains(t, body, "{\"exporterId\":\"camundaregion1\",\"status\":\"DISABLED\"}")
}

func enableElasticExportersToSecondary(t *testing.T) {
	t.Log("[FAILBACK] Enabling Elasticsearch Exporters to secondary 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	res, body := helpers.HttpRequest(t, "POST", fmt.Sprintf("http://%s/actuator/exporters/camundaregion1/enable", endpoint), bytes.NewBuffer([]byte(`{"initializeFrom":"camundaregion0"}`)))
	if res == nil {
		t.Fatal("[FAILBACK] Failed to create request")
		return
	}

	require.Equal(t, 202, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "ENABLED")
	require.Contains(t, body, "PARTITION_ENABLE_EXPORTER")

	// Check that the exporter was enabled
	// It can take a while until the exporter is fully enabled again
	for i := 0; i < 30; i++ {
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/exporters", endpoint), nil)
		if res == nil {
			t.Fatal("[FAILBACK] Failed to create request")
			return
		}

		if strings.Contains(body, "{\"exporterId\":\"camundaregion1\",\"status\":\"ENABLED\"}") {
			break
		}
		t.Log("[FAILBACK] Exporter not yet enabled, retrying...")
		time.Sleep(15 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "{\"exporterId\":\"camundaregion0\",\"status\":\"ENABLED\"}")
	require.Contains(t, body, "{\"exporterId\":\"camundaregion1\",\"status\":\"ENABLED\"}")
}

func addSecondaryBrokers(t *testing.T) {
	t.Log("[FAILBACK] Adding secondary brokers 🚀")
	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Redistribute to new brokers
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer([]byte(`{"brokers":{"add":[1,3,5,7]},"partitions":{"replicationFactor":4}}`)),
	)
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
		res, body = helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", endpoint), nil)
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

	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-connectors", retries, 15*time.Second)
}

func checkMigrationSucceed(t *testing.T) {
	t.Log("[MIGRATION CHECK] Checking if Camunda Platform Migration is running 🚦")

	k8s.RunKubectl(t, &primary.KubectlNamespace, "get", "pods")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "get", "pods")

	// Waiting for the importer to be ready
	k8s.WaitUntilDeploymentAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-migration-importer", retries, 15*time.Second)
	k8s.WaitUntilDeploymentAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-migration-importer", retries, 15*time.Second)

	// If the Job succeeds, then the migration was successfully completed
	k8s.WaitUntilJobSucceed(t, &primary.KubectlNamespace, "camunda-zeebe-migration-data", retries, 30*time.Second)
	k8s.WaitUntilJobSucceed(t, &secondary.KubectlNamespace, "camunda-zeebe-migration-data", retries, 30*time.Second)
}

func postMigrationCleanup(t *testing.T) {
	t.Log("[MIGRATION CLEANUP] Disabling old exporters after Camunda Platform Migration 🚦")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, service.Name, "camunda-zeebe-gateway")

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Disable old migration exporters
	exporterIDs := []string{"elasticsearchregion0", "elasticsearchregion1"}
	for _, id := range exporterIDs {
		t.Logf("[MIGRATION CLEANUP] Disabling exporter %s", id)
		res, body := helpers.HttpRequest(
			t,
			"POST",
			fmt.Sprintf("http://%s/actuator/exporters/%s/disable", endpoint, id),
			nil,
		)
		if res == nil {
			t.Fatalf("[MIGRATION CLEANUP] Failed to create request for exporter %s", id)
			return
		}
		require.Equal(t, 202, res.StatusCode, "unexpected status disabling exporter %s", id)
		require.NotEmpty(t, body)
		require.Contains(t, body, "DISABLED")

		// Wait until the cluster reports the last change as COMPLETED before continuing
		for i := 0; i < 20; i++ {
			resCluster, bodyCluster := helpers.HttpRequest(
				t,
				"GET",
				fmt.Sprintf("http://%s/actuator/cluster", endpoint),
				nil,
			)
			if resCluster == nil {
				t.Fatal("[MIGRATION CLEANUP] Failed to query cluster status")
				return
			}
			if strings.Contains(bodyCluster, "\"lastChange\"") &&
				strings.Contains(bodyCluster, "\"status\":\"COMPLETED\"") &&
				!strings.Contains(bodyCluster, "pendingChange") {
				t.Log("[MIGRATION CLEANUP] Cluster lastChange status is COMPLETED")
				break
			}
			if i == 19 {
				t.Fatalf("[MIGRATION CLEANUP] Cluster lastChange did not reach COMPLETED. Body: %s", bodyCluster)
			}
			t.Log("[MIGRATION CLEANUP] Cluster change not yet COMPLETED, retrying...")
			time.Sleep(10 * time.Second)
		}
	}

	// Confirm both exporters are disabled
	var (
		res  *http.Response
		body string
	)
	for i := 0; i < 10; i++ {
		res, body = helpers.HttpRequest(
			t,
			"GET",
			fmt.Sprintf("http://%s/actuator/exporters", endpoint),
			nil,
		)
		if res == nil {
			t.Fatal("[MIGRATION CLEANUP] Failed to query exporters status")
			return
		}

		if strings.Contains(body, "\"exporterId\":\"elasticsearchregion0\",\"status\":\"DISABLED\"") &&
			strings.Contains(body, "\"exporterId\":\"elasticsearchregion1\",\"status\":\"DISABLED\"") {
			break
		}
		t.Log("[MIGRATION CLEANUP] Exporters not yet disabled, retrying...")
		time.Sleep(10 * time.Second)
	}

	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)
	require.Contains(t, body, "\"exporterId\":\"elasticsearchregion0\",\"status\":\"DISABLED\"")
	require.Contains(t, body, "\"exporterId\":\"elasticsearchregion1\",\"status\":\"DISABLED\"")
	t.Log("[MIGRATION CLEANUP] Successfully disabled migration exporters")
}
