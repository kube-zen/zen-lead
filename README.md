# Zen-Lead

**High Availability Made Simple for Kubernetes**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

> **"Don't code High Availability. Configure it."**

Zen-Lead is an **open-source Kubernetes controller** that provides leader election for components that don't support it natively. Perfect for CronJobs, DaemonSets, legacy applications, and third-party containers.

**Key Differentiator:** Works with **any** Kubernetes workload via simple annotations - no code changes required!

## 🚀 Quick Start

### Install Zen-Lead

```bash
kubectl apply -f config/crd/bases/
kubectl apply -f config/rbac/
kubectl apply -f deploy/
```

### Make Your Controller HA

**Step 1:** Create a LeaderPolicy

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-controller-pool
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby
```

**Step 2:** Annotate your Deployment

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

**Result:** Only 1 of 3 replicas is active. Others are in standby mode, ready for failover.

## ✨ Features

- ✅ **Open Source** - Fully open-source Kubernetes controller
- ✅ **Works with ANY Component** - CronJobs, DaemonSets, legacy apps, third-party containers
- ✅ **Zero Code Changes** - Just add annotations - works with unmodifiable components
- ✅ **Language Agnostic** - Works with Python, Node.js, Java, Go, Bash, etc.
- ✅ **Automatic Failover** - Leader crashes? New leader elected in seconds
- ✅ **Standard Kubernetes** - Uses `coordination.k8s.io/Lease` API
- ✅ **Status API** - Query who's the leader via CRD status

## 📖 Use Cases

### Use Case 1: CronJobs Without Leader Election ⭐ **Primary Use Case**

**Problem:** CronJob runs on 3 nodes, sends 3 duplicate emails. CronJobs don't have built-in leader election.

**Solution:**
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
spec:
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            zen-lead/pool: report-generator
            zen-lead/join: "true"
```

**Result:** Only 1 cluster instance executes, even with 10 nodes. Your application checks `zen-lead/role` annotation.

### Use Case 2: Legacy Applications

**Problem:** Legacy application can't be modified, but needs HA.

**Solution:** Add annotations to Deployment. Application checks pod annotation (via kubectl or initContainer).

**Result:** Leader election without code changes.

### Use Case 3: Third-Party Containers

**Problem:** Vendor container doesn't support leader election, but you need HA.

**Solution:** Use zen-lead annotations. Check leader status via sidecar or initContainer.

**Result:** Leader election for unmodifiable containers.

### Use Case 4: Controller HA

**Problem:** Your operator runs 3 replicas, but they all try to reconcile the same resources.

**Solution:**
```yaml
# Annotate your Deployment
annotations:
  zen-lead/pool: flow-controller
```

**Result:** Only 1 replica actively reconciles. Others wait in standby.

### Use Case 3: Distributed Locking

**Problem:** Two pods writing to shared S3 bucket corrupts the file.

**Solution:** Use zen-lead to acquire a lock before critical operations.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Kubernetes Cluster                          │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   Pod A      │  │   Pod B      │  │   Pod C      │ │
│  │ (Candidate)  │  │ (Candidate)  │  │ (Candidate)  │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │
│         │                 │                 │          │
│         └─────────────────┼─────────────────┘          │
│                           │                             │
│                    zen-lead/pool                        │
│                    annotation                           │
│                           │                             │
│  ┌───────────────────────▼──────────────────────────┐ │
│  │         zen-lead Controller                       │ │
│  │  - Watches Pods with zen-lead/pool annotation    │ │
│  │  - Manages Lease resources                       │ │
│  │  - Updates LeaderPolicy status                   │ │
│  └───────────────────────┬──────────────────────────┘ │
│                           │                             │
│                    Lease Resource                       │
│              (coordination.k8s.io)                     │
└─────────────────────────────────────────────────────────┘
```

## 📚 Documentation

- [OSS Use Cases](docs/OSS_USE_CASES.md) - Detailed use cases for components without leader election
- [Comparison](docs/COMPARISON.md) - zen-lead vs zen-sdk/pkg/leader
- [OSS Positioning](docs/OSS_POSITIONING.md) - Why zen-lead is open-source
- [Architecture](docs/ARCHITECTURE.md) - Detailed architecture documentation
- [API Reference](docs/API.md) - LeaderPolicy CRD specification
- [Examples](examples/) - Example configurations
- [Integration Guide](docs/INTEGRATION.md) - How to integrate with your controllers

## 🤝 When to Use zen-lead vs zen-sdk

### Use zen-lead (OSS Controller) When:

- ✅ **CronJobs** - No native leader election support
- ✅ **DaemonSets** - Need only one active pod
- ✅ **Legacy Apps** - Can't modify application code
- ✅ **Third-Party Apps** - Vendor containers you can't change
- ✅ **Multi-Language** - Python, Node.js, Java, Bash, etc.
- ✅ **No Code Access** - Can't import libraries

### Use zen-sdk/pkg/leader (Library) When:

- ✅ **Controller-Runtime** - Using controller-runtime framework
- ✅ **Go Applications** - Can import Go libraries
- ✅ **Source Code Access** - Can modify application code

**Example:** zen-flow and zen-lock use `zen-sdk/pkg/leader` (they're controller-runtime based).  
**Example:** CronJobs and legacy apps use `zen-lead` (they can't import libraries).

See [COMPARISON.md](docs/COMPARISON.md) for detailed comparison.

### zen-watcher + zen-lead

```yaml
# zen-watcher Deployment
annotations:
  zen-lead/pool: watcher-primary
```

**Result:** Only 1 replica writes to Observation CRDs (prevents duplicate events).

### zen-lock + zen-lead

```yaml
# zen-lock Deployment
annotations:
  zen-lead/pool: lock-webhook
```

**Result:** Only leader handles webhook traffic. Followers scale to 0 (saves resources).

## 🔧 Configuration

### LeaderPolicy Spec

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: my-pool
spec:
  # Lease duration in seconds (how long leader holds the lock)
  leaseDurationSeconds: 15
  
  # Identity strategy: 'pod' (uses Pod Name/UID) or 'custom' (uses annotation)
  identityStrategy: pod
  
  # Follower behavior: 'standby' (pods stay running) or 'scaleDown' (scale to 0)
  followerMode: standby
```

### Pod Annotations

- `zen-lead/pool`: Name of the LeaderPolicy to join
- `zen-lead/join`: Set to "true" to participate in election
- `zen-lead/role`: Automatically set by zen-lead ("leader" or "follower")

## 📊 Status API

Query the current leader:

```bash
kubectl get leaderpolicy my-pool -o yaml
```

Status shows:
- `phase`: Electing or Stable
- `currentHolder`: Current leader pod name/UID
- `candidates`: Number of pods in the pool

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

## 🌟 Why Open Source?

Zen-Lead is **open-source** because leader election is needed by many Kubernetes workloads, but not all support it:
- CronJobs don't have leader election
- DaemonSets don't have leader election  
- Legacy applications can't be modified
- Third-party containers can't be changed

**zen-lead provides leader election for components that don't support it.**

---

**Repository:** [github.com/kube-zen/zen-lead](https://github.com/kube-zen/zen-lead)  
**License:** Apache License 2.0  
**Status:** Production-ready OSS  
**Version:** 0.1.0-alpha

