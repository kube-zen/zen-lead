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

# Or using kubectl
kubectl apply -f config/rbac/
kubectl apply -f deploy/
```

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

**Result:** Leader selection always chooses oldest Ready pod (no sticky behavior).

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

### Check EndpointSlice

```bash
# Get EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader

# Check endpoints
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].addresses}'
# Should show exactly one IP (leader pod)
```

### Test Failover

```bash
# Get current leader pod
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].targetRef.name}'

# Delete leader pod
kubectl delete pod <leader-pod-name>

# Watch for new leader
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -w
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
