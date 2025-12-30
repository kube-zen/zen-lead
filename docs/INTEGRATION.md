# Zen-Lead Integration Guide

## Overview

This guide explains how to integrate zen-lead with your Kubernetes workloads to enable High Availability (HA).

## Quick Integration

### Step 1: Install Zen-Lead

```bash
# Install CRDs
kubectl apply -f config/crd/bases/

# Install RBAC
kubectl apply -f config/rbac/

# Install Controller
kubectl apply -f deploy/
```

### Step 2: Create LeaderPolicy

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-controller-pool
  namespace: default
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby
```

### Step 3: Annotate Your Deployment

```yaml
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

**That's it!** Only 1 of 3 replicas will be active.

## Integration Patterns

### Pattern 1: Check Annotation in Application

**Use Case:** Your application needs to know if it's the leader

**Implementation:**

```go
package main

import (
    "os"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

func isLeader() bool {
    podName := os.Getenv("POD_NAME")
    namespace := os.Getenv("POD_NAMESPACE")
    
    // Get pod
    config, _ := rest.InClusterConfig()
    clientset, _ := kubernetes.NewForConfig(config)
    pod, _ := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
    
    // Check annotation
    role, ok := pod.Annotations["zen-lead/role"]
    return ok && role == "leader"
}

func main() {
    if isLeader() {
        // Only leader does this work
        startReconciliation()
    } else {
        // Follower waits
        waitForLeadership()
    }
}
```

### Pattern 2: Query LeaderPolicy Status

**Use Case:** External application needs to know who's the leader

**Implementation:**

```bash
# Get current leader
LEADER=$(kubectl get leaderpolicy my-pool -o jsonpath='{.status.currentHolder.name}')

# Check if current pod is leader
if [ "$LEADER" == "$POD_NAME" ]; then
    echo "I am the leader"
fi
```

### Pattern 3: Environment Variable Injection

**Use Case:** Inject leader status as environment variable

**Implementation:**

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: IS_LEADER
          valueFrom:
            fieldRef:
              fieldPath: metadata.annotations['zen-lead/role']
        # Note: This requires Kubernetes 1.28+ for annotation fieldRef
```

## Integration Examples

### Example 1: zen-flow Integration

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

**Result:** Only 1 replica actively reconciles JobFlows.

### Example 2: zen-watcher Integration

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

**Result:** Only 1 replica writes to Observation CRDs.

### Example 3: CronJob Integration

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
  annotations:
    zen-lead/pool: report-generator
    zen-lead/join: "true"
spec:
  schedule: "0 0 * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            zen-lead/pool: report-generator
            zen-lead/join: "true"
```

**Result:** Only 1 cluster instance executes, even with multiple nodes.

## Best Practices

### 1. Use Descriptive Pool Names

```yaml
# Good
zen-lead/pool: flow-controller
zen-lead/pool: watcher-primary

# Bad
zen-lead/pool: pool1
zen-lead/pool: test
```

### 2. Set Appropriate Lease Duration

```yaml
# For fast failover (high availability)
leaseDurationSeconds: 10
renewDeadlineSeconds: 7

# For stable workloads (default)
leaseDurationSeconds: 15
renewDeadlineSeconds: 10
```

### 3. Monitor Leader Transitions

```bash
# Alert on frequent leadership changes
kubectl get leaderpolicy my-pool -o jsonpath='{.status.conditions[?(@.type=="LeaderElected")]}'
```

### 4. Use Standby Mode for Most Cases

```yaml
# Recommended for most workloads
followerMode: standby

# Only use scaleDown if you need resource savings
followerMode: scaleDown  # Phase 2 feature
```

## Troubleshooting

### Issue: No Leader Elected

**Symptoms:**
- `status.phase: Electing`
- `status.currentHolder: null`

**Possible Causes:**
1. No pods with pool annotation
2. Pods not running
3. RBAC issues

**Solutions:**
```bash
# Check for candidates
kubectl get pods -l zen-lead/pool=my-pool

# Check pod annotations
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'

# Check RBAC
kubectl auth can-i create leases --namespace=default
```

### Issue: Multiple Leaders

**Symptoms:**
- Multiple pods with `zen-lead/role: leader`

**Possible Causes:**
1. Clock skew
2. Network partition
3. Lease API issues

**Solutions:**
```bash
# Check system time
date

# Check Lease resource
kubectl get lease my-pool -o yaml

# Verify network connectivity
```

### Issue: Leader Not Processing

**Symptoms:**
- `status.currentHolder` is set
- But application not doing work

**Possible Causes:**
1. Application not checking annotation
2. Application crashed
3. Context cancellation

**Solutions:**
```bash
# Check pod logs
kubectl logs <leader-pod>

# Verify annotation
kubectl get pod <leader-pod> -o jsonpath='{.metadata.annotations.zen-lead/role}'

# Check application code
```

## Advanced Configuration

### Custom Identity Strategy

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
spec:
  identityStrategy: custom
```

**Requires:**
- Set `ZEN_LEAD_IDENTITY` environment variable in pods
- Or use `zen-lead/identity` annotation

### Multiple Pools

You can have multiple LeaderPolicies for different workloads:

```yaml
# Pool 1: Controller HA
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: controller-pool

---
# Pool 2: CronJob
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: cronjob-pool
```

## Migration from Custom Leader Election

If you have existing leader election code:

1. **Remove custom code:**
   - Delete leader election implementation
   - Remove leaderelection imports

2. **Add annotations:**
   ```yaml
   annotations:
     zen-lead/pool: my-pool
     zen-lead/join: "true"
   ```

3. **Update application:**
   - Check `zen-lead/role` annotation instead of `IsLeader()`
   - Or query LeaderPolicy status

4. **Test:**
   - Verify only 1 leader
   - Test failover
   - Monitor transitions

---

**See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture information.**

