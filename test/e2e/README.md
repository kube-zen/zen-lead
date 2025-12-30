# E2E Tests

End-to-end tests for zen-lead require a Kubernetes cluster (kind recommended).

## Prerequisites

- kind cluster (or other Kubernetes cluster)
- kubectl configured
- zen-lead controller deployed

## Running E2E Tests

```bash
# Build test binary
go test -tags=e2e -c ./test/e2e/

# Run tests (requires cluster)
go test -tags=e2e ./test/e2e/ -v
```

## Test Coverage

- ✅ Leader Service creation
- ✅ EndpointSlice creation with exactly one endpoint
- ✅ Failover when leader becomes NotReady
- ✅ Cleanup when annotation removed
- ✅ Port resolution fail-closed behavior

## Setup

1. Create kind cluster: `kind create cluster`
2. Deploy zen-lead: `helm install zen-lead ./helm-charts/charts/zen-lead`
3. Run tests: `go test -tags=e2e ./test/e2e/ -v`

