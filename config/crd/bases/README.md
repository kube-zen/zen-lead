# CRD Bases

This directory contains generated CRD manifests.

## Generating CRDs

To generate CRDs from the API types, run:

```bash
# Install controller-gen if not already installed
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Generate CRDs
controller-gen rbac:roleName=zen-lead-role crd webhook paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases
```

## CRD Files

After generation, you should see:
- `coordination.kube-zen.io_leaderpolicies.yaml` - LeaderPolicy CRD definition

## Installing CRDs

```bash
kubectl apply -f config/crd/bases/
```

