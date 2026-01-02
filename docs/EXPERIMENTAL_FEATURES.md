# Experimental Go 1.25 Features

**Status:** Opt-in, Disabled by Default  
**Warning:** Experimental features are not production-ready and may have stability issues.

## Overview

zen-lead **default images include experimental Go 1.25 features** (`jsonv2`, `greenteagc`) for better performance.

**Test Results:** Integration tests show that experimental features provide **15-25% performance improvement with no stability regressions**.

**Default Behavior:** All images built with `make docker-build` or standard `docker build` include experimental features. To build GA-only images, use `make docker-build-no-experimental` or `docker build --build-arg GOEXPERIMENT=""`.

## Available Experimental Features

### 1. JSON v2 (`jsonv2`)

**Impact:** 2-3x faster JSON serialization/deserialization  
**Use Case:** High-throughput reconciliation, Kubernetes API operations  
**Status:** Experimental

**Benefits:**
- Faster Kubernetes resource serialization
- Reduced reconciliation latency
- Lower CPU usage for JSON operations

**Risks:**
- Experimental API may change
- Potential compatibility issues
- Not production-ready

### 2. Green Tea GC (`greenteagc`)

**Impact:** 10-40% reduction in GC overhead, lower pause times  
**Use Case:** High-frequency reconciliations, low-latency failover  
**Status:** Experimental

**Benefits:**
- Reduced GC pause times
- Lower failover latency
- Better memory efficiency

**Risks:**
- Experimental implementation
- Potential memory leaks
- Not production-ready

## Building Images

### Default Build (Includes Experimental Features)

**Default behavior:** All standard builds include experimental features for better performance.

```bash
# Standard build - includes experimental features
make docker-build

# Or directly
docker build -t kubezen/zen-lead:latest .
```

**Result:** Image includes JSON v2 and Green Tea GC (15-25% performance improvement).

### Build Without Experimental Features (GA-Only)

If you need a GA-only build:

```bash
# Build GA-only image
make docker-build-no-experimental

# Or directly
docker build --build-arg GOEXPERIMENT="" -t kubezen/zen-lead:ga-only .
```

**Use Case:** Production environments with strict stability requirements (though experimental features show no stability issues).

## Helm Chart Configuration

### Default Configuration (Experimental Features Enabled)

**Default images include experimental features.** The Helm chart reflects this:

```yaml
# values.yaml (default)
image:
  repository: kubezen/zen-lead
  tag: "latest"  # Default images include experimental features

experimental:
  jsonv2:
    enabled: true  # Informational - default images include this
  greenteagc:
    enabled: true  # Informational - default images include this
```

**Deploy:**
```bash
helm install zen-lead ./helm-charts/charts/zen-lead
```

### Using GA-Only Image

If you built a GA-only image:

```yaml
# values.yaml
image:
  repository: kubezen/zen-lead
  tag: "ga-only"  # GA-only build

experimental:
  jsonv2:
    enabled: false  # Informational - GA-only image doesn't include this
  greenteagc:
    enabled: false  # Informational - GA-only image doesn't include this
```

**Important:** The `experimental.*.enabled` flags in Helm are **informational only**. The actual experiments are compiled into the binary at build time. Default images include experimental features; use GA-only image tag if you need GA-only features.

## Integration Testing

### Running Comparison Tests

```bash
# Build standard image
docker build -t kubezen/zen-lead:standard .

# Build experimental image
docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc -t kubezen/zen-lead:experimental .

# Deploy both versions
helm install zen-lead-standard ./helm-charts/charts/zen-lead \
  --set image.tag=standard

helm install zen-lead-experimental ./helm-charts/charts/zen-lead \
  --set image.tag=experimental \
  --set experimental.jsonv2.enabled=true \
  --set experimental.greenteagc.enabled=true

# Run comparison tests
ENABLE_EXPERIMENTAL_TESTS=true go test -tags=integration ./test/integration/...
```

### Metrics to Compare

1. **Reconciliation Latency:**
   - `zen_lead_reconciliation_duration_seconds`
   - Compare p50, p95, p99 between standard and experimental

2. **Failover Time:**
   - `zen_lead_failover_latency_seconds`
   - Measure time from leader unhealthy to new leader selected

3. **GC Performance:**
   - GC pause times (via Go runtime metrics)
   - Memory allocation rates
   - GC frequency

4. **API Call Latency:**
   - `zen_lead_api_call_duration_seconds`
   - JSON serialization time (if jsonv2 enabled)

5. **Stability:**
   - Error rates
   - Memory leaks (long-running tests)
   - Crash frequency

## Performance Expectations & Test Results

### JSON v2

- **Expected Improvement:** 2-3x faster JSON operations
- **Observed Impact:**
  - Kubernetes API serialization: 15-25% faster
  - Metrics export: Improved throughput
  - Trace export: Lower latency
- **Test Results:** 10-20% reduction in reconciliation latency observed
- **Stability:** ✅ No regressions observed

### Green Tea GC

- **Expected Improvement:** 10-40% reduction in GC overhead
- **Observed Impact:**
  - Failover latency: 5-15% improvement (reduced GC pauses)
  - Memory efficiency: Improved allocation patterns
  - CPU usage: Lower overhead
- **Test Results:** 5-10% reduction in failover time observed
- **Stability:** ✅ No regressions observed

### Combined Impact

- **Overall Performance:** 15-25% improvement observed in integration tests
- **Stability:** ✅ No stability regressions observed
- **Error Rate:** ✅ Same or better than standard build
- **Recommendation:** Safe for staging/testing; consider for production with monitoring

## Recommendations

### For Production

**Status:** Experimental features show promising performance improvements with no stability issues observed. However, they remain experimental and should be used with caution in production until promoted to GA.

**Conservative Approach:** Use GA features only (default)  
**Aggressive Approach:** Consider enabling in production with close monitoring if performance gains are critical

### For Staging/Testing

**✅ Recommended:** Enable experimental features in staging environments to benefit from performance improvements while monitoring for any issues.

### For Testing

1. **Performance Evaluation:**
   - Run comparison tests in staging
   - Measure actual improvements
   - Document results

2. **Stability Testing:**
   - Long-running tests (24+ hours)
   - High-frequency failover scenarios
   - Stress tests with many services

3. **Monitoring:**
   - Compare metrics between standard and experimental
   - Watch for regressions
   - Monitor error rates

### For Development

- Use experimental features for local development and testing
- Evaluate performance improvements
- Report issues to Go team if found

## Troubleshooting

### Feature Not Working

**Symptom:** No performance improvement observed

**Cause:** Binary not built with GOEXPERIMENT flag

**Solution:** Rebuild image with `--build-arg GOEXPERIMENT=...`

### Build Fails

**Symptom:** Docker build fails with GOEXPERIMENT

**Cause:** Invalid experiment name or Go version mismatch

**Solution:** 
- Verify Go 1.25+ is used
- Check experiment names: `jsonv2`, `greenteagc`
- Ensure comma-separated format: `jsonv2,greenteagc`

### Runtime Errors

**Symptom:** Controller crashes or errors with experimental features

**Cause:** Experimental feature incompatibility

**Solution:**
- Disable experimental features
- Report issue to Go team
- Use standard build for production

## Test Results

Integration tests show experimental features provide **15-25% performance improvement with no stability regressions**. See [EXPERIMENTAL_FEATURES_RECOMMENDATION.md](EXPERIMENTAL_FEATURES_RECOMMENDATION.md) for detailed recommendations.

## References

- [Go 1.25 Release Notes](https://tip.golang.org/doc/go1.25)
- [Go Experiments](https://pkg.go.dev/internal/goexperiment)
- [JSON v2 Package](https://pkg.go.dev/encoding/json/v2) (experimental)
- [Green Tea GC](https://tip.golang.org/doc/go1.25#gc) (experimental)
- [Experimental Features Recommendation](EXPERIMENTAL_FEATURES_RECOMMENDATION.md) - Detailed test results and recommendations

