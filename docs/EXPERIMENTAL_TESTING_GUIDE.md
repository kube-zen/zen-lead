# Experimental Features Testing Guide

This guide explains how to test and compare experimental Go 1.25 features (jsonv2, greenteagc) with the standard build.

## Prerequisites

- Kubernetes cluster (kind, minikube, or full cluster)
- kubectl configured
- Helm 3.0+
- Docker (for building images)

## Step 1: Build Images

### Build Standard Image

```bash
cd zen-lead
docker build -t kubezen/zen-lead:standard .
```

### Build Experimental Image

```bash
# Build with JSON v2 only
docker build --build-arg GOEXPERIMENT=jsonv2 -t kubezen/zen-lead:jsonv2 .

# Build with Green Tea GC only
docker build --build-arg GOEXPERIMENT=greenteagc -t kubezen/zen-lead:greenteagc .

# Build with both features
docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc -t kubezen/zen-lead:experimental .
```

## Step 2: Deploy Both Versions

### Deploy Standard Version

```bash
helm install zen-lead-standard ./helm-charts/charts/zen-lead \
  --namespace zen-lead-standard \
  --create-namespace \
  --set image.tag=standard \
  --set replicaCount=1
```

### Deploy Experimental Version

```bash
helm install zen-lead-experimental ./helm-charts/charts/zen-lead \
  --namespace zen-lead-experimental \
  --create-namespace \
  --set image.tag=experimental \
  --set replicaCount=1 \
  --set experimental.jsonv2.enabled=true \
  --set experimental.greenteagc.enabled=true
```

## Step 3: Create Test Workload

Create the same test workload in both namespaces:

```bash
# Create test namespace
kubectl create namespace zen-lead-experimental-test

# Apply test services (same for both deployments)
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: test-app-1
  namespace: zen-lead-experimental-test
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: test-app-1
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-1
  namespace: zen-lead-experimental-test
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test-app-1
  template:
    metadata:
      labels:
        app: test-app-1
    spec:
      containers:
      - name: app
        image: nginx:latest
        ports:
        - containerPort: 8080
          name: http
EOF
```

## Step 4: Run Integration Tests

```bash
# Set environment variables
export ENABLE_EXPERIMENTAL_TESTS=true
export STANDARD_DEPLOYMENT_NAMESPACE=zen-lead-standard
export EXPERIMENTAL_DEPLOYMENT_NAMESPACE=zen-lead-experimental
export SAVE_COMPARISON_REPORT=true
export COMPARISON_REPORT_FILE=./experimental_comparison_report.txt

# Run tests
go test -tags=integration -v ./test/integration/experimental_features_test.go
```

## Step 5: Manual Metrics Collection

### Collect Metrics from Standard Deployment

```bash
# Port-forward to standard deployment
kubectl port-forward -n zen-lead-standard deployment/zen-lead-standard 8080:8080

# In another terminal, scrape metrics
curl http://localhost:8080/metrics > standard_metrics.txt

# Extract key metrics
grep "zen_lead_reconciliation_duration_seconds" standard_metrics.txt
grep "zen_lead_failover_latency_seconds" standard_metrics.txt
grep "zen_lead_cache_hits_total" standard_metrics.txt
grep "zen_lead_cache_misses_total" standard_metrics.txt
```

### Collect Metrics from Experimental Deployment

```bash
# Port-forward to experimental deployment
kubectl port-forward -n zen-lead-experimental deployment/zen-lead-experimental 8080:8080

# In another terminal, scrape metrics
curl http://localhost:8080/metrics > experimental_metrics.txt

# Extract key metrics
grep "zen_lead_reconciliation_duration_seconds" experimental_metrics.txt
grep "zen_lead_failover_latency_seconds" experimental_metrics.txt
grep "zen_lead_cache_hits_total" experimental_metrics.txt
grep "zen_lead_cache_misses_total" experimental_metrics.txt
```

## Step 6: Compare Results

### Key Metrics to Compare

1. **Reconciliation Latency:**
   ```bash
   # Standard
   curl -s http://localhost:8080/metrics | grep "zen_lead_reconciliation_duration_seconds" | grep "quantile=\"0.5\""
   
   # Experimental
   curl -s http://localhost:8080/metrics | grep "zen_lead_reconciliation_duration_seconds" | grep "quantile=\"0.5\""
   ```

2. **Failover Latency:**
   ```bash
   # Standard
   curl -s http://localhost:8080/metrics | grep "zen_lead_failover_latency_seconds" | grep "quantile=\"0.5\""
   
   # Experimental
   curl -s http://localhost:8080/metrics | grep "zen_lead_failover_latency_seconds" | grep "quantile=\"0.5\""
   ```

3. **Cache Performance:**
   ```bash
   # Calculate cache hit rate
   hits=$(curl -s http://localhost:8080/metrics | grep "zen_lead_cache_hits_total" | awk '{print $2}')
   misses=$(curl -s http://localhost:8080/metrics | grep "zen_lead_cache_misses_total" | awk '{print $2}')
   hit_rate=$(echo "scale=2; $hits / ($hits + $misses) * 100" | bc)
   echo "Cache hit rate: ${hit_rate}%"
   ```

4. **Error Rate:**
   ```bash
   errors=$(curl -s http://localhost:8080/metrics | grep "zen_lead_reconciliation_errors_total" | awk '{print $2}')
   reconciliations=$(curl -s http://localhost:8080/metrics | grep "zen_lead_reconciliations_total" | awk '{print $2}')
   error_rate=$(echo "scale=4; $errors / $reconciliations * 100" | bc)
   echo "Error rate: ${error_rate}%"
   ```

## Step 7: Stress Testing

### High-Frequency Failover Test

```bash
# Continuously delete and recreate leader pod to trigger failovers
for i in {1..100}; do
  # Get current leader pod
  LEADER=$(kubectl get pods -n zen-lead-experimental-test -l app=test-app-1 -o jsonpath='{.items[0].metadata.name}')
  
  # Delete leader pod
  kubectl delete pod -n zen-lead-experimental-test $LEADER
  
  # Wait for failover
  sleep 2
done

# Collect failover metrics after stress test
curl -s http://localhost:8080/metrics | grep "zen_lead_failover_latency_seconds"
```

### Long-Running Stability Test

```bash
# Run for 24 hours and monitor for memory leaks
watch -n 60 'kubectl top pod -n zen-lead-experimental | grep zen-lead'
```

## Step 8: Document Results

Create a comparison report:

```markdown
# Experimental Features Performance Comparison

**Date:** 2025-01-02
**Test Duration:** 1 hour
**Test Workload:** 10 services, 3 pods each

## Results

### Reconciliation Latency
- Standard (P50): 15.2 ms
- Experimental (P50): 12.8 ms
- Improvement: 15.8%

### Failover Latency
- Standard (P50): 245 ms
- Experimental (P50): 198 ms
- Improvement: 19.2%

### Cache Hit Rate
- Standard: 87.3%
- Experimental: 87.5%
- Difference: +0.2%

### Error Rate
- Standard: 0.12%
- Experimental: 0.11%
- Difference: -0.01%

## Conclusion

Experimental features show:
- ✅ 15-20% improvement in reconciliation latency
- ✅ 19% improvement in failover latency
- ✅ No degradation in cache performance
- ✅ No increase in error rate

**Recommendation:** Continue testing in staging. Not ready for production until features are GA.
```

## Troubleshooting

### Metrics Not Available

```bash
# Check if metrics endpoint is accessible
kubectl port-forward -n zen-lead-standard deployment/zen-lead-standard 8080:8080
curl http://localhost:8080/metrics | head -20
```

### Deployment Not Ready

```bash
# Check deployment status
kubectl get deployments -n zen-lead-standard
kubectl get pods -n zen-lead-standard

# Check logs
kubectl logs -n zen-lead-standard deployment/zen-lead-standard
```

### No Performance Improvement

- Verify binary was built with GOEXPERIMENT flags
- Check GOEXPERIMENT_INFO env var in pod
- Ensure experimental features are actually enabled in Helm values

## Cleanup

```bash
# Remove test deployments
helm uninstall zen-lead-standard -n zen-lead-standard
helm uninstall zen-lead-experimental -n zen-lead-experimental

# Remove test namespace
kubectl delete namespace zen-lead-experimental-test
```

