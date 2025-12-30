# Zen-Lead Roadmap

## Phase 1: Basic Leader Election ✅ (Current)

**Status:** ✅ Complete

- [x] LeaderPolicy CRD
- [x] Annotation-based participation
- [x] Lease-based leader election
- [x] Status API
- [x] Pod role annotations
- [x] Controller implementation
- [x] Documentation

## Phase 2: Follower Mode Enhancements

**Status:** 🔄 Planned

### ScaleDown Mode

- [ ] Implement scaleDown follower mode
- [ ] HPA integration for automatic scaling
- [ ] Resource optimization
- [ ] Documentation

**Use Case:** Save resources by scaling followers to 0

### Standby Enhancements

- [ ] Health check integration
- [ ] Readiness probe based on leader status
- [ ] Metrics for follower/leader distribution

## Phase 3: Distributed Locking

**Status:** 🔄 Planned

### ManualLock CRD

- [ ] ManualLock CRD definition
- [ ] Acquire/release lock operations
- [ ] Lock expiration
- [ ] Integration with zen-flow

**Use Case:** Prevent parallel execution of critical sections

### Lock API

- [ ] REST API for lock operations
- [ ] gRPC API for lock operations
- [ ] Client libraries

## Phase 4: Status API

**Status:** 🔄 Planned

### gRPC Endpoint

- [ ] gRPC service for leader status
- [ ] Query leader by pool name
- [ ] Subscribe to leader changes

### HTTP Endpoint

- [ ] HTTP REST API
- [ ] `/v1/leader/pool/{name}` endpoint
- [ ] JSON response format

**Use Case:** External applications querying leader status without Kubernetes API access

## Phase 5: Advanced Features

**Status:** 🔄 Future

### Multi-Region Support

- [ ] Cross-cluster leader election
- [ ] Region-aware leader selection
- [ ] Failover across regions

### Metrics & Observability

- [ ] Prometheus metrics
- [ ] Leader transition metrics
- [ ] Candidate count metrics
- [ ] Grafana dashboards

### Performance Optimizations

- [ ] Caching for frequent queries
- [ ] Batch updates
- [ ] Optimized reconciliation

## Integration Roadmap

### Zen Suite Integration

- [ ] zen-flow integration guide
- [ ] zen-watcher integration guide
- [ ] zen-lock integration guide
- [ ] zen-gc integration guide

### Community Adoption

- [ ] Operator SDK integration
- [ ] Kubebuilder integration
- [ ] Helm chart
- [ ] ArtifactHub publishing

---

**Current Version:** 0.1.0-alpha  
**Next Milestone:** Phase 2 - Follower Mode Enhancements

