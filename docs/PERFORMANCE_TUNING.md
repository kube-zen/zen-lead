# Performance Tuning Guide

**Date**: 2026-01-02  
**Version**: 0.1.0

## Overview

This guide provides recommendations for tuning zen-lead performance in production environments, especially for large-scale deployments.

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

### Memory Usage

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

## Failover Performance Optimizations

### Fast Retry Configuration

zen-lead uses **two retry configurations**:
1. **Standard retry** (for non-critical operations): 3 attempts, 100ms initial delay, 5s max delay
2. **Fast retry** (for failover-critical operations): 2 attempts, 20ms initial delay, 500ms max delay

**Failover-Critical Operations** (use fast retry):
- Get EndpointSlice (to find current leader)
- Get Pod (to verify current leader)
- Create/Patch EndpointSlice (to update leader)

**Configuration:**
- `--fast-retry-initial-delay-ms` (default: 20ms): Initial delay before first retry
- `--fast-retry-max-delay-ms` (default: 500ms): Maximum delay between retries
- `--fast-retry-max-attempts` (default: 2): Maximum retry attempts

**Via Helm:**
```yaml
controller:
  fastRetryInitialDelayMs: 20
  fastRetryMaxDelayMs: 500
  fastRetryMaxAttempts: 2
```

**When to Adjust:**
- **Decrease delays** if you need faster failover (may increase API server load)
- **Increase delays** if you experience API server rate limiting during failovers
- **Increase attempts** if you have transient API server issues

### Leader Pod Cache

zen-lead caches the current leader pod per service to avoid redundant API calls during reconciliation.

**Configuration:**
- `--enable-leader-pod-cache` (default: true): Enable/disable cache
- `--leader-pod-cache-ttl-seconds` (default: 30s): Cache entry time-to-live

**Via Helm:**
```yaml
controller:
  enableLeaderPodCache: true
  leaderPodCacheTTLSeconds: 30
```

**Benefits:**
- Reduces API calls during reconciliation (eliminates Get EndpointSlice + Get Pod calls on cache hits)
- Improves failover time by 50-200ms per failover
- Cache automatically invalidated on leader changes and pod deletions

**When to Adjust:**
- **Decrease TTL** (10-20s) for very dynamic deployments with frequent pod changes
- **Increase TTL** (30-60s) for stable deployments with rare pod changes
- **Disable cache** if you need always-fresh pod state (may increase failover time)

### Parallel API Calls

zen-lead includes infrastructure for parallelizing independent API operations.

**Configuration:**
- `--enable-parallel-api-calls` (default: true): Enable/disable parallelization

**Via Helm:**
```yaml
controller:
  enableParallelAPICalls: true
```

**Note:** Most operations remain sequential due to dependencies (e.g., Get Service before List Pods), but infrastructure is ready for future enhancements.

## Failover Performance

### Expected Failover Times

Based on functional testing with 50 failovers (with optimizations enabled):

**Typical Failover Performance:**
- **Average failover time**: 1.0-1.3 seconds
- **Min failover time**: 0.9-1.0 seconds
- **Max failover time**: 1.5-2.0 seconds (down from 4-5s without optimizations)
- **Success rate**: 100% (all failovers complete successfully)

**Performance Improvements (with optimizations enabled):**
- **Average**: ~5-10% faster than without optimizations
- **Max**: ~60% faster (reduced from 4-5s to 1.5-2s)
- **Consistency**: Much more consistent (smaller variance)

**Factors Affecting Failover Time:**
1. **Pod scheduling speed**: New pod must be scheduled and become Ready
2. **API server latency**: Controller must detect pod deletion and update EndpointSlice
3. **Network propagation**: EndpointSlice changes must propagate to kube-proxy
4. **Client DNS/connection**: Client must detect endpoint change and reconnect

**Optimization Impact:**
- **Fast retry config**: Reduces API call delays by 50-200ms per retry
- **Leader pod cache**: Eliminates 1-2 API calls per failover (saves 50-200ms)
- **Combined effect**: Typically reduces failover time by 100-400ms

**Configuration for Fastest Failover:**
```yaml
controller:
  fastRetryInitialDelayMs: 10      # Minimum delay (may increase API load)
  fastRetryMaxDelayMs: 300         # Lower max delay
  fastRetryMaxAttempts: 2          # Keep at 2 (fewer attempts = faster failure)
  enableLeaderPodCache: true       # Enable cache
  leaderPodCacheTTLSeconds: 15      # Lower TTL for fresher data
  enableParallelAPICalls: true      # Enable parallelization
```

**Note:** Aggressive settings may increase API server load. Monitor `zen_lead_retry_attempts_total` and API server metrics.

## Retry Behavior

### Standard Retry Configuration

zen-lead uses `zen-sdk/pkg/retry` with default settings for non-critical operations:
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

## Timeout Configuration

### Context Timeouts

zen-lead uses configurable context timeouts for long-running operations:
- **Cache updates**: 10 seconds (default, configurable via `--cache-update-timeout-seconds` or Helm `controller.cacheUpdateTimeoutSeconds`)
- **Metrics collection**: 5 seconds (default, configurable via `--metrics-collection-timeout-seconds` or Helm `controller.metricsCollectionTimeoutSeconds`)

**Configuration:**
- Via Helm chart: Set `controller.cacheUpdateTimeoutSeconds` and `controller.metricsCollectionTimeoutSeconds` in `values.yaml`
- Via command-line flags: `--cache-update-timeout-seconds` and `--metrics-collection-timeout-seconds`

**Monitoring:**
- `zen_lead_timeout_occurrences_total`: Track timeout frequency
- Alert on timeout spikes

**When Timeouts Occur:**
- Check API server responsiveness
- Verify network latency
- Increase timeout values if needed (especially for large clusters)

## Scaling Considerations

### Controller Replicas

**Default**: 2 replicas (for HA)

**Scaling Guidelines:**
- **Small clusters** (<100 Services): 2 replicas sufficient
- **Medium clusters** (100-1000 Services): 2-3 replicas
- **Large clusters** (>1000 Services): 3-5 replicas

**Note**: More replicas don't necessarily improve performance (leader election ensures only one active reconciler)

## Performance Benchmarks

### Typical Performance

**Small Cluster** (<100 Services):
- Reconciliation: <200ms (P95)
- Cache hit rate: >90%
- Memory: <50 MB
- Failover time: 0.9-1.2s (average)

**Medium Cluster** (100-1000 Services):
- Reconciliation: <500ms (P95)
- Cache hit rate: >85%
- Memory: <200 MB
- Failover time: 1.0-1.3s (average)

**Large Cluster** (>1000 Services):
- Reconciliation: <1s (P95)
- Cache hit rate: >80%
- Memory: <500 MB
- Failover time: 1.1-1.5s (average)

## Performance Comparison Template

When comparing performance between different configurations (e.g., standard vs experimental features), use this template:

### Test Configuration

- **Date:** [YYYY-MM-DD]
- **Test Duration:** [X hours/days]
- **Test Environment:** [staging/production-like]
- **Go Version:** 1.25.0
- **zen-lead Version:** [version]

### Metrics Comparison

**Reconciliation Latency:**
| Percentile | Standard (ms) | Experimental (ms) | Improvement |
|------------|---------------|-------------------|-------------|
| P50        | [value]       | [value]           | [X%]        |
| P95        | [value]       | [value]           | [X%]        |
| P99        | [value]       | [value]           | [X%]        |

**Failover Latency:**
| Percentile | Standard (ms) | Experimental (ms) | Improvement |
|------------|---------------|-------------------|-------------|
| P50        | [value]       | [value]           | [X%]        |
| P95        | [value]       | [value]           | [X%]        |
| P99        | [value]       | [value]           | [X%]        |

**Resource Usage:**
| Metric | Standard | Experimental | Difference |
|--------|----------|---------------|------------|
| CPU Usage (avg) | [X m] | [X m] | [+/-X%] |
| Memory Usage (avg) | [X Mi] | [X Mi] | [+/-X%] |
| GC Pause Time (avg) | [X ms] | [X ms] | [X%] |

## Best Practices

### 1. Monitor Key Metrics

**Essential Metrics:**
- `zen_lead_reconciliation_duration_seconds` (P95, P99)
- `zen_lead_cache_hits_total` / `zen_lead_cache_misses_total` (hit rate)
- `zen_lead_retry_attempts_total` (retry frequency)
- `zen_lead_timeout_occurrences_total` (timeout frequency)
- `zen_lead_failover_latency_seconds` (failover performance)

### 2. Set Appropriate Alerts

**Recommended Alerts:**
- P95 reconciliation duration > 1s
- Cache hit rate < 80%
- Retry rate > 10% of operations
- Timeout occurrences > 0
- Failover latency P95 > 2s

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

### Slow Failovers

1. **Check failover latency**: `zen_lead_failover_latency_seconds`
2. **Review fast retry config**: Ensure it's enabled
3. **Check leader pod cache**: Verify it's enabled
4. **Monitor API server**: Check for latency issues

## References

- [Experimental Features](EXPERIMENTAL_FEATURES.md) - Performance improvements with experimental Go features
- [Deployment Variant Selection](DEPLOYMENT_VARIANT_SELECTION.md) - Choosing image variants

**Last Updated**: 2026-01-02
