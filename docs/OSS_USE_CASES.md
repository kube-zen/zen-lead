# Zen-Lead OSS Use Cases

Zen-Lead is an **open-source Kubernetes controller** designed to provide leader election for components that don't support it natively.

## Target Audience

Zen-Lead is perfect for:

1. **Components without leader election support**
   - CronJobs
   - DaemonSets
   - Custom workloads
   - Legacy applications

2. **Components that can't be modified**
   - Third-party applications
   - Vendor-provided containers
   - Applications without source code access

3. **Simple leader election needs**
   - Don't want to write custom code
   - Don't want to import libraries
   - Just want annotations

## Use Case 1: CronJob Leader Election

### Problem

You have a CronJob that runs a daily report. If it runs on 3 nodes, it sends 3 duplicate emails.

### Solution

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: report-generator
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby

---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: daily-report
spec:
  schedule: "0 0 * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          annotations:
            zen-lead/pool: report-generator
            zen-lead/join: "true"
        spec:
          containers:
          - name: report-generator
            image: report-generator:latest
            # Only the leader pod will execute
```

**Result:** Only 1 cluster instance executes, even with multiple nodes.

### How It Works

1. CronJob creates Job pods with `zen-lead/pool` annotation
2. zen-lead controller detects these pods
3. zen-lead manages leader election via Kubernetes Lease API
4. Only the leader pod has `zen-lead/role: leader` annotation
5. Your application checks the annotation to decide if it should run

### Application Code

```bash
#!/bin/bash
# In your CronJob script

ROLE=$(kubectl get pod $HOSTNAME -o jsonpath='{.metadata.annotations.zen-lead/role}')

if [ "$ROLE" != "leader" ]; then
    echo "Not the leader, exiting"
    exit 0
fi

# Only leader executes this
./generate-report.sh
```

Or in Python:

```python
import os
import subprocess

pod_name = os.environ.get('HOSTNAME')
result = subprocess.run(
    ['kubectl', 'get', 'pod', pod_name, '-o', 'jsonpath={.metadata.annotations.zen-lead/role}'],
    capture_output=True, text=True
)

if result.stdout.strip() != 'leader':
    print("Not the leader, exiting")
    exit(0)

# Only leader executes this
generate_report()
```

## Use Case 2: DaemonSet Leader Election

### Problem

You have a DaemonSet that should only run on one node, but DaemonSets run on all nodes by default.

### Solution

```yaml
apiVersion: coordination.kube-zen.io/v1alpha1
kind: LeaderPolicy
metadata:
  name: singleton-daemon
spec:
  leaseDurationSeconds: 15
  identityStrategy: pod
  followerMode: standby

---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: singleton-service
spec:
  template:
    metadata:
      annotations:
        zen-lead/pool: singleton-daemon
        zen-lead/join: "true"
    spec:
      containers:
      - name: service
        image: my-service:latest
        # Check annotation to decide if active
```

**Result:** All pods run, but only the leader is active.

## Use Case 3: Legacy Application

### Problem

You have a legacy application that can't be modified. It doesn't support leader election, but you need HA.

### Solution

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: legacy-app
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: legacy-app-pool
        zen-lead/join: "true"
    spec:
      containers:
      - name: legacy-app
        image: legacy-app:latest
        # Use initContainer or sidecar to check leader status
        env:
        - name: IS_LEADER
          valueFrom:
            fieldRef:
              fieldPath: metadata.annotations['zen-lead/role']
```

**Result:** Legacy app can check `IS_LEADER` environment variable (Kubernetes 1.28+).

## Use Case 4: Third-Party Application

### Problem

You're using a third-party application container that you can't modify. You need leader election.

### Solution

Use an initContainer or sidecar to check leader status:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: third-party-app
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        zen-lead/pool: third-party-pool
        zen-lead/join: "true"
    spec:
      initContainers:
      - name: leader-check
        image: busybox
        command:
        - sh
        - -c
        - |
          ROLE=$(kubectl get pod $HOSTNAME -o jsonpath='{.metadata.annotations.zen-lead/role}')
          if [ "$ROLE" != "leader" ]; then
            echo "Not leader, waiting..."
            sleep 3600  # Sleep for 1 hour
          fi
      containers:
      - name: third-party-app
        image: third-party/app:latest
        # Only runs if initContainer succeeds (leader)
```

## Use Case 5: Multi-Language Applications

### Problem

You have applications in different languages (Python, Node.js, Java) that need leader election.

### Solution

All languages can check the pod annotation:

**Python:**
```python
import os
import subprocess

def is_leader():
    pod_name = os.environ.get('HOSTNAME')
    result = subprocess.run(
        ['kubectl', 'get', 'pod', pod_name, '-o', 'jsonpath={.metadata.annotations.zen-lead/role}'],
        capture_output=True, text=True
    )
    return result.stdout.strip() == 'leader'
```

**Node.js:**
```javascript
const { execSync } = require('child_process');

function isLeader() {
    const podName = process.env.HOSTNAME;
    const role = execSync(`kubectl get pod ${podName} -o jsonpath='{.metadata.annotations.zen-lead/role}'`).toString().trim();
    return role === 'leader';
}
```

**Java:**
```java
public boolean isLeader() {
    String podName = System.getenv("HOSTNAME");
    Process process = Runtime.getRuntime().exec(
        "kubectl get pod " + podName + " -o jsonpath='{.metadata.annotations.zen-lead/role}'"
    );
    String role = new String(process.getInputStream().readAllBytes()).trim();
    return "leader".equals(role);
}
```

## Comparison: zen-sdk vs zen-lead

### zen-sdk/pkg/leader

**Use when:**
- ✅ You control the source code
- ✅ You can import Go libraries
- ✅ You're using controller-runtime
- ✅ You want library-based leader election

**Example:** zen-flow, zen-lock (controller-runtime based)

### zen-lead (OSS Controller)

**Use when:**
- ✅ You can't modify the application code
- ✅ You're using CronJobs, DaemonSets
- ✅ You have multi-language applications
- ✅ You want annotation-based leader election
- ✅ You need leader election for third-party apps

**Example:** CronJobs, legacy apps, third-party containers

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  zen-lead Controller (OSS)                              │
│  - Watches Pods with zen-lead/pool annotation           │
│  - Manages Kubernetes Lease resources                   │
│  - Updates pod annotations (zen-lead/role)              │
└─────────────────────────────────────────────────────────┘
                        │
                        │ Watches
                        ▼
┌─────────────────────────────────────────────────────────┐
│  Your Application Pods                                  │
│  - CronJob pods                                          │
│  - DaemonSet pods                                        │
│  - Deployment pods                                       │
│  - Any pod with zen-lead/pool annotation                │
└─────────────────────────────────────────────────────────┘
                        │
                        │ Checks annotation
                        ▼
┌─────────────────────────────────────────────────────────┐
│  Application Logic                                      │
│  if (role == "leader") {                                 │
│      // Do work                                          │
│  } else {                                                │
│      // Wait or exit                                     │
│  }                                                       │
└─────────────────────────────────────────────────────────┘
```

## Benefits

1. ✅ **No Code Changes**: Works with unmodifiable applications
2. ✅ **Language Agnostic**: Works with any language
3. ✅ **Simple**: Just add annotations
4. ✅ **Flexible**: Works with CronJobs, DaemonSets, Deployments
5. ✅ **Open Source**: Fully open-source, community-driven

## Installation

```bash
# Install zen-lead controller
kubectl apply -f https://github.com/kube-zen/zen-lead/releases/latest/download/install.yaml

# Create LeaderPolicy
kubectl apply -f examples/leaderpolicy.yaml

# Annotate your pods
kubectl annotate deployment my-app zen-lead/pool=my-pool
kubectl annotate deployment my-app zen-lead/join=true
```

## Summary

Zen-Lead is the **OSS solution** for leader election when:
- Your component doesn't support leader election
- You can't modify the application code
- You need annotation-based leader election
- You want a simple, declarative approach

**Perfect for:** CronJobs, DaemonSets, legacy apps, third-party containers, multi-language applications.

---

**zen-lead: Leader election for components that don't support it.**

