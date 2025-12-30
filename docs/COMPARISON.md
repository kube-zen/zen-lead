# zen-lead vs zen-sdk/pkg/leader

## Overview

Both zen-lead and zen-sdk provide leader election, but they serve different use cases.

## zen-lead (OSS Controller)

**Type:** Standalone Kubernetes controller  
**Approach:** Annotation-based  
**Target:** Components without leader election support

### When to Use

✅ **CronJobs** - No native leader election  
✅ **DaemonSets** - Need only one active pod  
✅ **Legacy Applications** - Can't modify code  
✅ **Third-Party Containers** - Vendor apps you can't change  
✅ **Multi-Language Apps** - Python, Node.js, Java, etc.  
✅ **No Code Access** - Can't import libraries  

### How It Works

1. Deploy zen-lead controller
2. Create LeaderPolicy CRD
3. Annotate pods: `zen-lead/pool: my-pool`
4. zen-lead manages leader election
5. Application checks `zen-lead/role` annotation

### Example

```yaml
# CronJob
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            zen-lead/pool: my-pool
            zen-lead/join: "true"
```

```bash
# Application checks annotation
ROLE=$(kubectl get pod $HOSTNAME -o jsonpath='{.metadata.annotations.zen-lead/role}')
if [ "$ROLE" != "leader" ]; then exit 0; fi
```

### Pros

- ✅ Works with any component
- ✅ No code changes required
- ✅ Language agnostic
- ✅ Works with unmodifiable apps
- ✅ Simple annotations

### Cons

- ❌ Requires zen-lead controller deployment
- ❌ Requires kubectl access in pods (or initContainer)
- ❌ Slight delay in leader detection

---

## zen-sdk/pkg/leader (Library)

**Type:** Go library  
**Approach:** Library-based  
**Target:** Controller-runtime based applications

### When to Use

✅ **Controller-Runtime** - Using controller-runtime framework  
✅ **Go Applications** - Can import Go libraries  
✅ **Source Code Access** - Can modify application code  
✅ **Library-Based** - Want library-based leader election  

### How It Works

1. Import `github.com/kube-zen/zen-sdk/pkg/leader`
2. Configure leader options
3. Pass to `ctrl.NewManager()`
4. controller-runtime handles leader election

### Example

```go
import "github.com/kube-zen/zen-sdk/pkg/leader"

opts := leader.Options{
    LeaseName: "my-controller",
    Enable:    true,
}
mgr, err := ctrl.NewManager(cfg, ctrl.Options{}, leader.Setup(opts))
```

### Pros

- ✅ No extra controller needed
- ✅ Direct integration
- ✅ Faster leader detection
- ✅ Type-safe API
- ✅ Well-tested

### Cons

- ❌ Requires Go and controller-runtime
- ❌ Requires code modifications
- ❌ Only works with controller-runtime

---

## Comparison Table

| Feature | zen-lead | zen-sdk/pkg/leader |
|---------|----------|-------------------|
| **Type** | Controller | Library |
| **Approach** | Annotations | Code integration |
| **Works with** | Any pod | controller-runtime only |
| **Code Changes** | None | Required |
| **Language** | Any | Go only |
| **Deployment** | Requires controller | No extra deployment |
| **Use Case** | CronJobs, legacy apps | Controllers, operators |

## Decision Tree

```
Do you have source code access?
├─ No → Use zen-lead (annotation-based)
└─ Yes
   ├─ Is it controller-runtime based?
   │  ├─ Yes → Use zen-sdk/pkg/leader (library)
   │  └─ No → Use zen-lead (annotation-based)
   └─ Is it a CronJob/DaemonSet?
      ├─ Yes → Use zen-lead (annotation-based)
      └─ No → Depends on framework
```

## Real-World Examples

### Example 1: CronJob

**Component:** Daily report CronJob  
**Choice:** zen-lead  
**Reason:** CronJobs don't support controller-runtime

```yaml
annotations:
  zen-lead/pool: report-generator
  zen-lead/join: "true"
```

### Example 2: zen-flow Controller

**Component:** Kubernetes controller  
**Choice:** zen-sdk/pkg/leader  
**Reason:** controller-runtime based, can import libraries

```go
import "github.com/kube-zen/zen-sdk/pkg/leader"
leaderOpts := leader.Options{...}
```

### Example 3: Legacy Python App

**Component:** Legacy Python application  
**Choice:** zen-lead  
**Reason:** Can't modify code, can check annotations

```python
role = check_pod_annotation("zen-lead/role")
if role != "leader":
    exit()
```

## Summary

- **zen-lead**: OSS controller for components without leader election support
- **zen-sdk/pkg/leader**: Library for controller-runtime based applications

**Both are valuable** - they serve different use cases in the Kubernetes ecosystem.

---

**Choose zen-lead when you need leader election for components that don't support it.**

