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

### Core Functionality
- ✅ Leader Service creation (`TestLeaderServiceCreation`)
- ✅ EndpointSlice creation with exactly one endpoint (`TestEndpointSliceCreation`)
- ✅ Failover when leader becomes NotReady (`TestFailover`)
- ✅ Cleanup when annotation removed (`TestCleanup`)
- ✅ Port resolution fail-closed behavior (`TestPortResolutionFailClosed`)

### Concurrency & Edge Cases
- ✅ Concurrent Service updates (`TestConcurrentServiceUpdates`)
- ✅ Multiple Services in same namespace (`TestMultipleServicesSameNamespace`)

## Setup

1. Create kind cluster: `kind create cluster`
2. Deploy zen-lead: `helm install zen-lead ./helm-charts/charts/zen-lead`
3. Run tests: `go test -tags=e2e ./test/e2e/ -v`

