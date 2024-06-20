package kubectlHelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/camunda/camunda/clients/go/v8/pkg/zbc"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/stretchr/testify/require"
)

func CrossClusterCommunication(t *testing.T, withDNS bool, k8sManifests string, primary, secondary helpers.Cluster) {
	kubeResourcePath := fmt.Sprintf("%s/%s", k8sManifests, "nginx.yml")

	defer k8s.KubectlDelete(t, &primary.KubectlNamespace, kubeResourcePath)
	defer k8s.KubectlDelete(t, &secondary.KubectlNamespace, kubeResourcePath)

	k8s.KubectlApply(t, &primary.KubectlNamespace, kubeResourcePath)
	k8s.KubectlApply(t, &secondary.KubectlNamespace, kubeResourcePath)

	k8s.WaitUntilServiceAvailable(t, &primary.KubectlNamespace, "sample-nginx-peer", 10, 5*time.Second)
	k8s.WaitUntilServiceAvailable(t, &secondary.KubectlNamespace, "sample-nginx-peer", 10, 5*time.Second)

	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "sample-nginx", 10, 5*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "sample-nginx", 10, 5*time.Second)

	podPrimary := k8s.GetPod(t, &primary.KubectlNamespace, "sample-nginx")
	podSecondary := k8s.GetPod(t, &secondary.KubectlNamespace, "sample-nginx")

	if withDNS {
		// Check if the pods can reach each other via the service

		// wrapped in a for loop since the reload of CoreDNS needs a bit of time to be propagated
		for i := 0; i < 6; i++ {
			outputPrimary, errPrimary := k8s.RunKubectlAndGetOutputE(t, &primary.KubectlNamespace, "exec", podPrimary.Name, "--", "curl", "--max-time", "15", fmt.Sprintf("sample-nginx.sample-nginx-peer.%s.svc.cluster.local", secondary.KubectlNamespace.Namespace))
			outputSecondary, errSecondary := k8s.RunKubectlAndGetOutputE(t, &secondary.KubectlNamespace, "exec", podSecondary.Name, "--", "curl", "--max-time", "15", fmt.Sprintf("sample-nginx.sample-nginx-peer.%s.svc.cluster.local", primary.KubectlNamespace.Namespace))
			if errPrimary != nil || errSecondary != nil {
				t.Logf("[CROSS CLUSTER COMMUNICATION] Error: %s", errPrimary)
				t.Logf("[CROSS CLUSTER COMMUNICATION] Error: %s", errSecondary)
				t.Log("[CROSS CLUSTER COMMUNICATION] CoreDNS not resolving yet, waiting ...")
				time.Sleep(15 * time.Second)
			}

			if outputPrimary != "" && outputSecondary != "" {
				t.Logf("[CROSS CLUSTER COMMUNICATION] Success: %s", outputPrimary)
				t.Logf("[CROSS CLUSTER COMMUNICATION] Success: %s", outputSecondary)
				t.Log("[CROSS CLUSTER COMMUNICATION] Communication established")
				break
			}
		}
	} else {
		// Check if the pods can reach each other via the IPs directly
		podPrimaryIP := podPrimary.Status.PodIP
		require.NotEmpty(t, podPrimaryIP)

		podSecondaryIP := podSecondary.Status.PodIP
		require.NotEmpty(t, podSecondaryIP)

		k8s.RunKubectl(t, &primary.KubectlNamespace, "exec", podPrimary.Name, "--", "curl", "--max-time", "15", podSecondaryIP)
		k8s.RunKubectl(t, &secondary.KubectlNamespace, "exec", podSecondary.Name, "--", "curl", "--max-time", "15", podPrimaryIP)

		t.Log("[CROSS CLUSTER COMMUNICATION] Communication established")
	}
}

func CheckCoreDNSReload(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	pods := k8s.ListPods(t, kubectlOptions, metav1.ListOptions{LabelSelector: "k8s-app=kube-dns"})

	for _, pod := range pods {
		for i := 0; i < 8; i++ {
			t.Logf("[COREDNS RELOAD] Checking CoreDNS logs for pod %s", pod.Name)
			logs := k8s.GetPodLogs(t, kubectlOptions, &pod, "coredns")

			if !strings.Contains(logs, "Reloading complete") {
				t.Log("[COREDNS RELOAD] CoreDNS not reloaded yet. Waiting...")
				time.Sleep(15 * time.Second)
			} else {
				t.Log("[COREDNS RELOAD] CoreDNS reloaded successfully")
				break
			}
		}
	}
}

func TeardownC8Helm(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	helmOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
	}

	helm.Delete(t, helmOptions, "camunda", true)

	t.Logf("[C8 HELM TEARDOWN] removing all PVCs and PVs from namespace %s", kubectlOptions.Namespace)

	pvcs := k8s.ListPersistentVolumeClaims(t, kubectlOptions, metav1.ListOptions{})

	for _, pvc := range pvcs {
		k8s.RunKubectl(t, kubectlOptions, "delete", "pvc", pvc.Name)
	}

	pvs := k8s.ListPersistentVolumes(t, kubectlOptions, metav1.ListOptions{})

	for _, pv := range pvs {
		if pv.Spec.ClaimRef.Namespace == kubectlOptions.Namespace {
			k8s.RunKubectl(t, kubectlOptions, "delete", "pv", pv.Name)
		}
	}

}

func CheckOperateForProcesses(t *testing.T, cluster helpers.Cluster) {
	t.Logf("[C8 PROCESS] Checking for Cluster %s whether Operate contains deployed processes", cluster.ClusterName)

	// strange behaviour since service is on 80 but pod on 8080
	tunnelOperate := k8s.NewTunnel(&cluster.KubectlNamespace, k8s.ResourceTypeService, "camunda-operate", 0, 8080)
	defer tunnelOperate.Close()
	tunnelOperate.ForwardPort(t)

	// the Cookie grants access since we don't have an API key
	resp, err := http.Post(fmt.Sprintf("http://%s/api/login?username=demo&password=demo", tunnelOperate.Endpoint()), "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("[C8 PROCESS] %s", err)
		return
	}

	csrfTokenName := "OPERATE-X-CSRF-TOKEN"
	csrfToken := resp.Header.Get("Operate-X-Csrf-Token")
	if csrfToken == "" {
		csrfToken = resp.Header.Get("X-Csrf-Token")
		csrfTokenName = "X-CSRF-TOKEN"
	}

	var cookieAuth string
	var csrfTokenId string
	for _, val := range resp.Cookies() {
		if val.Name == "OPERATE-SESSION" {
			cookieAuth = val.Value
		}
		if val.Name == csrfTokenName {
			csrfTokenId = val.Value
		}
	}
	require.NotEmpty(t, cookieAuth)

	// create http client to add cookie to the request
	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/process-definitions/search", tunnelOperate.Endpoint()), strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("[C8 PROCESS] %s", err)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	// > 8.5.1, we need to supply the csrf token
	if csrfTokenId != "" {
		req.Header.Add("Cookie", fmt.Sprintf("OPERATE-SESSION=%s; %s=%s", cookieAuth, csrfTokenName, csrfTokenId))
		req.Header.Add(csrfTokenName, csrfToken)
		req.Header.Add("accept", "application/json")
	} else {
		req.Header.Add("Cookie", fmt.Sprintf("OPERATE-SESSION=%s", cookieAuth))
	}

	var bodyString string
	for i := 0; i < 8; i++ {
		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("[C8 PROCESS] %s", err)
			return
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("[C8 PROCESS] %s", err)
			return
		}

		bodyString = string(body)

		t.Logf("[C8 Process] %s", bodyString)
		if !strings.Contains(bodyString, "\"total\":0") {
			t.Log("[C8 PROCESS] processes are present, breaking and asserting")
			break
		}
		t.Log("[C8 PROCESS] not imported yet, waiting...")
		time.Sleep(15 * time.Second)
	}

	require.Contains(t, bodyString, "Big variable process")
	require.Contains(t, bodyString, "bigVarProcess")
}

func RunSensitiveKubectlCommand(t *testing.T, kubectlOptions *k8s.KubectlOptions, command ...string) {
	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard
	k8s.RunKubectl(t, kubectlOptions, command...)
}

func ConfigureElasticBackup(t *testing.T, cluster helpers.Cluster, clusterName string) {
	t.Logf("[ELASTICSEARCH] Configuring Elasticsearch backup for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XPUT", "http://localhost:9200/_snapshot/camunda_backup", "-H", "Content-Type: application/json", "-d", fmt.Sprintf("{\"type\": \"s3\", \"settings\": {\"bucket\": \"%s-elastic-backup\", \"client\": \"camunda\", \"base_path\": \"backups\"}}", clusterName))
	if err != nil {
		t.Fatalf("[ELASTICSEARCH] Error: %s", err)
		return
	}

	if !strings.Contains(output, "acknowledged") {
		t.Fatalf("[ELASTICSEARCH] Error: %s", output)
		return
	}

	require.Contains(t, output, "acknowledged")
	t.Logf("[ELASTICSEARCH] Success: %s", output)
}

func CreateElasticBackup(t *testing.T, cluster helpers.Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Creating Elasticsearch backup for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-X", "PUT", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s?wait_for_completion=true", backupName))
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Created backup: %s", output)
}

func CheckThatElasticBackupIsPresent(t *testing.T, cluster helpers.Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Checking that Elasticsearch backup is present for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XGET", "localhost:9200/_snapshot/camunda_backup/_all")
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, backupName)
	t.Logf("[ELASTICSEARCH BACKUP] Backup present: %s", output)
}

func RestoreElasticBackup(t *testing.T, cluster helpers.Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Restoring Elasticsearch backup for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XPOST", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s/_restore?wait_for_completion=true", backupName))
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Restored backup: %s", output)

}

func createZeebeContactPoints(t *testing.T, size int, namespace0, namespace1 string) string {
	zeebeContactPoints := ""

	for i := 0; i < size; i++ {
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.%s.svc.cluster.local:26502,", strconv.Itoa((i)), namespace0)
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.%s.svc.cluster.local:26502,", strconv.Itoa((i)), namespace1)
	}

	// Cut the last character "," from the string
	zeebeContactPoints = zeebeContactPoints[:len(zeebeContactPoints)-1]

	return zeebeContactPoints
}

func InstallUpgradeC8Helm(t *testing.T, kubectlOptions *k8s.KubectlOptions, remoteChartVersion, remoteChartName, remoteChartSource, namespace0, namespace1, namespace0Failover, namespace1Failover string, region int, upgrade, failover, esSwitch bool, setValues map[string]string) {
	zeebeContactPoints := createZeebeContactPoints(t, 4, namespace0, namespace1)

	valuesFiles := []string{"../aws/dual-region/kubernetes/camunda-values.yml"}

	filePath := "../aws/dual-region/kubernetes/camunda-values.yml"
	if failover {
		filePath = fmt.Sprintf("../aws/dual-region/kubernetes/region%d/camunda-values-failover.yml", region)

		valuesFiles = append(valuesFiles, filePath)
	} else {
		valuesFiles = append(valuesFiles, fmt.Sprintf("../aws/dual-region/kubernetes/region%d/camunda-values.yml", region))
	}

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

	// Replace Elasticsearch endpoints with namespace specific ones
	modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-primary.svc.cluster.local:9200", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace0), -1)
	modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace1), -1)

	if failover {
		modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-primary-failover.svc.cluster.local:9200", fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace0Failover), -1)
	}

	if esSwitch && !failover {
		modifiedContent = strings.Replace(modifiedContent, fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace1), fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace0Failover), -1)
	}
	if esSwitch && failover {
		modifiedContent = strings.Replace(modifiedContent, fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace0Failover), fmt.Sprintf("http://camunda-elasticsearch-master-hl.%s.svc.cluster.local:9200", namespace1), -1)
	}

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("[C8 HELM] Error writing file: %v\n", err)
		return
	}

	helmOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
		Version:        remoteChartVersion,
		ValuesFiles:    valuesFiles,
		SetValues:      setValues,
	}

	if !strings.Contains(remoteChartVersion, "snapshot") {
		helm.AddRepo(t, helmOptions, "camunda", remoteChartSource)
	}

	if upgrade {
		// Terratest is actively ignoring the version in an upgrade
		helmOptions.ExtraArgs = map[string][]string{"upgrade": []string{"--version", remoteChartVersion}}
		helm.Upgrade(t, helmOptions, remoteChartName, "camunda")
	} else {
		helm.Install(t, helmOptions, remoteChartName, "camunda")
	}

	// Write the old file back to the file - mostly for local development
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		t.Fatalf("[C8 HELM] Error writing file: %v\n", err)
		return
	}
}

func StatefulSetContains(t *testing.T, kubectlOptions *k8s.KubectlOptions, statefulset, searchValue string) bool {
	t.Logf("[STATEFULSET] Checking whether StatefulSet %s contains %s", statefulset, searchValue)

	// Temporarily disable logging
	// Output might be too large
	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard

	output, err := k8s.RunKubectlAndGetOutputE(t, kubectlOptions, "get", "statefulset", statefulset, "-o", "yaml")
	if err != nil {
		t.Fatalf("[STATEFULSET] %s", err)
		return false
	}

	val := strings.Contains(output, searchValue)
	t.Logf("[STATEFULSET] StatefulSet %s contains %s: %t", statefulset, searchValue, val)

	return val
}

func GetZeebeBrokerId(t *testing.T, kubectlOptions *k8s.KubectlOptions, podName string) int {
	t.Logf("[ZEEBE BROKER ID] Getting Zeebe Broker ID for pod %s", podName)

	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard

	output, err := k8s.RunKubectlAndGetOutputE(t, kubectlOptions, "exec", podName, "--", "pgrep", "java")
	if err != nil {
		t.Fatalf("[ZEEBE BROKER ID] %s", err)
		return -1
	}

	output, err = k8s.RunKubectlAndGetOutputE(t, kubectlOptions, "exec", podName, "--", "cat", fmt.Sprintf("/proc/%s/environ", output))
	if err != nil {
		t.Fatalf("[ZEEBE BROKER ID] %s", err)
		return -1
	}

	return helpers.CutOutString(output, "ZEEBE_BROKER_CLUSTER_NODEID=[0-9]")
}

func CheckC8RunningProperly(t *testing.T, primary helpers.Cluster, namespace0, namespace1 string) {
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
		if strings.Contains(broker.Host, namespace0) {
			primaryCount++
		} else if strings.Contains(broker.Host, namespace1) {
			secondaryCount++
		}
		t.Logf("[C8 CHECK] Broker ID: %d, Address: %s, Partitions: %v\n", broker.NodeId, broker.Host, broker.Partitions)
	}

	require.Equal(t, 4, primaryCount)
	require.Equal(t, 4, secondaryCount)
}

func DeployC8processAndCheck(t *testing.T, primary helpers.Cluster, secondary helpers.Cluster, resourceDir string) {
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
	CheckOperateForProcesses(t, primary)
	CheckOperateForProcesses(t, secondary)
}

func CreateAllNamespaces(t *testing.T, source helpers.Cluster, namespaces, namespacesFailover string) {
	// Get all namespaces
	arr := strings.Split(namespaces+","+namespacesFailover, ",")

	for _, ns := range arr {
		k8s.CreateNamespace(t, &source.KubectlNamespace, ns)
	}
}

func CreateAllRequiredSecrets(t *testing.T, source helpers.Cluster, namespaces, namespacesFailover string) {
	t.Log("[ELASTICSEARCH] Creating AWS Secret for Elasticsearch 🚀")

	S3AWSAccessKey := helpers.GetEnv("S3_AWS_ACCESS_KEY", "")
	S3AWSSecretAccessKey := helpers.GetEnv("S3_AWS_SECRET_KEY", "")

	arr := strings.Split(namespaces+","+namespacesFailover, ",")

	for _, ns := range arr {
		RunSensitiveKubectlCommand(t, &source.KubectlNamespace, "create", "--namespace", ns, "secret", "generic", "elasticsearch-env-secret", fmt.Sprintf("--from-literal=S3_SECRET_KEY=%s", S3AWSSecretAccessKey), fmt.Sprintf("--from-literal=S3_ACCESS_KEY=%s", S3AWSAccessKey))
	}
}

func DumpAllPodLogs(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Logf("[POD LOGS] Dumping logs for pod %s", kubectlOptions.Namespace)

	// Temporarily disable logging to not overflow with all logs
	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard

	pods := k8s.ListPods(t, kubectlOptions, metav1.ListOptions{})

	for _, pod := range pods {
		podLogs, err := k8s.GetPodLogsE(t, kubectlOptions, &pod, "")
		if err != nil {
			t.Fatalf("Error getting pod logs: %v", err)
		}

		// Write logs to a file
		err = os.WriteFile(fmt.Sprintf("%s-%s.log", kubectlOptions.Namespace, pod.Name), []byte(podLogs), 0644)
		if err != nil {
			t.Fatalf("Error writing logs to file: %v", err)
		}
	}
}
