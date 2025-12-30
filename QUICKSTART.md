# Zen-Lead Quick Start

Get zen-lead up and running in 5 minutes!

## Prerequisites

- Kubernetes cluster (1.26+)
- kubectl configured
- Go 1.24+ (for building from source)

## Step 1: Install Dependencies

```bash
# Install controller-gen (for CRD generation)
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Install Go dependencies
go mod tidy
```

## Step 2: Generate CRDs

```bash
make generate
```

This creates the CRD manifests in `config/crd/bases/`.

## Step 3: Install Zen-Lead

```bash
# Install CRDs
make install

# Install RBAC
kubectl apply -f config/rbac/

# Install Controller
kubectl apply -f deploy/
```

## Step 4: Verify Installation

```bash
# Check controller is running
kubectl get pods -n zen-system -l app=zen-lead

# Check CRD is installed
kubectl get crd leaderpolicies.coordination.kube-zen.io
```

## Step 5: Create Your First LeaderPolicy

```bash
kubectl apply -f examples/leaderpolicy.yaml
```

## Step 6: Annotate a Deployment

```bash
# Create a test deployment
kubectl create deployment test-app --image=nginx --replicas=3

# Add annotations
kubectl annotate deployment test-app zen-lead/pool=my-controller-pool
kubectl annotate deployment test-app zen-lead/join=true

# Restart pods to pick up annotations
kubectl rollout restart deployment test-app
```

## Step 7: Check Status

```bash
# Check LeaderPolicy status
kubectl get leaderpolicy my-controller-pool

# Check which pod is the leader
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/role}{"\n"}{end}'
```

## Next Steps

- Read [INTEGRATION.md](docs/INTEGRATION.md) for detailed integration examples
- Check [USE_CASES.md](docs/USE_CASES.md) for use case patterns
- Review [ARCHITECTURE.md](docs/ARCHITECTURE.md) for architecture details

## Troubleshooting

See [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) for common issues and solutions.

---

**That's it! You now have HA leader election configured.** 🎉

