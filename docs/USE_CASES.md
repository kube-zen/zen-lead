# Zen-Lead Use Cases

## Use Case 1: Controller High Availability

### Problem

Your Kubernetes controller runs multiple replicas for high availability, but they all try to reconcile the same resources, causing:
- Duplicate work
- Race conditions
- Resource conflicts

### Solution

Use zen-lead to ensure only one replica actively reconciles.

### Configuration

```yaml
# 1. Create LeaderPolicy
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-controller-pool
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby

---
# 2. Annotate Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: my-controller-pool
        zen-lead/join: "true"
```

### Application Code

```go
// Check if this pod is the leader
func isLeader() bool {
    podName := os.Getenv("POD_NAME")
    // Query pod annotation or LeaderPolicy status
    // Only leader processes reconciliations
    return checkLeaderStatus(podName)
}
```

### Benefits

- ✅ Only 1 replica processes reconciliations
- ✅ Automatic failover if leader crashes
- ✅ Other replicas ready for immediate takeover
- ✅ No code changes required (annotation-based)

---

## Use Case 2: Exclusive CronJob Execution

### Problem

You have a CronJob that runs a daily report. If it runs on 3 nodes, it sends 3 duplicate emails.

### Solution

Use zen-lead to ensure only one cluster instance executes, even with multiple nodes.

### Configuration

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: report-generator
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby

---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
  annotations:
    zen-lead/pool: report-generator
    zen-lead/join: "true"
spec:
  schedule: "0 0 * * *"  # Daily at midnight
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            zen-lead/pool: report-generator
            zen-lead/join: "true"
        spec:
          containers:
          - name: report-generator
            image: report-generator:latest
            # Only the leader pod will execute
```

### Benefits

- ✅ Only 1 instance executes globally
- ✅ Works across multiple nodes
- ✅ No duplicate reports
- ✅ Automatic failover

---

## Use Case 3: Distributed Locking

### Problem

A processing job needs to write to a shared S3 bucket. If two pods run simultaneously, they corrupt the file.

### Solution

Use zen-lead to acquire a lock before critical operations.

### Configuration

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: s3-writer-lock
spec:
  leaseDurationSeconds: 60  # Longer lease for operations
  identityStrategy: pod
  followerMode: standby
```

### Application Code

```go
func writeToS3() error {
    // Check if this pod is the leader
    if !isLeader() {
        return fmt.Errorf("not the leader, skipping")
    }
    
    // Only leader writes to S3
    return s3Client.PutObject(...)
}
```

### Benefits

- ✅ Prevents parallel writes
- ✅ Ensures data consistency
- ✅ Simple lock mechanism
- ✅ Automatic lock release on pod termination

---

## Use Case 4: Primary/Secondary Pattern

### Problem

You want a primary instance handling all traffic, with secondary instances ready for failover.

### Solution

Use zen-lead with follower mode to keep secondaries in standby.

### Configuration

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: primary-service
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby  # Secondaries stay running
```

### Application Code

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    if !isLeader() {
        // Follower: redirect or return 503
        http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
        return
    }
    
    // Leader: handle request
    processRequest(w, r)
}
```

### Benefits

- ✅ Primary handles all traffic
- ✅ Secondaries ready for instant failover
- ✅ No load balancer configuration needed
- ✅ Automatic failover

---

## Use Case 5: Resource-Intensive Operations

### Problem

You have a resource-intensive operation that should only run on one pod to save resources.

### Solution

Use zen-lead to ensure only the leader performs expensive operations.

### Configuration

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: expensive-operation
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby
```

### Application Code

```go
func performExpensiveOperation() {
    if !isLeader() {
        log.Info("Not leader, skipping expensive operation")
        return
    }
    
    // Only leader performs expensive operation
    expensiveOperation()
}
```

### Benefits

- ✅ Saves resources (CPU, memory)
- ✅ Prevents duplicate work
- ✅ Automatic failover
- ✅ Simple configuration

---

## Use Case 6: Integration with Zen Suite

### zen-flow Integration

**Problem:** Multiple zen-flow replicas try to create the same Job.

**Solution:**
```yaml
annotations:
  zen-lead/pool: flow-controller
  zen-lead/join: "true"
```

**Result:** Only 1 replica actively reconciles JobFlows.

### zen-watcher Integration

**Problem:** Multiple zen-watcher replicas write duplicate Observations.

**Solution:**
```yaml
annotations:
  zen-lead/pool: watcher-primary
  zen-lead/join: "true"
```

**Result:** Only 1 replica writes to Observation CRDs.

### zen-lock Integration

**Problem:** Multiple zen-lock webhooks handle the same requests.

**Solution:**
```yaml
annotations:
  zen-lead/pool: lock-webhook
  zen-lead/join: "true"
```

**Result:** Only leader handles webhook traffic (or use scaleDown mode to save resources).

---

## Best Practices

1. **Use Descriptive Pool Names**
   - `flow-controller` not `pool1`
   - `watcher-primary` not `test`

2. **Set Appropriate Lease Duration**
   - Fast failover: 10s
   - Stable workloads: 15s (default)
   - Long operations: 30s+

3. **Monitor Leader Transitions**
   - Alert on frequent changes
   - Track transition metrics

4. **Use Standby Mode for Most Cases**
   - Keeps replicas ready
   - Instant failover
   - Only use scaleDown if resource savings needed

---

**See [INTEGRATION.md](INTEGRATION.md) for detailed integration examples.**

