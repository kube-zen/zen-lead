# Zen-Lead Roadmap

**Last Updated:** 2025-12-30

## Phase 1: Day-0 MVP ✅ (Complete)

**Status:** ✅ Complete

- [x] Service-annotation opt-in (`zen-lead.io/enabled: "true"`)
- [x] Selector-less leader Service creation
- [x] EndpointSlice management for leader routing
- [x] Controller-driven leader selection (sticky, oldest Ready)
- [x] Automatic failover on leader pod failure
- [x] Fail-closed port resolution (named targetPort support)
- [x] Prometheus metrics and Grafana dashboard
- [x] Helm chart with secure defaults
- [x] Comprehensive documentation

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

### Latency Optimizations (Optional)

**Status:** 🔄 Future (Not Required)

The following optimizations can reduce failover latency but are **not required** for zen-lead to work on vanilla Kubernetes:

- [ ] **eBPF Dataplanes (Cilium)**: Cilium's eBPF-based kube-proxy replacement can provide faster EndpointSlice propagation
- [ ] **IPVS kube-proxy**: IPVS mode can reduce failover latency compared to iptables mode
- [ ] **kube-proxy Tuning**: Tuning kube-proxy sync periods and conntrack settings

**Note:** zen-lead works correctly on vanilla Kubernetes with default kube-proxy (iptables mode). These optimizations are optional and only reduce dataplane convergence time, not controller-side detection time.

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

