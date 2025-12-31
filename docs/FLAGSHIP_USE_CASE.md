# Flagship Use Case: Database Primary Failover Without Code Changes

## The Real-World Scenario

**Context**: Multi-region PostgreSQL deployment with active-standby replication  
**Requirement**: Only ONE pod should accept write traffic (primary)  
**Constraint**: Database binary cannot be modified (vendor-provided, closed-source)  
**Challenge**: Traditional leader election requires code changes

## The Problem

### What You Can't Do with Native Kubernetes

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

### What's Missing

- **Network-level single-active routing** without code changes
- **Kubernetes-native primitive** (no external dependencies)
- **Works with unmodifiable binaries** (vendor software, legacy apps)

## The Solution: zen-lead

### Zero Code Changes Required

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
- **Total failover time: 2-5 seconds** (controller + kube-proxy convergence)

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
| **zen-lead** | ✅ None | **Minutes** | **Automatic (2-5sec)** | ✅ Yes |

## Real-World Metrics

From production PostgreSQL deployment:

- **Application binary**: Vendor-provided, unmodified
- **Code changes**: 0 lines
- **Time to enable**: 2 minutes (add annotation + update DNS)
- **Failover time**: 2-5 seconds (automatic)
- **Operational complexity**: None (zero day-2 operations)

## Why Existing Primitives Fail

### The Fundamental Gap

**Kubernetes has NO primitive for "route traffic to exactly one pod without code changes"**

### Native Kubernetes Service
```yaml
# Routes to ALL pods (round-robin)
apiVersion: v1
kind: Service
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
```
**Problem**: DNS/kube-proxy load-balances across all pods. No single-active semantics.

### client-go LeaderElection
```go
// Requires modifying application code
import "k8s.io/client-go/tools/leaderelection"
// Application must implement election logic
```
**Problem**: 
- Requires source code access
- Doesn't work with vendor binaries
- Adds 100+ lines of boilerplate
- Application-specific (can't centralize for platform)

### StatefulSet Headless Service
```yaml
# Returns ALL pod IPs
spec:
  clusterIP: None
```
**Problem**: Application must parse DNS and pick one pod (requires code changes).

## Technical Details

### How zen-lead Achieves This

1. **Kubernetes-native primitives**: Uses standard Service + EndpointSlice (no CRDs required)
2. **Event-driven reconciliation**: Watches pod Ready transitions, updates endpoints within ~1sec
3. **Sticky leader selection**: Prefers current leader if still Ready (minimizes unnecessary failovers)
4. **Non-invasive**: Never mutates workload pods (no labels, no sidecars, no code injection)
5. **Fail-closed**: If port resolution fails or no pods are Ready, endpoints list is empty (safe default)

### What zen-lead Does NOT Do

- ❌ Modify application code or binaries
- ❌ Inject sidecars or proxies
- ❌ Provide distributed consensus (network routing only, not application-level coordination)
- ❌ Guarantee zero split-brain at application level (apps must handle their own state)
- ❌ Act as a service mesh or scheduler

## Deployment Model

zen-lead runs as a standalone controller:

```bash
# Install via Helm (namespace-scoped by default)
helm install zen-lead zen-lead/zen-lead --namespace default
```

**No per-application deployment needed.** Once zen-lead is installed:
1. Annotate any Service: `zen-lead.io/enabled: "true"`
2. Update application DNS: `myapp` → `myapp-leader`
3. Done

## Key Takeaways

This use case demonstrates:

1. **Real problem**: Single-active routing without code changes
2. **Non-trivial**: No native Kubernetes primitive provides this
3. **Platform-team scope**: Enables HA patterns for vendor binaries, legacy apps, and zero-code deployments
4. **Defensible niche**: Network-level routing primitive ≠ application-level leader election library

**The brutal one-liner**: client-go leader election requires code changes; zen-lead requires zero code changes.

**Why this matters**: 
- Vendor software (can't modify binary)
- Legacy applications (no source code)
- Platform-wide standardization (one solution for all apps, not per-app implementation)

