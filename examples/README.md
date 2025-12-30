# Zen-Lead Examples

This directory contains example configurations for using zen-lead.

## Examples

### 1. Basic LeaderPolicy

**File:** `leaderpolicy.yaml`

Creates a basic LeaderPolicy with default settings.

```bash
kubectl apply -f leaderpolicy.yaml
```

### 2. Deployment with Pool

**File:** `deployment-with-pool.yaml`

Shows how to annotate a Deployment to participate in leader election.

**Key Annotations:**
- `zen-lead/pool: my-controller-pool` - Joins the pool
- `zen-lead/join: "true"` - Participates in election

**Usage:**
1. Create the LeaderPolicy first:
   ```bash
   kubectl apply -f leaderpolicy.yaml
   ```

2. Apply the Deployment:
   ```bash
   kubectl apply -f deployment-with-pool.yaml
   ```

3. Check status:
   ```bash
   kubectl get leaderpolicy my-controller-pool
   kubectl get pods -l app=my-controller -o jsonpath='{.items[*].metadata.annotations.zen-lead/role}'
   ```

### 3. CronJob with Pool

**File:** `cronjob-with-pool.yaml`

Shows how to use zen-lead with CronJobs to ensure only one instance runs.

**Use Case:** Prevent duplicate job execution across multiple nodes.

**Usage:**
```bash
kubectl apply -f cronjob-with-pool.yaml
```

## Integration Examples

### Example: zen-flow Integration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-flow-controller
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: flow-controller
        zen-lead/join: "true"
```

### Example: zen-watcher Integration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-watcher
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: watcher-primary
        zen-lead/join: "true"
```

## Testing Leader Election

### Test Failover

1. Deploy with 3 replicas:
   ```bash
   kubectl scale deployment my-controller --replicas=3
   ```

2. Check current leader:
   ```bash
   kubectl get leaderpolicy my-controller-pool -o jsonpath='{.status.currentHolder.name}'
   ```

3. Delete the leader pod:
   ```bash
   kubectl delete pod <leader-pod-name>
   ```

4. Watch for new leader:
   ```bash
   kubectl get leaderpolicy my-controller-pool -w
   ```

### Verify Role Annotations

```bash
# Get all pods and their roles
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/role}{"\n"}{end}'
```

## Troubleshooting

### No Leader Elected

```bash
# Check if LeaderPolicy exists
kubectl get leaderpolicy my-pool

# Check for candidates
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/pool}{"\n"}{end}'

# Check Lease resource
kubectl get lease my-pool
```

### Multiple Leaders

This should not happen, but if it does:

```bash
# Check Lease resource
kubectl get lease my-pool -o yaml

# Check pod annotations
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/role}{"\n"}{end}'
```

