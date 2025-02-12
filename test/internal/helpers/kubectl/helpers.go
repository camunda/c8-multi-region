package kubectlHelpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
)

type Partition struct {
	PartitionId int    `json:"partitionId"`
	Role        string `json:"role"`
	Health      string `json:"health"`
}

type Broker struct {
	NodeId     int         `json:"nodeId"`
	Host       string      `json:"host"`
	Port       int         `json:"port"`
	Partitions []Partition `json:"partitions"`
	Version    string      `json:"version"`
}

type ClusterInfo struct {
	Brokers           []Broker `json:"brokers"`
	ClusterSize       int      `json:"clusterSize"`
	PartitionsCount   int      `json:"partitionsCount"`
	ReplicationFactor int      `json:"replicationFactor"`
	GatewayVersion    string   `json:"gatewayVersion"`
}

func CrossClusterCommunication(t *testing.T, withDNS bool, k8sManifests string, primary, secondary helpers.Cluster, kubeConfigPrimary, kubeConfigSecondary string) {
	kubeResourcePath := fmt.Sprintf("%s/%s", k8sManifests, "nginx.yml")

	if withDNS {
		primaryNamespaceArr := strings.Split(helpers.GetEnv("CLUSTER_0_NAMESPACE_ARR", ""), ",")
		secondaryNamespaceArr := strings.Split(helpers.GetEnv("CLUSTER_1_NAMESPACE_ARR", ""), ",")
		for i := 0; i < len(primaryNamespaceArr); i++ {
			os.Setenv("CLUSTER_0", primary.ClusterName)
			os.Setenv("CAMUNDA_NAMESPACE_0", primaryNamespaceArr[i])
			os.Setenv("CLUSTER_1", secondary.ClusterName)
			os.Setenv("CAMUNDA_NAMESPACE_1", secondaryNamespaceArr[i])
			os.Setenv("KUBECONFIG", kubeConfigPrimary+":"+kubeConfigSecondary)

			output := shell.RunCommandAndGetOutput(t, shell.Command{
				Command: "sh",
				Args: []string{
					"../aws/dual-region/scripts/test_dns_chaining.sh",
				},
			})

			// Check the output for success or failure messages
			if strings.Contains(output, "Failed to reach the target instance") {
				t.Fatalf("Script failed: %s", output)
			} else {
				t.Logf("Script output: %s", output)
			}
		}

	} else {
		// Check if the pods can reach each other via the IPs directly

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
	csrfToken := resp.Header.Get(csrfTokenName)
	if csrfToken == "" {
		csrfTokenName = "X-CSRF-TOKEN"
		csrfToken = resp.Header.Get(csrfTokenName)
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

func ConfigureElasticBackup(t *testing.T, cluster helpers.Cluster, clusterName, inputVersion string) {
	t.Logf("[ELASTICSEARCH] Configuring Elasticsearch backup for cluster %s", cluster.ClusterName)

	// Replace dots with dashes in the version string.
	version := strings.ReplaceAll(inputVersion, ".", "-")

	var output string
	var err error

	if helpers.isTeleportEnabled() {
		// Teleport mode: use BACKUP_BUCKET and BACKUP_NAME from the environment.
		output, err = k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--",
			"curl", "-XPUT", "http://localhost:9200/_snapshot/camunda_backup",
			"-H", "Content-Type: application/json",
			"-d", fmt.Sprintf("{\"type\": \"s3\", \"settings\": {\"bucket\": \"%s\", \"client\": \"camunda\", \"base_path\": \"%s/%s-backups\"}}",
				os.Getenv("BACKUP_BUCKET"), os.Getenv("BACKUP_NAME"), version))
	} else {
		// Default mode: use the provided clusterName and version.
		output, err = k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--",
			"curl", "-XPUT", "http://localhost:9200/_snapshot/camunda_backup",
			"-H", "Content-Type: application/json",
			"-d", fmt.Sprintf("{\"type\": \"s3\", \"settings\": {\"bucket\": \"%s-elastic-backup\", \"client\": \"camunda\", \"base_path\": \"%s-backups\"}}",
				clusterName, version))
	}

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

func CheckThatElasticBackupIsPresent(t *testing.T, cluster helpers.Cluster, backupName, clusterName, remoteChartVersion string) {
	t.Logf("[ELASTICSEARCH BACKUP] Checking that Elasticsearch backup is present for cluster %s", cluster.ClusterName)

	output := ""
	var err error

	for i := 0; i < 3; i++ {
		output, err = getAllElasticBackups(t, cluster)
		if err == nil && output != "" {
			break
		}
		removeElasticBackup(t, cluster)
		time.Sleep(5 * time.Second)
		ConfigureElasticBackup(t, cluster, clusterName, remoteChartVersion)
		time.Sleep(5 * time.Second)
	}

	require.Contains(t, output, backupName)
	t.Logf("[ELASTICSEARCH BACKUP] Backup present: %s", output)
}

func removeElasticBackup(t *testing.T, cluster helpers.Cluster) {
	t.Logf("[ELASTICSEARCH BACKUP] Backup not found, removing backup store to recreate %s", cluster.ClusterName)

	k8s.RunKubectl(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XDELETE", "localhost:9200/_snapshot/camunda_backup")
}

func getAllElasticBackups(t *testing.T, cluster helpers.Cluster) (string, error) {
	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XGET", "localhost:9200/_snapshot/camunda_backup/_all")
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return "", err
	}

	return output, nil
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

	if helpers.isTeleportEnabled() {
		// Set environment variables for the script
		os.Setenv("CAMUNDA_NAMESPACE_0", namespace0)
		os.Setenv("CAMUNDA_NAMESPACE_1", namespace1)
		os.Setenv("HELM_RELEASE_NAME", "camunda")
		os.Setenv("ZEEBE_CLUSTER_SIZE", "8")
	}

	// Run the script and capture its output
	cmd := exec.Command("bash", "-c", "../aws/dual-region/scripts/generate_zeebe_helm_values.sh")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("[C8 HELM] Error running script: %v\n", err)
		return
	}

	// Convert byte slice to string
	scriptOutput := string(output)

	// Extract the replacement text for the initial contact points and Elasticsearch URLs
	initialContact := extractReplacementText(scriptOutput, "ZEEBE_BROKER_CLUSTER_INITIALCONTACTPOINTS")
	elastic0 := extractReplacementText(scriptOutput, "ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION0_ARGS_URL")
	elastic1 := extractReplacementText(scriptOutput, "ZEEBE_BROKER_EXPORTERS_ELASTICSEARCHREGION1_ARGS_URL")

	require.NotEmpty(t, initialContact, "Initial contact points should not be empty")
	require.NotEmpty(t, elastic0, "Elasticsearch region 0 URL should not be empty")
	require.NotEmpty(t, elastic1, "Elasticsearch region 1 URL should not be empty")

	valuesFiles := []string{"../aws/dual-region/kubernetes/camunda-values.yml"}

	filePath := "../aws/dual-region/kubernetes/camunda-values.yml"
	valuesFiles = append(valuesFiles, fmt.Sprintf("../aws/dual-region/kubernetes/region%d/camunda-values.yml", region))

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("[C8 HELM] Error reading file: %v\n", err)
		return
	}

	// Convert byte slice to string
	fileContent := string(content)

	// Replace the placeholders with the replacement strings
	modifiedContent := strings.Replace(fileContent, "PLACEHOLDER", initialContact, -1)
	modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-primary.svc.cluster.local:9200", elastic0, -1)
	modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200", elastic1, -1)

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
func extractReplacementText(output, variableName string) string {
	startMarker := fmt.Sprintf("- name: %s\n  value: ", variableName)
	startIndex := strings.Index(output, startMarker)
	if startIndex == -1 {
		return ""
	}
	startIndex += len(startMarker)
	endIndex := strings.Index(output[startIndex:], "\n")
	if endIndex == -1 {
		return output[startIndex:]
	}
	return output[startIndex : startIndex+endIndex]
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

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 8080)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	// Get the topology of the Zeebe cluster
	code, body := http_helper.HttpGet(t, fmt.Sprintf("http://%s/v2/topology", tunnel.Endpoint()), nil)
	if code != 200 {
		t.Fatalf("[C8 CHECK] Failed to get topology: %s", body)
		return
	}

	var topology ClusterInfo

	err := json.Unmarshal([]byte(body), &topology)
	if err != nil {
		t.Fatalf("[C8 CHECK] Error unmarshalling JSON:", err)
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

	tunnel := k8s.NewTunnel(&primary.KubectlNamespace, k8s.ResourceTypeService, "camunda-zeebe-gateway", 0, 8080)
	defer tunnel.Close()
	tunnel.ForwardPort(t)

	file, err := os.Open(fmt.Sprintf("%s/single-task.bpmn", resourceDir))
	if err != nil {
		t.Fatalf("[C8 PROCESS] can't open file - %s", err)
		return
	}
	defer file.Close()

	reqBody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqBody)

	part, err := writer.CreateFormFile("resources", filepath.Base(file.Name()))
	if err != nil {
		t.Fatalf("[C8 PROCESS] can't create form file - %s", err)
		return
	}

	_, err = io.Copy(part, file)
	if err != nil {
		t.Fatalf("[C8 PROCESS] can't copy file - %s", err)
		return
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("[C8 PROCESS] can't close writer - %s", err)
		return
	}

	code, resBody := http_helper.HTTPDoWithOptions(t, http_helper.HttpDoOptions{
		Method:    "POST",
		Url:       fmt.Sprintf("http://%s/v2/deployments", tunnel.Endpoint()),
		Body:      reqBody,
		Headers:   map[string]string{"Content-Type": writer.FormDataContentType(), "Accept": "application/json"},
		TlsConfig: nil,
		Timeout:   30,
	})
	if code != 200 {
		t.Fatalf("[C8 PROCESS] Failed to deploy process: %s", resBody)
		return
	}

	t.Logf("[C8 PROCESS] Created process: %s", resBody)
	require.NotEmpty(t, resBody)
	require.Contains(t, resBody, "bigVarProcess")

	t.Log("[C8 PROCESS] Sleeping shortly to let process be propagated")
	time.Sleep(30 * time.Second)

	t.Log("[C8 PROCESS] Starting another Process instance 🚀")
	code, resBody = http_helper.HTTPDoWithOptions(t, http_helper.HttpDoOptions{
		Method:    "POST",
		Url:       fmt.Sprintf("http://%s/v2/process-instances", tunnel.Endpoint()),
		Body:      strings.NewReader("{\"processDefinitionId\":\"bigVarProcess\"}"),
		Headers:   map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
		TlsConfig: nil,
		Timeout:   30,
	})
	if code != 200 {
		t.Fatalf("[C8 PROCESS] Failed to start process instance: %s", resBody)
		return
	}

	t.Logf("[C8 PROCESS] Created process: %s", resBody)
	require.NotEmpty(t, resBody)
	require.Contains(t, resBody, "bigVarProcess")

	// check that was exported to ElasticSearch and available via Operate
	CheckOperateForProcesses(t, primary)
	CheckOperateForProcesses(t, secondary)
}

func DumpAllPodLogs(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Logf("[POD LOGS] Dumping logs for namespace %s", kubectlOptions.Namespace)

	// Temporarily disable logging to not overflow with all logs
	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard

	pods := k8s.ListPods(t, kubectlOptions, metav1.ListOptions{})

	for _, pod := range pods {
		t.Logf("[POD LOGS] Found pod %s", pod.Name)

		type containerInfo struct {
			name          string
			containerType string
		}
		var allContainers []containerInfo

		for _, container := range pod.Spec.InitContainers {
			t.Logf("[POD LOGS] Found init container %s in pod %s", container.Name, pod.Name)

			allContainers = append(allContainers, containerInfo{
				name:          container.Name,
				containerType: "init",
			})
		}

		for _, container := range pod.Spec.Containers {
			t.Logf("[POD LOGS] Found container %s in pod %s", container.Name, pod.Name)
			allContainers = append(allContainers, containerInfo{
				name:          container.Name,
				containerType: "",
			})
		}

		for _, container := range allContainers {
			t.Logf("[POD LOGS] Dumping logs for %s container %s in pod %s", container.containerType, container.name, pod.Name)

			podLogs, err := k8s.GetPodLogsE(t, kubectlOptions, &pod, container.name)
			if err != nil {
				t.Fatalf("Error getting pod logs: %v", err)
			}

			// Write logs to a file
			err = os.WriteFile(fmt.Sprintf("%s-%s-%s-%s.log", kubectlOptions.Namespace, pod.Name, container.containerType, container.name), []byte(podLogs), 0644)
			if err != nil {
				t.Fatalf("Error writing logs to file: %v", err)
			}
		}
	}
}
