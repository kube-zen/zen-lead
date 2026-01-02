# Performance Tuning Guide

**Date**: 2026-01-02  
**Version**: 0.1.0

---

## Overview

This guide provides recommendations for tuning zen-lead performance in production environments, especially for large-scale deployments.

---

## Cache Configuration

### Cache Size Limits

zen-lead maintains an in-memory cache of opted-in Services per namespace to optimize pod-to-service mapping. By default, the cache is limited to **1000 services per namespace**.

**Configuration:**
- Default: 1000 services per namespace
- Configurable: Set via `--max-cache-size-per-namespace` flag or Helm `controller.maxCacheSizePerNamespace`
- Eviction: LRU-style (keeps most recently accessed services when limit exceeded)

**When to Adjust:**
- **Increase limit** if you have >1000 opted-in Services per namespace
- **Decrease limit** if memory usage is a concern
- **Monitor**: Use `zen_lead_cache_size` metric to track cache growth

**Example:**
```yaml
# Helm chart values.yaml
controller:
  maxCacheSizePerNamespace: 5000  # For large namespaces
```

Or via command-line flag:
```bash
--max-cache-size-per-namespace=5000
```

---

## API Server Performance

### QPS and Burst Limits

zen-lead configures Kubernetes API client QPS/Burst settings:
- **QPS**: 50 requests/second (default, configurable via `--qps` flag or Helm `controller.qps`)
- **Burst**: 100 requests (default, configurable via `--burst` flag or Helm `controller.burst`)

**Configuration:**
- Via Helm chart: Set `controller.qps` and `controller.burst` in `values.yaml`
- Via command-line flags: `--qps` and `--burst`
- Defaults are higher than controller-runtime defaults (20/30) for better throughput

**When to Adjust:**
- **Increase** if you have many Services and need faster reconciliation
- **Decrease** if you see rate limiting errors (429) or API server overload
- **Monitor**: Watch for `zen_lead_retry_attempts_total` spikes and `zen_lead_api_call_duration_seconds`

**Configuration:**
- Set via `leader.ApplyRestConfigDefaults()` in `cmd/manager/main.go`
- Can be customized per deployment needs

---

## Reconciliation Performance

### Reconciliation Duration

Typical reconciliation takes:
- **Fast path** (no changes): <100ms
- **Leader Service creation**: 200-500ms
- **EndpointSlice update**: 100-300ms
- **Cache refresh**: 50-200ms

**Optimization Tips:**
1. **Monitor** `zen_lead_reconciliation_duration_seconds` histogram
2. **Alert** on P95 > 1 second
3. **Investigate** slow reconciliations using tracing spans

---

## Cache Performance

### Cache Hit Rate

Target cache hit rate: **>80%**

**Metrics:**
- `zen_lead_cache_hits_total`: Successful cache lookups
- `zen_lead_cache_misses_total`: Cache refreshes required

**Calculation:**
```
hit_rate = cache_hits / (cache_hits + cache_misses)
```

**When Hit Rate is Low (<80%):**
- Check if many namespaces are being accessed
- Verify cache invalidation is working correctly
- Consider increasing cache size limits

---

## Memory Usage

### Cache Memory Footprint

**Estimate:**
- Per cached service: ~200 bytes (name + selector)
- Per namespace: `num_services * 200 bytes`
- Example: 1000 services = ~200 KB per namespace

**Monitoring:**
- Use `zen_lead_cache_size` metric
- Monitor controller pod memory usage
- Set appropriate memory limits in Deployment

**Recommendations:**
- **Small clusters** (<100 namespaces): Default limits are fine
- **Large clusters** (>1000 namespaces): Consider cache size limits
- **Very large clusters**: Monitor and tune based on metrics

---

## Retry Behavior

### Retry Configuration

zen-lead uses `zen-sdk/pkg/retry` with default settings:
- **Max Attempts**: 3
- **Initial Delay**: 100ms
- **Max Delay**: 5s
- **Multiplier**: 2.0 (exponential backoff)

**Monitoring:**
- `zen_lead_retry_attempts_total`: Track retry frequency
- `zen_lead_retry_success_after_retry_total`: Success after retry

**When Retries are High:**
- Check API server health
- Verify network connectivity
- Review QPS/Burst settings
- Check for rate limiting

---

## Scaling Considerations

### Controller Replicas

**Default**: 2 replicas (for HA)

**Scaling Guidelines:**
- **Small clusters** (<100 Services): 2 replicas sufficient
- **Medium clusters** (100-1000 Services): 2-3 replicas
- **Large clusters** (>1000 Services): 3-5 replicas

**Note**: More replicas don't necessarily improve performance (leader election ensures only one active reconciler)

---

## Timeout Configuration

### Context Timeouts

zen-lead uses context timeouts for long-running operations:
- **Cache updates**: 10 seconds
- **Metrics collection**: 5 seconds

**Monitoring:**
- `zen_lead_timeout_occurrences_total`: Track timeout frequency
- Alert on timeout spikes

**When Timeouts Occur:**
- Check API server responsiveness
- Verify network latency
- Review timeout values (may need adjustment)

---

## Best Practices

### 1. Monitor Key Metrics

**Essential Metrics:**
- `zen_lead_reconciliation_duration_seconds` (P95, P99)
- `zen_lead_cache_hits_total` / `zen_lead_cache_misses_total` (hit rate)
- `zen_lead_retry_attempts_total` (retry frequency)
- `zen_lead_timeout_occurrences_total` (timeout frequency)

### 2. Set Appropriate Alerts

**Recommended Alerts:**
- P95 reconciliation duration > 1s
- Cache hit rate < 80%
- Retry rate > 10% of operations
- Timeout occurrences > 0

### 3. Tune Based on Workload

**High Pod Churn:**
- Monitor reconciliation frequency
- Consider adjusting reconciliation intervals (future feature)

**Many Services:**
- Monitor cache size
- Adjust cache limits if needed

**API Server Issues:**
- Monitor retry metrics
- Adjust QPS/Burst if needed

---

## Troubleshooting

### Slow Reconciliations

1. **Check metrics**: `zen_lead_reconciliation_duration_seconds`
2. **Review tracing spans**: Use OpenTelemetry traces
3. **Check API server**: Verify responsiveness
4. **Review retry metrics**: High retries indicate API issues

### High Memory Usage

1. **Check cache size**: `zen_lead_cache_size`
2. **Review cache limits**: Adjust if needed
3. **Monitor per-namespace**: Identify large namespaces

### Cache Misses

1. **Check hit rate**: Calculate from metrics
2. **Review cache invalidation**: Verify it's working
3. **Check namespace access patterns**: Many namespaces = more misses

---

## Performance Benchmarks

### Typical Performance

**Small Cluster** (<100 Services):
- Reconciliation: <200ms (P95)
- Cache hit rate: >90%
- Memory: <50 MB

**Medium Cluster** (100-1000 Services):
- Reconciliation: <500ms (P95)
- Cache hit rate: >85%
- Memory: <200 MB

**Large Cluster** (>1000 Services):
- Reconciliation: <1s (P95)
- Cache hit rate: >80%
- Memory: <500 MB

---

## Future Optimizations

### Planned Improvements

1. **Configurable cache limits** via environment variables
2. **Periodic cache cleanup** for unused namespaces
3. **Reconciliation interval tuning** for high-churn workloads
4. **Batch operations** for multiple Service updates

---

**Last Updated**: 2026-01-02  
**Next Review**: After production deployment

