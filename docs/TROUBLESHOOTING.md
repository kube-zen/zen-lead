# Zen-Lead Troubleshooting Guide

## Common Issues

### Issue: No Leader Elected

**Symptoms:**
- `kubectl get leaderpolicy` shows `phase: Electing`
- `status.currentHolder` is `null`
- No pods are marked as leader

**Diagnosis:**
```bash
# Check if LeaderPolicy exists
kubectl get leaderpolicy <pool-name>

# Check for candidate pods
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/pool}{"\n"}{end}'

# Check if pods are participating
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/join}{"\n"}{end}'

# Check Lease resource
kubectl get lease <pool-name>
```

**Possible Causes:**
1. No pods with pool annotation
2. Pods not running
3. Pods missing `zen-lead/join: "true"` annotation
4. RBAC issues preventing lease creation

**Solutions:**
```bash
# Verify pod annotations
kubectl get pod <pod-name> -o yaml | grep -A 5 annotations

# Check RBAC
kubectl auth can-i create leases --namespace=<namespace>

# Check controller logs
kubectl logs -n zen-system -l app=zen-lead
```

---

### Issue: Multiple Leaders

**Symptoms:**
- Multiple pods show `zen-lead/role: leader`
- Duplicate processing occurs

**Diagnosis:**
```bash
# Check pod roles
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.zen-lead/role}{"\n"}{end}'

# Check Lease resource
kubectl get lease <pool-name> -o yaml

# Check system time (clock skew)
date
```

**Possible Causes:**
1. Clock skew between nodes
2. Network partition
3. Lease API bug (rare)
4. Controller bug

**Solutions:**
```bash
# Verify system time synchronization
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.nodeInfo.systemUUID}{"\n"}{end}'

# Check Lease holder
kubectl get lease <pool-name> -o jsonpath='{.spec.holderIdentity}'

# Restart controller if needed
kubectl rollout restart deployment/zen-lead-controller-manager -n zen-system
```

---

### Issue: Leader Not Processing

**Symptoms:**
- `status.currentHolder` is set
- Pod has `zen-lead/role: leader`
- But application not doing work

**Diagnosis:**
```bash
# Check leader pod
kubectl get pod <leader-pod-name> -o yaml

# Check pod logs
kubectl logs <leader-pod-name>

# Verify annotation
kubectl get pod <leader-pod-name> -o jsonpath='{.metadata.annotations.zen-lead/role}'
```

**Possible Causes:**
1. Application not checking annotation
2. Application crashed
3. Context cancellation
4. Application waiting for other conditions

**Solutions:**
```bash
# Check application logs
kubectl logs <leader-pod-name> -c <container-name>

# Verify application is checking annotation
# Application should check: metadata.annotations['zen-lead/role'] == 'leader'

# Check pod status
kubectl describe pod <leader-pod-name>
```

---

### Issue: Frequent Leadership Changes

**Symptoms:**
- Leader changes frequently
- `status.lastTransitionTime` changes often
- Pods alternating between leader/follower

**Diagnosis:**
```bash
# Watch leader changes
kubectl get leaderpolicy <pool-name> -w

# Check API server latency
kubectl get lease <pool-name> -v=9

# Check pod resource usage
kubectl top pods -l app=<your-app>
```

**Possible Causes:**
1. High API server latency
2. Pod resource constraints (CPU throttling, OOM)
3. Network issues
4. Lease duration too short

**Solutions:**
```bash
# Increase lease duration
kubectl patch leaderpolicy <pool-name> --type=merge -p '{"spec":{"leaseDurationSeconds":30}}'

# Check for OOMKills
kubectl describe pod <pod-name> | grep -i oom

# Check API server health
kubectl get --raw /healthz
```

---

### Issue: Pod Not Joining Pool

**Symptoms:**
- Pod has `zen-lead/pool` annotation
- But not showing up in `status.candidates`

**Diagnosis:**
```bash
# Check pod annotations
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'

# Verify join annotation
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations.zen-lead/join}'

# Check pod phase
kubectl get pod <pod-name> -o jsonpath='{.status.phase}'
```

**Possible Causes:**
1. Missing `zen-lead/join: "true"` annotation
2. Pod not in Running phase
3. Wrong namespace
4. Pool name mismatch

**Solutions:**
```bash
# Add join annotation
kubectl annotate pod <pod-name> zen-lead/join=true

# Verify pod is running
kubectl get pod <pod-name>

# Check namespace matches LeaderPolicy
kubectl get leaderpolicy <pool-name> -o jsonpath='{.metadata.namespace}'
```

---

### Issue: RBAC Errors

**Symptoms:**
- Controller logs show permission errors
- Lease resources not created
- Pod annotations not updated

**Diagnosis:**
```bash
# Check controller logs
kubectl logs -n zen-system -l app=zen-lead | grep -i error

# Test RBAC
kubectl auth can-i create leases --namespace=<namespace> --as=system:serviceaccount:zen-system:zen-lead-controller-manager

# Check ServiceAccount
kubectl get serviceaccount zen-lead-controller-manager -n zen-system
```

**Solutions:**
```bash
# Reapply RBAC
kubectl apply -f config/rbac/

# Verify RoleBinding
kubectl get clusterrolebinding zen-lead-rolebinding

# Check ClusterRole
kubectl get clusterrole zen-lead-role
```

---

## Debugging Tips

### Enable Verbose Logging

```bash
# Set log level in Deployment
kubectl set env deployment/zen-lead-controller-manager LOG_LEVEL=debug -n zen-system
```

### Check Controller Status

```bash
# Get controller pod
kubectl get pods -n zen-system -l app=zen-lead

# Check controller logs
kubectl logs -n zen-system -l app=zen-lead --tail=100

# Check controller metrics
kubectl port-forward -n zen-system svc/zen-lead-controller-manager 8080:8080
curl http://localhost:8080/metrics
```

### Monitor Leader Changes

```bash
# Watch LeaderPolicy status
kubectl get leaderpolicy <pool-name> -w -o jsonpath='{.status.currentHolder.name}{"\n"}'

# Watch pod role annotations
watch -n 1 'kubectl get pods -o jsonpath="{range .items[*]}{.metadata.name}{\"\t\"}{.metadata.annotations.zen-lead/role}{\"\n\"}{end}"'
```

### Check Lease Resource

```bash
# Get Lease details
kubectl get lease <pool-name> -o yaml

# Check holder identity
kubectl get lease <pool-name> -o jsonpath='{.spec.holderIdentity}'

# Check lease duration
kubectl get lease <pool-name> -o jsonpath='{.spec.leaseDurationSeconds}'
```

---

## Performance Issues

### High Reconciliation Frequency

**Symptom:** Controller reconciling too often

**Solution:** Adjust requeue interval in controller (default: 5 seconds)

### API Server Overload

**Symptom:** High API server latency

**Solution:**
- Increase lease duration
- Reduce reconciliation frequency
- Check API server health

---

## Getting Help

If you're still experiencing issues:

1. **Check Logs:** Controller and application logs
2. **Verify Configuration:** LeaderPolicy spec and pod annotations
3. **Test RBAC:** Ensure permissions are correct
4. **Open an Issue:** Provide logs and configuration

---

**See [INTEGRATION.md](INTEGRATION.md) for integration examples.**

