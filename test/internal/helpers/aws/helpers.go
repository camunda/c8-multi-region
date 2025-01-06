package awsHelpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eks_types "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"
)

// AWS Helpers
func WaitForNodeGroup(region, clusterName, nodegroupName string) string {
	awsProfile := helpers.GetEnv("AWS_PROFILE", "infex")

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
	awsProfile := helpers.GetEnv("AWS_PROFILE", "infex")

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
	awsProfile := helpers.GetEnv("AWS_PROFILE", "infex")

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

func CreateLoadBalancers(t *testing.T, source helpers.Cluster, k8sManifests string) {
	t.Logf("[LOAD BALANCER] Creating load balancer for source cluster %s", source.ClusterName)

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

	t.Logf("[LOAD BALANCER] Private IPs: %v", privateIPs)
}
func DNSChaining(t *testing.T, source, target helpers.Cluster, k8sManifests, namespacesPrimary, namespacesSecondary string) {
	// Split the namespace arrays
	namespacesPrimaryArr := strings.Split(namespacesPrimary, ",")
	namespacesSecondaryArr := strings.Split(namespacesSecondary, ",")

	// Ensure both arrays have the same length
	if len(namespacesPrimaryArr) != len(namespacesSecondaryArr) {
		t.Fatalf("Namespace arrays must have the same length")
		return
	}

	var cluster0DnsEntries string
	var cluster1DnsEntries string

	for i := 0; i < len(namespacesPrimaryArr); i++ {
		nsP := namespacesPrimaryArr[i]
		nsS := namespacesSecondaryArr[i]

		// Set environment variables for the script
		os.Setenv("CLUSTER_0", source.ClusterName)
		os.Setenv("CLUSTER_1", target.ClusterName)
		os.Setenv("CAMUNDA_NAMESPACE_0", nsP)
		os.Setenv("CAMUNDA_NAMESPACE_1", nsS)
		os.Setenv("REGION_0", source.Region)
		os.Setenv("REGION_1", target.Region)

		// Run the script and capture its output
		output := shell.RunCommandAndGetOutput(t, shell.Command{
			Command: "sh",
			Args: []string{
				"../aws/dual-region/scripts/generate_core_dns_entry.sh",
			},
		})

		t.Logf("[DNS CHAINING] Output from script: %s", output)

		// Extract the replacement text for "Cluster 0"
		cluster0DNSEntry := extractReplacementText(output, "Cluster 0")
		t.Logf("[DNS CHAINING] Replacement text for Cluster 0: %s", cluster0DNSEntry)

		// Accumulate the replacement texts with proper indentation
		cluster0DnsEntries += formatReplacementText(cluster0DNSEntry)

		// Extract the replacement text for "Cluster 1"
		cluster1DNSEntry := extractReplacementText(output, "Cluster 1")
		t.Logf("[DNS CHAINING] Replacement text for Cluster 1: %s", cluster1DNSEntry)

		// Accumulate the replacement texts with proper indentation
		cluster1DnsEntries += formatReplacementText(cluster1DNSEntry)
	}

	// Generate and apply the CoreDNS manifest
	generateAndApplyCoreDNSManifest(t, source, k8sManifests, cluster0DnsEntries)
	generateAndApplyCoreDNSManifest(t, target, k8sManifests, cluster1DnsEntries)

}

func generateAndApplyCoreDNSManifest(t *testing.T, target helpers.Cluster, k8sManifests, dnsEntries string) {
	// Replace the placeholder text in the existing CoreDNS ConfigMap
	t.Logf("[DNS CHAINING] Replacing PLACEHOLDER in CoreDNS ConfigMap with generated replacement text")
	filePath := fmt.Sprintf("%s/%s", k8sManifests, "coredns.yml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Error reading file: %v\n", err)
		return
	}

	// Convert byte slice to string
	fileContent := string(content)

	// Replace the placeholder with the accumulated replacement string
	modifiedContent := strings.Replace(fileContent, "PLACEHOLDER", dnsEntries, -1)

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("Error writing file: %v\n", err)
		return
	}

	// Apply the CoreDNS change to the target cluster to let it know how to reach the source cluster
	k8s.KubectlApply(t, &target.KubectlSystem, filePath)

	t.Log("[DNS CHAINING] Writing Placeholder CoreDNS ConfigMap back to file")
	// Write the old file back to the file - required for bidirectional communication
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		t.Fatalf("Error writing file: %v\n", err)
		return
	}
}

func extractReplacementText(output, clusterMarker string) string {
	startMarker := fmt.Sprintf("### %s - Start ###\n", clusterMarker)
	endMarker := fmt.Sprintf("\n### %s - End ###", clusterMarker)
	startIndex := strings.Index(output, startMarker)
	if startIndex == -1 {
		return ""
	}
	startIndex += len(startMarker)
	endIndex := strings.Index(output[startIndex:], endMarker)
	if endIndex == -1 {
		return ""
	}
	return strings.TrimSpace(output[startIndex : startIndex+endIndex])
}

func formatReplacementText(replacement string) string {
	lines := strings.Split(replacement, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func ClusterReadyCheck(t *testing.T, cluster helpers.Cluster) {
	clusterStatus := WaitForCluster(cluster.Region, cluster.ClusterName)

	require.Equal(t, "ACTIVE", clusterStatus)

	nodeGroupStatus := WaitForNodeGroup(cluster.Region, cluster.ClusterName, "services")

	require.Equal(t, "ACTIVE", nodeGroupStatus)
}

func TestSetupTerraform(t *testing.T, terraformDir, clusterName, awsProfile, tfBinary string) {
	CI := helpers.GetEnv("CI", "false") // always true on GHA
	np_desired_node_count := 4

	if CI == "true" {
		np_desired_node_count = 5
	}

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformBinary: tfBinary,
		TerraformDir:    terraformDir,
		Vars: map[string]interface{}{
			"cluster_name":          clusterName,
			"aws_profile":           awsProfile,
			"np_desired_node_count": np_desired_node_count,
			// Disabling spot instances for now since tests have become very flakey
			// "np_capacity_type":  "SPOT",
			// "np_instance_types": []string{"m6i.xlarge", "m5.xlarge", "m5d.xlarge"},
		},
		NoColor: true,
	})

	terraform.InitAndApply(t, terraformOptions)
}

func GenerateAWSKubeConfig(t *testing.T, clusterName, awsProfile, awsRegion, regionName string) {
	t.Log("[TF SETUP] Generating kubeconfig files 📜")

	cmd := exec.Command("aws", "eks", "--region", awsRegion, "update-kubeconfig", "--name", fmt.Sprintf("%s-%s", clusterName, regionName), "--profile", awsProfile, "--kubeconfig", fmt.Sprintf("kubeconfig-%s", regionName))

	_, err := cmd.Output()
	if err != nil {
		t.Fatalf("[TF SETUP] could not run command: %v", err)
		return
	}

	require.FileExists(t, fmt.Sprintf("kubeconfig-%s", regionName), fmt.Sprintf("kubeconfig-%s file does not exist", regionName))
}

func TestTeardownTerraform(t *testing.T, terraformDir, clusterName, awsProfile, tfBinary string) {
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformBinary: tfBinary,
		TerraformDir:    terraformDir,
		Vars: map[string]interface{}{
			"cluster_name": clusterName,
			"aws_profile":  awsProfile,
			// Disabling spot instances for now since tests have become very flakey
			// "np_capacity_type":  "SPOT",
			// "np_instance_types": []string{"m6i.xlarge", "m5.xlarge", "m5d.xlarge"},
		},
		NoColor: true,
	})

	terraform.Init(t, terraformOptions)
	terraform.Destroy(t, terraformOptions)
}

func TestRemoveKubeConfig(t *testing.T, regionName string) {
	os.Remove(fmt.Sprintf("kubeconfig-%s", regionName))
	require.NoFileExists(t, fmt.Sprintf("kubeconfig-%s", regionName), fmt.Sprintf("kubeconfig-%s file still exists", regionName))
}
