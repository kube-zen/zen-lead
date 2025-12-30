# Zen-Lead Architecture

## Overview

Zen-Lead provides a standardized High Availability (HA) solution for Kubernetes workloads. Instead of requiring developers to implement leader election code, they simply annotate their Deployments.

## Core Concepts

### LeaderPolicy

A `LeaderPolicy` CRD defines a pool (group) of replicas that participate in leader election.

**Key Fields:**
- `spec.leaseDurationSeconds`: How long a leader holds the lease (default: 15s)
- `spec.identityStrategy`: How pod identity is derived ("pod" or "custom")
- `spec.followerMode`: What followers do ("standby" or "scaleDown")
- `status.currentHolder`: Current leader information
- `status.candidates`: Number of pods in the pool

### Pod Annotations

Pods participate in leader election via annotations:

- `zen-lead/pool`: Name of the LeaderPolicy to join
- `zen-lead/join`: Set to "true" to participate
- `zen-lead/role`: Automatically set by zen-lead ("leader" or "follower")

### Lease Resource

Zen-Lead uses Kubernetes `Lease` resources (coordination.k8s.io) for leader election. The Lease is created with the same name as the LeaderPolicy.

## Architecture Flow

```
1. User creates LeaderPolicy
   └─> zen-lead controller watches for LeaderPolicy

2. User annotates Deployment pods
   └─> zen-lead controller detects pods with zen-lead/pool annotation

3. Controller finds candidates
   └─> Lists all pods with matching pool annotation

4. Controller monitors Lease
   └─> Reads Lease resource to determine current leader

5. Controller updates pod annotations
   └─> Sets zen-lead/role: leader or follower

6. Controller updates LeaderPolicy status
   └─> Shows current leader, phase, candidates count
```

## Components

### LeaderPolicy Controller

**Responsibilities:**
- Reconciles LeaderPolicy resources
- Finds candidate pods
- Monitors Lease resources
- Updates pod role annotations
- Updates LeaderPolicy status

**Reconciliation Loop:**
1. Fetch LeaderPolicy
2. Find all candidate pods (with pool annotation)
3. Get Lease resource
4. Determine current leader from Lease
5. Update pod role annotations
6. Update LeaderPolicy status
7. Requeue after 5 seconds

### Pod Event Handler

**Responsibilities:**
- Watches Pod resources
- Triggers reconciliation when pods with pool annotations change

**Events Handled:**
- Pod created with pool annotation
- Pod updated with pool annotation
- Pod deleted with pool annotation

### Election Wrapper

**Responsibilities:**
- Wraps client-go leaderelection library
- Manages lease acquisition
- Handles callbacks

**Note:** Currently used for reference. In future, pods could use this directly to participate in election.

### Pool Manager

**Responsibilities:**
- Finds pods participating in a pool
- Updates pod role annotations
- Helper functions for annotation management

## Leader Election Mechanism

### Standard Kubernetes Pattern

Zen-Lead uses the standard Kubernetes leader election pattern:

1. **Lease Resource**: Created in the same namespace as the LeaderPolicy
2. **Lease Duration**: How long a leader holds the lease (default: 15s)
3. **Renew Deadline**: Time to renew before losing leadership (default: 10s)
4. **Retry Period**: How often to retry acquiring leadership (default: 2s)

### Identity Strategy

**Pod Strategy (default):**
- Uses Pod Name + UID
- Format: `pod-name-uid` or `pod-name-timestamp`

**Custom Strategy:**
- Uses annotation value from `zen-lead/identity`
- Requires `ZEN_LEAD_IDENTITY` environment variable

### Follower Mode

**Standby (default):**
- Pods stay running
- Marked with `zen-lead/role: follower`
- Applications can check annotation to know if they're leader

**ScaleDown (future):**
- Pods scale to 0 for followers
- Requires HPA integration
- Advanced feature for Phase 2

## Status API

### LeaderPolicy Status

```yaml
status:
  phase: Stable  # Electing or Stable
  currentHolder:
    name: my-pod-123
    uid: 5d4b123-c4f5-1234-5678
    startTime: "2025-12-28T10:00:00Z"
  candidates: 3
  conditions:
    - type: LeaderElected
      status: "True"
    - type: CandidatesAvailable
      status: "True"
```

### Querying Status

```bash
# Get current leader
kubectl get leaderpolicy my-pool -o jsonpath='{.status.currentHolder.name}'

# Get number of candidates
kubectl get leaderpolicy my-pool -o jsonpath='{.status.candidates}'

# Get phase
kubectl get leaderpolicy my-pool -o jsonpath='{.status.phase}'
```

## Integration Patterns

### Pattern 1: Annotation-Based (Recommended)

**Use Case:** Any workload (Deployment, StatefulSet, CronJob)

**Steps:**
1. Create LeaderPolicy
2. Annotate pods with `zen-lead/pool` and `zen-lead/join: "true"`
3. Application checks `zen-lead/role` annotation to know if it's leader

**Example:**
```yaml
annotations:
  zen-lead/pool: my-pool
  zen-lead/join: "true"
```

### Pattern 2: Status Query (Future)

**Use Case:** External applications querying leader status

**Steps:**
1. Query LeaderPolicy status via Kubernetes API
2. Check if current pod is the leader
3. Act accordingly

## Security

### RBAC Requirements

Zen-Lead requires the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
rules:
- apiGroups: ["coordination.kube-zen.io"]
  resources: ["leaderpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "update", "patch"]
```

## Performance Considerations

### Reconciliation Frequency

- Default: Every 5 seconds
- Adjustable via RequeueAfter in controller
- Pod events trigger immediate reconciliation

### Lease Renewal

- Standard Kubernetes defaults (15s/10s/2s)
- Configurable per LeaderPolicy
- Appropriate for most use cases

### Resource Usage

- Minimal overhead (just lease updates and status updates)
- No additional goroutines beyond controller-runtime
- Scales with number of LeaderPolicies and candidates

## Future Enhancements

### Phase 2: Follower ScaleDown

- Integrate with HPA
- Scale followers to 0
- Save resources

### Phase 3: Distributed Locking

- ManualLock CRD
- Acquire/release locks
- Prevent parallel execution

### Phase 4: gRPC/HTTP Status API

- Query leader status via API
- No need to query Kubernetes API
- Better for external integrations

