package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiregiontests/internal/helpers"
	kubectlHelpers "multiregiontests/internal/helpers/kubectl"

	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
)

const (
	mockServerManifest         = "./fixtures/mock-server/mock-api-server.yml"
	mockServerTeleportManifest = "./fixtures/mock-server/mock-api-server-teleport.yml"
	connectorBpmnPath          = "./c8-multi-region-dummy-connector-flow.bpmn"
	webhookTriggerCount        = 10
	basicAuthDemoHeader        = "Basic ZGVtbzpkZW1v" // demo:demo base64 encoded
)

// TestConnectorWebhookFlow tests the end-to-end connector flow:
// 1. Deploys a mock API server to receive POST requests
// 2. Deploys a BPMN process with webhook inbound and REST outbound connector
// 3. Triggers the workflow via webhook 10 times
// 4. Verifies the mock server received exactly 10 requests
func TestConnectorWebhookFlow(t *testing.T) {
	t.Log("[CONNECTOR TEST] Testing Connector Webhook Flow in multi-region mode 🚀")

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
		{"TestDeployMockApiServer", deployMockApiServer},
		{"TestWaitForMockApiServerReady", waitForMockApiServerReady},
		{"TestDeployConnectorBpmnProcess", deployConnectorBpmnProcess},
		{"TestTriggerWebhookWorkflow", triggerWebhookWorkflow},
		{"TestVerifyMockServerReceivedRequests", verifyMockServerReceivedRequests},
		{"TestVerifyConnectorsProcessedJobs", verifyConnectorsProcessedJobs},
		{"TestCleanupMockApiServer", cleanupMockApiServer},
	} {
		t.Run(testFuncs.name, testFuncs.tfunc)
	}
}

// deployMockApiServer deploys the mock API server
func deployMockApiServer(t *testing.T) {
	t.Log("[MOCK SERVER] Deploying mock API server to primary region 🚀")

	k8s.KubectlApply(t, &primary.KubectlNamespace, mockServerManifest)

	// Apply Teleport tolerations and affinity patch if running on Teleport
	if helpers.IsTeleportEnabled() {
		t.Log("[MOCK SERVER] Applying Teleport tolerations and affinity patch")
		k8s.RunKubectl(t, &primary.KubectlNamespace, "patch", "statefulset", "mock-api-server",
			"--patch-file", mockServerTeleportManifest, "--type=strategic")
	}
}

// waitForMockApiServerReady waits for the mock API server to be ready
func waitForMockApiServerReady(t *testing.T) {
	t.Log("[MOCK SERVER] Waiting for mock API server to be ready 🕐")

	// Wait for the StatefulSet pod to be available in primary region
	k8s.WaitUntilPodAvailable(t, &primary.KubectlNamespace, "mock-api-server-0", 20, 10*time.Second)
	t.Log("[MOCK SERVER] Mock API server ready in primary region")

	// Verify the service is accessible by checking the health endpoint
	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "mock-api-server", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/health", endpoint), nil)
	require.NotNil(t, res, "Failed to query mock server health")
	require.Equal(t, 200, res.StatusCode)
	require.Contains(t, body, "healthy")

	t.Log("[MOCK SERVER] Mock API server health check passed ✅")
}

// deployConnectorBpmnProcess deploys the BPMN process with connectors to Zeebe
func deployConnectorBpmnProcess(t *testing.T) {
	t.Log("[CONNECTOR PROCESS] Deploying connector BPMN process to Zeebe 🚀")

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-zeebe-gateway", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	file, err := os.Open(connectorBpmnPath)
	require.NoError(t, err, "Failed to open BPMN file")
	defer file.Close()

	reqBody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqBody)

	part, err := writer.CreateFormFile("resources", filepath.Base(file.Name()))
	require.NoError(t, err, "Failed to create form file")

	_, err = io.Copy(part, file)
	require.NoError(t, err, "Failed to copy file content")

	// Add default tenant ID for multi-tenancy support
	err = writer.WriteField("tenantId", "<default>")
	require.NoError(t, err, "Failed to write tenantId field")

	err = writer.Close()
	require.NoError(t, err, "Failed to close multipart writer")

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

	require.Equal(t, 200, code, "Failed to deploy BPMN process: %s", resBody)
	require.Contains(t, resBody, "c8-multi-region-dummy-connector-flow")

	t.Logf("[CONNECTOR PROCESS] Deployed process: %s", resBody)

	// Wait for process to be propagated
	t.Log("[CONNECTOR PROCESS] Waiting for process to be propagated...")
	time.Sleep(15 * time.Second)
}

// triggerWebhookWorkflow triggers the webhook workflow the specified number of times
func triggerWebhookWorkflow(t *testing.T) {
	t.Logf("[WEBHOOK TRIGGER] Triggering webhook workflow %d times 🚀", webhookTriggerCount)

	// Get the mock server endpoint from primary region to use as target URL
	mockEndpoint, mockCloseFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "mock-api-server", 0, 8080, 5, 10*time.Second)
	defer mockCloseFn()

	// Clear any existing requests on the mock server first
	res, _ := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/requests/clear", mockEndpoint), nil)
	require.Equal(t, 200, res.StatusCode, "Failed to clear mock server requests")
	t.Log("[WEBHOOK TRIGGER] Cleared mock server request history")

	// Get the connectors webhook endpoint
	connectorsEndpoint, connectorsCloseFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "camunda-connectors", 0, 8080, 5, 10*time.Second)
	defer connectorsCloseFn()

	// The mock server URL that the connector will call (using Kubernetes internal DNS)
	// The connector runs inside the cluster, so it needs to use the StatefulSet pod DNS name
	// Format: <pod-name>.<headless-service>.<namespace>.svc.cluster.local
	mockServerInternalUrl := fmt.Sprintf("http://mock-api-server-0.mock-api-server-peer.%s.svc.cluster.local:8080/webhook-callback", primary.KubectlNamespace.Namespace)

	for i := 1; i <= webhookTriggerCount; i++ {
		t.Logf("[WEBHOOK TRIGGER] Triggering workflow %d/%d", i, webhookTriggerCount)

		// Create the webhook payload with the mock server URL
		payload := map[string]interface{}{
			"URL": mockServerInternalUrl,
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err, "Failed to marshal webhook payload")

		// Trigger the webhook - the context path is "trigger-random-number" as defined in the BPMN
		webhookUrl := fmt.Sprintf("http://%s/inbound/trigger-random-number", connectorsEndpoint)

		t.Logf("[WEBHOOK TRIGGER] Sending payload: %s", string(payloadBytes))

		req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(payloadBytes))
		require.NoError(t, err, "Failed to create webhook request")
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err, "Failed to trigger webhook")

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("[WEBHOOK TRIGGER] Webhook %d response: %d - %s", i, resp.StatusCode, string(body))
		require.Equal(t, 200, resp.StatusCode, "Webhook trigger failed for instance %d", i)

		// Small delay between triggers to avoid overwhelming the system
		time.Sleep(500 * time.Millisecond)
	}

	t.Logf("[WEBHOOK TRIGGER] Successfully triggered %d webhook workflows", webhookTriggerCount)

	// Wait for all workflows to complete and send their REST calls
	t.Log("[WEBHOOK TRIGGER] Waiting for workflows to complete...")
	time.Sleep(30 * time.Second)
}

// MockServerRequestsResponse represents the response from /requests endpoint
type MockServerRequestsResponse struct {
	Requests []MockServerRequest `json:"requests"`
}

// MockServerRequest represents a single request stored by the mock server
type MockServerRequest struct {
	Timestamp string                 `json:"timestamp"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers"`
	Body      map[string]interface{} `json:"body"`
}

// verifyMockServerReceivedRequests verifies that the mock server received exactly the expected number of requests
func verifyMockServerReceivedRequests(t *testing.T) {
	t.Logf("[VERIFICATION] Verifying mock server received exactly %d requests 🔍", webhookTriggerCount)

	endpoint, closeFn := kubectlHelpers.NewServiceTunnelWithRetry(t, &primary.KubectlNamespace, "mock-api-server", 0, 8080, 5, 10*time.Second)
	defer closeFn()

	var requestsResponse MockServerRequestsResponse
	var lastBody string

	// Retry a few times in case workflows are still completing
	maxRetries := 12
	for i := 0; i < maxRetries; i++ {
		res, body := helpers.HttpRequest(t, "GET", fmt.Sprintf("http://%s/requests", endpoint), nil)
		require.NotNil(t, res, "Failed to query mock server requests")
		require.Equal(t, 200, res.StatusCode)

		lastBody = body
		err := json.Unmarshal([]byte(body), &requestsResponse)
		require.NoError(t, err, "Failed to parse mock server response")

		receivedCount := len(requestsResponse.Requests)
		t.Logf("[VERIFICATION] Mock server received %d/%d requests (attempt %d/%d)", receivedCount, webhookTriggerCount, i+1, maxRetries)

		if receivedCount >= webhookTriggerCount {
			break
		}

		if i < maxRetries-1 {
			t.Log("[VERIFICATION] Waiting for more requests to arrive...")
			time.Sleep(10 * time.Second)
		}
	}

	// Final verification
	receivedCount := len(requestsResponse.Requests)
	require.Equal(t, webhookTriggerCount, receivedCount,
		"Expected exactly %d requests, but received %d. Response: %s",
		webhookTriggerCount, receivedCount, lastBody)

	t.Logf("[VERIFICATION] ✅ Mock server received exactly %d requests as expected", webhookTriggerCount)

	// Log some details about the received requests
	for i, req := range requestsResponse.Requests {
		t.Logf("[VERIFICATION] Request %d: path=%s, timestamp=%s", i+1, req.Path, req.Timestamp)
		if req.Body != nil {
			if text, ok := req.Body["text"].(string); ok {
				t.Logf("[VERIFICATION]   Body text: %s", text)
			}
			if randomNumber, ok := req.Body["randomNumber"].(float64); ok {
				t.Logf("[VERIFICATION]   Random number: %.0f", randomNumber)
			}
		}
	}
}

// verifyConnectorsProcessedJobs checks that both connector deployments have processed jobs
func verifyConnectorsProcessedJobs(t *testing.T) {
	t.Log("[CONNECTORS CHECK] Verifying both connector deployments processed jobs 🔍")

	// Check primary region connectors
	primaryLogs, err := k8s.RunKubectlAndGetOutputE(t, &primary.KubectlNamespace, "logs", "deployment/camunda-connectors", "--tail=1000")
	require.NoError(t, err, "Failed to get primary region connector logs")
	primaryJobCount := strings.Count(primaryLogs, "Completing job")
	t.Logf("[CONNECTORS CHECK] Primary region connectors completed %d jobs", primaryJobCount)

	// Check secondary region connectors
	secondaryLogs, err := k8s.RunKubectlAndGetOutputE(t, &secondary.KubectlNamespace, "logs", "deployment/camunda-connectors", "--tail=1000")
	require.NoError(t, err, "Failed to get secondary region connector logs")
	secondaryJobCount := strings.Count(secondaryLogs, "Completing job")
	t.Logf("[CONNECTORS CHECK] Secondary region connectors completed %d jobs", secondaryJobCount)

	// Both regions should have processed jobs
	require.Greater(t, primaryJobCount, 0, "Primary region connectors did not process any jobs (no 'Completing job' in logs)")
	require.Greater(t, secondaryJobCount, 0, "Secondary region connectors did not process any jobs (no 'Completing job' in logs)")

	// Log total jobs processed
	totalJobs := primaryJobCount + secondaryJobCount
	t.Logf("[CONNECTORS CHECK] Total jobs completed: %d (primary: %d, secondary: %d)", totalJobs, primaryJobCount, secondaryJobCount)

	// Verify total matches expected (each webhook triggers one REST connector job)
	require.GreaterOrEqual(t, totalJobs, webhookTriggerCount,
		"Expected at least %d jobs to be completed, but only %d were found", webhookTriggerCount, totalJobs)

	t.Log("[CONNECTORS CHECK] ✅ Both connector deployments have processed jobs")
}

// cleanupMockApiServer removes the mock API server
func cleanupMockApiServer(t *testing.T) {
	t.Log("[CLEANUP] Removing mock API server 🧹")

	k8s.KubectlDelete(t, &primary.KubectlNamespace, mockServerManifest)
}
