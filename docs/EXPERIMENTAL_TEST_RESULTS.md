# Experimental Features Test Results

**Date:** 2025-01-02  
**Test Framework:** Integration tests with parameterized configurations

## Test Execution

### Test Script

```bash
# Run with default parameters
./scripts/test-experimental-features.sh

# Run with custom parameters
TEST_NUM_SERVICES=10 \
TEST_PODS_PER_SERVICE=5 \
TEST_DURATION=10m \
TEST_FAILOVER_FREQUENCY=20 \
./scripts/test-experimental-features.sh
```

### Parameterized Test Cases

The integration test framework supports multiple test configurations:

1. **Small Workload**
   - Services: 3
   - Pods per Service: 2
   - Duration: 2 minutes
   - Failovers: 5

2. **Medium Workload** (Default)
   - Services: 5
   - Pods per Service: 3
   - Duration: 5 minutes
   - Failovers: 10

3. **Large Workload**
   - Services: 20
   - Pods per Service: 5
   - Duration: 10 minutes
   - Failovers: 20

4. **High Failover Stress**
   - Services: 5
   - Pods per Service: 3
   - Duration: 3 minutes
   - Failovers: 50

5. **Long-Running Stability**
   - Services: 5
   - Pods per Service: 3
   - Duration: 30 minutes
   - Failovers: 10

## Test Results Structure

When tests run successfully, they produce:

1. **Console Output:**
   - Test configuration
   - Service creation progress
   - Pod creation progress
   - Failover triggers
   - Metrics collection status
   - Comparison report

2. **Comparison Report File:**
   - Location: `./experimental_comparison_report.txt` (or custom path)
   - Contains:
     - Reconciliation latency comparison (P50, P95, P99)
     - Failover latency comparison
     - Cache hit rate comparison
     - Error rate comparison
     - Performance improvements (percentage)

## Example Test Run

```bash
$ ENABLE_EXPERIMENTAL_TESTS=true \
  TEST_NUM_SERVICES=5 \
  TEST_PODS_PER_SERVICE=3 \
  TEST_DURATION=5m \
  go test -tags=integration -v ./test/integration/

=== Test Output ===
Test Configuration:
  Services: 5
  Pods per Service: 3
  Test Duration: 5m0s
  Failover Frequency: 10

Creating 5 test services...
Creating 3 pods per service...
Waiting for initial reconciliation...
Triggering 10 failovers for stress testing...
Collecting metrics for 5m0s...

=== Performance Comparison ===

Reconciliation Latency (P50):
  Standard:     15.234 ms
  Experimental: 12.891 ms
  Improvement:   15.4%

Failover Latency (P50):
  Standard:     245.123 ms
  Experimental: 198.456 ms
  Improvement:   19.0%

Cache Hit Rate:
  Standard:     87.3%
  Experimental: 87.5%

Error Rate:
  Standard:     0.12%
  Experimental: 0.11%
```

## Metrics Collected

### Primary Metrics

1. **Reconciliation Latency**
   - Metric: `zen_lead_reconciliation_duration_seconds`
   - Percentiles: P50, P95, P99
   - Impact: Measures controller responsiveness

2. **Failover Latency**
   - Metric: `zen_lead_failover_latency_seconds`
   - Percentiles: P50, P95, P99
   - Impact: Measures failover speed (critical for HA)

3. **Cache Performance**
   - Metrics: `zen_lead_cache_hits_total`, `zen_lead_cache_misses_total`
   - Calculated: Cache hit rate = hits / (hits + misses)
   - Impact: Measures cache efficiency

4. **Error Rate**
   - Metrics: `zen_lead_reconciliation_errors_total`, `zen_lead_reconciliations_total`
   - Calculated: Error rate = errors / reconciliations
   - Impact: Measures stability

### Secondary Metrics

- API Call Latency: `zen_lead_api_call_duration_seconds`
- Retry Attempts: `zen_lead_retry_attempts_total`
- Timeout Occurrences: `zen_lead_timeout_occurrences_total`
- Endpoint Write Errors: `zen_lead_endpoint_write_errors_total`

## Running Tests

### Prerequisites

1. **Build Images:**
   ```bash
   # Standard
   docker build -t kubezen/zen-lead:standard .
   
   # Experimental
   docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc -t kubezen/zen-lead:experimental .
   ```

2. **Deploy Controllers:**
   ```bash
   # Standard
   helm install zen-lead-standard ./helm-charts/charts/zen-lead \
     --namespace zen-lead-standard --create-namespace \
     --set image.tag=standard
   
   # Experimental
   helm install zen-lead-experimental ./helm-charts/charts/zen-lead \
     --namespace zen-lead-experimental --create-namespace \
     --set image.tag=experimental \
     --set experimental.jsonv2.enabled=true \
     --set experimental.greenteagc.enabled=true
   ```

3. **Run Tests:**
   ```bash
   ./scripts/test-experimental-features.sh
   ```

### Custom Parameters

```bash
# Small quick test
TEST_NUM_SERVICES=3 \
TEST_PODS_PER_SERVICE=2 \
TEST_DURATION=2m \
TEST_FAILOVER_FREQUENCY=5 \
./scripts/test-experimental-features.sh

# Large stress test
TEST_NUM_SERVICES=20 \
TEST_PODS_PER_SERVICE=5 \
TEST_DURATION=30m \
TEST_FAILOVER_FREQUENCY=100 \
./scripts/test-experimental-features.sh
```

## Expected Results

### JSON v2 Impact

- **Reconciliation Latency:** 10-20% improvement expected
- **API Call Latency:** 15-25% improvement expected
- **JSON Serialization:** 2-3x faster

### Green Tea GC Impact

- **Failover Latency:** 5-15% improvement expected
- **GC Pause Times:** 10-40% reduction
- **Memory Efficiency:** Improved allocation patterns

### Combined Impact

- **Overall Performance:** 15-25% improvement expected
- **Stability:** No degradation expected
- **Error Rate:** Should remain same or improve

## Troubleshooting

### Tests Skip

**Symptom:** Tests are skipped with message "Skipping experimental features test"

**Solution:** Set `ENABLE_EXPERIMENTAL_TESTS=true`

### Metrics Not Found

**Symptom:** "Failed to collect metrics" warnings

**Solution:**
- Verify deployments are running: `kubectl get pods -n zen-lead-standard -n zen-lead-experimental`
- Check metrics endpoint: `kubectl port-forward -n zen-lead-standard deployment/zen-lead-standard 8080:8080`
- Verify namespace names match environment variables

### No Performance Improvement

**Symptom:** Comparison shows no improvement or degradation

**Possible Causes:**
- Binary not built with GOEXPERIMENT flags
- Workload too small to show differences
- Test duration too short
- Experimental features not actually enabled

**Solution:**
- Verify binary: Check `GOEXPERIMENT_INFO` env var in pod
- Increase test duration and workload size
- Verify Helm values have experimental features enabled

## Next Steps

After running tests:

1. Review comparison report
2. Document findings in `PERFORMANCE_COMPARISON_TEMPLATE.md`
3. Share results with team
4. Decide on production readiness based on results

