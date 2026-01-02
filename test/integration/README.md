# Integration Tests for Experimental Features

This directory contains integration tests for comparing standard vs experimental Go 1.25 features.

## Test Files

- `experimental_features_test.go` - Main comparison test with metrics collection
- `experimental_features_test_table.go` - Parameterized test cases

## Quick Start

### Prerequisites

1. Kubernetes cluster (kind, minikube, or full cluster)
2. Standard and experimental controller deployments running
3. kubectl configured

### Run Tests

```bash
# With default parameters
ENABLE_EXPERIMENTAL_TESTS=true go test -tags=integration -v ./test/integration/

# With custom parameters
ENABLE_EXPERIMENTAL_TESTS=true \
TEST_NUM_SERVICES=10 \
TEST_PODS_PER_SERVICE=5 \
TEST_DURATION=10m \
TEST_FAILOVER_FREQUENCY=20 \
go test -tags=integration -v ./test/integration/

# Or use the test script
./scripts/test-experimental-features.sh
```

## Test Parameters

All parameters can be configured via environment variables:

| Parameter | Env Var | Default | Description |
|-----------|---------|---------|-------------|
| Services | `TEST_NUM_SERVICES` | 5 | Number of test services to create |
| Pods per Service | `TEST_PODS_PER_SERVICE` | 3 | Number of pods per service |
| Duration | `TEST_DURATION` | 5m | Test duration for metrics collection |
| Failover Frequency | `TEST_FAILOVER_FREQUENCY` | 10 | Number of failovers to trigger |

## Test Configurations

The framework includes pre-configured test cases:

1. **Small Workload** - Quick validation test
2. **Medium Workload** - Standard test (default)
3. **Large Workload** - Stress test
4. **High Failover Stress** - Failover performance test
5. **Long-Running** - Stability test

## Metrics Collected

- Reconciliation latency (P50, P95, P99)
- Failover latency (P50, P95, P99)
- Cache hit rate
- Error rate
- API call latency
- GC statistics (if available)

## Output

Tests generate:
1. Console output with test progress
2. Comparison report (if `SAVE_COMPARISON_REPORT=true`)
3. Metrics comparison between standard and experimental builds

## See Also

- [Experimental Testing Guide](../docs/EXPERIMENTAL_TESTING_GUIDE.md)
- [Performance Comparison Template](../docs/PERFORMANCE_COMPARISON_TEMPLATE.md)
- [Test Results Documentation](../docs/EXPERIMENTAL_TEST_RESULTS.md)

