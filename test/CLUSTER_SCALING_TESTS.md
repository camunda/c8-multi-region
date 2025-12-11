# Cluster Scaling Tests

This document describes the multi-region cluster scaling tests implemented according to the [Camunda documentation](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/).

## Overview

Three comprehensive test scenarios validate Zeebe cluster scaling in a dual-region AWS EKS setup:

1. **Scale Brokers Only** - Increases/decreases broker count while maintaining partition count
2. **Scale Partitions Only** - Increases partition count across existing brokers
3. **Scale Brokers and Partitions** - Simultaneously scales both brokers and partitions

All tests operate on a dual-region architecture with brokers split between `eu-west-2` (London) and `eu-west-3` (Paris).

## Test Scenarios

### 1. TestAWSClusterScaling_ScaleBrokers

Tests horizontal scaling of Zeebe brokers across regions.

**Initial State:**
- 8 brokers total (4 per region)
- Broker IDs: Primary (0,2,4,6), Secondary (1,3,5,7)
- 8 partitions
- Replication factor: 4

**Target State:**
- 12 brokers total (6 per region)
- Broker IDs: Primary (0,2,4,6,8,10), Secondary (1,3,5,7,9,11)
- 8 partitions (unchanged)
- Replication factor: 4

**Test Steps:**
1. Verify initial broker count (8 brokers)
2. Scale StatefulSets from 4 to 6 replicas in each region
3. Wait for new broker pods to become ready
4. Send API request to add brokers [8,9,10,11] to the cluster
5. Poll cluster status until scaling completes
6. Verify 12 brokers exist with proper distribution
7. Deploy and validate process instances
8. Scale down by removing brokers [8,9,10,11]
9. Scale StatefulSets back to 4 replicas
10. Verify cluster returns to 8 brokers

**Run Test:**
```bash
cd test
go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScaleBrokers
```

---

### 2. TestAWSClusterScaling_ScalePartitions

Tests partition scaling without changing broker count.

**Initial State:**
- 8 brokers (4 per region)
- 8 partitions
- Replication factor: 4

**Target State:**
- 8 brokers (unchanged)
- 12 partitions
- Replication factor: 4

**Test Steps:**
1. Verify initial partition count (8 partitions)
2. Send API request to increase partitions to 12 with replication factor 4
3. Poll cluster status until partition redistribution completes
4. Verify 12 partitions exist across all brokers
5. Deploy and validate process instances
6. Check Elasticsearch cluster health

**Important Notes:**
- **Partition count can only be increased, never decreased** (Zeebe limitation)
- Partition scaling triggers data redistribution across existing brokers
- Backups taken before scaling can only be restored to clusters with the same partition count

**Run Test:**
```bash
cd test
go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScalePartitions
```

---

### 3. TestAWSClusterScaling_ScaleBrokersAndPartitions

Tests simultaneous scaling of both brokers and partitions.

**Initial State:**
- 8 brokers (4 per region)
- 8 partitions
- Replication factor: 4

**Target State (Scale Up):**
- 12 brokers (6 per region)
- 12 partitions
- Replication factor: 4

**Final State (Scale Down):**
- 8 brokers (4 per region)
- 12 partitions (cannot be decreased)
- Replication factor: 4

**Test Steps:**
1. Verify initial cluster size
2. Scale StatefulSets from 4 to 6 replicas in each region
3. Wait for new broker pods to become ready
4. Send combined API request to add brokers [8,9,10,11] and scale partitions to 12
5. Poll cluster status until both operations complete
6. Verify 12 brokers and 12 partitions
7. Deploy and validate process instances
8. Check Elasticsearch cluster health
9. Scale down brokers (partitions remain at 12)
10. Verify cluster returns to 8 brokers

**Run Test:**
```bash
cd test
go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScaleBrokersAndPartitions
```

---

## Multi-Region Considerations

### Broker ID Assignment Pattern

The tests maintain the dual-region broker ID pattern:

- **Primary Region (eu-west-2):** Even broker IDs (0,2,4,6,8,10...)
- **Secondary Region (eu-west-3):** Odd broker IDs (1,3,5,7,9,11...)

This pattern ensures cross-region partition distribution and fault tolerance.

### StatefulSet Scaling

Brokers are deployed as Kubernetes StatefulSets in each region:

```bash
# Primary region (London)
kubectl scale statefulset/camunda-zeebe --replicas=6 -n c8-snap-cluster-0

# Secondary region (Paris)
kubectl scale statefulset/camunda-zeebe --replicas=6 -n c8-snap-cluster-1
```

StatefulSet pods are named sequentially:
- Primary: `camunda-zeebe-0` through `camunda-zeebe-5`
- Secondary: `camunda-zeebe-0` through `camunda-zeebe-5`

However, their Zeebe broker IDs follow the even/odd pattern above.

### DNS Chaining

Cross-region communication requires DNS chaining to be configured before running tests. See `TestAWSDNSChaining` in `multi_region_aws_dns_chaining_test.go`.

---

## API Usage Patterns

### Scaling API Endpoints

All scaling operations use the Zeebe Gateway's management endpoint:

```bash
# Port-forward to access the gateway
kubectl port-forward svc/camunda-zeebe-gateway 9600:9600
```

#### Add Brokers
```bash
curl -X 'PATCH' \
  'http://localhost:9600/actuator/cluster' \
  -H 'Content-Type: application/json' \
  -d '{"brokers":{"add":[8,9,10,11]}}'
```

#### Remove Brokers
```bash
curl -X 'PATCH' \
  'http://localhost:9600/actuator/cluster' \
  -H 'Content-Type: application/json' \
  -d '{"brokers":{"remove":[8,9,10,11]}}'
```

#### Scale Partitions
```bash
curl -X 'PATCH' \
  'http://localhost:9600/actuator/cluster' \
  -H 'Content-Type: application/json' \
  -d '{"partitions":{"count":12,"replicationFactor":4}}'
```

#### Scale Both
```bash
curl -X 'PATCH' \
  'http://localhost:9600/actuator/cluster' \
  -H 'Content-Type: application/json' \
  -d '{
    "brokers":{"add":[8,9,10,11]},
    "partitions":{"count":12,"replicationFactor":4}
  }'
```

#### Dry Run
Add `?dryRun=true` to preview changes without executing:
```bash
curl -X 'PATCH' \
  'http://localhost:9600/actuator/cluster?dryRun=true' \
  -H 'Content-Type: application/json' \
  -d '{"brokers":{"add":[8,9,10,11]}}'
```

#### Monitor Progress
```bash
curl 'http://localhost:9600/actuator/cluster'
```

Response includes:
- `changeId`: Unique identifier for the scaling operation
- `brokers`: Current broker list and partition distribution
- `lastChange`: Most recently completed operation (status: COMPLETED)
- `pendingChange`: Ongoing operation (only present during scaling)

---

## Environment Variables

Configure tests using environment variables (see `multi_region_aws_camunda_test.go`):

```bash
# Cluster identity
export CLUSTER_NAME="scaling-test"
export BACKUP_NAME="scaling-backup"

# AWS configuration
export AWS_PROFILE="infraex"

# Namespaces
export CLUSTER_0_NAMESPACE="c8-snap-cluster-0"
export CLUSTER_1_NAMESPACE="c8-snap-cluster-1"

# Helm chart
export HELM_CHART_VERSION="13.3.0"
export HELM_CHART_NAME="camunda/camunda-platform"

# Image overrides (optional)
export GLOBAL_IMAGE_TAG="8.7.0"  # Override all Camunda images
```

---

## Prerequisites

Before running scaling tests:

1. **Infrastructure Setup:**
   ```bash
   go test --count=1 -v -timeout 120m -run TestSetupTerraform
   go test --count=1 -v -timeout 120m -run TestAWSKubeConfigCreation
   go test --count=1 -v -timeout 120m -run TestClusterPrerequisites
   ```

2. **DNS Chaining:**
   ```bash
   go test --count=1 -v -timeout 120m -run TestAWSDNSChaining
   ```

3. **Deploy Camunda:**
   ```bash
   go test --count=1 -v -timeout 120m -run TestAWSDeployDualRegCamunda
   ```

4. **Run Scaling Tests:**
   ```bash
   # Individual scenarios
   go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScaleBrokers
   go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScalePartitions
   go test --count=1 -v -timeout 120m -run TestAWSClusterScaling_ScaleBrokersAndPartitions
   ```

---

## Cleanup

After testing:

```bash
# Remove Camunda installations
go test --count=1 -v -timeout 120m -run TestAWSDualRegCleanup

# Remove internal load balancers
go test --count=1 -v -timeout 120m -run TestClusterCleanup

# Destroy infrastructure
go test --count=1 -v -timeout 120m -run TestTeardownTerraform
```

**Important:** Always run `TestClusterCleanup` before `TestTeardownTerraform` to remove internal load balancers, otherwise Terraform destroy will fail.

---

## Troubleshooting

### Scaling Operation Stuck

If scaling doesn't complete:

1. Check cluster status:
   ```bash
   kubectl port-forward svc/camunda-zeebe-gateway 9600:9600
   curl http://localhost:9600/actuator/cluster
   ```

2. Verify all brokers are healthy:
   ```bash
   kubectl get pods -l app.kubernetes.io/component=zeebe-broker
   ```

3. Check broker logs:
   ```bash
   kubectl logs camunda-zeebe-0 -f
   ```

### Pod Not Starting

If new broker pods don't start:

1. Check pod events:
   ```bash
   kubectl describe pod camunda-zeebe-4
   ```

2. Verify PVC availability:
   ```bash
   kubectl get pvc
   ```

3. Check resource limits:
   ```bash
   kubectl top nodes
   kubectl top pods
   ```

### Partition Distribution Issues

If partitions aren't properly distributed:

1. Query topology:
   ```bash
   kubectl port-forward svc/camunda-zeebe-gateway 8080:8080
   curl http://localhost:8080/v2/topology | jq
   ```

2. Verify DNS resolution between regions:
   ```bash
   go test --count=1 -v -timeout 120m -run TestCrossClusterCommunicationWithDNS
   ```

---

## References

- [Camunda Cluster Scaling Documentation](https://docs.camunda.io/docs/self-managed/components/orchestration-cluster/zeebe/operations/cluster-scaling/)
- [Dual-Region Setup Guide](https://docs.camunda.io/docs/self-managed/concepts/multi-region/dual-region/)
- [Zeebe Cluster Management API](https://github.com/camunda/camunda/blob/main/dist/src/main/resources/api/cluster/cluster-api.yaml)

---

## Important Notes

1. **Partition Count is Immutable Down**: Once partitions are added, they cannot be removed. Plan capacity carefully.

2. **Backup Compatibility**: Backups taken before partition scaling can only be restored to clusters with the same partition count. Scale first, then take new backups.

3. **Replication Factor**: The tests use replication factor 4 to ensure data is replicated across both regions (2 copies per region).

4. **StatefulSet Scale Down**: When scaling down brokers, Kubernetes terminates pods with the highest ordinals first (e.g., `camunda-zeebe-5`, `camunda-zeebe-4`). Always scale down the Zeebe cluster logically before scaling down StatefulSets.

5. **Production Considerations**:
   - Always take backups before scaling operations
   - Test scaling in non-production environments first
   - Monitor cluster health during and after scaling
   - Plan scaling during maintenance windows to minimize impact
