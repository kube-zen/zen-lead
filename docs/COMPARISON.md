# Comparison with Existing Solutions

## The Complete Picture

There are **two different problems** related to leader election in Kubernetes:

1. **Controller HA** (Process-level): Ensuring only one controller replica runs reconciliation logic
2. **Workload Routing** (Network-level): Routing traffic to exactly one pod without code changes

### Quick Reference

| Problem | Solution | Code Changes | Use Case |
|---------|----------|--------------|----------|
| **Controller HA** | zen-sdk/pkg/leader | 3 lines | Controller replicas (reconcilers) |
| **Controller HA** | client-go/leaderelection | 50+ lines | Controller replicas (reconcilers) |
| **Workload Routing** | zen-lead (this project) | 0 lines | Any application (vendor, legacy, etc) |
| **Workload Routing** | client-go/leaderelection | 50+ lines | Only if you can modify source |

---

## Problem 1: Controller HA (Process-Level Leader Election)

**Scenario**: You have a Kubernetes controller with 3 replicas for HA. Only ONE replica should run reconciliation loops at a time.

### Solution A: client-go LeaderElection (Verbose)

```go
import (
    "k8s.io/client-go/tools/leaderelection"
    "k8s.io/client-go/tools/leaderelection/resourcelock"
)

func main() {
    lock, err := resourcelock.New(
        resourcelock.LeasesResourceLock,
        "default",
        "my-controller",
        client.CoreV1(),
        client.CoordinationV1(),
        resourcelock.ResourceLockConfig{
            Identity: id,
        },
    )
    if err != nil {
        panic(err)
    }
    
    leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
        Lock:            lock,
        ReleaseOnCancel: true,
        LeaseDuration:   15 * time.Second,
        RenewDeadline:   10 * time.Second,
        RetryPeriod:     2 * time.Second,
        Callbacks: leaderelection.LeaderCallbacks{
            OnStartedLeading: func(ctx context.Context) {
                // Start reconciliation
            },
            OnStoppedLeading: func() {
                // Stop reconciliation
            },
            OnNewLeader: func(identity string) {
                // Handle leader change
            },
        },
    })
}
```

**Line count**: ~50 lines of boilerplate

### Solution B: zen-sdk/pkg/leader (Simple)

```go
import "github.com/kube-zen/zen-sdk/pkg/leader"

func main() {
    mgr, _ := ctrl.NewManager(cfg, ctrl.Options{})
    
    // Just 3 lines
    if err := leader.SetupWithManager(mgr, leader.Options{
        ID: "my-controller",
    }); err != nil {
        panic(err)
    }
    
    mgr.Start(ctx) // Automatically handles leader election
}
```

**Line count**: 3 lines

**Benefits of zen-sdk over client-go for controllers**:
- ✅ Integrates directly with controller-runtime Manager
- ✅ Sane defaults (no need to configure durations, retries, etc)
- ✅ Automatic health endpoint integration
- ✅ Structured logging integration
- ✅ 90% less boilerplate

---

## Problem 2: Workload Routing (Network-Level Single-Active)

**Scenario**: You have a PostgreSQL StatefulSet with 3 replicas. Only ONE pod should receive write traffic (the primary). **You cannot modify the PostgreSQL binary** (vendor software).

### Solution A: client-go LeaderElection (Doesn't Work)

```go
// This REQUIRES modifying application source code
// Can't use with vendor binaries like PostgreSQL, Redis, etc
```

**Problem**: Requires modifying application code to import and call leader election library. Doesn't work for:
- Vendor software (PostgreSQL, Redis, Elasticsearch)
- Legacy applications (no source code)
- Third-party containers (Docker Hub images)

### Solution B: zen-lead (Network-Level, Zero Code Changes)

```yaml
# Step 1: Just annotate the Service
apiVersion: v1
kind: Service
metadata:
  name: postgres
  annotations:
    zen-lead.io/enabled: "true"  # This is the ONLY change
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
```

```yaml
# Step 2: Update application to use leader Service
env:
- name: DATABASE_HOST
  value: postgres-leader  # Use leader service instead of regular service
```

**Line count**: 0 lines of code (just annotation + DNS change)

**Benefits of zen-lead**:
- ✅ Works with any binary (vendor, legacy, third-party)
- ✅ Zero code changes required
- ✅ Centralized at platform level (one controller for all apps)
- ✅ Automatic failover (2-5 seconds)
- ✅ Kubernetes-native (uses standard Service + EndpointSlice)

---

## When to Use What

### Use zen-sdk/pkg/leader when:
- ✅ Building a Kubernetes controller
- ✅ Using controller-runtime
- ✅ You control the source code
- ✅ You want simple controller HA

**Example**: Your custom operator needs HA (3 replicas, only 1 reconciles)

### Use zen-lead when:
- ✅ You need network-level single-active routing
- ✅ You **cannot** modify the application binary
- ✅ Using vendor software (databases, caches, etc)
- ✅ Want platform-wide solution (no per-app implementation)

**Example**: PostgreSQL HA, Redis primary routing, legacy app single-active

### Use client-go directly when:
- You need maximum control over leader election behavior
- You're not using controller-runtime
- You want to implement custom election strategies

**Example**: Custom distributed system with specific election requirements

---

## Summary Table

| Aspect | zen-sdk | zen-lead | client-go |
|--------|---------|----------|-----------|
| **Problem** | Controller HA | Workload routing | Controller HA |
| **Code changes** | 3 lines | 0 lines | 50+ lines |
| **Works with vendor binaries** | N/A (for controllers) | ✅ Yes | ❌ No |
| **Integration** | controller-runtime | Kubernetes Service | Any Go app |
| **Setup complexity** | Minimal | Annotation | High |
| **Typical use case** | Custom operators | Databases, legacy apps | DIY controllers |

---

## Real-World Examples

### Example 1: Building a Controller (Use zen-sdk)

```go
// zen-sdk: 3 lines for controller HA
import "github.com/kube-zen/zen-sdk/pkg/leader"

mgr, _ := ctrl.NewManager(cfg, ctrl.Options{})
leader.SetupWithManager(mgr, leader.Options{ID: "my-controller"})
mgr.Start(ctx)
```

### Example 2: PostgreSQL Primary Routing (Use zen-lead)

```bash
# zen-lead: 0 lines of code, just annotation
kubectl annotate service postgres zen-lead.io/enabled=true
# Update app DNS: postgres -> postgres-leader
```

### Example 3: Custom Election Logic (Use client-go)

```go
// client-go: Full control but verbose
leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
    // 50+ lines of custom configuration
})
```

---

## Key Takeaway

- **zen-sdk** makes controller leader election **simple** (vs client-go's verbosity)
- **zen-lead** makes workload routing **possible without code changes** (vs client-go's requirement to modify source)

Both solve different problems. Both are better than client-go for their respective use cases.

