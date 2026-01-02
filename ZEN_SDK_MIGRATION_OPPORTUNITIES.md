# Zen-SDK Migration Opportunities for Zen-Lead

**Date:** 2026-01-02  
**Status:** Analysis Complete

## Summary

Zen-lead currently uses `zen-sdk/pkg/leader` and `zen-sdk/pkg/logging`. This document identifies additional opportunities to leverage zen-sdk utilities.

## Current Usage ✅

1. **`zen-sdk/pkg/leader`** - ✅ Already used
   - `leader.RequirePodNamespace()`
   - `leader.ApplyRestConfigDefaults()`
   - `leader.ApplyLeaderElection()`

2. **`zen-sdk/pkg/logging`** - ✅ Already used
   - `sdklog.NewLogger()`
   - Package-level logger pattern

## Opportunities

### 1. Metrics: Use zen-sdk Base Metrics (Medium Priority)

**Current State:**
- zen-lead has custom metrics implementation (`pkg/metrics/metrics.go`)
- Implements all metrics from scratch
- Has zen-lead-specific metrics (failover, sticky leader, etc.)

**Opportunity:**
- Use `zen-sdk/pkg/metrics` for base reconciliation metrics
- Keep zen-lead-specific metrics as extensions
- Reduces code duplication for standard metrics

**Benefits:**
- Standardized metric names across all Zen tools
- Less code to maintain
- Consistent metric structure

**Implementation:**
```go
// Instead of custom implementation, compose zen-sdk metrics
import "github.com/kube-zen/zen-sdk/pkg/metrics"

type Recorder struct {
    *metrics.Recorder  // Base metrics from zen-sdk
    // zen-lead-specific metrics
    failoverCountTotal *prometheus.CounterVec
    stickyLeaderHitsTotal *prometheus.CounterVec
    // ...
}
```

**Recommendation:** ⚠️ **Low Priority** - Current implementation is fine. Consider migration if zen-sdk metrics API stabilizes.

---

### 2. Retry Logic: Use zen-sdk/pkg/retry (High Priority)

**Current State:**
- Direct K8s API calls without retry logic
- Errors like `IsConflict`, `IsServerTimeout` are not automatically retried
- Could benefit from exponential backoff

**Opportunity:**
- Wrap K8s API calls with `zen-sdk/pkg/retry.Do()`
- Automatic retry on transient errors (conflicts, timeouts, rate limits)
- Reduces reconciliation failures due to transient API errors

**Locations:**
- `r.Get()` calls (lines 194, 396, 408, 441, 545, 561, 843, 867)
- `r.Create()` calls (lines 614, 881)
- `r.Patch()` calls (lines 678, 910)
- `r.Delete()` calls (line 546)
- `r.List()` calls (lines 241, 936, 946)

**Example:**
```go
import "github.com/kube-zen/zen-sdk/pkg/retry"

// Before:
if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
    return ctrl.Result{}, err
}

// After:
if err := retry.Do(ctx, retry.DefaultConfig(), func() error {
    return r.Get(ctx, req.NamespacedName, svc)
}); err != nil {
    return ctrl.Result{}, err
}
```

**Benefits:**
- Automatic retry on transient errors
- Exponential backoff reduces API server load
- Better resilience to temporary failures

**Recommendation:** ✅ **High Priority** - Would improve reliability significantly.

---

### 3. Lifecycle: Use zen-sdk/pkg/lifecycle (Low Priority)

**Current State:**
- Uses `ctrl.SetupSignalHandler()` for shutdown
- Works fine but less structured

**Opportunity:**
- Use `zen-sdk/pkg/lifecycle.ShutdownContext()` for more structured shutdown
- Better logging and observability

**Current Code:**
```go
if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
    setupLog.Error(err, "problem running manager")
    os.Exit(1)
}
```

**Proposed:**
```go
import "github.com/kube-zen/zen-sdk/pkg/lifecycle"

ctx, cancel := lifecycle.ShutdownContext(context.Background(), "zen-lead")
defer cancel()

if err := mgr.Start(ctx); err != nil {
    setupLog.Error(err, "problem running manager")
    os.Exit(1)
}
```

**Benefits:**
- Structured shutdown logging
- Consistent with other Zen tools
- Better observability

**Recommendation:** ⚠️ **Low Priority** - Current approach works fine. Nice-to-have improvement.

---

### 4. Webhook: Not Applicable

**Current State:**
- No webhooks in zen-lead (by design - Day-0 contract)

**Opportunity:** None

---

### 5. Config Validation: Not Applicable

**Current State:**
- Uses command-line flags, not environment variables

**Opportunity:** None

---

## Priority Recommendations

### High Priority ✅

1. **Add retry logic using `zen-sdk/pkg/retry`**
   - Wrap all K8s API calls with retry logic
   - Improves reliability significantly
   - Low risk, high benefit

### Medium Priority ⚠️

2. **Consider metrics migration** (if zen-sdk metrics API stabilizes)
   - Current implementation is fine
   - Migration would reduce code but requires careful refactoring

### Low Priority 📝

3. **Use `zen-sdk/pkg/lifecycle` for shutdown**
   - Nice-to-have improvement
   - Better observability
   - Low impact

## Implementation Plan

### Phase 1: Add Retry Logic (Recommended)

1. Import `zen-sdk/pkg/retry`
2. Wrap critical K8s API calls:
   - `Get()` operations (especially for leader pod lookup)
   - `Create()` operations (leader Service, EndpointSlice)
   - `Patch()` operations (updates)
3. Test thoroughly with conflict scenarios
4. Monitor for improved reliability

**Estimated Impact:**
- **Reliability:** +20-30% (fewer transient failures)
- **Code Changes:** ~50-100 lines
- **Risk:** Low (retry logic is well-tested in zen-sdk)

---

## Conclusion

The highest-value opportunity is **adding retry logic** using `zen-sdk/pkg/retry`. This would significantly improve reliability by automatically handling transient K8s API errors.

Other opportunities are lower priority but could be considered for consistency and code reduction.

