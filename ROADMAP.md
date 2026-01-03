# Zen-Lead Roadmap

## Phase 1: Day-0 MVP ✅ (Complete)

**Status:** ✅ Complete

- [x] Service-annotation opt-in (`zen-lead.io/enabled: "true"`)
- [x] Selector-less leader Service creation
- [x] EndpointSlice management for leader routing
- [x] Controller-driven leader selection (sticky, earliest Ready)
- [x] Automatic failover on leader pod failure
- [x] Fail-closed port resolution (named targetPort support)
- [x] Prometheus metrics and Grafana dashboard
- [x] Helm chart with secure defaults
- [x] Comprehensive documentation

## Future Enhancements (Optional)

**Note:** All future enhancements will maintain the Day-0 contract. The core product will always remain CRD-free, webhook-free, and pod-mutation-free.

### Optional Add-ons (If Introduced)

If advanced features are introduced in the future, they will be:
- Separate optional modules/charts
- Never required for core functionality
- Clearly documented as optional enhancements

**Examples of potential future add-ons:**
- Advanced configuration options (if introduced, would be optional CRD-based)
- Synthetic health checks (if introduced, would be optional)
- Multi-election patterns (if introduced, would be optional)

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

