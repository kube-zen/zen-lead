# Zen-Lead

**Network-Level Single-Active Routing for Kubernetes - Zero Code Changes**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> **"Annotate a Service → zen-lead creates `<svc>-leader` selector-less Service + EndpointSlice"**

Zen-Lead is a **non-invasive leader election controller** for Kubernetes that provides network-level single-active routing without requiring application code changes or mutating workload pods.

## 🎯 What Zen-Lead Does

Zen-Lead watches Services with the `zen-lead.io/enabled: "true"` annotation and automatically:
- Creates a selector-less `<service-name>-leader` Service
- Creates an EndpointSlice pointing to exactly one Ready pod (the leader)
- Updates the EndpointSlice when the leader changes (automatic failover)
- Cleans up when the annotation is removed

**Your application:** Just connect to `<service-name>-leader` instead of `<service-name>`. That's it.

## ✨ Key Features

- ✅ **Zero Code Changes**: Applications don't need to know about leader election
- ✅ **Non-Invasive**: No pod mutation, no changes to user resources
- ✅ **Service-First Opt-In**: Annotate any Service with `zen-lead.io/enabled: "true"`
- ✅ **Automatic Failover**: Controller-driven leader selection based on pod readiness
- ✅ **Production-Ready**: Secure defaults, namespace-scoped, event-driven reconciliation
- ✅ **Small Footprint**: No sidecars, minimal RBAC, K8s-native primitives only
- ✅ **Safe-by-Default**: Fail-closed port resolution, no pod mutation, HA controller with mandatory leader election

## 🚀 Quick Start

### Step 1: Deploy Your Application

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: app
        image: my-app:latest
        ports:
        - containerPort: 8080
          name: http
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
    name: http
```

### Step 2: Enable Zen-Lead

```bash
# Annotate the Service
kubectl annotate service my-app zen-lead.io/enabled=true
```

### Step 3: Use the Leader Service

```yaml
# Update your application config
env:
- name: SERVICE_NAME
  value: my-app-leader  # Points only to current leader
```

**That's it!** Zen-Lead automatically:
- Creates `my-app-leader` Service (selector-less)
- Creates EndpointSlice pointing to leader pod
- Updates EndpointSlice when leader changes
- Cleans up when annotation is removed

## 📖 Usage Examples

### Basic Usage

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
```

**Result:** `my-app-leader` Service routes to exactly one Ready pod.

### Named TargetPort

Zen-Lead automatically resolves named targetPorts:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: http  # Named port
    name: http
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
        ports:
        - containerPort: 8080
          name: http  # Matches targetPort name
```

**Result:** EndpointSlice uses port 8080 (resolved from container port name).

### Custom Leader Service Name

```yaml
metadata:
  annotations:
    zen-lead.io/enabled: "true"
    zen-lead.io/leader-service-name: "my-app-primary"
```

**Result:** Creates `my-app-primary` instead of `my-app-leader`.

## 🔧 Installation

### Helm Installation (Recommended)

```bash
# Add Helm repository
helm repo add zen-lead https://kube-zen.github.io/zen-lead/charts
helm repo update

# Install zen-lead (namespace-scoped, non-invasive defaults)
helm install zen-lead zen-lead/zen-lead \
  --namespace default \
  --create-namespace
```

### Verify Installation

```bash
# Check controller pods
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Verify no pod mutation permissions
kubectl auth can-i patch pods --as=system:serviceaccount:default:zen-lead
# Should return: no
```

## 📊 Metrics & Observability

Zen-Lead exposes Prometheus metrics at `/metrics` (port 8080):

- `zen_lead_leader_duration_seconds` - How long a pod has been leader
- `zen_lead_failover_count_total` - Total number of failovers
- `zen_lead_reconciliation_duration_seconds` - Reconciliation duration
- `zen_lead_pods_available` - Ready pods count
- `zen_lead_port_resolution_failures_total` - Port resolution failures
- `zen_lead_reconciliation_errors_total` - Reconciliation errors

See [deploy/prometheus/prometheus-rules.yaml](deploy/prometheus/prometheus-rules.yaml) for alert rules and [deploy/grafana/dashboard.json](deploy/grafana/dashboard.json) for Grafana dashboard.

## 🛠️ Troubleshooting

### Leader Service Has No Endpoints

```bash
# Check EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader

# Check if any pods are Ready
kubectl get pods -l <selector> --field-selector=status.phase=Running

# Check controller logs
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=100
```

**Solutions:**
1. Ensure at least one pod is Ready (check readiness probe)
2. Verify Service has a selector
3. Check controller logs for errors

### Port Resolution Fails

```bash
# Check events
kubectl get events --field-selector involvedObject.name=<service> --sort-by='.lastTimestamp'

# Verify container port names match targetPort
kubectl get pod <leader-pod> -o jsonpath='{.spec.containers[*].ports[*].name}'
```

**Solutions:**
1. Ensure container port names match Service targetPort names
2. Check that leader pod has the named port
3. Controller fails closed (no endpoints) if port resolution fails - check events for details

### Leader Doesn't Change on Failure

```bash
# Check pod readiness
kubectl get pod <leader-pod> -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'

# Check controller reconciliation
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i reconcile
```

**Solutions:**
1. Verify controller is running and reconciling
2. Check if sticky leader is enabled (may delay failover)
3. Ensure pod readiness transitions are detected

## 📋 Day-0 Contract (Guaranteed)

**Zen-Lead v0.1.0 provides a minimal, production-ready leader election solution with these guarantees:**

### ✅ What's Included (Day-0)

- **CRD-Free**: No CustomResourceDefinitions required. Works with standard Kubernetes resources only.
- **No Webhook**: No validating or mutating admission webhooks. Zero impact on API server performance.
- **No Pod Mutation**: Never patches or updates workload pods. Completely non-invasive.
- **Service Annotation Opt-In**: Simple annotation-based opt-in (`zen-lead.io/enabled: "true"`).
- **Managed Resources Only**: Creates only two resources per opted-in Service:
  - Selector-less `<service-name>-leader` Service
  - Single-endpoint EndpointSlice with Pod targetRef
- **Vanilla Kubernetes**: Works on any Kubernetes cluster (1.24+) with default kube-proxy (iptables mode).
- **Event-Driven**: Fast failover detection via Pod watch predicates (< 1 second controller-side).
- **Secure Defaults**: Namespace-scoped RBAC, restricted security contexts, mandatory controller leader election.

### 🚫 What's NOT Included (Day-0)

- **No CRDs**: No CustomResourceDefinitions required.
- **No Webhooks**: No admission webhooks.
- **No Pod Mutation**: No leader labels, annotations, or role assignments on workload pods.
- **No Advanced Policies**: No multi-election, synthetic health checks, or complex configuration.
- **No Dataplane Acceleration**: eBPF/Cilium/IPVS optimizations are optional and not required.

### 🔮 Roadmap (Optional Add-ons)

Future enhancements may include:
- **Dataplane Acceleration**: Optional guidance for eBPF (Cilium), IPVS kube-proxy, or kube-proxy tuning to reduce dataplane convergence time.
- **Advanced Configuration**: If introduced, will be a separate optional module/chart, never required for core functionality.

**Important:** Roadmap items will never compromise the day-0 guarantee. The core product will always remain CRD-free, webhook-free, and pod-mutation-free.

## ⚠️ Limitations

### Network-Level Routing Only

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

### NetworkPolicy Compatibility

**NetworkPolicy is pod-based** - it applies to pods based on pod labels, not Service selectors. The leader Service being selector-less does **not** bypass NetworkPolicy.

**Normal behavior:** NetworkPolicy rules that select pods by labels work correctly with zen-lead. Traffic to the leader pod is controlled by the same NetworkPolicy rules that apply to all pods in the Service.

**Known limitation:** If you rely on NetworkPolicy rules that are keyed to Service selectors (non-standard pattern), you may need to adapt your policies. Standard pod-label-based NetworkPolicy works without changes.

### Headless Services

If the source Service is headless (`spec.clusterIP: None`), zen-lead still allows opt-in. The leader Service defaults to `ClusterIP` (normal) unless explicitly overridden. This ensures the leader Service is routable even when the source Service is headless.

## 🔒 Security

Zen-Lead follows Kubernetes security best practices:

- **Non-Root Execution**: Runs as UID 65534 (nobody)
- **Read-Only Root Filesystem**: Enabled by default
- **No Privilege Escalation**: `allowPrivilegeEscalation: false`
- **Dropped Capabilities**: All capabilities dropped
- **Least-Privilege RBAC**: Minimal permissions by default (no pod mutation)

## 📚 Documentation

- [Client Resilience Guide](docs/CLIENT_RESILIENCE.md) - **Read this for failover expectations and client best practices**
- [Architecture](docs/ARCHITECTURE.md) - How zen-lead works internally
- [Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues and solutions
- [Examples](examples/) - Example configurations

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

Zen-Lead is part of the [Kube-ZEN](https://github.com/kube-zen) project.
