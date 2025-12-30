# Changelog

All notable changes to zen-lead will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0-alpha] - 2025-01-XX

### 🎉 Initial Alpha Release

**First release** of Zen-Lead as a High Availability standardization solution for Kubernetes.

#### Added

**Core Features:**
- **LeaderPolicy CRD**: Defines pools of candidates for leader election
- **Annotation-based participation**: Pods join pools via annotations (no code changes)
- **Automatic leader election**: Uses Kubernetes Lease API
- **Status API**: LeaderPolicy status shows current leader and candidates
- **Pod role annotations**: Automatically sets `zen-lead/role: leader` or `follower`

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

