package helpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eks_types "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AWS Helpers
func WaitForNodeGroup(region, clusterName, nodegroupName string) string {
	awsProfile := GetEnv("AWS_PROFILE", "infex")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		fmt.Println("[CLUSTER CHECK] Error creating session:", err)
		return err.Error()
	}

	client := eks.NewFromConfig(cfg)

	for i := 0; i < 20; i++ {
		resp, err := client.DescribeNodegroup(context.TODO(), &eks.DescribeNodegroupInput{
			ClusterName:   &clusterName,
			NodegroupName: &nodegroupName,
		})
		if err != nil {
			fmt.Println("[CLUSTER CHECK] Error describing nodegroup:", err)
			return err.Error()
		}

		if resp.Nodegroup.Status == eks_types.NodegroupStatus("ACTIVE") {
			fmt.Printf("[CLUSTER CHECK] Nodegroup %s in cluster %s is ready!\n", nodegroupName, clusterName)
			return string(resp.Nodegroup.Status)
		}

		fmt.Printf("[CLUSTER CHECK] Nodegroup %s in cluster %s is not ready yet. Waiting...\n", nodegroupName, clusterName)
		time.Sleep(30 * time.Second)
	}

	return ""
}

func WaitForCluster(region, clusterName string) string {
	awsProfile := GetEnv("AWS_PROFILE", "infex")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		fmt.Println("[CLUSTER CHECK] Error creating session:", err)
	}

	client := eks.NewFromConfig(cfg)

	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	for i := 0; i < 20; i++ {

		resp, err := client.DescribeCluster(context.TODO(), input)
		if err != nil {
			fmt.Println("[CLUSTER CHECK] Error describing cluster:", err)
			return err.Error()
		}

		if resp.Cluster.Status == eks_types.ClusterStatus("ACTIVE") {
			fmt.Printf("[CLUSTER CHECK] Cluster %s is ACTIVE\n", *resp.Cluster.Name)
			return string(resp.Cluster.Status)
		}

		time.Sleep(15 * time.Second)
	}

	return ""
}

func GetPrivateIPsForInternalLB(region, description string) []string {
	awsProfile := GetEnv("AWS_PROFILE", "infex")

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		fmt.Println("[DNS CHAINING] Error creating session:", err)
	}

	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2_types.Filter{
			{
				Name:   aws.String("description"),
				Values: []string{*aws.String(description)},
			},
		},
	}

	result, _ := client.DescribeNetworkInterfaces(context.TODO(), input)

	var privateIPs []string
	iterations := 0

	// It takes a while for the private IPs to be available
	// Therefore we loop 3 times over it with 15 seconds sleep
	for len(privateIPs) == 0 && iterations < 5 {
		for _, ni := range result.NetworkInterfaces {
			for _, addr := range ni.PrivateIpAddresses {
				privateIPs = append(privateIPs, *addr.PrivateIpAddress)
			}
		}
		iterations++
		fmt.Println("[DNS CHAINING] Private IPs not available yet. Waiting...")
		time.Sleep(15 * time.Second)

		result, _ = client.DescribeNetworkInterfaces(context.TODO(), input)
	}

	fmt.Println("[DNS CHAINING] Private IPs available: ", privateIPs)

	return privateIPs
}

// Struct Helper

type Cluster struct {
	Region           string
	ClusterName      string
	KubectlNamespace k8s.KubectlOptions
	KubectlSystem    k8s.KubectlOptions
	KubectlFailover  k8s.KubectlOptions
}

// Kubernetes Helpers

func CrossClusterCommunication(t *testing.T, withDNS bool, k8sManifests string, primary, secondary Cluster) {
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
			outputPrimary, errPrimary := k8s.RunKubectlAndGetOutputE(t, &primary.KubectlNamespace, "exec", podPrimary.Name, "--", "curl", "--max-time", "15", "sample-nginx.sample-nginx-peer.camunda-secondary.svc.cluster.local")
			outputSecondary, errSecondary := k8s.RunKubectlAndGetOutputE(t, &secondary.KubectlNamespace, "exec", podSecondary.Name, "--", "curl", "--max-time", "15", "sample-nginx.sample-nginx-peer.camunda-primary.svc.cluster.local")
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

func DNSChaining(t *testing.T, source, target Cluster, k8sManifests string) {

	t.Logf("[DNS CHAINING] applying from source %s to configure target %s", source.ClusterName, target.ClusterName)

	kubeResourcePath := fmt.Sprintf("%s/%s", k8sManifests, "internal-dns-lb.yml")

	k8s.KubectlApply(t, &source.KubectlSystem, kubeResourcePath)
	k8s.WaitUntilServiceAvailable(t, &source.KubectlSystem, "internal-dns-lb", 15, 6*time.Second)

	host := k8s.GetService(t, &source.KubectlSystem, "internal-dns-lb")
	hostName := strings.Split(host.Status.LoadBalancer.Ingress[0].Hostname, ".")
	hostName = strings.Split(hostName[0], "-")

	awsDescriptor := fmt.Sprintf("ELB net/%s/%s", hostName[0], hostName[1])
	require.NotEmpty(t, awsDescriptor)
	t.Logf("[DNS CHAINING] AWS Descriptor: %s", awsDescriptor)

	privateIPs := GetPrivateIPsForInternalLB(source.Region, awsDescriptor)

	require.NotEmpty(t, privateIPs)
	require.Greater(t, len(privateIPs), 1)

	// Just a check that the ConfigMap exists
	k8s.GetConfigMap(t, &target.KubectlSystem, "coredns")

	// Replace template placeholder for IPs
	t.Logf("[DNS CHAINING] Replacing CoreDNS ConfigMap with private IPs: %s", strings.Join(privateIPs, " "))
	filePath := fmt.Sprintf("%s/%s", k8sManifests, "coredns.yml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Convert byte slice to string
	fileContent := string(content)

	// Define the template and replacement string
	template := "PLACEHOLDER"
	replacement := fmt.Sprintf(`
    %s.svc.cluster.local:53 {
        errors
        cache 30
        forward . %s {
            force_tcp
        }
    }
    %s-failover.svc.cluster.local:53 {
        errors
        cache 30
        forward . %s {
            force_tcp
        }
    }`,
		source.KubectlNamespace.Namespace,
		strings.Join(privateIPs, " "),
		source.KubectlNamespace.Namespace,
		strings.Join(privateIPs, " "),
	)

	// Replace the template with the replacement string
	modifiedContent := strings.Replace(fileContent, template, replacement, -1)

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	// Apply the CoreDNS change to the target cluster to let it know how to reach the source cluster
	k8s.KubectlApply(t, &target.KubectlSystem, filePath)

	t.Log("[DNS CHAINING] Writing Placeholder CoreDNS ConfigMap back to file")
	// Write the old file back to the file - required for bidirectional communication
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
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

func CheckOperateForProcesses(t *testing.T, cluster Cluster) {
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

	var cookieAuth string
	for _, val := range resp.Cookies() {
		if val.Name == "OPERATE-SESSION" {
			cookieAuth = val.Value
		}
	}

	// create http client to add cookie to the request
	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/process-definitions/search", tunnelOperate.Endpoint()), strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("[C8 PROCESS] %s", err)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Cookie", fmt.Sprintf("OPERATE-SESSION=%s", cookieAuth))

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

func ConfigureElasticBackup(t *testing.T, cluster Cluster, clusterName string) {
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

func CreateElasticBackup(t *testing.T, cluster Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Creating Elasticsearch backup for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-X", "PUT", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s?wait_for_completion=true", backupName))
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Created backup: %s", output)
}

func CheckThatElasticBackupIsPresent(t *testing.T, cluster Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Checking that Elasticsearch backup is present for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XGET", "localhost:9200/_snapshot/camunda_backup/_all")
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, backupName)
	require.Contains(t, output, "\"total\":1")
	t.Logf("[ELASTICSEARCH BACKUP] Backup present: %s", output)
}

func RestoreElasticBackup(t *testing.T, cluster Cluster, backupName string) {
	t.Logf("[ELASTICSEARCH BACKUP] Restoring Elasticsearch backup for cluster %s", cluster.ClusterName)

	output, err := k8s.RunKubectlAndGetOutputE(t, &cluster.KubectlNamespace, "exec", "camunda-elasticsearch-master-0", "--", "curl", "-XPOST", fmt.Sprintf("localhost:9200/_snapshot/camunda_backup/%s/_restore?wait_for_completion=true", backupName))
	if err != nil {
		t.Fatalf("[ELASTICSEARCH BACKUP] %s", err)
		return
	}

	require.Contains(t, output, "\"failed\":0")
	t.Logf("[ELASTICSEARCH BACKUP] Restored backup: %s", output)

}

func InstallUpgradeC8Helm(t *testing.T, kubectlOptions *k8s.KubectlOptions, remoteChartVersion, remoteChartName, remoteChartSource string, region int, secrets, upgrade, failover, esSwitch bool, setValues map[string]string) {
	zeebeContactPoints := ""

	for i := 0; i < 4; i++ {
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.camunda-primary.svc.cluster.local:26502,", strconv.Itoa((i)))
		zeebeContactPoints += fmt.Sprintf("camunda-zeebe-%s.camunda-zeebe.camunda-secondary.svc.cluster.local:26502,", strconv.Itoa((i)))
	}

	// Cut the last character "," from the string
	zeebeContactPoints = zeebeContactPoints[:len(zeebeContactPoints)-1]

	valuesFiles := []string{"./resources/aws/2-region/kubernetes/camunda-values.yml"}

	filePath := "./resources/aws/2-region/kubernetes/camunda-values.yml"
	if failover {
		filePath = fmt.Sprintf("./resources/aws/2-region/kubernetes/region%d/camunda-values-failover.yml", region)

		valuesFiles = append(valuesFiles, filePath)
	} else {
		valuesFiles = append(valuesFiles, fmt.Sprintf("./resources/aws/2-region/kubernetes/region%d/camunda-values.yml", region))
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

	if esSwitch && !failover {
		modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200", "http://camunda-elasticsearch-master-hl.camunda-primary-failover.svc.cluster.local:9200", -1)
	}
	if esSwitch && failover {
		modifiedContent = strings.Replace(modifiedContent, "http://camunda-elasticsearch-master-hl.camunda-primary-failover.svc.cluster.local:9200", "http://camunda-elasticsearch-master-hl.camunda-secondary.svc.cluster.local:9200", -1)
	}

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("[C8 HELM] Error writing file: %v\n", err)
		return
	}

	// Get Secrets required for some upgrades
	if secrets && upgrade {
		secretConnectors := k8s.GetSecret(t, kubectlOptions, "camunda-connectors-auth-credentials")
		setValues["connectors.inbound.auth.existingSecret"] = string(secretConnectors.Data["connectors-secret"])
	}

	helmOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
		Version:        remoteChartVersion,
		ValuesFiles:    valuesFiles,
		SetValues:      setValues,
	}

	helm.AddRepo(t, helmOptions, "camunda", remoteChartSource)

	if upgrade {
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

	return cutOutString(output, "ZEEBE_BROKER_CLUSTER_NODEID=[0-9]")
}

// Go Helpers
func GetEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func cutOutString(originalString, searchString string) int {
	re := regexp.MustCompile(searchString)
	matches := re.FindStringSubmatch(originalString)

	if (len(matches)) == 0 {
		return -1
	}

	length := len(matches[0])
	if (length) == 0 {
		return -1
	}

	num, err := strconv.Atoi(string(matches[0][length-1]))
	if err != nil {
		return -1
	}

	return num
}

func IsEven(num int) bool {
	if num < 0 {
		return false
	}

	return num%2 == 0
}

func IsOdd(num int) bool {
	if num < 0 {
		return false
	}

	return num%2 == 1
}

// Terraform Helpers
func FetchSensitiveTerraformOutput(t *testing.T, options *terraform.Options, name string) string {
	defer func() {
		options.Logger = nil
	}()
	options.Logger = logger.Discard
	return terraform.Output(t, options, name)
}
