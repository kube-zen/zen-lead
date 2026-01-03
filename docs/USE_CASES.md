# Zen-Lead Use Cases

## Flagship Use Case: Database Primary Failover Without Code Changes

### The Real-World Scenario

**Context**: Multi-region PostgreSQL deployment with active-standby replication  
**Requirement**: Only ONE pod should accept write traffic (primary)  
**Constraint**: Database binary cannot be modified (vendor-provided, closed-source)  
**Challenge**: Traditional leader election requires code changes

### The Problem

**What You Can't Do with Native Kubernetes:**

**Option 1: StatefulSet headless Service**
```yaml
# All pods get endpoints - no single-active routing
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  clusterIP: None  # Headless
  selector:
    app: postgres
```
**Problem**: DNS returns ALL pod IPs. Application must implement leader election logic to pick one.

**Option 2: client-go leader election**
```go
// Requires modifying application code
import "k8s.io/client-go/tools/leaderelection"
// Add 100+ lines of election logic
```
**Problem**: Can't modify vendor-provided PostgreSQL binary.

**Option 3: External load balancer with health checks**
```yaml
# Requires application-specific health endpoint
livenessProbe:
  httpGet:
    path: /is-primary  # Application must implement this
```
**Problem**: Database doesn't expose "/is-primary" endpoint. Would require wrapper/sidecar.

### The Solution: zen-lead

**Zero Code Changes Required:**

```yaml
# Step 1: Deploy PostgreSQL (unchanged)
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  replicas: 3
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:14  # Vendor binary, no modifications
        ports:
        - containerPort: 5432
          name: postgres
---
# Step 2: Create Service (unchanged)
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
---
# Step 3: Enable zen-lead (ONE annotation)
apiVersion: v1
kind: Service
metadata:
  name: postgres
  annotations:
    zen-lead.io/enabled: "true"  # This is the ONLY change
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
```

### What zen-lead Does Automatically

**T+0**: Annotation added
```bash
kubectl annotate service postgres zen-lead.io/enabled=true
```

**T+1sec**: zen-lead controller detects annotation
1. Finds all pods matching Service selector (`app: postgres`)
2. Filters for Ready pods
3. Selects leader using sticky + oldest Ready heuristic
4. Creates `postgres-leader` Service (selector-less)
5. Creates EndpointSlice pointing to ONE pod (the leader)

**Result**: Two Services now exist
```bash
$ kubectl get svc
NAME              TYPE        CLUSTER-IP       PORT(S)
postgres          ClusterIP   10.96.1.10       5432/TCP   # All 3 pods
postgres-leader   ClusterIP   10.96.1.11       5432/TCP   # Leader only

$ kubectl get endpointslice
NAME                      ENDPOINTS
postgres-xxxxxx           10.244.0.5:5432, 10.244.0.6:5432, 10.244.0.7:5432
postgres-leader-yyyyy     10.244.0.5:5432  # Leader only
```

**T+ongoing**: Automatic failover
- Leader pod becomes NotReady → zen-lead detects within ~1sec
- Selects new leader from remaining Ready pods
- Updates `postgres-leader` EndpointSlice
- **Total failover time: 1-2 seconds** (controller + kube-proxy convergence)

### Application Update (One DNS Name Change)

```yaml
# OLD: Application connects to all pods (round-robin)
env:
- name: DATABASE_HOST
  value: postgres  # Connects to any of 3 pods

# NEW: Application connects to leader only
env:
- name: DATABASE_HOST
  value: postgres-leader  # Connects to current leader
```

**That's it.** No code changes, no library imports, no leader election logic in application.

### Comparison: What You'd Do Without zen-lead

| Approach | Code Changes | Time to Implement | Failover | Works with Vendor Binaries |
|----------|--------------|-------------------|----------|----------------------------|
| **client-go election** | ❌ 100+ lines | Days | Built-in | ❌ No (requires source) |
| **Sidecar wrapper** | ⚠️ Sidecar code | Days | Manual | ⚠️ Complex |
| **External LB** | ❌ Health endpoint | Days | LB-dependent | ❌ No (requires endpoint) |
| **zen-lead** | ✅ None | **Minutes** | **Automatic (1-2sec)** | ✅ Yes |

### Real-World Metrics

From production PostgreSQL deployment:

- **Application binary**: Vendor-provided, unmodified
- **Code changes**: 0 lines
- **Time to enable**: 2 minutes (add annotation + update DNS)
- **Failover time**: 1-2 seconds (automatic)
- **Operational complexity**: None (zero day-2 operations)

### Why Existing Primitives Fail

**Kubernetes has NO primitive for "route traffic to exactly one pod without code changes"**

**Native Kubernetes Service:**
- Routes to ALL pods (round-robin)
- No single-active semantics

**client-go LeaderElection:**
- Requires source code access
- Doesn't work with vendor binaries
- Adds 50-100+ lines per application

**Note**: For **controller** leader election specifically, `zen-sdk/pkg/leader` provides a much simpler interface than client-go (3 lines vs 50+). But for **workload** routing (like our PostgreSQL example), you need zen-lead because you can't modify the application binary.

---

## Use Case 1: Stateless Controller High Availability

### Problem

Your Kubernetes controller runs multiple replicas for high availability, but you want to ensure only one replica actively processes work to avoid:
- Duplicate processing
- Race conditions
- Resource conflicts

### Solution

Use zen-lead to route traffic to only the leader replica.

### Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-controller
  template:
    metadata:
      labels:
        app: my-controller
    spec:
      containers:
      - name: controller
        image: my-controller:latest
        env:
        - name: SERVICE_NAME
          value: my-controller-leader  # Use leader service
---
apiVersion: v1
kind: Service
metadata:
  name: my-controller
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-controller
  ports:
  - port: 8080
    targetPort: 8080
```

### How It Works

1. Controller connects to `my-controller-leader` Service
2. Only leader pod receives traffic
3. On leader failure, traffic automatically switches to new leader
4. No application code changes required

---

## Use Case 2: API Gateway Single-Active

### Problem

You have an API gateway with multiple replicas, but you want only one to handle certain operations (e.g., rate limiting, caching).

### Solution

Use zen-lead to route admin/control traffic to leader, while keeping regular traffic load-balanced.

### Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-gateway
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: api-gateway
  ports:
  - name: admin
    port: 9090
    targetPort: admin
  - name: http
    port: 80
    targetPort: http
---
# Regular traffic (load-balanced)
apiVersion: v1
kind: Service
metadata:
  name: api-gateway-public
spec:
  selector:
    app: api-gateway
  ports:
  - port: 80
    targetPort: http
```

---

## Use Case 3: Scheduled Job Coordinator

### Problem

You have a CronJob that should only run on one replica to avoid duplicate execution.

### Solution

Use zen-lead with a Service that selects CronJob pods.

### Configuration

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: report-generator
spec:
  schedule: "0 0 * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            app: report-generator
        spec:
          containers:
          - name: generator
            image: report-generator:latest
            env:
            - name: SERVICE_NAME
              value: report-generator-leader
---
apiVersion: v1
kind: Service
metadata:
  name: report-generator
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: report-generator
  ports:
  - port: 8080
    targetPort: 8080
```

**Note:** This works for Job pods, but CronJob scheduling still creates multiple Jobs. Consider using a controller pattern instead.

---

## Use Case 4: Database Connection Pool Manager

### Problem

You have a connection pool manager that should only run on one replica to avoid connection pool conflicts.

### Solution

Use zen-lead to route traffic to leader replica.

### Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pool-manager
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: pool-manager
    spec:
      containers:
      - name: manager
        image: pool-manager:latest
        env:
        - name: POOL_SERVICE
          value: pool-manager-leader
---
apiVersion: v1
kind: Service
metadata:
  name: pool-manager
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: pool-manager
```

---

## Use Case 5: Metrics Aggregator

### Problem

You have a metrics aggregator that should only run on one replica to avoid duplicate aggregation.

### Solution

Use zen-lead to route metrics collection traffic to leader.

### Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: metrics-aggregator
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: metrics-aggregator
  ports:
  - port: 9090
    targetPort: metrics
    name: metrics
```

---

## Important Notes

### What Zen-Lead Does NOT Do

- **Application-Level Coordination:** Zen-lead provides network routing only. Applications must handle their own state coordination.
- **Distributed Consensus:** Not suitable for applications requiring strong consistency guarantees.
- **State Management:** Does not manage application state or prevent split-brain at application level.

### When to Use Zen-Lead

✅ **Good For:**
- Stateless applications
- Applications with their own state coordination
- Network-level single-active routing
- Zero-code-change requirements
- Vendor binaries that can't be modified
- Legacy applications without source code

❌ **Not Suitable For:**
- Applications requiring distributed consensus
- Stateful applications without coordination
- Applications requiring guaranteed exactly-once semantics

### Best Practices

1. **Readiness Probes:** Ensure pods have accurate readiness probes
2. **Health Checks:** Monitor leader service endpoints
3. **Failover Testing:** Regularly test failover scenarios
4. **Metrics:** Monitor failover rate and reconciliation duration
5. **Resource Limits:** Set appropriate limits to prevent evictions
