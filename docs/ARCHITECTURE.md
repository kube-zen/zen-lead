# Zen-Lead Architecture

## Overview

Zen-Lead provides network-level single-active routing for Kubernetes workloads without requiring application code changes or mutating workload pods. It uses a **Service-annotation opt-in** approach that is completely non-invasive.

## Core Concepts

### Service Annotation Opt-In

Services opt into zen-lead by adding the annotation:

```yaml
metadata:
  annotations:
    zen-lead.io/enabled: "true"
```

### Selector-Less Leader Service

For each opted-in Service `S`, zen-lead creates a selector-less Service `S-leader`:

- `spec.selector: null` - No selector, endpoints managed manually
- Ports mirrored from source Service `S`
- Labels: `app.kubernetes.io/managed-by: zen-lead`, `zen-lead.io/source-service: S`
- Owner reference to source Service `S` (for garbage collection)

### Managed EndpointSlice

Zen-lead creates and manages a single EndpointSlice for `S-leader`:

- Label: `kubernetes.io/service-name: S-leader`
- Label: `endpointslice.kubernetes.io/managed-by: zen-lead`
- Exactly one endpoint pointing to the current leader pod
- Owner reference to `S-leader` Service

### Leader Selection

**Algorithm (deterministic + low churn):**

1. **Sticky Leader (default):** If current leader (from EndpointSlice targetRef) is still Ready → keep it
2. **Fallback:** Choose earliest Ready pod (tie-breaker: lexical pod name)
3. **No Candidates:** If zero Ready pods → EndpointSlice endpoints empty (clean failure mode)

**Candidates:** Pods matching `S.spec.selector` in the same namespace

## Architecture Flow

```
1. User annotates Service with zen-lead.io/enabled: "true"
   └─> ServiceDirectorReconciler watches for Service changes

2. Controller finds pods matching Service selector
   └─> Lists pods in same namespace with matching labels

3. Controller selects leader pod
   └─> Sticky: keep current if Ready, else earliest Ready pod

4. Controller creates/updates selector-less leader Service
   └─> Mirrors ports from source Service

5. Controller creates/updates EndpointSlice
   └─> Points to leader pod IP with resolved ports

6. Traffic routes to leader pod
   └─> Applications connect to <service>-leader
```

## Components

### ServiceDirectorReconciler

**Responsibilities:**
- Watches `corev1.Service` resources
- Detects `zen-lead.io/enabled: "true"` annotation
- Creates/updates selector-less leader Service
- Creates/updates EndpointSlice pointing to leader pod
- Resolves named targetPorts from pod container ports
- Handles failover when leader pod becomes NotReady

**Reconciliation Loop:**
1. Fetch Service
2. Check for `zen-lead.io/enabled: "true"` annotation
3. Validate Service has selector
4. List pods matching selector
5. Select leader pod (sticky + earliest Ready)
6. Resolve Service ports (handle named targetPort)
7. Reconcile leader Service (create/update)
8. Reconcile EndpointSlice (create/update)
9. Record metrics

**Event-Driven:**
- Service changes → reconcile that Service
- Pod changes → find matching Services and reconcile
- EndpointSlice changes → reconcile source Service (drift detection)

### Port Resolution

**Problem:** Service `targetPort` may be named, but EndpointSlice needs numeric port.

**Solution:**
1. For each ServicePort:
   - If `targetPort` is int → use that int
   - If `targetPort` is name → resolve from leader pod container ports
   - If unresolved → fallback to ServicePort.port and emit Warning Event

**Example:**
```yaml
# Service
ports:
- port: 80
  targetPort: http  # Named port

# Pod
ports:
- containerPort: 8080
  name: http  # Matches targetPort name

# EndpointSlice (resolved)
ports:
- port: 8080  # Resolved from container port name
```

### Leader Selection Strategy

**Sticky Leader (default):**
- Keeps current leader if still Ready
- Reduces churn and unnecessary failovers
- Disabled via `zen-lead.io/sticky: "false"`

**Earliest Ready Pod Selection:**
- Selects pod with earliest `creationTimestamp`
- Tie-breaker: lexical pod name
- Ensures deterministic selection

**No Ready Pods:**
- EndpointSlice has zero endpoints
- Leader Service exists but routes nowhere
- Clean failure mode (no errors)

## Ownership & Cleanup

**Ownership Chain:**
```
Source Service (S)
  └─> Leader Service (S-leader) [ownerRef → S]
        └─> EndpointSlice [ownerRef → S-leader]
```

**Cleanup Behavior:**
- Remove `zen-lead.io/enabled` annotation → Leader Service deleted → EndpointSlice deleted (GC)
- Delete source Service → Leader Service deleted → EndpointSlice deleted (GC)
- Controller-side safety cleanup for stale resources (label-based sweep)

## RBAC Requirements

**Minimum Permissions (day-0):**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]  # Read-only
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

**No Permissions For:**
- `pods/patch` or `pods/update` (no pod mutation)
- `coordination.k8s.io/leases` (not used)
- `coordination.kube-zen.io/leaderpolicies` (not used)

## Performance Considerations

### Reconciliation Frequency

- **Event-Driven:** Reconciliation triggered by Service/Pod/EndpointSlice changes
- **No Polling:** Controller doesn't poll, only reacts to events
- **Fast Failover:** Bounded by readiness transition + controller reconcile + kube-proxy update (~2-5 seconds)
- **Configurable Concurrency:** `maxConcurrentReconciles` controls parallel reconciliation (default: 10)

### Resource Usage

- **Minimal Overhead:** Just Service and EndpointSlice management
- **Scales Linearly:** With number of opted-in Services
- **No Additional Goroutines:** Beyond controller-runtime defaults
- **In-Memory Cache:** LRU cache for opted-in Services (default: 1000 per namespace, configurable)

### Configuration

Key performance-related configuration options:
- `maxCacheSizePerNamespace`: Cache size limit per namespace (default: 1000)
- `maxConcurrentReconciles`: Maximum concurrent reconciliations (default: 10)
- `cacheUpdateTimeoutSeconds`: Timeout for cache updates (default: 10s)
- `metricsCollectionTimeoutSeconds`: Timeout for metrics collection (default: 5s)
- `qps`: Kubernetes API client QPS (default: 50)
- `burst`: Kubernetes API client burst limit (default: 100)

See `docs/PERFORMANCE_TUNING.md` for detailed tuning guidance.

## Limitations

### Network-Level Only

Zen-Lead provides **network-level single-active routing**. It does NOT:
- Guarantee application-level correctness
- Provide distributed consensus
- Handle application state coordination
- Prevent split-brain at application level

**Use Case:** Suitable for stateless applications or applications that handle their own state coordination.

### Failover Latency

Failover is bounded by:
- Pod readiness transition latency
- Controller reconciliation latency (~1-2 seconds)
- kube-proxy EndpointSlice update latency (~1-2 seconds)

**Total:** Typically 2-5 seconds for complete failover.

## Security

### Non-Invasive Design

- **No Pod Mutation:** Controller never patches or updates pods
- **Read-Only Pod Access:** Controller only reads pod status
- **Least-Privilege RBAC:** Minimal permissions required

### Resource Isolation

- **Ownership:** All generated resources owned by source Service
- **Labels:** Clear labeling for identification (`app.kubernetes.io/managed-by: zen-lead`)
- **Garbage Collection:** Automatic cleanup via owner references
