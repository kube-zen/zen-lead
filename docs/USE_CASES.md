# Zen-Lead Use Cases

## Use Case 1: Stateless Controller High Availability

### Problem

Your Kubernetes controller runs multiple replicas for high availability, but you want to ensure only one replica actively processes work to avoid:
- Duplicate processing
- Race conditions
- Resource conflicts

### Solution

Use zen-lead to route traffic to only the leader replica.

### Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-controller
  template:
    metadata:
      labels:
        app: my-controller
    spec:
      containers:
      - name: controller
        image: my-controller:latest
        env:
        - name: SERVICE_NAME
          value: my-controller-leader  # Use leader service
---
apiVersion: v1
kind: Service
metadata:
  name: my-controller
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-controller
  ports:
  - port: 8080
    targetPort: 8080
```

### How It Works

1. Controller connects to `my-controller-leader` Service
2. Only leader pod receives traffic
3. On leader failure, traffic automatically switches to new leader
4. No application code changes required

---

## Use Case 2: API Gateway Single-Active

### Problem

You have an API gateway with multiple replicas, but you want only one to handle certain operations (e.g., rate limiting, caching).

### Solution

Use zen-lead to route admin/control traffic to leader, while keeping regular traffic load-balanced.

### Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-gateway
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: api-gateway
  ports:
  - name: admin
    port: 9090
    targetPort: admin
  - name: http
    port: 80
    targetPort: http
---
# Regular traffic (load-balanced)
apiVersion: v1
kind: Service
metadata:
  name: api-gateway-public
spec:
  selector:
    app: api-gateway
  ports:
  - port: 80
    targetPort: http
```

### Application Logic

```go
// Application checks if it's receiving admin traffic
if isAdminRequest(req) {
    // Only leader handles admin requests
    // Other replicas return 503 or redirect
}
```

---

## Use Case 3: Scheduled Job Coordinator

### Problem

You have a CronJob that should only run on one replica to avoid duplicate execution.

### Solution

Use zen-lead with a Service that selects CronJob pods.

### Configuration

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: report-generator
spec:
  schedule: "0 0 * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            app: report-generator
        spec:
          containers:
          - name: generator
            image: report-generator:latest
            env:
            - name: SERVICE_NAME
              value: report-generator-leader
---
apiVersion: v1
kind: Service
metadata:
  name: report-generator
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: report-generator
  ports:
  - port: 8080
    targetPort: 8080
```

**Note:** This works for Job pods, but CronJob scheduling still creates multiple Jobs. Consider using a controller pattern instead.

---

## Use Case 4: Database Connection Pool Manager

### Problem

You have a connection pool manager that should only run on one replica to avoid connection pool conflicts.

### Solution

Use zen-lead to route traffic to leader replica.

### Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pool-manager
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: pool-manager
    spec:
      containers:
      - name: manager
        image: pool-manager:latest
        env:
        - name: POOL_SERVICE
          value: pool-manager-leader
---
apiVersion: v1
kind: Service
metadata:
  name: pool-manager
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: pool-manager
```

---

## Use Case 5: Metrics Aggregator

### Problem

You have a metrics aggregator that should only run on one replica to avoid duplicate aggregation.

### Solution

Use zen-lead to route metrics collection traffic to leader.

### Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: metrics-aggregator
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: metrics-aggregator
  ports:
  - port: 9090
    targetPort: metrics
    name: metrics
```

---

## Important Notes

### What Zen-Lead Does NOT Do

- **Application-Level Coordination:** Zen-lead provides network routing only. Applications must handle their own state coordination.
- **Distributed Consensus:** Not suitable for applications requiring strong consistency guarantees.
- **State Management:** Does not manage application state or prevent split-brain at application level.

### When to Use Zen-Lead

✅ **Good For:**
- Stateless applications
- Applications with their own state coordination
- Network-level single-active routing
- Zero-code-change requirements

❌ **Not Suitable For:**
- Applications requiring distributed consensus
- Stateful applications without coordination
- Applications requiring guaranteed exactly-once semantics

### Best Practices

1. **Readiness Probes:** Ensure pods have accurate readiness probes
2. **Health Checks:** Monitor leader service endpoints
3. **Failover Testing:** Regularly test failover scenarios
4. **Metrics:** Monitor failover rate and reconciliation duration
5. **Resource Limits:** Set appropriate limits to prevent evictions
