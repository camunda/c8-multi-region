package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
)

// TestAWSClusterScaling_ScaleBrokers tests scaling Zeebe brokers in a multi-region setup
// Initial state: 8 brokers (4 per region), 8 partitions
// Target state: 12 brokers (6 per region), 8 partitions
// Reference: https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/
func TestAWSClusterScaling_ScaleBrokers(t *testing.T) {
	t.Log("[CLUSTER SCALING TEST] Testing Zeebe broker scaling in multi-region mode 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestVerifyClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 8, 8) }},
		{"TestScaleUpBrokerStatefulSets", func(t *testing.T) { scaleUpBrokerStatefulSets(t, 6) }},
		{"TestWaitForNewBrokersToStart", waitForNewBrokersToStart},
		{"TestAddNewBrokersToCluster", func(t *testing.T) { addNewBrokersToCluster(t, []int{8, 9, 10, 11}) }},
		{"TestWaitForBrokerScalingComplete", waitForBrokerScalingComplete},
		{"TestVerifyScaledBrokerTopology", func(t *testing.T) { verifyClusterTopology(t, 12, 8) }},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// TestAWSClusterScaling_ScalePartitions tests scaling partitions in a multi-region setup
// Initial state: 8 brokers (4 per region), 8 partitions
// Target state: 8 brokers (4 per region), 12 partitions
// Reference: https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/
func TestAWSClusterScaling_ScalePartitions(t *testing.T) {
	t.Log("[CLUSTER SCALING TEST] Testing Zeebe partition scaling in multi-region mode 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestVerifyClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 12, 8) }},
		{"TestScaleUpPartitions", func(t *testing.T) { scaleUpPartitions(t, 12, 4) }},
		{"TestWaitForPartitionScalingComplete", waitForPartitionScalingComplete},
		{"TestVerifyScaledPartitionTopology", func(t *testing.T) { verifyClusterTopology(t, 12, 12) }},
		{"TestCheckElasticsearchClusterHealth", checkElasticsearchClusterHealth},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// TestAWSClusterScaling_ScaleBrokersAndPartitions tests scaling both brokers and partitions simultaneously
// Initial state: 8 brokers (4 per region), 8 partitions
// Target state: 12 brokers (6 per region), 12 partitions
// Reference: https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/
func TestAWSClusterScaling_ScaleBrokersAndPartitions(t *testing.T) {
	t.Log("[CLUSTER SCALING TEST] Testing Zeebe broker and partition scaling in multi-region mode 🚀")

	if globalImageTag != "" {
		t.Log("[GLOBAL IMAGE TAG] Overwriting image tag for all Camunda images with " + globalImageTag)
		baseHelmVars = helpers.OverwriteImageTag(baseHelmVars, globalImageTag)
	}

	// Runs the tests sequentially
	for _, testFuncs := range []struct {
		name  string
		tfunc func(*testing.T)
	}{
		{"TestInitKubernetesHelpers", initKubernetesHelpers},
		{"TestVerifyClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 12, 12) }},
		{"TestScaleUpBrokerStatefulSets", func(t *testing.T) { scaleUpBrokerStatefulSets(t, 7) }},
		{"TestWaitForNewBrokersToStart", waitForNewBrokersToStart},
		{"TestScaleUpBrokersAndPartitions", func(t *testing.T) { scaleUpBrokersAndPartitions(t, []int{12, 13}, 14, 4) }},
		{"TestWaitForCombinedScalingComplete", waitForCombinedScalingComplete},
		{"TestVerifyScaledClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 14, 14) }},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// Helper functions for cluster scaling tests

// verifyClusterTopology verifies the cluster has the expected broker and partition counts
func verifyClusterTopology(t *testing.T, clusterSizeExpected, partitionCountExpected int) {
	t.Helper()
	t.Logf("[SCALING] Verifying cluster topology: expecting %d brokers and %d partitions 🔍", clusterSizeExpected, partitionCountExpected)

	clusterInfo := getClusterTopology(t)
	require.Equal(t, clusterSizeExpected, clusterInfo.ClusterSize, "Expected %d brokers", clusterSizeExpected)
	require.Equal(t, partitionCountExpected, clusterInfo.PartitionsCount, "Expected %d partitions", partitionCountExpected)

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions, replication factor %d",
		clusterInfo.ClusterSize, clusterInfo.PartitionsCount, clusterInfo.ReplicationFactor)
}

// scaleUpBrokerStatefulSets scales the Zeebe StatefulSets to the specified number of replicas in each region
func scaleUpBrokerStatefulSets(t *testing.T, replicasPerRegion int) {
	t.Helper()
	t.Logf("[SCALING] Scaling up Zeebe StatefulSets to %d replicas in both regions 🚀", replicasPerRegion)

	replicasArg := fmt.Sprintf("--replicas=%d", replicasPerRegion)

	t.Logf("[SCALING] Scaling primary region StatefulSet to %d replicas", replicasPerRegion)
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", replicasArg)

	t.Logf("[SCALING] Scaling secondary region StatefulSet to %d replicas", replicasPerRegion)
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", replicasArg)

	t.Log("[SCALING] StatefulSets scaled up successfully")
}

// waitForNewBrokersToStart waits for the new broker pods to be ready
func waitForNewBrokersToStart(t *testing.T) {
	t.Log("[SCALING] Waiting for new broker pods to start 🕐")

	// Wait for new pods in primary region (camunda-zeebe-4 and camunda-zeebe-5)
	t.Log("[SCALING] Waiting for primary region pods to be ready")
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-4", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "camunda-zeebe-5", 20, 15*time.Second)

	// Wait for new pods in secondary region (camunda-zeebe-4 and camunda-zeebe-5)
	t.Log("[SCALING] Waiting for secondary region pods to be ready")
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-4", 20, 15*time.Second)
	k8s.WaitUntilPodAvailable(t, &secondary.KubectlNamespace, "camunda-zeebe-5", 20, 15*time.Second)

	t.Log("[SCALING] All new broker pods are ready")
}

// addNewBrokersToCluster sends API request to add new brokers to the cluster
func addNewBrokersToCluster(t *testing.T, brokersToAdd []int) {
	t.Helper()
	t.Logf("[SCALING] Adding new brokers %v to the cluster via API 🚀", brokersToAdd)

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Build the payload with the brokers to add
	payloadData := map[string]interface{}{
		"brokers": map[string]interface{}{
			"add": brokersToAdd,
		},
	}
	payloadBytes, err := json.Marshal(payloadData)
	require.NoError(t, err, "Failed to marshal payload")

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview partition distribution")
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Logf("[SCALING] Executing broker addition: adding brokers %v", brokersToAdd)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err = json.Unmarshal([]byte(body), &response)
	require.NoError(t, err)
	changeId := response["changeId"]
	t.Logf("[SCALING] Broker addition initiated with changeId: %v", changeId)
}

// waitForBrokerScalingComplete polls the cluster status until scaling is complete
func waitForBrokerScalingComplete(t *testing.T) {
	t.Log("[SCALING] Waiting for broker scaling to complete 🕐")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Poll the cluster status until scaling is complete
	maxRetries := 40 // 10 minutes max (40 * 15 seconds)
	for i := 0; i < maxRetries; i++ {
		res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", endpoint), nil)
		require.NotNil(t, res, "Failed to query cluster status")
		require.Equal(t, 200, res.StatusCode)

		// Check if there's no pending change
		if !strings.Contains(body, "pendingChange") {
			t.Log("[SCALING] Broker scaling completed successfully")
			require.Contains(t, body, "COMPLETED", "Expected lastChange status to be COMPLETED")
			return
		}

		t.Logf("[SCALING] Scaling in progress... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(15 * time.Second)
	}

	t.Fatal("[SCALING] Broker scaling did not complete within the expected time")
}

// scaleUpPartitions sends API request to increase partition count
func scaleUpPartitions(t *testing.T, partitionCount, replicationFactor int) {
	t.Helper()
	t.Logf("[SCALING] Scaling up to %d partitions with replication factor %d 🚀", partitionCount, replicationFactor)

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Build the payload with the partition configuration
	payloadData := map[string]interface{}{
		"partitions": map[string]interface{}{
			"count":             partitionCount,
			"replicationFactor": replicationFactor,
		},
	}
	payloadBytes, err := json.Marshal(payloadData)
	require.NoError(t, err, "Failed to marshal payload")

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview partition distribution")
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Logf("[SCALING] Executing partition scaling: %d partitions with replication factor %d", partitionCount, replicationFactor)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err = json.Unmarshal([]byte(body), &response)
	require.NoError(t, err)
	changeId := response["changeId"]
	t.Logf("[SCALING] Partition scaling initiated with changeId: %v", changeId)
}

// waitForPartitionScalingComplete polls the cluster status until partition scaling is complete
func waitForPartitionScalingComplete(t *testing.T) {
	t.Log("[SCALING] Waiting for partition scaling to complete 🕐")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Poll the cluster status until scaling is complete
	// Partition scaling can take longer due to data redistribution
	maxRetries := 60 // 15 minutes max (60 * 15 seconds)
	for i := 0; i < maxRetries; i++ {
		res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", endpoint), nil)
		require.NotNil(t, res, "Failed to query cluster status")
		require.Equal(t, 200, res.StatusCode)

		// Check if there's no pending change
		if !strings.Contains(body, "pendingChange") {
			t.Log("[SCALING] Partition scaling completed successfully")
			require.Contains(t, body, "COMPLETED", "Expected lastChange status to be COMPLETED")
			return
		}

		t.Logf("[SCALING] Partition scaling in progress... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(15 * time.Second)
	}

	t.Fatal("[SCALING] Partition scaling did not complete within the expected time")
}

// scaleUpBrokersAndPartitions sends API request to scale both brokers and partitions
func scaleUpBrokersAndPartitions(t *testing.T, brokersToAdd []int, partitionCount, replicationFactor int) {
	t.Helper()
	t.Logf("[SCALING] Scaling up brokers %v and partitions to %d simultaneously 🚀", brokersToAdd, partitionCount)

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Build the payload with both brokers and partitions configuration
	payloadData := map[string]interface{}{
		"brokers": map[string]interface{}{
			"add": brokersToAdd,
		},
		"partitions": map[string]interface{}{
			"count":             partitionCount,
			"replicationFactor": replicationFactor,
		},
	}
	payloadBytes, err := json.Marshal(payloadData)
	require.NoError(t, err, "Failed to marshal payload")

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview combined scaling")
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Logf("[SCALING] Executing combined scaling: adding brokers %v and scaling partitions to %d", brokersToAdd, partitionCount)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payloadBytes),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err = json.Unmarshal([]byte(body), &response)
	require.NoError(t, err)
	changeId := response["changeId"]
	t.Logf("[SCALING] Combined scaling initiated with changeId: %v", changeId)
}

// waitForCombinedScalingComplete polls the cluster status until both broker and partition scaling complete
func waitForCombinedScalingComplete(t *testing.T) {
	t.Log("[SCALING] Waiting for combined broker and partition scaling to complete 🕐")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Poll the cluster status until scaling is complete
	// Combined scaling can take longer due to broker addition and data redistribution
	maxRetries := 60 // 15 minutes max (60 * 15 seconds)
	for i := 0; i < maxRetries; i++ {
		res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/actuator/cluster", endpoint), nil)
		require.NotNil(t, res, "Failed to query cluster status")
		require.Equal(t, 200, res.StatusCode)

		// Check if there's no pending change
		if !strings.Contains(body, "pendingChange") {
			t.Log("[SCALING] Combined scaling completed successfully")
			require.Contains(t, body, "COMPLETED", "Expected lastChange status to be COMPLETED")
			return
		}

		t.Logf("[SCALING] Combined scaling in progress... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(15 * time.Second)
	}

	t.Fatal("[SCALING] Combined scaling did not complete within the expected time")
}

// getClusterTopology retrieves the current cluster topology information
func getClusterTopology(t *testing.T) kubectlHelpers.ClusterInfo {
	t.Helper()

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 15*time.Second)
	defer closeFn()

	// Get topology from v2/topology endpoint
	res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/v2/topology", endpoint), nil)
	require.NotNil(t, res, "Failed to query topology")
	require.Equal(t, 200, res.StatusCode)
	require.NotEmpty(t, body)

	// Parse the topology response
	var clusterInfo kubectlHelpers.ClusterInfo
	err := json.Unmarshal([]byte(body), &clusterInfo)
	require.NoError(t, err, "Failed to parse topology response")

	return clusterInfo
}
