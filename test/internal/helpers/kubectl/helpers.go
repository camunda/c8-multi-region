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

// basicAuthDemoHeader is the precomputed base64 for credentials demo:demo -> echo -n 'demo:demo' | base64
// Used only for test/demo authentication against the Camunda components.
const basicAuthDemoHeader = "Basic ZGVtbzpkZW1v"

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

type ElasticsearchClusterHealth struct {
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"`
	TimedOut    bool   `json:"timed_out"`
}

// NewServiceTunnelWithRetry establishes a port-forward tunnel to a Kubernetes Service with retry logic.
// Parameters:
//
//	t: *testing.T
//	kubectlOptions: target kubectl options
//	serviceName: name of the Service (must exist)
//	localPort: local port to bind (0 lets kubectl choose a random free port)
//	remotePort: target Service port
//	maxRetries: how many times to retry establishing the tunnel
//	backoff: sleep duration between retries
//
// Returns: (endpoint string, close func())
func NewServiceTunnelWithRetry(t *testing.T, kubectlOptions *k8s.KubectlOptions, serviceName string, localPort, remotePort, maxRetries int, backoff time.Duration) (string, func()) {
	t.Helper()

	// Ensure service exists early (gives clearer error)
	svc := k8s.GetService(t, kubectlOptions, serviceName)
	require.Equal(t, serviceName, svc.Name)

	if maxRetries < 1 {
		maxRetries = 1
	}
	if backoff <= 0 {
		backoff = 5 * time.Second
	}

	tunnel := k8s.NewTunnel(kubectlOptions, k8s.ResourceTypeService, serviceName, localPort, remotePort)
	var err error
	for i := 0; i < maxRetries; i++ {
		err = tunnel.ForwardPortE(t)
		if err == nil {
			break
		}
		t.Logf("[TUNNEL] port-forward attempt %d/%d failed for %s:%d -> %s: %v", i+1, maxRetries, serviceName, remotePort, kubectlOptions.Namespace, err)
		if i < maxRetries-1 {
			time.Sleep(backoff)
		}
	}
	if err != nil {
		t.Fatalf("[TUNNEL] failed to establish port-forward after %d attempts: %v", maxRetries, err)
		return "", func() {}
	}

	cleanup := func() { tunnel.Close() }
	return tunnel.Endpoint(), cleanup
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

func CheckOperateForProcesses(t *testing.T, cluster helpers.Cluster, tenantId string) {
	t.Logf("[C8 PROCESS] Checking for Cluster %s whether Operate contains deployed processes", cluster.ClusterName)

	endpoint, closeFn := NewServiceTunnelWithRetry(t, &cluster.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 15*time.Second)
	defer closeFn()

	// create http client
	client := &http.Client{}

	// Prepare request body with tenantId if provided
	var instanceRequestBody string
	if tenantId != "" {
		instanceRequestBody = fmt.Sprintf(`{"filter": { "tenantId":"%s" }}`, tenantId)
	} else {
		instanceRequestBody = `{}`
	}

	var bodyString string
	for i := 0; i < 8; i++ {
		// fresh request each iteration to avoid reusing consumed body
		req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v2/process-definitions/search", endpoint), strings.NewReader(instanceRequestBody))
		if err != nil {
			t.Fatalf("[C8 PROCESS] %s", err)
			return
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", basicAuthDemoHeader)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("[C8 PROCESS] %s", err)
			return
		}

		body, err := io.ReadAll(res.Body)
		closeErr := res.Body.Close()
		if err != nil {
			t.Fatalf("[C8 PROCESS] %s", err)
			return
		}
		if closeErr != nil {
			t.Fatalf("[C8 PROCESS] close body: %v", closeErr)
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

func CheckOperateForProcessInstances(t *testing.T, cluster helpers.Cluster, size int, tenantId string) {

	t.Logf("[C8 PROCESS INSTANCES] Checking for Cluster %s whether instances of bigVarProcess are created", cluster.ClusterName)

	endpoint, closeFn := NewServiceTunnelWithRetry(t, &cluster.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	// create http client
	client := &http.Client{}

	// Prepare request body with tenantId if provided
	var instanceRequestBody string
	if tenantId != "" {
		instanceRequestBody = fmt.Sprintf(`{"filter": { "tenantId":"%s" }}`, tenantId)
	} else {
		instanceRequestBody = `{}`
	}

	var bodyString string
	for i := 0; i < 8; i++ {
		// fresh request each iteration to avoid reusing consumed body
		req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v2/process-instances/search", endpoint), strings.NewReader(instanceRequestBody))
		if err != nil {
			t.Fatalf("[C8 PROCESS INSTANCES] %s", err)
			return
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", basicAuthDemoHeader)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("[C8 PROCESS INSTANCES] %s", err)
			return
		}

		body, err := io.ReadAll(res.Body)
		closeErr := res.Body.Close()
		if err != nil {
			t.Fatalf("[C8 PROCESS INSTANCES] %s", err)
			return
		}
		if closeErr != nil {
			t.Fatalf("[C8 PROCESS INSTANCES] close body: %v", closeErr)
			return
		}

		bodyString = string(body)

		t.Logf("[C8 Process INSTANCES] %s", bodyString)
		if !strings.Contains(bodyString, "\"totalItems\":0") {
			t.Log("[C8 PROCESS INSTANCES] processes are present, breaking and asserting")
			break
		}
		t.Log("[C8 PROCESS INSTANCES] not imported yet, waiting...")
		time.Sleep(15 * time.Second)
	}

	require.Contains(t, bodyString, "Big variable process")
	require.Contains(t, bodyString, "bigVarProcess")
	require.Contains(t, bodyString, fmt.Sprintf("\"totalItems\":%d", size))
}

func RunSensitiveKubectlCommand(t *testing.T, kubectlOptions *k8s.KubectlOptions, command ...string) {
	defer func() {
		kubectlOptions.Logger = nil
	}()
	kubectlOptions.Logger = logger.Discard
	k8s.RunKubectl(t, kubectlOptions, command...)
}

func ConfigureElasticBackup(t *testing.T, cluster helpers.Cluster, backupBucket, inputVersion string) {
	t.Logf("[ELASTICSEARCH] Configuring Elasticsearch backup for cluster %s", cluster.ClusterName)

	// Replace dots with dashes in the version string.
	version := strings.ReplaceAll(inputVersion, ".", "-")

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--",
		"curl", "-XPUT", "http://localhost:9200/_snapshot/camunda_backup",
		"-H", "Content-Type: application/json",
		"-d", fmt.Sprintf("{\"type\": \"s3\", \"settings\": {\"bucket\": \"%s\", \"client\": \"camunda\", \"base_path\": \"%s/%s-backups\"}}",
			backupBucket, backupBucket, version))

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

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-X", "PUT", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s?wait_for_completion=true", backupName), "-H", "Content-Type: application/json", "-d", `{"include_global_state":true}`)
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Created backup: %s", output)
}

func CheckThatElasticBackupIsPresent(t *testing.T, cluster helpers.Cluster, backupName, backupBucket, remoteChartVersion string) {
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
		ConfigureElasticBackup(t, cluster, backupBucket, remoteChartVersion)
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

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XPOST", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s/_restore?wait_for_completion=true", backupName), "-H", "Content-Type: application/json", "-d", `{"include_global_state":true}`)
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Restored backup: %s", output)

}

func InstallUpgradeC8Helm(t *testing.T, kubectlOptions *k8s.KubectlOptions, remoteChartVersion, remoteChartName, remoteChartSource, namespace0, namespace1 string, valuesYamlFiles []string, region int, setValues, setStringValues map[string]string) {

	if !helpers.IsTeleportEnabled() {
		// Set environment variables for the script
		os.Setenv("CAMUNDA_NAMESPACE_0", namespace0)
		os.Setenv("CAMUNDA_NAMESPACE_1", namespace1)
		os.Setenv("CAMUNDA_RELEASE_NAME", "camunda")
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
	elastic0 := extractReplacementText(scriptOutput, "ZEEBE_BROKER_EXPORTERS_CAMUNDAREGION0_ARGS_CONNECT_URL")
	elastic1 := extractReplacementText(scriptOutput, "ZEEBE_BROKER_EXPORTERS_CAMUNDAREGION1_ARGS_CONNECT_URL")

	require.NotEmpty(t, initialContact, "Initial contact points should not be empty")
	require.NotEmpty(t, elastic0, "Elasticsearch region 0 URL should not be empty")
	require.NotEmpty(t, elastic1, "Elasticsearch region 1 URL should not be empty")

	valuesFiles := valuesYamlFiles

	if helpers.IsTeleportEnabled() {
		valuesFiles = append(valuesFiles, "./fixtures/teleport-affinities-tolerations.yml")
	}

	filePath := valuesFiles[0]

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
		SetStrValues:   setStringValues,
	}

	if !strings.Contains(remoteChartVersion, "snapshot") {
		helm.AddRepo(t, helmOptions, "camunda", remoteChartSource)
	}

	// Terratest is actively ignoring the version in an upgrade
	helmOptions.ExtraArgs = map[string][]string{
		"upgrade": {"--version", remoteChartVersion, "--install"},
	}
	helm.Upgrade(t, helmOptions, remoteChartName, "camunda")

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

	return helpers.CutOutString(output, "ORCHESTRATION_NODE_ID=[0-9]")
}

func CheckC8RunningProperly(t *testing.T, primary helpers.Cluster, namespace0, namespace1 string) {
	endpoint, closeFn := NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	// Get the topology of the Zeebe cluster
	code, body := http_helper.HTTPDoWithOptions(t, http_helper.HttpDoOptions{
		Method: "GET",
		Url:    fmt.Sprintf("http://%s/v2/topology", endpoint),
		Headers: map[string]string{
			"Authorization": basicAuthDemoHeader,
			"Accept":        "application/json",
		},
		TlsConfig: nil,
		Timeout:   30,
	})
	if code != 200 {
		t.Fatalf("[C8 CHECK] Failed to get topology: %s", body)
		return
	}

	var topology ClusterInfo

	err := json.Unmarshal([]byte(body), &topology)
	if err != nil {
		t.Fatalf("[C8 CHECK] Error unmarshalling JSON: %v", err)
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

func DeployC8processAndCheck(t *testing.T, kubectlOptions helpers.Cluster, resourceDir, tenantId string) {
	endpoint, closeFn := NewServiceTunnelWithRetry(t, &kubectlOptions.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

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

	// Add tenantId field if provided
	if tenantId != "" {
		err = writer.WriteField("tenantId", tenantId)
		if err != nil {
			t.Fatalf("[C8 PROCESS] can't write tenantId field - %s", err)
			return
		}
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("[C8 PROCESS] can't close writer - %s", err)
		return
	}

	code, resBody := http_helper.HTTPDoWithOptions(t, http_helper.HttpDoOptions{
		Method: "POST",
		Url:    fmt.Sprintf("http://%s/v2/deployments", endpoint),
		Body:   reqBody,
		Headers: map[string]string{
			"Content-Type":  writer.FormDataContentType(),
			"Accept":        "application/json",
			"Authorization": basicAuthDemoHeader,
		},
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

	const instancesToStart = 6

	for i := 1; i <= instancesToStart; i++ {
		t.Logf("[C8 PROCESS] Starting Process instance %d/%d 🚀", i, instancesToStart)

		// Prepare request body with tenantId if provided
		var instanceRequestBody string
		if tenantId != "" {
			instanceRequestBody = fmt.Sprintf(`{"processDefinitionId":"bigVarProcess","tenantId":"%s"}`, tenantId)
		} else {
			instanceRequestBody = `{"processDefinitionId":"bigVarProcess"}`
		}

		code, resBody = http_helper.HTTPDoWithOptions(t, http_helper.HttpDoOptions{
			Method: "POST",
			Url:    fmt.Sprintf("http://%s/v2/process-instances", endpoint),
			Body:   strings.NewReader(instanceRequestBody),
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Accept":        "application/json",
				"Authorization": basicAuthDemoHeader,
			},
			TlsConfig: nil,
			Timeout:   30,
		})
		if code != 200 {
			t.Fatalf("[C8 PROCESS] Failed to start process instance (%d): %s", i, resBody)
			return
		}
		t.Logf("[C8 PROCESS] Created process instance %d: %s", i, resBody)
		require.NotEmpty(t, resBody)
		require.Contains(t, resBody, "bigVarProcess")
	}

	t.Log("[C8 PROCESS] Sleeping shortly to let instances be propagated")
	time.Sleep(30 * time.Second)
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

// CreateTenant creates a tenant via the Camunda API
func CreateTenant(t *testing.T, cluster helpers.Cluster, tenantId, name, description string) {
	t.Logf("[TENANT] Creating tenant '%s' in cluster %s", tenantId, cluster.ClusterName)

	endpoint, closeFn := NewServiceTunnelWithRetry(t, &cluster.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	// Prepare request body
	requestBody := fmt.Sprintf(`{
  "tenantId": "%s",
  "name": "%s",
  "description": "%s"
}`, tenantId, name, description)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v2/tenants", endpoint), strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("[TENANT] Failed to create request: %v", err)
		return
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", basicAuthDemoHeader)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("[TENANT] Failed to create tenant: %v", err)
		return
	}

	body, err := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if err != nil {
		t.Fatalf("[TENANT] Failed to read response: %v", err)
		return
	}
	if closeErr != nil {
		t.Fatalf("[TENANT] Failed to close body: %v", closeErr)
		return
	}

	if res.StatusCode != 200 && res.StatusCode != 201 {
		t.Fatalf("[TENANT] Failed to create tenant (status %d): %s", res.StatusCode, string(body))
		return
	}

	t.Logf("[TENANT] Successfully created tenant: %s", string(body))
	require.Contains(t, string(body), tenantId)
}

// AssignRoleToTenant assigns a role to a tenant via the Camunda API
func AssignRoleToTenant(t *testing.T, cluster helpers.Cluster, tenantId, roleID string) {
	t.Logf("[TENANT] Assigning role '%s' to tenant '%s' in cluster %s", roleID, tenantId, cluster.ClusterName)

	endpoint, closeFn := NewServiceTunnelWithRetry(t, &cluster.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/v2/tenants/%s/roles/%s", endpoint, tenantId, roleID), nil)
	if err != nil {
		t.Fatalf("[TENANT] Failed to create request: %v", err)
		return
	}

	req.Header.Add("Authorization", basicAuthDemoHeader)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("[TENANT] Failed to assign role: %v", err)
		return
	}

	body, err := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if err != nil {
		t.Fatalf("[TENANT] Failed to read response: %v", err)
		return
	}
	if closeErr != nil {
		t.Fatalf("[TENANT] Failed to close body: %v", closeErr)
		return
	}

	if res.StatusCode != 200 && res.StatusCode != 204 {
		t.Fatalf("[TENANT] Failed to assign role (status %d): %s", res.StatusCode, string(body))
		return
	}

	t.Logf("[TENANT] Successfully assigned role '%s' to tenant '%s'", roleID, tenantId)
}

// CheckTenantExists verifies that a tenant exists via the Camunda API
func CheckTenantExists(t *testing.T, cluster helpers.Cluster, tenantId string) {
	t.Logf("[TENANT] Checking if tenant '%s' exists in cluster %s", tenantId, cluster.ClusterName)

	endpoint, closeFn := NewServiceTunnelWithRetry(t, &cluster.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/v2/tenants/%s", endpoint, tenantId), nil)
	if err != nil {
		t.Fatalf("[TENANT] Failed to create request: %v", err)
		return
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", basicAuthDemoHeader)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("[TENANT] Failed to check tenant: %v", err)
		return
	}

	body, err := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if err != nil {
		t.Fatalf("[TENANT] Failed to read response: %v", err)
		return
	}
	if closeErr != nil {
		t.Fatalf("[TENANT] Failed to close body: %v", closeErr)
		return
	}

	if res.StatusCode != 200 {
		t.Fatalf("[TENANT] Tenant not found (status %d): %s", res.StatusCode, string(body))
		return
	}

	bodyString := string(body)
	t.Logf("[TENANT] Tenant exists: %s", bodyString)
	require.Contains(t, bodyString, tenantId)
}

// CheckElasticsearchClusterHealth verifies that the Elasticsearch cluster health is green
func CheckElasticsearchClusterHealth(t *testing.T, cluster helpers.Cluster) {
	t.Logf("[ELASTICSEARCH HEALTH] Checking cluster health for %s", cluster.ClusterName)

	var output string
	var err error

	// Retry up to 10 times with 15 second intervals to allow for cluster stabilization
	for i := 0; i < 10; i++ {
		output, err = k8s.RunKubectlAndGetOutputE(
			t,
			&cluster.KubectlNamespace,
			"exec",
			"camunda-elasticsearch-master-0",
			"-c",
			"elasticsearch",
			"--",
			"curl",
			"-s",
			"localhost:9200/_cluster/health",
		)

		if err != nil {
			t.Logf("[ELASTICSEARCH HEALTH] Attempt %d/10: Error executing command: %v", i+1, err)
			if i < 9 {
				time.Sleep(15 * time.Second)
				continue
			}
			t.Fatalf("[ELASTICSEARCH HEALTH] Failed to get cluster health after 10 attempts: %v", err)
			return
		}

		// Parse the JSON response
		var health ElasticsearchClusterHealth
		if err := json.Unmarshal([]byte(output), &health); err != nil {
			t.Logf("[ELASTICSEARCH HEALTH] Attempt %d/10: Failed to parse response: %v", i+1, err)
			if i < 9 {
				time.Sleep(15 * time.Second)
				continue
			}
			t.Fatalf("[ELASTICSEARCH HEALTH] Failed to parse health response after 10 attempts: %v", err)
			return
		}

		t.Logf("[ELASTICSEARCH HEALTH] Attempt %d/10: Status = %s", i+1, health.Status)

		// Check if status is green (case-insensitive)
		if strings.ToLower(health.Status) == "green" {
			t.Logf("[ELASTICSEARCH HEALTH] Cluster health is green for %s", cluster.ClusterName)
			require.False(t, health.TimedOut, "Health check should not time out")
			return
		}

		if i < 9 {
			t.Logf("[ELASTICSEARCH HEALTH] Status is %s, waiting for green status...", health.Status)
			time.Sleep(15 * time.Second)
		}
	}

	t.Fatalf("[ELASTICSEARCH HEALTH] Cluster did not reach green status after 10 attempts. Last output: %s", output)
}
