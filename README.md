# Zen-Lead

**Universal High Availability for Kubernetes - Zero Friction, Zero Code Changes**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> **"Set it and forget it. HA just works."**

Zen-Lead is a **universal High Availability controller** for Kubernetes that provides automatic leader election and traffic routing. It works with **any** Kubernetes workload—Deployments, StatefulSets, Jobs, CronJobs—without requiring code changes.

## 🎯 The "Grand Unification" Philosophy

**Zero-Opinionated HA**: If you install zen-lead, you want HA. zen-lead automatically detects your workload type and applies the correct strategy.

- **Traffic Director**: For Deployments/Services—routes traffic to leader pod via Service selector
- **Gatekeeper Pattern**: For Deployments—actively blocks non-leader Pod creation at the API level
- **State Guard**: For Jobs/CronJobs—ensures only leader executes logic
- **Smart Defaults**: If replicas > 1, HA is automatically enabled
- **Auto-Detection**: Automatically detects workload Kind and applies correct policy

**No configuration needed. No code changes. Just add one label and scale your replicas.**

## 🚀 Quick Start

### Installation

```bash
# Install zen-lead (one-time, cluster-wide)
helm install zen-lead zen-lead/zen-lead \
  --namespace zen-lead-system \
  --create-namespace
```

### Auto-Discovery: How It Works

**Step 1:** Create a LeaderPolicy (defines a pool)

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: zen-flow-pool
  namespace: zen-flow-system
spec:
  leaseDurationSeconds: 15
```

**Step 2:** Add one label to your Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-flow-controller
  namespace: zen-flow-system
  labels:
    zen-lead/pool: zen-flow-pool  # Add this label
spec:
  replicas: 3  # Scale to 3 for HA
```

**That's it!** zen-lead automatically:
- **Auto-detects** workload Kind (Deployment → TrafficDirector, Job → StateGuard)
- **Gatekeeper Mode**: Blocks non-leader Pod creation at the API level (Validating Admission Webhook)
- **Traffic Director**: Creates a Service (`zen-flow-pool-director`) that routes traffic to the leader
- **Labels**: Labels the leader pod (`zen-lead/role: leader`)
- **Self-Healing**: Updates Service selector when leader changes

**Your application:** 
- **Zero code changes** - zen-lead blocks non-leaders at the API level
- **Zero configuration** - Just add the label and scale replicas
- **HA-unaware** - Your app doesn't need to know about leader election

**Result:** Only the leader Pod is created. Followers are blocked by zen-lead's webhook. Your application runs normally, completely unaware of HA.

## ✨ Features

### Zero Friction

- ✅ **One Label**: Add `zen-lead/pool: <name>` to your Deployment
- ✅ **Auto-Detection**: Automatically detects resource type and applies correct strategy
- ✅ **Smart Defaults**: Replicas > 1 = HA enabled automatically
- ✅ **Zero Code Changes**: Applications are completely HA-unaware (no `IsLeader()` checks)
- ✅ **Gatekeeper Pattern**: Actively blocks non-leader Pod creation at the API level
- ✅ **Standard Primitives**: Uses native Kubernetes Services and Validating Admission Webhooks

### Universal Compatibility

- ✅ **Any Workload**: Deployments, StatefulSets, Jobs, CronJobs
- ✅ **Any Language**: Python, Node.js, Java, Go, Bash, etc.
- ✅ **Any Framework**: Works with controller-runtime, client-go, or no framework
- ✅ **Legacy Apps**: Works with unmodifiable applications

### Automatic Failover

- ✅ **Millisecond Switching**: Traffic switches in ~2-3 seconds
- ✅ **Self-Healing**: Detects leader changes automatically
- ✅ **No Manual Intervention**: Everything happens automatically

## 🏗️ Architecture

### The "Non-Invasive Traffic Director" Pattern

**Key Principle:** zen-lead does NOT mutate workload pods or interfere with existing Services. It creates a new selector-less Service with controller-managed EndpointSlice.

```
┌─────────────────────────────────────────────────────────────┐
│                    User Workflow                           │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
        1. Add label: zen-lead/pool: zen-flow-pool
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                  zen-lead Controller                        │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Auto-Detect  │  │ Select       │  │ NO POD       │     │
│  │ Resource     │─▶│ Leader Pod   │─▶│ MUTATION     │     │
│  │ Type         │  │ (oldest Ready)│  │ (non-invasive)│     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────┐          │
│  │ Create Selector-Less Leader Service           │          │
│  │ spec.selector: nil (no selector!)             │          │
│  │                                                │          │
│  │ Create/Update EndpointSlice                   │          │
│  │ Points to single leader pod IP                │          │
│  └──────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│          Kubernetes Service Discovery                       │
│                                                              │
│  Service: zen-flow-leader (selector-less)                  │
│  EndpointSlice: Managed by controller                       │
│    - Single endpoint: leader pod IP                         │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Leader Pod   │  │ Follower    │  │ Follower    │        │
│  │ (receives    │  │ Pod         │  │ Pod         │        │
│  │  traffic via │  │ (untouched) │  │ (untouched) │        │
│  │  leader svc) │  │             │  │             │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                              │
│  Original Service: zen-flow-controller (unchanged)         │
│  Still works normally, routes to all pods                   │
└─────────────────────────────────────────────────────────────┘
```

**Non-Invasive Benefits:**
- ✅ **No pod mutation**: Workload pods are never patched or labeled
- ✅ **No Service interference**: Original Service continues working normally
- ✅ **Controller-driven selection**: Leader chosen by controller based on pod readiness
- ✅ **Automatic failover**: When leader becomes unhealthy, controller selects new leader

### Auto-Detection Logic

zen-lead automatically detects the resource type and applies the correct strategy:

| Resource Type | Strategy | Behavior |
|--------------|----------|----------|
| Deployment | TrafficDirector | Creates selector-less Service + EndpointSlice pointing to leader |
| StatefulSet | TrafficDirector | Creates selector-less Service + EndpointSlice pointing to leader |
| Service | TrafficDirector | Creates selector-less leader Service + EndpointSlice (annotate Service with zen-lead.io/enabled: "true") |
| Job | StateGuard | Only leader executes logic |
| CronJob | StateGuard | Only leader executes logic |

**Smart Defaults:**
- If `replicas > 1` → HA is automatically enabled
- If `replicas = 1` → HA is skipped (single replica mode)

## 📖 Integration Guide

### For zen-flow, zen-lock, zen-watcher

**Step 1:** Install zen-lead (if not already installed)

```bash
helm install zen-lead zen-lead/zen-lead \
  --namespace zen-lead-system \
  --create-namespace
```

**Step 2:** Create LeaderPolicy

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: zen-flow-pool
  namespace: zen-flow-system
spec:
  leaseDurationSeconds: 15
```

**Step 3:** Add label to Deployment

```yaml
metadata:
  labels:
    zen-lead/pool: zen-flow-pool  # Add this label
spec:
  replicas: 3  # Scale to 3 for HA
```

**Step 4:** Configure application to use Leader Service

```yaml
# In your application config or environment variables
SERVICE_NAME: zen-flow-leader  # Points only to current leader pod
```

**That's it!** No code changes required. Traffic automatically routes to the leader.

**Alternative: Service-based opt-in** (no Deployment label needed)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    zen-lead.io/enabled: "true"
    zen-lead/pool: my-pool
spec:
  selector:
    app: my-app
  ports:
  - port: 80
```

zen-lead will automatically create `my-app-leader` Service pointing to the leader pod.

### For Any Kubernetes Application

Same process:

1. Install zen-lead (one-time, cluster-wide)
2. Create LeaderPolicy
3. Add `zen-lead/pool: <pool-name>` label to Deployment (or annotate Service with `zen-lead.io/enabled: "true"`)
4. Configure app to use `<deployment-name>-leader` Service (or `<service-name>-leader` if using Service annotation)

**Works with any application—no code changes needed!**

## 🔧 Simple Query API

For applications that want to check leader status programmatically, zen-lead provides a simple client SDK:

```go
import "github.com/kube-zen/zen-lead/pkg/client"

// Create client
zenleadClient, _ := client.NewClient(mgr.GetClient())

// Check if this pod is the leader
isLeader, err := zenleadClient.IsLeader(ctx, "zen-flow-pool")
if err != nil {
    // Fail-safe: assume leader if zen-lead not installed
}
if !isLeader {
    // Skip processing - not the leader
    return reconcile.Result{}, nil
}

// Proceed with leader-only logic
```

**Fail-Safe Behavior:**
- If zen-lead is not installed → returns `true` (allows app to work)
- If pod name cannot be determined → returns `true` (local dev)
- If API error → returns `false` (conservative default)

## 📊 Configuration

### LeaderPolicy Spec

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-pool
  namespace: my-namespace
spec:
  # Lease duration in seconds (how long leader holds the lock)
  leaseDurationSeconds: 15
  
  # Renew deadline in seconds (time to renew before losing leadership)
  renewDeadlineSeconds: 10
  
  # Retry period in seconds (how often to retry acquiring leadership)
  retryPeriodSeconds: 2
  
  # Identity strategy: 'pod' (uses Pod Name/UID) or 'custom' (uses annotation)
  identityStrategy: pod
  
  # Follower behavior: 'standby' (pods stay running) or 'scaleDown' (scale to 0)
  followerMode: standby
```

### Deployment Labels

- `zen-lead/pool: <pool-name>` - **Required**: Name of the LeaderPolicy to join

### Leader Service

zen-lead automatically creates a **selector-less** Service named `<deployment-name>-leader` (or `<service-name>-leader` for Service-based opt-in):

- **Selector**: `nil` (no selector - endpoints managed via EndpointSlice)
- **Type**: ClusterIP (default)
- **Ports**: Automatically detected from source Service or Deployment container ports
- **EndpointSlice**: Controller-managed, contains single endpoint pointing to current leader pod

**Important:** zen-lead does NOT mutate workload pods. The leader selection is controller-driven based on pod readiness (oldest Ready pod).

## 🎯 Use Cases

### Use Case 1: zen-flow with HA

**Scenario:** Deploy zen-flow with 3 replicas, ensure only one processes JobFlows.

**Solution:**
```bash
# 1. Install zen-lead
helm install zen-lead zen-lead/zen-lead --namespace zen-lead-system --create-namespace

# 2. Create LeaderPolicy
kubectl apply -f leaderpolicy.yaml

# 3. Add label to zen-flow Deployment
kubectl label deployment zen-flow-controller zen-lead/pool=zen-flow-pool

# 4. Scale to 3 replicas
kubectl scale deployment zen-flow-controller --replicas=3
```

**Result:**
- 3 zen-flow replicas running (pods are never mutated)
- Only leader pod receives traffic via `zen-flow-leader` Service (selector-less, endpoints managed via EndpointSlice)
- Original `zen-flow-controller` Service continues working normally
- On leader failure, controller selects new leader (oldest Ready pod) and updates EndpointSlice
- Zero code changes in zen-flow

### Use Case 2: Multi-Component Coordination

**Scenario:** Coordinate multiple components (zen-flow, zen-lock, zen-watcher) using a single zen-lead controller.

**Solution:**
```bash
# Install zen-lead once (cluster-wide)
helm install zen-lead zen-lead/zen-lead --namespace zen-lead-system --create-namespace

# Create separate pools for each component
kubectl apply -f zen-flow-pool.yaml
kubectl apply -f zen-lock-pool.yaml
kubectl apply -f zen-watcher-pool.yaml

# Add pool labels to each deployment
# zen-flow: zen-lead/pool: zen-flow-pool
# zen-lock: zen-lead/pool: zen-lock-pool
# zen-watcher: zen-lead/pool: zen-watcher-pool
```

**Result:**
- One zen-lead controller manages all pools
- Each component has its own leader Service (selector-less, non-invasive)
- Independent leader election per component
- Centralized HA management
- No interference with existing Services or pods

### Use Case 3: Standard Kubernetes Application

**Scenario:** Make an existing Kubernetes application HA-aware without code changes.

**Solution:**
```yaml
# 1. Create LeaderPolicy
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-app-pool
  namespace: production
spec:
  leaseDurationSeconds: 15

# 2. Add label to existing Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: production
  labels:
    zen-lead/pool: my-app-pool  # Add this
spec:
  replicas: 3
  # ... existing spec

# 3. Update application config to use Director Service
# OLD: SERVICE_NAME=my-app
# NEW: SERVICE_NAME=my-app-pool-director
```

**Result:**
- Application becomes HA-aware
- No code changes required
- No pod mutations (pods remain untouched)
- Traffic automatically routes to leader via selector-less Service
- Failover handled by controller (selects new leader when current leader becomes unhealthy)

## 🔍 Troubleshooting

### Service Not Routing Traffic

**Symptoms:** Director Service exists but no endpoints

**Diagnosis:**
```bash
# Check leader Service (should have no selector)
kubectl get service zen-flow-leader -o yaml
# spec.selector should be null or empty

# Check EndpointSlice (managed by controller)
kubectl get endpointslice -l discovery.k8s.io/service-name=zen-flow-leader -o yaml

# Check which pod is the leader (from EndpointSlice)
kubectl get endpointslice -l discovery.k8s.io/service-name=zen-flow-leader -o jsonpath='{.items[0].endpoints[0].targetRef.name}'

# Check pod readiness
kubectl get pods -l app=zen-flow
```

**Solutions:**
1. Ensure leader Service has `spec.selector: null` (no selector)
2. Check EndpointSlice contains exactly one endpoint pointing to leader pod
3. Verify leader pod is Ready (controller selects oldest Ready pod)
4. Check zen-lead controller logs for errors

### No Leader Elected

**Symptoms:** Leader Service has no endpoints or EndpointSlice is empty

**Diagnosis:**
```bash
# Check LeaderPolicy status
kubectl get leaderpolicy zen-flow-pool -o yaml

# Check EndpointSlice
kubectl get endpointslice -l discovery.k8s.io/service-name=zen-flow-leader

# Check candidate pods (must be Ready)
kubectl get pods -l zen-lead/pool=zen-flow-pool
# Look for pods with Ready condition = True

# Check controller logs
kubectl logs -n zen-lead-system deployment/zen-lead-controller
```

**Solutions:**
1. Ensure pods have `zen-lead/pool` label (on Deployment, not pods)
2. Ensure LeaderPolicy exists
3. Verify at least one pod is Ready (controller selects oldest Ready pod)
4. Check zen-lead controller logs for errors
5. Note: zen-lead does NOT label pods - leader selection is controller-driven

## 📚 Documentation

- [Comprehensive Guide](/tmp/zen-lead-traffic-director-comprehensive.md) - Complete documentation
- [API Reference](docs/API.md) - LeaderPolicy CRD specification
- [Examples](examples/) - Example configurations
- [Architecture](docs/ARCHITECTURE.md) - Detailed architecture documentation

## 🤝 When to Use zen-lead vs zen-sdk

### Use zen-lead (Universal Controller) When:

- ✅ **Any Workload**: Deployments, StatefulSets, Jobs, CronJobs
- ✅ **Zero Code Changes**: Can't modify application code
- ✅ **Multi-Language**: Python, Node.js, Java, Bash, etc.
- ✅ **Legacy Apps**: Unmodifiable applications
- ✅ **Traffic Routing**: Need automatic traffic routing to leader

### Use zen-sdk/pkg/leader (Library) When:

- ✅ **Controller-Runtime**: Using controller-runtime framework
- ✅ **Go Applications**: Can import Go libraries
- ✅ **Source Code Access**: Can modify application code

**Example:** zen-flow and zen-lock can use either:
- **zen-lead**: Zero code changes, automatic traffic routing
- **zen-sdk/pkg/leader**: Library-based, more control

## 🛠️ Development

```bash
# Build
make build

# Run tests
make test

# Install CRDs
make install

# Run controller locally
make run
```

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Repository:** [github.com/kube-zen/zen-lead](https://github.com/kube-zen/zen-lead)  
**License:** Apache License 2.0  
**Status:** Production-ready  
**Version:** 0.2.0
