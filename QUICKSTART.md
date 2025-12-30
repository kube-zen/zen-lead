# Zen-Lead Quick Start

Get zen-lead up and running in 5 minutes!

## Prerequisites

- Kubernetes cluster (1.23+)
- kubectl configured
- Helm 3.0+ (for installation)

## Step 1: Install Zen-Lead

```bash
# Using Helm (recommended)
helm install zen-lead zen-lead/zen-lead \
  --namespace default \
  --create-namespace

# Or using kubectl
kubectl apply -f config/rbac/
kubectl apply -f deploy/
```

## Step 2: Verify Installation

```bash
# Check controller is running
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check metrics endpoint
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead
```

## Step 3: Deploy Your Application

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
        image: nginx:latest
        ports:
        - containerPort: 80
          name: http
---
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
    targetPort: 80
    name: http
```

## Step 4: Verify Leader Service

```bash
# Check leader Service was created
kubectl get service my-app-leader

# Verify selector is null (selector-less)
kubectl get service my-app-leader -o jsonpath='{.spec.selector}'
# Should return: null

# Check EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader

# Verify exactly one endpoint (leader pod)
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[*].endpoints[*].addresses}'
```

## Step 5: Use the Leader Service

Update your application to connect to `my-app-leader` instead of `my-app`:

```yaml
# Deployment environment variable
env:
- name: SERVICE_NAME
  value: my-app-leader  # Points only to current leader
```

## Step 6: Test Failover

```bash
# Get current leader pod
LEADER_POD=$(kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[0].endpoints[0].targetRef.name}')

# Delete leader pod
kubectl delete pod $LEADER_POD

# Watch for new leader
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -w
```

## Next Steps

- See [INTEGRATION.md](docs/INTEGRATION.md) for detailed integration patterns
- See [USE_CASES.md](docs/USE_CASES.md) for use case examples
- See [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) for common issues

## Uninstall

```bash
# Remove annotation from Services
kubectl annotate service my-app zen-lead.io/enabled-

# Uninstall zen-lead
helm uninstall zen-lead
# Or
kubectl delete -f deploy/
kubectl delete -f config/rbac/
```
