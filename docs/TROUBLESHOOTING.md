# Zen-Lead Troubleshooting Guide

## Common Issues

### Issue: Leader Service Has No Endpoints

**Symptoms:**
- `kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader` shows no endpoints
- Leader Service exists but traffic doesn't route anywhere
- Metrics show `zen_lead_leader_service_without_endpoints = 1`

**Diagnosis:**
```bash
# Check EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o yaml

# Check if any pods are Ready
kubectl get pods -l <selector> --field-selector=status.phase=Running

# Check pod readiness
kubectl get pods -l <selector> -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'

# Check controller logs
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=100 | grep -i reconcile
```

**Possible Causes:**
1. No pods matching Service selector
2. No pods are Ready (readiness probe failing)
3. Service has no selector
4. Controller not running or has errors

**Solutions:**
```bash
# Verify Service has selector
kubectl get service <service> -o jsonpath='{.spec.selector}'

# Check pod readiness probes
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].readinessProbe}'

# Verify controller is running
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check for reconciliation errors
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i error
```

---

### Issue: Port Resolution Fails

**Symptoms:**
- Warning events: `NamedPortResolutionFailed`
- Metrics show `zen_lead_port_resolution_failures_total > 0`
- EndpointSlice uses fallback port instead of container port

**Diagnosis:**
```bash
# Check events
kubectl get events --field-selector involvedObject.name=<service> --sort-by='.lastTimestamp'

# Verify container port names match targetPort
kubectl get pod <leader-pod> -o jsonpath='{.spec.containers[*].ports[*].name}'
kubectl get service <service> -o jsonpath='{.spec.ports[*].targetPort}'

# Check EndpointSlice ports
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o jsonpath='{.items[*].ports[*].port}'
```

**Possible Causes:**
1. Container port name doesn't match Service targetPort name
2. Leader pod doesn't have the named port
3. Multiple containers with different port names

**Solutions:**
```bash
# Ensure container port names match Service targetPort names
# Example:
# Service: targetPort: http
# Pod: containerPort: 8080, name: http  # Must match

# Verify leader pod has the port
kubectl get pod <leader-pod> -o yaml | grep -A 5 ports
```

---

### Issue: Leader Doesn't Change on Failure

**Symptoms:**
- Leader pod becomes NotReady but EndpointSlice still points to it
- Failover doesn't occur
- Old leader pod still receives traffic

**Diagnosis:**
```bash
# Check pod readiness
kubectl get pod <leader-pod> -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'

# Check EndpointSlice endpoint
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o yaml

# Check controller reconciliation
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i reconcile

# Check sticky leader setting
kubectl get service <service> -o jsonpath='{.metadata.annotations.zen-lead\.io/sticky}'
```

**Possible Causes:**
1. Controller not reconciling (check logs)
2. Pod readiness not transitioning properly
3. Sticky leader enabled but old leader still appears Ready
4. Controller pod not running

**Solutions:**
```bash
# Verify controller is running
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check reconciliation frequency
kubectl logs -l app.kubernetes.io/name=zen-lead | grep "Reconciling"

# Disable sticky leader if needed (for testing)
kubectl annotate service <service> zen-lead.io/sticky=false

# Force reconciliation by updating Service annotation
kubectl annotate service <service> zen-lead.io/enabled=true --overwrite
```

---

### Issue: Multiple Endpoints in EndpointSlice

**Symptoms:**
- EndpointSlice has more than one endpoint
- Multiple pods receiving traffic

**Diagnosis:**
```bash
# Check EndpointSlice endpoints
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o jsonpath='{.items[*].endpoints[*].addresses}'

# Verify only one endpoint exists
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o yaml
```

**Possible Causes:**
1. Controller bug (should never happen)
2. Manual EndpointSlice modification
3. Multiple controllers running

**Solutions:**
```bash
# Delete EndpointSlice (controller will recreate)
kubectl delete endpointslice -l kubernetes.io/service-name=<service>-leader

# Verify only one controller instance
kubectl get pods -l app.kubernetes.io/name=zen-lead
```

---

### Issue: Leader Service Not Created

**Symptoms:**
- Service has `zen-lead.io/enabled: "true"` annotation
- No `<service>-leader` Service exists

**Diagnosis:**
```bash
# Verify annotation
kubectl get service <service> -o jsonpath='{.metadata.annotations.zen-lead\.io/enabled}'

# Check controller logs
kubectl logs -l app.kubernetes.io/name=zen-lead | grep <service>

# Check for errors
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i error
```

**Possible Causes:**
1. Annotation value is not exactly "true" (case-sensitive)
2. Service has no selector
3. Controller not running
4. RBAC permissions missing

**Solutions:**
```bash
# Verify annotation format
kubectl get service <service> -o yaml | grep zen-lead.io

# Ensure Service has selector
kubectl get service <service> -o jsonpath='{.spec.selector}'

# Check RBAC
kubectl auth can-i create services --as=system:serviceaccount:<namespace>:zen-lead

# Check controller logs for specific errors
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=200
```

---

### Issue: High Failover Rate

**Symptoms:**
- Metrics show `zen_lead_failover_count_total` increasing rapidly
- Leader changes frequently
- Alert: `ZenLeadHighFailoverRate`

**Diagnosis:**
```bash
# Check failover rate
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead_failover_count_total

# Check pod readiness stability
kubectl get pods -l <selector> -w

# Check reconciliation duration
curl http://localhost:8080/metrics | grep zen_lead_reconciliation_duration_seconds
```

**Possible Causes:**
1. Pod readiness probes flapping
2. Pods restarting frequently
3. Network issues causing readiness failures
4. Resource constraints causing pod evictions

**Solutions:**
```bash
# Check pod readiness probe configuration
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].readinessProbe}'

# Review pod events
kubectl describe pod <pod-name>

# Check resource usage
kubectl top pods -l <selector>

# Adjust readiness probe if too aggressive
# Consider increasing initialDelaySeconds or periodSeconds
```

---

## Debugging Commands

### Inspect Leader Service

```bash
# Get leader Service
kubectl get service <service>-leader -o yaml

# Check labels
kubectl get service <service>-leader -o jsonpath='{.metadata.labels}'

# Verify selector is null
kubectl get service <service>-leader -o jsonpath='{.spec.selector}'
```

### Inspect EndpointSlice

```bash
# Get EndpointSlice
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o yaml

# Check endpoints
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o jsonpath='{.items[*].endpoints[*]}'

# Verify managed-by label
kubectl get endpointslice -l kubernetes.io/service-name=<service>-leader -o jsonpath='{.items[*].metadata.labels.endpointslice\.kubernetes\.io/managed-by}'
```

### Check Controller Status

```bash
# Get controller pods
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check logs
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=100

# Check metrics
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead
```

### Verify RBAC

```bash
# Check ServiceAccount
kubectl get serviceaccount -l app.kubernetes.io/name=zen-lead

# Verify permissions
kubectl auth can-i create services --as=system:serviceaccount:<namespace>:zen-lead
kubectl auth can-i create endpointslices --as=system:serviceaccount:<namespace>:zen-lead
kubectl auth can-i patch pods --as=system:serviceaccount:<namespace>:zen-lead
# Should return: no (no pod mutation)
```

---

## Troubleshooting Runbook

### Quick Diagnostic Commands

```bash
# Check if controller is running
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check controller logs
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=100

# List all leader Services
kubectl get services -l app.kubernetes.io/managed-by=zen-lead

# List all managed EndpointSlices
kubectl get endpointslice -l endpointslice.kubernetes.io/managed-by=zen-lead

# Check metrics endpoint
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead
```

### Common Scenarios

#### Scenario 1: Leader Service Created But No Endpoints

**Symptoms:**
- Leader Service exists (`<service>-leader`)
- EndpointSlice exists but has no endpoints
- Metrics show `zen_lead_leader_service_without_endpoints = 1`

**Diagnosis:**
```bash
# Check if pods are Ready
kubectl get pods -l <selector> -o wide

# Check pod readiness conditions
kubectl get pods -l <selector> -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'

# Check controller logs for errors
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i error
```

**Resolution:**
1. Ensure at least one pod is Ready (readiness probe passing)
2. Verify Service has a selector
3. Check controller logs for reconciliation errors
4. Verify controller has proper RBAC permissions

#### Scenario 2: High Failover Rate

**Symptoms:**
- Metrics show `rate(zen_lead_failover_count_total[5m]) > 0.1`
- Leaders changing frequently
- Alert: `ZenLeadHighFailoverRate`

**Diagnosis:**
```bash
# Check failover reasons
kubectl get events --field-selector involvedObject.name=<service>-leader --sort-by='.lastTimestamp'

# Check pod stability
kubectl get pods -l <selector> -w

# Check readiness probe configuration
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].readinessProbe}'
```

**Resolution:**
1. Review readiness probe configuration (may be too sensitive)
2. Check pod resource limits (may be causing OOMKilled)
3. Review node stability (check node conditions)
4. Consider increasing `zen-lead.io/min-ready-duration` annotation

#### Scenario 3: Slow Reconciliation

**Symptoms:**
- Metrics show `histogram_quantile(0.95, zen_lead_reconciliation_duration_seconds) > 2`
- Alert: `ZenLeadSlowReconciliation`
- Controller logs show slow operations

**Diagnosis:**
```bash
# Check API call latency
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead_api_call_duration_seconds

# Check API server health
kubectl get --raw /healthz

# Check controller resource usage
kubectl top pod -l app.kubernetes.io/name=zen-lead
```

**Resolution:**
1. Check API server load (may be overloaded)
2. Increase `controller.maxConcurrentReconciles` if too low
3. Check network connectivity to API server
4. Review controller resource limits

#### Scenario 4: Cache Size Approaching Limit

**Symptoms:**
- Metrics show `zen_lead_cache_size / 1000 > 0.8`
- Alert: `ZenLeadCacheSizeApproachingLimit`
- Cache miss rate increasing

**Diagnosis:**
```bash
# Check cache metrics
kubectl port-forward -l app.kubernetes.io/name=zen-lead 8080:8080
curl http://localhost:8080/metrics | grep zen_lead_cache

# Count opted-in Services
kubectl get services --all-namespaces -o json | jq '.items[] | select(.metadata.annotations."zen-lead.io/enabled" == "true") | .metadata.namespace' | sort | uniq -c
```

**Resolution:**
1. Increase `controller.maxCacheSizePerNamespace` in Helm chart
2. Consider namespace sharding if you have >5000 Services per namespace
3. Monitor cache hit ratio (should be >80%)

#### Scenario 5: High API Call Latency

**Symptoms:**
- Metrics show `histogram_quantile(0.95, zen_lead_api_call_duration_seconds) > 1`
- Alert: `ZenLeadHighAPICallLatency`
- Retry attempts increasing

**Diagnosis:**
```bash
# Check API server metrics
kubectl get --raw /metrics | grep apiserver_request_duration_seconds

# Check for rate limiting
kubectl logs -l app.kubernetes.io/name=zen-lead | grep -i "rate limit\|too many requests"

# Check controller QPS/Burst settings
kubectl get deployment zen-lead-controller-manager -o yaml | grep -i qps
```

**Resolution:**
1. Check API server load and capacity
2. Review QPS/Burst settings (default: 20 QPS, 30 Burst)
3. Reduce `controller.maxConcurrentReconciles` if causing overload
4. Check network latency to API server

### Performance Tuning

See `docs/PERFORMANCE_TUNING.md` for detailed performance tuning guidance.

## Getting Help

- **Documentation**: See `docs/` directory
- **Issues**: https://github.com/kube-zen/zen-lead/issues
- **Metrics**: Check Prometheus metrics endpoint at `:8080/metrics`
- **Logs**: Controller logs include structured logging with operation context
