# Outstanding Items for Zen-Lead

**Date**: 2026-01-02  
**Status**: Critical items completed, medium/low priority items remain

---

## ✅ Completed (Critical Items)

1. **Security Scanning** ✅
   - Added `govulncheck` to CI pipeline
   - Runs on every push/PR
   - Status: **DONE**

2. **E2E Tests** ✅
   - All 5 E2E tests implemented
   - Test infrastructure (setup_kind.sh) created
   - Makefile targets added
   - Status: **DONE**

3. **Cache Thread Safety** ✅
   - Added `sync.RWMutex` for cache protection
   - Status: **DONE**

4. **Cache Invalidation** ✅
   - Cache updates on Service changes
   - Status: **DONE**

5. **Context Timeouts** ✅
   - Added timeouts for cache updates and metrics collection
   - Status: **DONE**

6. **Cache Metrics** ✅
   - Cache size, hits, misses, update duration metrics
   - Status: **DONE**

7. **Error Messages** ✅
   - Improved error messages with context
   - Status: **DONE**

---

## ⚠️ Outstanding Items

### 🟡 MEDIUM Priority

#### 1. Retry Metrics Not Recorded

**Status**: Metrics defined but not called

**Current State**:
- `RecordRetryAttempt()` and `RecordRetrySuccessAfterRetry()` are defined in `pkg/metrics/metrics.go`
- Metrics are registered: `zen_lead_retry_attempts_total`, `zen_lead_retry_success_after_retry_total`
- **But**: No calls to these methods in `pkg/director/service_director.go`

**Issue**:
- `zen-sdk/pkg/retry` doesn't have callback support
- Retry logic wraps API calls but doesn't expose attempt information

**Solution Options**:
1. **Create retry wrapper** that records metrics (2-3 hours)
   ```go
   func retryWithMetrics(ctx context.Context, cfg retry.Config, fn func() error, namespace, service, operation string) error {
       attempt := 0
       return retry.Do(ctx, cfg, func() error {
           attempt++
           err := fn()
           if err != nil {
               r.Metrics.RecordRetryAttempt(namespace, service, operation, fmt.Sprintf("%d", attempt))
           }
           return err
       })
   }
   ```

2. **Add callback support to zen-sdk/pkg/retry** (requires zen-sdk changes)
   - More reusable but requires SDK changes

3. **Use OpenTelemetry metrics** (if available)
   - Alternative approach

**Impact**: Medium - Missing observability into retry behavior

**Effort**: 2-3 hours

---

#### 2. Metrics Package Coverage Below Target

**Status**: 56.1% coverage (target: 70%+)

**Current State**:
- `pkg/metrics`: 56.1% coverage
- Some metric recording functions not tested
- Edge cases not covered

**Gaps**:
- `RecordRetryAttempt`: 0% (not called, but should be tested)
- `RecordRetrySuccessAfterRetry`: 0% (not called, but should be tested)
- `FailoverCountTotal`: 0% (needs tests)
- `PortResolutionFailuresTotal`: 0% (needs tests)
- Edge cases: nil metrics, invalid labels, etc.

**Recommendation**:
- Add tests for all metric recording functions
- Test edge cases (nil metrics, invalid labels)
- Test histogram bucket calculations

**Impact**: Medium - Better test coverage

**Effort**: 1-2 days

---

### 🟢 LOW Priority (Backlog)

#### 3. OpenTelemetry Tracing

**Status**: Not implemented

**Current State**:
- No distributed tracing
- Hard to debug in distributed systems

**Solution**:
- Use `zen-sdk/pkg/observability` for tracing
- Add spans for reconciliation operations
- Add spans for API calls

**Impact**: Low - Nice-to-have for debugging

**Effort**: 1-2 days

---

#### 4. Cache Size Limits

**Status**: No limits implemented

**Current State**:
- Cache can grow unbounded
- No LRU eviction
- Could consume excessive memory in large clusters

**Solution**:
- Add LRU eviction or size limits
- Periodic cache cleanup for unused namespaces
- Configurable cache size limit

**Impact**: Low - Only affects very large clusters

**Effort**: 1 day

---

#### 5. Performance Tuning Guide

**Status**: No documentation

**Current State**:
- No guidance on performance optimization
- No documentation on tuning for scale
- No guidance on cache behavior

**Solution**:
- Document cache behavior
- Add performance tuning recommendations
- Document scaling considerations

**Impact**: Low - User experience

**Effort**: 1 day

---

#### 6. E2E Tests in CI

**Status**: Tests implemented but not in CI

**Current State**:
- E2E tests exist and can run locally
- Not integrated into GitHub Actions CI

**Solution**:
- Add E2E test job to `.github/workflows/ci.yml`
- Use kind action or setup kind cluster
- Run E2E tests on PRs (optional, can be manual)

**Impact**: Low - Tests can run manually

**Effort**: 2-3 hours

---

## Priority Summary

### Immediate (This Sprint) - ✅ COMPLETE
- ✅ Security scanning
- ✅ E2E tests

### Short Term (Next Sprint) - ⚠️ OUTSTANDING
- ⚠️ Implement retry metric recording (2-3 hours)
- ⚠️ Improve metrics coverage to 70%+ (1-2 days)

### Long Term (Backlog) - ⚠️ OUTSTANDING
- ⚠️ Add OpenTelemetry tracing (1-2 days)
- ⚠️ Add cache size limits (1 day)
- ⚠️ Add performance tuning guide (1 day)
- ⚠️ Add E2E tests to CI (2-3 hours)

---

## Overall Status

**Critical Items**: ✅ **100% Complete**
- Security scanning: ✅
- E2E tests: ✅

**Medium Priority Items**: ⚠️ **0% Complete**
- Retry metrics: ⚠️ Not recorded
- Metrics coverage: ⚠️ 56.1% (target 70%+)

**Low Priority Items**: ⚠️ **0% Complete**
- All items in backlog

---

## Recommendations

### Next Steps (Priority Order)

1. **Implement Retry Metric Recording** (2-3 hours)
   - Quick win for observability
   - Metrics already defined
   - Just need wrapper

2. **Improve Metrics Coverage** (1-2 days)
   - Reach 70%+ target
   - Better test quality

3. **Add E2E Tests to CI** (2-3 hours)
   - Automate E2E test execution
   - Better CI coverage

4. **Add OpenTelemetry Tracing** (1-2 days)
   - Advanced debugging capability
   - Nice-to-have

5. **Add Cache Size Limits** (1 day)
   - Only needed for very large clusters
   - Optimization

6. **Add Performance Tuning Guide** (1 day)
   - Documentation
   - User experience

---

**Last Updated**: 2026-01-02  
**Next Review**: After retry metrics implementation

