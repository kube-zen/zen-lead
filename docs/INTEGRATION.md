# Zen-Lead Integration Guide

## Overview

This guide explains how to integrate zen-lead with your Kubernetes workloads to enable network-level single-active routing.

## Quick Integration

### Step 1: Install Zen-Lead

```bash
# Using Helm (recommended)
helm install zen-lead zen-lead/zen-lead \
  --namespace default \
  --create-namespace
```

**Note:** Deployment manifests are managed via Helm chart in `helm-charts/charts/zen-lead/`.

### Step 2: Annotate Your Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"  # Enable zen-lead
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
```

### Step 3: Use the Leader Service

Update your application configuration to use `<service-name>-leader`:

```yaml
# Deployment environment variable
env:
- name: SERVICE_NAME
  value: my-app-leader  # Points only to current leader
```

**That's it!** Zen-lead automatically:
- Creates `my-app-leader` Service (selector-less)
- Creates EndpointSlice pointing to leader pod
- Updates EndpointSlice when leader changes
- Cleans up when annotation is removed

## Integration Patterns

### Pattern 1: Environment Variable

**Use Case:** Applications that read service name from environment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: SERVICE_NAME
          value: my-app-leader  # Use leader service
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-app
```

### Pattern 2: ConfigMap Reference

**Use Case:** Applications that read service name from ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  service-name: my-app-leader
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        envFrom:
        - configMapRef:
            name: app-config
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-app
```

### Pattern 3: Service Discovery

**Use Case:** Applications using Kubernetes service discovery

```yaml
# Application code (example)
# Instead of: my-app.default.svc.cluster.local
# Use: my-app-leader.default.svc.cluster.local

# DNS resolution automatically points to leader pod
```

## Advanced Configuration

### Custom Leader Service Name

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
    zen-lead.io/leader-service-name: "my-app-primary"  # Custom name
spec:
  selector:
    app: my-app
```

**Result:** Creates `my-app-primary` instead of `my-app-leader`.

### Disable Sticky Leader

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
    zen-lead.io/sticky: "false"  # Disable sticky leader
spec:
  selector:
    app: my-app
```

**Result:** Leader selection always chooses earliest Ready pod (no sticky behavior).

### Named TargetPort

Zen-lead automatically resolves named targetPorts:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-server
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: api-server
  ports:
  - port: 443
    targetPort: https  # Named port
    name: https
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
spec:
  template:
    spec:
      containers:
      - name: api
        ports:
        - containerPort: 8443
          name: https  # Must match targetPort name
```

**Result:** EndpointSlice uses port 8443 (resolved from container port name).

## Verification

### Check Leader Service

```bash
# Get leader Service
kubectl get service my-app-leader

# Verify selector is null
kubectl get service my-app-leader -o jsonpath='{.spec.selector}'
# Should return: null
```

### Check Leader Identity

```bash
# Describe leader Service to see leader annotations
kubectl describe service my-app-leader

# Output includes:
# Annotations:  zen-lead.io/leader-pod-name: my-app-abc123
#               zen-lead.io/leader-pod-uid: 12345678-1234-1234-1234-123456789abc
#               zen-lead.io/leader-last-switch-time: 2025-12-31T12:00:00Z

# Get leader pod name directly
kubectl get service my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-pod-name}'

# Get leader pod UID
kubectl get service my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-pod-uid}'

# Get last switch time
kubectl get service my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-last-switch-time}'
```

### Check EndpointSlice

```bash
# Get EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader

# Check endpoints
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].addresses}'
# Should show exactly one IP (leader pod)
```

### Check Events

```bash
# View events for the source Service
kubectl get events --field-selector involvedObject.name=my-app --sort-by='.lastTimestamp'

# Common events:
# - LeaderServiceCreated: Leader service creation event
# - LeaderRoutingAvailable: Leader routing is available
# - LeaderChanged: Leader pod changed
# - PortResolutionFailed: Port resolution failed (fail-closed)
# - NoReadyPods: No ready pods available
# - NoPodsFound: No pods found matching selector
```

### Test Failover

```bash
# Get current leader pod
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].targetRef.name}'

# Or use leader Service annotation
kubectl get service my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-pod-name}'

# Delete leader pod
kubectl delete pod <leader-pod-name>

# Watch for new leader
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -w

# Or watch events
kubectl get events --field-selector involvedObject.name=my-app -w
```

## Migration from Wrong Service Usage

### Problem: Using Source Service Instead of Leader Service

**Symptom:** Application connects to `my-app` instead of `my-app-leader`, receiving traffic from all pods instead of just the leader.

**Solution:** Update application configuration to use the leader Service.

### Migration Pattern 1: Environment Variable

```yaml
# Incorrect - uses all pods
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app-client
spec:
  template:
    spec:
      containers:
      - name: client
        env:
        - name: SERVICE_NAME
          value: my-app  # ❌ Wrong - routes to all pods
---
# After (correct - uses leader only)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app-client
spec:
  template:
    spec:
      containers:
      - name: client
        env:
        - name: SERVICE_NAME
          value: my-app-leader  # ✅ Correct - routes to leader only
```

### Migration Pattern 2: ConfigMap

```yaml
# Incorrect
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  service-name: my-app  # ❌ Wrong
---
# After (correct)
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  service-name: my-app-leader  # ✅ Correct
```

### Migration Pattern 3: Service Discovery

```yaml
# Application code change
# Without zen-lead: my-app.default.svc.cluster.local
# After:  my-app-leader.default.svc.cluster.local
```

### Verification After Migration

```bash
# Verify client is connecting to leader Service
# Check DNS resolution (from client pod)
kubectl exec -it <client-pod> -- nslookup my-app-leader

# Verify only one endpoint (leader pod)
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].addresses}'
# Should show exactly one IP

# Compare with source Service (should show multiple IPs)
kubectl get endpointslice -l kubernetes.io/service-name=my-app -o jsonpath='{.items[*].endpoints[*].addresses}'
# Should show multiple IPs (all pods)
```

## Troubleshooting

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues and solutions.

## Best Practices

1. **Use Readiness Probes:** Ensure pods have proper readiness probes for accurate leader selection
2. **Monitor Metrics:** Use Prometheus metrics to monitor failover rate and reconciliation duration
3. **Test Failover:** Regularly test failover scenarios to ensure reliability
4. **Resource Limits:** Set appropriate resource limits to prevent pod evictions
5. **Multiple Replicas:** Run at least 2-3 replicas for high availability

## Limitations

- **Network-Level Only:** Provides network routing, not application-level coordination
- **Failover Latency:** 2-5 seconds typical failover time
- **No State Coordination:** Applications must handle their own state coordination

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture information.
