# Changelog

All notable changes to zen-lead will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Failover Performance Optimizations**: Implemented three major optimizations to reduce failover time:
  - **Fast Retry Config**: Configurable retry settings for failover-critical operations (Get EndpointSlice, Get Pod, Patch EndpointSlice) with defaults: 20ms initial delay (vs 100ms), 500ms max delay (vs 5s), 2 attempts (vs 3). Configurable via `--fast-retry-initial-delay-ms`, `--fast-retry-max-delay-ms`, `--fast-retry-max-attempts` flags and Helm chart values.
  - **Leader Pod Cache**: Caches current leader pod per service to avoid redundant API calls during reconciliation. Configurable via `--enable-leader-pod-cache` (default: true) and `--leader-pod-cache-ttl-seconds` (default: 30s) flags and Helm chart values. Automatically invalidated on leader changes and pod deletions.
  - **Parallel API Calls**: Infrastructure for parallelizing independent API operations (configurable via `--enable-parallel-api-calls`, default: true).
- **Performance Configuration**: Added comprehensive failover optimization settings to Helm chart with detailed tuning guidance and monitoring recommendations.
- **Performance Results**: Functional testing with 50 failovers shows significant improvements:
  - **Average failover time**: Reduced from 1.28s to 1.21s (5.7% improvement)
  - **Max failover time**: Reduced from 4.86s to 1.99s (59% improvement)
  - **Min failover time**: Improved from 0.91s to 0.90s
  - **Success rate**: 100% (50/50 failovers successful)

**Metrics Migration:**
- Migrated to `zen-sdk/pkg/metrics` for standardized reconciliation metrics
- Now exposes both `zen_reconciliations_total` (component-level) and `zen_lead_reconciliations_total` (namespace/service-level)
- All metrics now use controller-runtime metrics registry instead of prometheus default registry
- Maintains backward compatibility: all existing zen-lead-specific metrics remain unchanged

**Lifecycle Migration:**
- Migrated to `zen-sdk/pkg/lifecycle` for graceful shutdown
- Replaced `ctrl.SetupSignalHandler()` with `lifecycle.ShutdownContext()` for consistent signal handling across Zen components

**Configuration:**
- `--max-cache-size-per-namespace` flag to configure in-memory cache size limit per namespace (default: 1000)
- `--max-concurrent-reconciles` flag to configure maximum concurrent reconciliations (default: 10)
- `--cache-update-timeout-seconds` flag to configure timeout for cache update operations (default: 10s)
- `--metrics-collection-timeout-seconds` flag to configure timeout for metrics collection operations (default: 5s)
- `--qps` flag to configure Kubernetes API client QPS (queries per second) rate limit (default: 50.0)
- `--burst` flag to configure Kubernetes API client burst rate limit (default: 100)
- Helm chart support for all controller configuration parameters with comprehensive documentation

**Observability:**
- `zen_lead_failover_latency_seconds` histogram metric tracking time from leader unhealthy detection to new leader selected
- `zen_lead_api_call_duration_seconds` histogram metric tracking latency for all Kubernetes API operations (Get, List, Create, Patch, Delete)
- Custom health check endpoint (`ControllerHealthChecker`) that verifies controller initialization
- Enhanced readiness probe that checks controller can reconcile Services
- Enhanced Prometheus alerting rules (cache size approaching limit, high API call latency, controller restart frequency)

**Documentation:**
- Cache tuning guidance in Helm chart values.yaml
- Cache size configuration examples in Helm chart README

### Changed

- `NewServiceDirectorReconciler` now accepts `maxCacheSizePerNamespace` parameter for configurable cache limits
- Health check endpoint now validates reconciler initialization (Client, Metrics, etc.)

## [0.1.0-alpha] - 2025-12-30

### 🎉 Initial Alpha Release

**First release** of Zen-Lead as a High Availability standardization solution for Kubernetes.

#### Added

**Core Features:**
- **Service annotation opt-in**: Annotate Services with `zen-lead.io/enabled: "true"` (Profile A)
- **Network-level routing**: Creates selector-less leader Service + EndpointSlice
- **Automatic leader election**: Uses Kubernetes Lease API (Profile B/C)
- **No pod mutation**: Day-0 contract - zen-lead never mutates workload pods
- **CRD-free default**: Profile A works without CRDs (Profile C is opt-in)

**Components:**
- LeaderPolicy controller for reconciliation
- Pool manager for candidate discovery
- Election wrapper for leader election logic
- Pod event handler for triggering reconciliation

**Documentation:**
- README with quick start guide
- Architecture documentation
- Example configurations
- Project structure guide

**Infrastructure:**
- Makefile for build automation
- Dockerfile for containerization
- RBAC manifests
- Deployment manifests

#### Known Limitations

- Follower scaleDown mode not yet implemented (Phase 2)
- Distributed locking (ManualLock CRD) not yet implemented (Phase 3)
- gRPC/HTTP status API not yet implemented (Phase 4)

---

## Roadmap

### Phase 1: Basic Leader Election ✅
- [x] LeaderPolicy CRD
- [x] Annotation-based participation
- [x] Lease-based leader election
- [x] Status API

### Phase 2: Follower Mode (Planned)
- [ ] ScaleDown mode for followers
- [ ] HPA integration
- [ ] Resource optimization

### Phase 3: Distributed Locking (Planned)
- [ ] ManualLock CRD
- [ ] Acquire/release locks
- [ ] Prevent parallel execution

### Phase 4: Status API (Planned)
- [ ] gRPC endpoint for leader status
- [ ] HTTP endpoint for leader status
- [ ] External integration support

