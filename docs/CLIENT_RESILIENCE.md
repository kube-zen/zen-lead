# Client Resilience and Failover Expectations

**Last Updated:** 2025-12-30

This guide explains what `zen-lead` guarantees, what clients must do to handle failover gracefully, and how to troubleshoot leader routing issues.

## What Zen-Lead Guarantees

### Controller-Side Guarantees

1. **Leader Service Routes to a Single Ready Pod**
   - The `<service-name>-leader` Service always points to exactly one Ready pod
   - If no Ready pods exist, the EndpointSlice has zero endpoints (clean failure mode)

2. **Failover is Controller-Driven**
   - Leader selection happens immediately when:
     - Current leader pod becomes NotReady
     - Current leader pod is terminating (deletionTimestamp set)
     - Current leader pod loses its PodIP
   - EndpointSlice updates happen promptly after detection (sub-second controller-side)

3. **Residual Disruption Depends on Client Connection Semantics**
   - Controller-side failover is fast, but clients may experience disruption due to:
     - Dataplane propagation delay (kube-proxy, CNI, etc.)
     - Long-lived connections that don't reconnect
     - DNS caching (if using DNS-based service discovery)

## What Clients Must Do

### Timeouts and Retries

**Critical:** Clients must implement proper timeouts and retries to handle failover gracefully.

#### Connect Timeouts

```go
// Go example: short connect timeout
client := &http.Client{
    Timeout: 5 * time.Second, // Total request timeout
    Transport: &http.Transport{
        DialTimeout: 2 * time.Second, // Connect timeout
    },
}
```

```python
# Python example: short connect timeout
import requests
response = requests.get(
    'http://my-app-leader:8080/api/endpoint',
    timeout=(2, 5)  # (connect_timeout, read_timeout)
)
```

#### Retry Logic

**Idempotent Requests Only:** Only retry operations that are safe to retry (GET, PUT with idempotent keys, etc.)

```go
// Go example: retry with jittered backoff
func retryWithBackoff(fn func() error, maxRetries int) error {
    baseDelay := 100 * time.Millisecond
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        if i == maxRetries-1 {
            return err
        }
        // Exponential backoff with jitter
        delay := baseDelay * time.Duration(1<<uint(i))
        jitter := time.Duration(rand.Intn(100)) * time.Millisecond
        time.Sleep(delay + jitter)
    }
    return fmt.Errorf("max retries exceeded")
}
```

```python
# Python example: retry with exponential backoff
import time
import random
from tenacity import retry, stop_after_attempt, wait_exponential

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=0.1, max=2))
def make_request():
    return requests.get('http://my-app-leader:8080/api/endpoint', timeout=(2, 5))
```

**Cap Retries:** Always limit the number of retries to prevent cascading failures.

### Long-Lived Connections and Streams

**Important:** Failover requires reconnect. Long-lived connections (WebSockets, gRPC streams, TCP connections) will break on leader change.

#### HTTP Long Polling / WebSockets

```go
// Go example: reconnect loop for WebSocket
func connectWebSocket(url string) (*websocket.Conn, error) {
    for {
        conn, _, err := websocket.DefaultDialer.Dial(url, nil)
        if err == nil {
            return conn, nil
        }
        log.Printf("WebSocket connection failed: %v, retrying in 5s", err)
        time.Sleep(5 * time.Second)
    }
}
```

#### gRPC Streams

```go
// Go example: gRPC stream with reconnect
func streamWithReconnect(ctx context.Context, client pb.MyServiceClient) error {
    for {
        stream, err := client.StreamData(ctx, &pb.StreamRequest{})
        if err != nil {
            log.Printf("Stream failed: %v, reconnecting in 2s", err)
            time.Sleep(2 * time.Second)
            continue
        }
        // Handle stream messages
        for {
            msg, err := stream.Recv()
            if err != nil {
                log.Printf("Stream receive error: %v, reconnecting", err)
                break // Exit inner loop, reconnect
            }
            // Process message
        }
    }
}
```

### HTTP-Specific Guidance

#### Keep-Alive Settings

```go
// Go example: reasonable keep-alive settings
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     30 * time.Second,
    DisableKeepAlives:   false, // Keep-alive helps with failover
}
```

**Why Keep-Alive Helps:** Keep-alive connections can detect connection failures faster and trigger reconnect.

#### Retry on Connection Errors

```go
// Go example: retry on connection reset/refused
func httpRequestWithRetry(url string) (*http.Response, error) {
    client := &http.Client{Timeout: 5 * time.Second}
    for i := 0; i < 3; i++ {
        resp, err := client.Get(url)
        if err == nil {
            return resp, nil
        }
        // Check if error is connection-related
        if isConnectionError(err) {
            log.Printf("Connection error: %v, retrying", err)
            time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
            continue
        }
        return nil, err
    }
    return nil, fmt.Errorf("max retries exceeded")
}

func isConnectionError(err error) bool {
    if err == nil {
        return false
    }
    errStr := err.Error()
    return strings.Contains(errStr, "connection refused") ||
           strings.Contains(errStr, "connection reset") ||
           strings.Contains(errStr, "no such host")
}
```

### gRPC-Specific Guidance

#### Deadlines on Every Call

```go
// Go example: deadline on every gRPC call
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.MyMethod(ctx, &pb.Request{})
if err != nil {
    if status.Code(err) == codes.DeadlineExceeded {
        // Retry with new context
    }
}
```

#### Retry Policy (Idempotent Methods Only)

```go
// Go example: gRPC retry policy (WARNING: only for idempotent methods)
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func grpcCallWithRetry(ctx context.Context, client pb.MyServiceClient) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        resp, err := client.MyMethod(ctx, &pb.Request{})
        if err == nil {
            return nil
        }
        // Only retry on transient errors
        code := status.Code(err)
        if code == codes.Unavailable || code == codes.DeadlineExceeded {
            if i < maxRetries-1 {
                time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
                continue
            }
        }
        return err
    }
    return fmt.Errorf("max retries exceeded")
}
```

**⚠️ Warning:** Do NOT retry non-idempotent gRPC methods (e.g., `CreateUser`, `TransferMoney`). Only retry idempotent methods (e.g., `GetUser`, `ListItems`).

## Avoiding the "Wrong Service" Problem

### Canonical Naming Migration

**Problem:** Users might accidentally connect to the original Service instead of the leader Service.

**Solution:** Rename the original Service and use the leader Service as the primary endpoint.

```yaml
# Step 1: Rename original Service
apiVersion: v1
kind: Service
metadata:
  name: my-app-all  # Renamed from my-app
spec:
  selector:
    app: my-app
  ports:
    - port: 8080

---
# Step 2: Annotate the renamed Service
apiVersion: v1
kind: Service
metadata:
  name: my-app-all
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: my-app
  ports:
    - port: 8080

# Step 3: Use my-app-all-leader as the primary endpoint
# (or create a new Service named "my-app" that points to my-app-all-leader)
```

### Ingress/Gateway Configuration

**Best Practice:** Configure Ingress or Gateway to target the leader Service only.

```yaml
# Ingress targeting leader Service
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-ingress
spec:
  rules:
    - host: my-app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app-leader  # Target leader Service
                port:
                  number: 8080
```

## Operational Expectations

### Failover Timeline

1. **Detection (Controller-Side):** < 1 second
   - Pod becomes NotReady → controller detects via watch predicate
   - Controller immediately selects new leader and updates EndpointSlice

2. **Dataplane Convergence:** 1-5 seconds (varies by CNI/kube-proxy)
   - kube-proxy updates iptables/IPVS rules
   - CNI updates network policies/routes
   - DNS cache expires (if using DNS)

3. **Client Reconnect:** Depends on client timeout/retry settings
   - Short timeouts (2-5s) → fast reconnect
   - Long timeouts → longer disruption

**Total Expected Disruption:** 2-10 seconds for most clients with proper timeouts.

### Troubleshooting Commands

#### Check Leader Service Endpoints

```bash
# Check if leader Service has endpoints
kubectl get endpointslices -l kubernetes.io/service-name=my-app-leader

# Check leader pod identity
kubectl get svc my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/current-leader}'

# Check leader pod UID
kubectl get svc my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-pod-uid}'

# Check last leader switch time
kubectl get svc my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/leader-last-switch-time}'
```

#### Check Events

```bash
# Check events on source Service
kubectl describe svc my-app | grep -A 10 Events

# Check events on leader Service
kubectl describe svc my-app-leader | grep -A 10 Events

# Check controller events
kubectl get events --field-selector involvedObject.name=my-app-leader --sort-by='.lastTimestamp'
```

#### Verify Leader Pod Status

```bash
# Get current leader pod name
LEADER_POD=$(kubectl get svc my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/current-leader}')

# Check leader pod readiness
kubectl get pod $LEADER_POD -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'

# Check leader pod IP
kubectl get pod $LEADER_POD -o jsonpath='{.status.podIP}'
```

#### Debug EndpointSlice

```bash
# Get EndpointSlice details
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o yaml

# Check endpoint targetRef
kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath='{.items[0].endpoints[0].targetRef}'
```

#### Monitor Failover

```bash
# Watch leader Service annotations (shows leader changes)
watch -n 1 'kubectl get svc my-app-leader -o jsonpath="{.metadata.annotations.zen-lead\.io/current-leader} {.metadata.annotations.zen-lead\.io/leader-last-switch-time}"'

# Watch EndpointSlice endpoints
watch -n 1 'kubectl get endpointslice -l kubernetes.io/service-name=my-app-leader -o jsonpath="{.items[0].endpoints[*].addresses}"'
```

## Common Issues and Solutions

### Issue: "Leader Service has no endpoints"

**Symptoms:**
- `kubectl get endpointslices -l kubernetes.io/service-name=my-app-leader` shows zero endpoints
- Clients cannot connect to leader Service

**Causes:**
1. No Ready pods match the Service selector
2. Port resolution failed (named `targetPort` cannot be resolved)
3. All pods are NotReady

**Solutions:**
```bash
# Check if any pods are Ready
kubectl get pods -l app=my-app -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'

# Check for port resolution errors
kubectl describe svc my-app | grep -i "port.*resolution\|warning"

# Check pod container ports
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].ports[*].name}'
```

### Issue: "Failover takes too long"

**Symptoms:**
- Leader pod is NotReady, but EndpointSlice still points to it
- Clients experience long disruption (>10 seconds)

**Causes:**
1. Controller not running or not leader
2. Event not received (informers not synced)
3. Client timeout too long

**Solutions:**
```bash
# Check controller status
kubectl get pods -l app.kubernetes.io/name=zen-lead

# Check controller logs
kubectl logs -l app.kubernetes.io/name=zen-lead --tail=100 | grep -i "leader\|failover"

# Check controller metrics
kubectl port-forward svc/zen-lead-metrics 8080:8080
curl http://localhost:8080/metrics | grep zen_lead_failover
```

### Issue: "Wrong pod is leader"

**Symptoms:**
- Leader Service points to a pod that shouldn't be leader
- Multiple pods think they are leader

**Causes:**
1. Sticky leader logic keeping unhealthy pod
2. Race condition during failover

**Solutions:**
```bash
# Check current leader pod readiness
LEADER_POD=$(kubectl get svc my-app-leader -o jsonpath='{.metadata.annotations.zen-lead\.io/current-leader}')
kubectl get pod $LEADER_POD

# Force failover by deleting leader pod (if safe)
kubectl delete pod $LEADER_POD

# Check if sticky is disabled
kubectl get svc my-app -o jsonpath='{.metadata.annotations.zen-lead\.io/sticky}'
```

## Best Practices Summary

1. **Always use timeouts:** Connect timeout: 2-5s, read timeout: 5-10s
2. **Retry idempotent requests only:** GET, idempotent PUT/PATCH
3. **Cap retries:** Maximum 3 retries with exponential backoff
4. **Handle long-lived connections:** Implement reconnect loops for WebSockets/gRPC streams
5. **Use leader Service directly:** Configure Ingress/Gateway to target `<service>-leader`
6. **Monitor failover metrics:** Track `zen_lead_failover_count_total` and `zen_lead_reconciliation_duration_seconds`
7. **Test failover:** Regularly test by deleting leader pod and verifying client reconnection

## See Also

- [README.md](../README.md) - Overview and quick start
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Detailed troubleshooting guide
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture and design details

