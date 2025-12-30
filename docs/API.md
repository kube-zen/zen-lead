# Zen-Lead API Reference

## LeaderPolicy CRD

**Group:** `coordination.kube-zen.io`  
**Version:** `v1alpha1`  
**Kind:** `LeaderPolicy`

### Specification

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: <pool-name>
  namespace: <namespace>
spec:
  # Lease duration in seconds (default: 15)
  leaseDurationSeconds: int32
  
  # Identity strategy: "pod" or "custom" (default: "pod")
  identityStrategy: string
  
  # Follower mode: "standby" or "scaleDown" (default: "standby")
  followerMode: string
  
  # Renew deadline in seconds (default: 10)
  renewDeadlineSeconds: int32
  
  # Retry period in seconds (default: 2)
  retryPeriodSeconds: int32
```

### Status

```yaml
status:
  # Phase: "Electing" or "Stable"
  phase: string
  
  # Current leader (if any)
  currentHolder:
    name: string
    uid: string
    namespace: string
    startTime: Time
  
  # Number of candidates
  candidates: int32
  
  # Last transition time
  lastTransitionTime: Time
  
  # Conditions
  conditions:
    - type: string
      status: string
      reason: string
      message: string
```

## Pod Annotations

### Required Annotations

- `zen-lead/pool`: Name of the LeaderPolicy to join
- `zen-lead/join`: Set to `"true"` to participate in election

### Automatic Annotations

- `zen-lead/role`: Set by zen-lead to `"leader"` or `"follower"`

### Optional Annotations

- `zen-lead/identity`: Custom identity (when `identityStrategy: custom`)

## Environment Variables

### For Pods Participating in Election

- `POD_NAME`: Pod name (automatically set by Kubernetes)
- `POD_UID`: Pod UID (automatically set by Kubernetes)
- `ZEN_LEAD_IDENTITY`: Custom identity (when using custom strategy)

### For zen-lead Controller

- `WATCH_NAMESPACE`: Namespace to watch (empty = all namespaces)
- `POD_NAMESPACE`: Controller namespace

## Kubernetes Resources

### Lease Resource

Zen-Lead creates a `Lease` resource for each LeaderPolicy:

```yaml
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: <leader-policy-name>
  namespace: <namespace>
spec:
  holderIdentity: <pod-identity>
  leaseDurationSeconds: <lease-duration>
  acquireTime: <timestamp>
  renewTime: <timestamp>
```

## API Examples

### Create LeaderPolicy

```bash
kubectl apply -f - <<EOF
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-pool
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby
EOF
```

### Get LeaderPolicy Status

```bash
kubectl get leaderpolicy my-pool -o yaml
```

### Query Current Leader

```bash
kubectl get leaderpolicy my-pool -o jsonpath='{.status.currentHolder.name}'
```

### List All LeaderPolicies

```bash
kubectl get leaderpolicies
```

### Watch Leader Changes

```bash
kubectl get leaderpolicy my-pool -w -o jsonpath='{.status.currentHolder.name}{"\n"}'
```

## Field Reference

### LeaderPolicySpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `leaseDurationSeconds` | int32 | 15 | How long a leader holds the lease |
| `identityStrategy` | string | "pod" | How pod identity is derived |
| `followerMode` | string | "standby" | What followers do |
| `renewDeadlineSeconds` | int32 | 10 | Time to renew before losing leadership |
| `retryPeriodSeconds` | int32 | 2 | How often to retry acquiring leadership |

### LeaderPolicyStatus

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current election phase ("Electing" or "Stable") |
| `currentHolder` | LeaderHolder | Current leader information |
| `candidates` | int32 | Number of pods in the pool |
| `lastTransitionTime` | Time | When phase last changed |
| `conditions` | []Condition | Status conditions |

### LeaderHolder

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Pod name or custom identity |
| `uid` | string | Pod UID (if identityStrategy is "pod") |
| `namespace` | string | Namespace of the leader pod |
| `startTime` | Time | When leader acquired the lease |

## Validation Rules

### leaseDurationSeconds
- Minimum: 5
- Maximum: 300
- Must be > renewDeadlineSeconds

### renewDeadlineSeconds
- Minimum: 2
- Maximum: 60
- Must be < leaseDurationSeconds

### retryPeriodSeconds
- Minimum: 1
- Maximum: 10

### identityStrategy
- Allowed values: "pod", "custom"

### followerMode
- Allowed values: "standby", "scaleDown"

## Status Conditions

### LeaderElected
- **Type:** `LeaderElected`
- **Status:** `True` when a leader is active
- **Reason:** `LeaderActive` or `NoLeader`

### CandidatesAvailable
- **Type:** `CandidatesAvailable`
- **Status:** `True` when candidates are found
- **Reason:** `CandidatesFound` or `NoCandidates`

