# Changelog

All notable changes to zen-lead will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

**Configuration:**
- `--max-cache-size-per-namespace` flag to configure in-memory cache size limit per namespace (default: 1000)
- Helm chart support for `controller.maxCacheSizePerNamespace` with comprehensive documentation

**Observability:**
- `zen_lead_failover_latency_seconds` histogram metric tracking time from leader unhealthy detection to new leader selected
- Custom health check endpoint (`ControllerHealthChecker`) that verifies controller initialization
- Enhanced readiness probe that checks controller can reconcile Services

**Documentation:**
- Comprehensive improvement suggestions document (`docs/IMPROVEMENT_SUGGESTIONS.md`)
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

