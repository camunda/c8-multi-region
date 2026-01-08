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
		{"TestScaleUpBrokerStatefulSets", scaleUpBrokerStatefulSets},
		{"TestWaitForNewBrokersToStart", waitForNewBrokersToStart},
		{"TestAddNewBrokersToCluster", addNewBrokersToCluster},
		{"TestWaitForBrokerScalingComplete", waitForBrokerScalingComplete},
		{"TestVerifyScaledBrokerTopology", verifyScaledBrokerTopology},
		{"TestScaleDownBrokerCluster", scaleDownBrokerCluster},
		{"TestWaitForBrokerScaleDownComplete", waitForBrokerScaleDownComplete},
		{"TestScaleDownBrokerStatefulSets", scaleDownBrokerStatefulSets},
		{"TestVerifyOriginalBrokerCount", verifyOriginalBrokerCount},
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
		{"TestVerifyClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 8, 8) }},
		{"TestScaleUpPartitions", scaleUpPartitions},
		{"TestWaitForPartitionScalingComplete", waitForPartitionScalingComplete},
		{"TestVerifyScaledPartitionTopology", verifyScaledPartitionTopology},
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
		{"TestVerifyClusterTopology", func(t *testing.T) { verifyClusterTopology(t, 8, 8) }},
		{"TestScaleUpBrokerStatefulSets", scaleUpBrokerStatefulSets},
		{"TestWaitForNewBrokersToStart", waitForNewBrokersToStart},
		{"TestScaleUpBrokersAndPartitions", scaleUpBrokersAndPartitions},
		{"TestWaitForCombinedScalingComplete", waitForCombinedScalingComplete},
		{"TestVerifyScaledClusterTopology", verifyScaledClusterTopology},
		{"TestCheckElasticsearchClusterHealth", checkElasticsearchClusterHealth},
		{"TestScaleDownBrokerCluster", scaleDownBrokerCluster},
		{"TestWaitForBrokerScaleDownComplete", waitForBrokerScaleDownComplete},
		{"TestScaleDownBrokerStatefulSets", scaleDownBrokerStatefulSets},
		{"TestVerifyOriginalClusterSize", verifyOriginalClusterSize},
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

// scaleUpBrokerStatefulSets scales the Zeebe StatefulSets from 4 to 6 replicas in each region
func scaleUpBrokerStatefulSets(t *testing.T) {
	t.Log("[SCALING] Scaling up Zeebe StatefulSets in both regions 🚀")

	// Scale up primary region: brokers 0,2,4,6 -> 0,2,4,6,8,10
	t.Log("[SCALING] Scaling primary region StatefulSet from 4 to 6 replicas")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", "--replicas=6")

	// Scale up secondary region: brokers 1,3,5,7 -> 1,3,5,7,9,11
	t.Log("[SCALING] Scaling secondary region StatefulSet from 4 to 6 replicas")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", "--replicas=6")

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
func addNewBrokersToCluster(t *testing.T) {
	t.Log("[SCALING] Adding new brokers to the cluster via API 🚀")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview partition distribution")
	dryRunPayload := []byte(`{"brokers":{"add":[8,9,10,11]}}`)
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(dryRunPayload),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Log("[SCALING] Executing broker addition: adding brokers [8,9,10,11]")
	payload := []byte(`{"brokers":{"add":[8,9,10,11]}}`)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payload),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
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

// verifyScaledBrokerTopology verifies the cluster has 12 brokers after scaling
func verifyScaledBrokerTopology(t *testing.T) {
	t.Log("[SCALING] Verifying scaled broker topology 🔍")

	clusterInfo := getClusterTopology(t)
	require.Equal(t, 12, clusterInfo.ClusterSize, "Expected 12 brokers after scaling")
	require.Equal(t, 8, clusterInfo.PartitionsCount, "Expected 8 partitions (unchanged)")

	// Verify we have brokers 0-11
	brokerIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		brokerIds[broker.NodeId] = true
	}
	for i := 0; i < 12; i++ {
		require.True(t, brokerIds[i], "Expected broker %d to exist", i)
	}

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions", clusterInfo.ClusterSize, clusterInfo.PartitionsCount)
}

// scaleUpPartitions sends API request to increase partition count
func scaleUpPartitions(t *testing.T) {
	t.Log("[SCALING] Scaling up partition count from 8 to 12 🚀")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview partition distribution")
	dryRunPayload := []byte(`{"partitions":{"count":12,"replicationFactor":4}}`)
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(dryRunPayload),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Log("[SCALING] Executing partition scaling: 8 -> 12 partitions with replication factor 4")
	payload := []byte(`{"partitions":{"count":12,"replicationFactor":4}}`)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payload),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
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

// verifyScaledPartitionTopology verifies the cluster has 12 partitions after scaling
func verifyScaledPartitionTopology(t *testing.T) {
	t.Log("[SCALING] Verifying scaled partition topology 🔍")

	clusterInfo := getClusterTopology(t)
	require.Equal(t, 8, clusterInfo.ClusterSize, "Expected 8 brokers (unchanged)")
	require.Equal(t, 12, clusterInfo.PartitionsCount, "Expected 12 partitions after scaling")
	require.Equal(t, 4, clusterInfo.ReplicationFactor, "Expected replication factor of 4")

	// Verify all partitions exist (1-12)
	partitionIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		for _, partition := range broker.Partitions {
			partitionIds[partition.PartitionId] = true
		}
	}
	for i := 1; i <= 12; i++ {
		require.True(t, partitionIds[i], "Expected partition %d to exist", i)
	}

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions, replication factor %d",
		clusterInfo.ClusterSize, clusterInfo.PartitionsCount, clusterInfo.ReplicationFactor)
}

// scaleUpBrokersAndPartitions sends API request to scale both brokers and partitions
func scaleUpBrokersAndPartitions(t *testing.T) {
	t.Log("[SCALING] Scaling up brokers and partitions simultaneously 🚀")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// First, do a dry run to verify the planned changes
	t.Log("[SCALING] Performing dry run to preview combined scaling")
	dryRunPayload := []byte(`{"brokers":{"add":[8,9,10,11]},"partitions":{"count":12,"replicationFactor":4}}`)
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster?dryRun=true", endpoint),
		bytes.NewBuffer(dryRunPayload),
	)
	require.NotNil(t, res, "Failed to create dry run request")
	require.Equal(t, 200, res.StatusCode, "Dry run should return 200")
	t.Logf("[SCALING] Dry run response: %s", body)

	// Now perform the actual scaling
	t.Log("[SCALING] Executing combined scaling: adding brokers [8,9,10,11] and scaling partitions to 12")
	payload := []byte(`{"brokers":{"add":[8,9,10,11]},"partitions":{"count":12,"replicationFactor":4}}`)
	res, body = helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payload),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")
	require.Contains(t, body, "changeId")

	// Parse and log the change ID
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
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

// verifyScaledClusterTopology verifies the cluster has 12 brokers and 12 partitions after scaling
func verifyScaledClusterTopology(t *testing.T) {
	t.Log("[SCALING] Verifying scaled cluster topology 🔍")

	clusterInfo := getClusterTopology(t)
	require.Equal(t, 12, clusterInfo.ClusterSize, "Expected 12 brokers after scaling")
	require.Equal(t, 12, clusterInfo.PartitionsCount, "Expected 12 partitions after scaling")
	require.Equal(t, 4, clusterInfo.ReplicationFactor, "Expected replication factor of 4")

	// Verify we have brokers 0-11
	brokerIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		brokerIds[broker.NodeId] = true
	}
	for i := 0; i < 12; i++ {
		require.True(t, brokerIds[i], "Expected broker %d to exist", i)
	}

	// Verify all partitions exist (1-12)
	partitionIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		for _, partition := range broker.Partitions {
			partitionIds[partition.PartitionId] = true
		}
	}
	for i := 1; i <= 12; i++ {
		require.True(t, partitionIds[i], "Expected partition %d to exist", i)
	}

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions, replication factor %d",
		clusterInfo.ClusterSize, clusterInfo.PartitionsCount, clusterInfo.ReplicationFactor)
}

// scaleDownBrokerCluster sends API request to remove brokers from the cluster
func scaleDownBrokerCluster(t *testing.T) {
	t.Log("[SCALING] Scaling down cluster by removing brokers [8,9,10,11] 🚀")

	service := k8s.GetService(t, &primary.KubectlNamespace, "camunda-zeebe-gateway")
	require.Equal(t, "camunda-zeebe-gateway", service.Name)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 9600, 5, 15*time.Second)
	defer closeFn()

	// Send scale down request
	t.Log("[SCALING] Executing broker removal: removing brokers [8,9,10,11]")
	payload := []byte(`{"brokers":{"remove":[8,9,10,11]}}`)
	res, body := helpers.HttpRequest(
		t,
		"PATCH",
		fmt.Sprintf("http://%s/actuator/cluster", endpoint),
		bytes.NewBuffer(payload),
	)
	require.NotNil(t, res, "Failed to create request")
	require.Equal(t, 202, res.StatusCode, "Expected 202 Accepted status")
	require.NotEmpty(t, body)
	require.Contains(t, body, "plannedChanges")

	// Parse and log the change ID
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
	require.NoError(t, err)
	changeId := response["changeId"]
	t.Logf("[SCALING] Broker removal initiated with changeId: %v", changeId)
}

// waitForBrokerScaleDownComplete polls the cluster status until scale down is complete
func waitForBrokerScaleDownComplete(t *testing.T) {
	t.Log("[SCALING] Waiting for broker scale down to complete 🕐")

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
			t.Log("[SCALING] Broker scale down completed successfully")
			require.Contains(t, body, "COMPLETED", "Expected lastChange status to be COMPLETED")
			return
		}

		t.Logf("[SCALING] Scale down in progress... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(15 * time.Second)
	}

	t.Fatal("[SCALING] Broker scale down did not complete within the expected time")
}

// scaleDownBrokerStatefulSets scales the Zeebe StatefulSets back to 4 replicas in each region
func scaleDownBrokerStatefulSets(t *testing.T) {
	t.Log("[SCALING] Scaling down Zeebe StatefulSets in both regions 🚀")

	// Scale down primary region: 6 -> 4 replicas
	t.Log("[SCALING] Scaling primary region StatefulSet from 6 to 4 replicas")
	k8s.RunKubectl(t, &primary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", "--replicas=4")

	// Scale down secondary region: 6 -> 4 replicas
	t.Log("[SCALING] Scaling secondary region StatefulSet from 6 to 4 replicas")
	k8s.RunKubectl(t, &secondary.KubectlNamespace, "scale", "statefulset/camunda-zeebe", "--replicas=4")

	// Give Kubernetes time to terminate the pods
	t.Log("[SCALING] Waiting for pods to terminate...")
	time.Sleep(30 * time.Second)

	t.Log("[SCALING] StatefulSets scaled down successfully")
}

// verifyOriginalBrokerCount verifies the cluster is back to 8 brokers
func verifyOriginalBrokerCount(t *testing.T) {
	t.Log("[SCALING] Verifying cluster is back to original broker count 🔍")

	clusterInfo := getClusterTopology(t)
	require.Equal(t, 8, clusterInfo.ClusterSize, "Expected 8 brokers after scale down")

	// Verify we only have brokers 0-7
	brokerIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		brokerIds[broker.NodeId] = true
		require.True(t, broker.NodeId < 8, "No broker ID should be >= 8 after scale down")
	}

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions", clusterInfo.ClusterSize, clusterInfo.PartitionsCount)
}

// verifyOriginalClusterSize verifies the cluster is back to original size
func verifyOriginalClusterSize(t *testing.T) {
	t.Log("[SCALING] Verifying cluster is back to original size 🔍")

	clusterInfo := getClusterTopology(t)
	require.Equal(t, 8, clusterInfo.ClusterSize, "Expected 8 brokers after scale down")
	// Note: Partition count cannot be decreased, so it remains at the scaled value

	// Verify we only have brokers 0-7
	brokerIds := make(map[int]bool)
	for _, broker := range clusterInfo.Brokers {
		brokerIds[broker.NodeId] = true
		require.True(t, broker.NodeId < 8, "No broker ID should be >= 8 after scale down")
	}

	t.Logf("[SCALING] Topology verified: %d brokers, %d partitions", clusterInfo.ClusterSize, clusterInfo.PartitionsCount)
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
