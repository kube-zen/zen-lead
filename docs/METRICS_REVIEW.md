# Zen-Lead Metrics Review

**Date**: 2025-01-XX  
**Total Metrics**: 24 metrics across 6 categories  
**Status**: ✅ Comprehensive coverage with recent enhancements

---

## Executive Summary

Zen-lead exposes **24 Prometheus metrics** covering:
- ✅ Leader lifecycle and failover
- ✅ Reconciliation performance
- ✅ Cache operations (NEW)
- ✅ Timeout tracking (NEW)
- ✅ Retry behavior (defined, not yet recorded)
- ✅ Resource management
- ✅ Error tracking

**Recent Additions** (2025-01-XX):
- Cache metrics (4 metrics): size, update duration, hits, misses
- Timeout metrics (1 metric): timeout occurrences
- Retry metrics (2 metrics): defined but not yet recorded (requires wrapper)

---

## Metrics by Category

### 1. Leader Lifecycle Metrics (6 metrics)

#### `zen_lead_leader_duration_seconds` (Gauge)
- **Labels**: `namespace`, `service`
- **Purpose**: Tracks how long the current leader pod has been the leader
- **Use Cases**:
  - Monitor leader stability
  - Detect frequent leader changes
  - Alert on short leader durations (potential instability)
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_failover_count_total` (Counter)
- **Labels**: `namespace`, `service`, `reason`
- **Purpose**: Tracks leader changes with reason
- **Reasons**: `notReady`, `terminating`, `noIP`, `noneReady`
- **Use Cases**:
  - Track failover frequency
  - Identify common failover causes
  - Alert on high failover rates
- **Cardinality**: Low (per service × 4 reasons)
- **Status**: ✅ Recorded

#### `zen_lead_leader_pod_age_seconds` (Gauge)
- **Labels**: `namespace`, `service`
- **Purpose**: Age of current leader pod since creation
- **Use Cases**:
  - Monitor pod lifecycle
  - Detect pod restarts
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_leader_stable` (Gauge)
- **Labels**: `namespace`, `service`
- **Purpose**: Binary indicator (1 = leader exists and Ready, 0 = otherwise)
- **Use Cases**:
  - Simple health check
  - Alert when leader is not stable
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_sticky_leader_hits_total` (Counter)
- **Labels**: `namespace`, `service`
- **Purpose**: Counts when sticky leader was kept (no change)
- **Use Cases**:
  - Monitor sticky leader effectiveness
  - Calculate hit rate: `hits / (hits + misses)`
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_sticky_leader_misses_total` (Counter)
- **Labels**: `namespace`, `service`
- **Purpose**: Counts when sticky leader was not available (new leader selected)
- **Use Cases**:
  - Monitor sticky leader effectiveness
  - Calculate miss rate
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

---

### 2. Reconciliation Metrics (4 metrics)

#### `zen_lead_reconciliation_duration_seconds` (Histogram)
- **Labels**: `namespace`, `service`, `result`
- **Buckets**: `[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0]`
- **Purpose**: Duration of reconciliation loops
- **Use Cases**:
  - Monitor reconciliation performance
  - Alert on slow reconciliations (P95 > 1s)
  - Track performance trends
- **Cardinality**: Low (per service × 2 results)
- **Status**: ✅ Recorded

#### `zen_lead_reconciliations_total` (Counter)
- **Labels**: `namespace`, `service`, `result`
- **Purpose**: Total number of reconciliations
- **Use Cases**:
  - Track reconciliation rate
  - Calculate success rate
- **Cardinality**: Low (per service × 2 results)
- **Status**: ✅ Recorded

#### `zen_lead_reconciliation_errors_total` (Counter)
- **Labels**: `namespace`, `service`, `error_type`
- **Purpose**: Tracks reconciliation errors by type
- **Error Types**: `service_not_found`, `list_pods_failed`, `reconcile_service_failed`, etc.
- **Use Cases**:
  - Monitor error rates
  - Identify common error patterns
  - Alert on error spikes
- **Cardinality**: Medium (per service × ~10 error types)
- **Status**: ✅ Recorded

#### `zen_lead_leader_selection_attempts_total` (Counter)
- **Labels**: `namespace`, `service`
- **Purpose**: Total leader selection operations
- **Use Cases**:
  - Track selection frequency
  - Monitor selection patterns
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

---

### 3. Resource Metrics (4 metrics)

#### `zen_lead_pods_available` (Gauge)
- **Labels**: `namespace`, `service`
- **Purpose**: Number of Ready pods available for leader selection
- **Use Cases**:
  - Monitor pod availability
  - Alert when no pods available
  - Track capacity
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_leader_services_total` (Gauge)
- **Labels**: `namespace`
- **Purpose**: Total leader Services managed per namespace
- **Use Cases**:
  - Monitor resource count
  - Track growth
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

#### `zen_lead_endpointslices_total` (Gauge)
- **Labels**: `namespace`
- **Purpose**: Total EndpointSlices managed per namespace
- **Use Cases**:
  - Monitor resource count
  - Track growth
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

#### `zen_lead_leader_service_without_endpoints` (Gauge)
- **Labels**: `namespace`, `service`
- **Purpose**: Binary indicator (1 = no endpoints, 0 = has endpoints)
- **Use Cases**:
  - Detect services without endpoints
  - Alert on missing endpoints
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

---

### 4. Cache Metrics (4 metrics) ⭐ NEW

#### `zen_lead_cache_size` (Gauge)
- **Labels**: `namespace`
- **Purpose**: Number of cached opted-in services per namespace
- **Use Cases**:
  - Monitor cache size
  - Detect cache growth
  - Plan for cache limits
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

#### `zen_lead_cache_update_duration_seconds` (Histogram)
- **Labels**: `namespace`
- **Buckets**: `[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0]`
- **Purpose**: Duration of cache update operations
- **Use Cases**:
  - Monitor cache update performance
  - Alert on slow updates (P95 > 5s)
  - Track API server responsiveness
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

#### `zen_lead_cache_hits_total` (Counter)
- **Labels**: `namespace`
- **Purpose**: Cache hits (namespace found in cache)
- **Use Cases**:
  - Calculate cache hit rate: `hits / (hits + misses)`
  - Monitor cache effectiveness
  - Alert on low hit rates (< 80%)
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

#### `zen_lead_cache_misses_total` (Counter)
- **Labels**: `namespace`
- **Purpose**: Cache misses (namespace not found, cache refreshed)
- **Use Cases**:
  - Calculate cache miss rate
  - Monitor cache refresh frequency
  - Detect cache invalidation issues
- **Cardinality**: Very Low (per namespace)
- **Status**: ✅ Recorded

---

### 5. Timeout Metrics (1 metric) ⭐ NEW

#### `zen_lead_timeout_occurrences_total` (Counter)
- **Labels**: `namespace`, `operation`
- **Purpose**: Operations that timed out
- **Operations**: `cache_update`, `metrics_collection`
- **Use Cases**:
  - Monitor timeout frequency
  - Alert on timeout spikes
  - Detect slow API server
- **Cardinality**: Low (per namespace × 2 operations)
- **Status**: ✅ Recorded

---

### 6. Error & Retry Metrics (5 metrics)

#### `zen_lead_port_resolution_failures_total` (Counter)
- **Labels**: `namespace`, `service`, `port_name`
- **Purpose**: Failures resolving named targetPorts
- **Use Cases**:
  - Track port resolution issues
  - Alert on resolution failures
  - Debug port configuration
- **Cardinality**: Medium (per service × port names)
- **Status**: ✅ Recorded

#### `zen_lead_endpoint_write_errors_total` (Counter)
- **Labels**: `namespace`, `service`
- **Purpose**: Errors writing EndpointSlice
- **Use Cases**:
  - Monitor write failures
  - Alert on write errors
  - Track API reliability
- **Cardinality**: Low (per service)
- **Status**: ✅ Recorded

#### `zen_lead_retry_attempts_total` (Counter) ⚠️ NOT RECORDED
- **Labels**: `namespace`, `service`, `operation`, `attempt`
- **Purpose**: Retry attempts for K8s API operations
- **Attempt Values**: `"1"`, `"2"`, `"3"`, `"max"`
- **Use Cases**:
  - Monitor retry patterns
  - Track API reliability
  - Identify problematic operations
- **Cardinality**: Medium (per service × operations × 4 attempts)
- **Status**: ⚠️ Defined but not recorded (requires retry wrapper)

#### `zen_lead_retry_success_after_retry_total` (Counter) ⚠️ NOT RECORDED
- **Labels**: `namespace`, `service`, `operation`
- **Purpose**: Operations that succeeded after retry
- **Use Cases**:
  - Track retry effectiveness
  - Monitor transient error recovery
- **Cardinality**: Medium (per service × operations)
- **Status**: ⚠️ Defined but not recorded (requires retry wrapper)

---

## Metrics Coverage Analysis

### ✅ Well Covered Areas

1. **Leader Lifecycle**: 6 metrics covering all aspects
2. **Reconciliation**: 4 metrics with duration, errors, and counts
3. **Resource Management**: 4 metrics tracking all resources
4. **Cache Operations**: 4 metrics (NEW) for full visibility
5. **Timeouts**: 1 metric (NEW) for timeout tracking

### ⚠️ Gaps & Opportunities

1. **Retry Metrics**: Defined but not recorded
   - **Issue**: Requires wrapping `zen-sdk/pkg/retry` or adding callbacks
   - **Priority**: MEDIUM
   - **Recommendation**: Create retry wrapper that records metrics, or add callback support to `zen-sdk/pkg/retry`

2. **API Operation Duration**: Not tracked per operation
   - **Issue**: Only reconciliation duration tracked, not individual API calls
   - **Priority**: LOW
   - **Recommendation**: Consider adding histogram for API call durations if needed

3. **Cache Hit Rate**: Calculated metric, not direct
   - **Status**: Can be calculated from hits/misses counters
   - **Priority**: LOW
   - **Recommendation**: Add Grafana dashboard with calculated hit rate

---

## Cardinality Analysis

### Low Cardinality Metrics (Safe)
- Most metrics use `namespace` and `service` labels only
- Estimated: ~100-1000 time series per namespace

### Medium Cardinality Metrics (Monitor)
- `reconciliation_errors_total`: per service × ~10 error types
- `port_resolution_failures_total`: per service × port names
- `retry_attempts_total`: per service × operations × 4 attempts (if recorded)

### Recommendations
- ✅ Current cardinality is acceptable
- ⚠️ Monitor if number of services grows significantly
- ✅ Cache metrics use namespace-only labels (very low cardinality)

---

## Alert Recommendations

### Critical Alerts

1. **No Leader Available**
   ```
   zen_lead_leader_stable == 0
   ```

2. **High Failover Rate**
   ```
   rate(zen_lead_failover_count_total[5m]) > 0.1
   ```

3. **Slow Reconciliations**
   ```
   histogram_quantile(0.95, zen_lead_reconciliation_duration_seconds) > 1.0
   ```

4. **Cache Timeouts**
   ```
   rate(zen_lead_timeout_occurrences_total{operation="cache_update"}[5m]) > 0
   ```

### Warning Alerts

1. **Low Cache Hit Rate**
   ```
   rate(zen_lead_cache_hits_total[5m]) / 
   (rate(zen_lead_cache_hits_total[5m]) + rate(zen_lead_cache_misses_total[5m])) < 0.8
   ```

2. **High Error Rate**
   ```
   rate(zen_lead_reconciliation_errors_total[5m]) > 0.1
   ```

3. **No Pods Available**
   ```
   zen_lead_pods_available == 0
   ```

---

## Dashboard Recommendations

### Overview Dashboard
- Leader Services Total
- Pods Available
- Failover Count
- Reconciliation Rate
- Error Rate

### Performance Dashboard
- Reconciliation Duration (P50, P95, P99)
- Cache Update Duration
- Cache Hit Rate
- Timeout Rate

### Leader Dashboard
- Leader Duration by Service
- Leader Pod Age
- Failover Reasons
- Sticky Leader Hit/Miss Rate

### Cache Dashboard (NEW)
- Cache Size by Namespace
- Cache Update Duration
- Cache Hit/Miss Rate
- Cache Miss Frequency

---

## Summary

### ✅ Strengths
- Comprehensive coverage of core functionality
- Low cardinality (scalable)
- Well-structured labels
- Recent additions (cache, timeout metrics) improve observability

### ⚠️ Areas for Improvement
1. **Retry Metrics**: Implement recording (requires wrapper)
2. **API Duration**: Consider per-operation duration tracking
3. **Documentation**: Update Grafana dashboards with new metrics

### 📊 Metrics Count
- **Total**: 24 metrics
- **Recorded**: 22 metrics
- **Defined but not recorded**: 2 metrics (retry)
- **Categories**: 6 categories

### 🎯 Overall Assessment
**Status**: ✅ **EXCELLENT**  
**Coverage**: Comprehensive  
**Cardinality**: Low (scalable)  
**Recommendations**: Implement retry metric recording, update dashboards

---

**Last Updated**: 2025-01-XX  
**Next Review**: After retry metrics implementation

