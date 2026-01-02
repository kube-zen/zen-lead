#!/bin/bash
# zen-lead Functional Test Script
# Tests failover behavior in a real Kubernetes cluster

set -euo pipefail

CONTEXT="${KUBECTL_CONTEXT:-k3d-astesterole}"
NAMESPACE="${TEST_NAMESPACE:-zen-lead-test}"
TEST_DEPLOYMENT="${TEST_DEPLOYMENT:-test-app}"
TEST_SERVICE="${TEST_SERVICE:-test-app}"
IMAGE_TAG="${IMAGE_TAG:-test}"
REPORT_FILE="${REPORT_FILE:-/tmp/zen-lead-functional-test-report-$(date +%Y%m%d-%H%M%S).md}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] ✅ $*${NC}"
}

log_error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ❌ $*${NC}"
}

log_warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] ⚠️  $*${NC}"
}

# Initialize report
init_report() {
    cat > "$REPORT_FILE" <<EOF
# zen-lead Functional Test Report

**Test Date:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')
**Cluster Context:** $CONTEXT
**Namespace:** $NAMESPACE
**Image Tag:** $IMAGE_TAG

## Test Summary

EOF
}

append_report() {
    echo "$*" >> "$REPORT_FILE"
}

# Test functions
test_install_zen_lead() {
    log "Installing zen-lead controller..."
    
    if kubectl --context "$CONTEXT" get namespace "$NAMESPACE" &>/dev/null; then
        log_warn "Namespace $NAMESPACE already exists"
    else
        kubectl --context "$CONTEXT" create namespace "$NAMESPACE"
    fi
    
    # Deploy zen-lead using Helm or manifests
    if command -v helm &>/dev/null; then
        log "Installing via Helm..."
        helm upgrade --install zen-lead \
            --namespace "$NAMESPACE" \
            --create-namespace \
            --set image.tag="$IMAGE_TAG" \
            --set image.repository=kubezen/zen-lead \
            ../../helm-charts/charts/zen-lead \
            --kube-context "$CONTEXT" || {
            log_error "Helm install failed, trying manifests..."
            deploy_manifests
        }
    else
        deploy_manifests
    fi
    
    log "Waiting for zen-lead to be ready..."
    kubectl --context "$CONTEXT" wait --for=condition=available \
        --timeout=300s \
        deployment/zen-lead-controller \
        -n "$NAMESPACE" || {
        log_error "zen-lead deployment not ready"
        return 1
    }
    
    log_success "zen-lead installed and ready"
    append_report "- ✅ zen-lead controller installed and ready"
}

deploy_manifests() {
    log "Deploying via manifests..."
    # Create a simple deployment manifest
    cat <<EOF | kubectl --context "$CONTEXT" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-lead-controller
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zen-lead-controller
  template:
    metadata:
      labels:
        app: zen-lead-controller
    spec:
      serviceAccountName: zen-lead-controller
      containers:
      - name: manager
        image: kubezen/zen-lead:$IMAGE_TAG
        imagePullPolicy: IfNotPresent
        args:
        - --metrics-bind-address=:8080
        - --health-probe-bind-address=:8081
        - --leader-election-id=zen-lead-controller-leader-election
        ports:
        - containerPort: 8080
          name: metrics
        - containerPort: 8081
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: zen-lead-controller
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: zen-lead-controller
rules:
- apiGroups: [""]
  resources: ["services", "pods", "events"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: zen-lead-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: zen-lead-controller
subjects:
- kind: ServiceAccount
  name: zen-lead-controller
  namespace: $NAMESPACE
EOF
}

test_create_test_deployment() {
    log "Creating test deployment with 3 replicas..."
    
    cat <<EOF | kubectl --context "$CONTEXT" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $TEST_DEPLOYMENT
  namespace: $NAMESPACE
spec:
  replicas: 3
  selector:
    matchLabels:
      app: $TEST_DEPLOYMENT
  template:
    metadata:
      labels:
        app: $TEST_DEPLOYMENT
    spec:
      containers:
      - name: app
        image: nginx:1.25-alpine
        ports:
        - containerPort: 80
          name: http
        readinessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 10
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: $TEST_SERVICE
  namespace: $NAMESPACE
  annotations:
    zen-lead.io/enabled: "true"
spec:
  selector:
    app: $TEST_DEPLOYMENT
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
    name: http
EOF
    
    log "Waiting for test deployment to be ready..."
    kubectl --context "$CONTEXT" wait --for=condition=available \
        --timeout=120s \
        deployment/$TEST_DEPLOYMENT \
        -n "$NAMESPACE"
    
    log "Waiting for pods to be ready..."
    kubectl --context "$CONTEXT" wait --for=condition=ready \
        --timeout=120s \
        pod -l app=$TEST_DEPLOYMENT \
        -n "$NAMESPACE" || true
    
    log_success "Test deployment created"
    append_report "- ✅ Test deployment created with 3 replicas"
}

test_verify_leader_service() {
    log "Verifying leader service creation..."
    
    local leader_service="${TEST_SERVICE}-leader"
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        if kubectl --context "$CONTEXT" get service "$leader_service" -n "$NAMESPACE" &>/dev/null; then
            log_success "Leader service created: $leader_service"
            append_report "- ✅ Leader service created: $leader_service"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    
    log_error "Leader service not created after ${max_wait}s"
    append_report "- ❌ Leader service not created"
    return 1
}

test_verify_endpointslice() {
    log "Verifying EndpointSlice creation..."
    
    local max_wait=60
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        local slices=$(kubectl --context "$CONTEXT" get endpointslice -n "$NAMESPACE" \
            -l kubernetes.io/service-name="${TEST_SERVICE}-leader" \
            -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
        
        if [ -n "$slices" ]; then
            local endpoints=$(kubectl --context "$CONTEXT" get endpointslice -n "$NAMESPACE" \
                -l kubernetes.io/service-name="${TEST_SERVICE}-leader" \
                -o jsonpath='{.items[0].endpoints[*].addresses[0]}' 2>/dev/null || echo "")
            
            if [ -n "$endpoints" ]; then
                log_success "EndpointSlice created with endpoint: $endpoints"
                append_report "- ✅ EndpointSlice created with endpoint: $endpoints"
                return 0
            fi
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    
    log_error "EndpointSlice not created after ${max_wait}s"
    append_report "- ❌ EndpointSlice not created"
    return 1
}

get_leader_pod() {
    local leader_service="${TEST_SERVICE}-leader"
    local endpoint=$(kubectl --context "$CONTEXT" get endpointslice -n "$NAMESPACE" \
        -l kubernetes.io/service-name="$leader_service" \
        -o jsonpath='{.items[0].endpoints[0].targetRef.name}' 2>/dev/null || echo "")
    echo "$endpoint"
}

test_failover() {
    log "Testing failover behavior..."
    
    # Get initial leader pod
    local initial_leader=$(get_leader_pod)
    if [ -z "$initial_leader" ]; then
        log_error "Could not determine initial leader pod"
        append_report "- ❌ Could not determine initial leader pod"
        return 1
    fi
    
    log "Initial leader pod: $initial_leader"
    append_report ""
    append_report "## Failover Test"
    append_report ""
    append_report "**Initial Leader Pod:** $initial_leader"
    
    # Record start time
    local start_time=$(date +%s.%N)
    log "Deleting leader pod at $(date -u '+%Y-%m-%d %H:%M:%S UTC')..."
    append_report "**Failover Start Time:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    
    # Delete the leader pod
    kubectl --context "$CONTEXT" delete pod "$initial_leader" -n "$NAMESPACE" --grace-period=0
    
    # Wait for new leader to be selected
    local max_wait=60
    local elapsed=0
    local new_leader=""
    
    log "Waiting for new leader selection..."
    while [ $elapsed -lt $max_wait ]; do
        sleep 1
        elapsed=$((elapsed + 1))
        
        new_leader=$(get_leader_pod)
        if [ -n "$new_leader" ] && [ "$new_leader" != "$initial_leader" ]; then
            local end_time=$(date +%s.%N)
            local downtime=$(echo "$end_time - $start_time" | bc)
            
            log_success "New leader selected: $new_leader"
            log_success "Failover time: ${downtime}s"
            
            append_report "**New Leader Pod:** $new_leader"
            append_report "**Failover End Time:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
            append_report "**Downtime:** ${downtime}s"
            append_report ""
            append_report "- ✅ Failover test completed successfully"
            
            return 0
        fi
        
        if [ $((elapsed % 5)) -eq 0 ]; then
            log "Still waiting... (${elapsed}s elapsed)"
        fi
    done
    
    log_error "New leader not selected after ${max_wait}s"
    append_report "- ❌ New leader not selected after ${max_wait}s"
    return 1
}

test_multiple_failovers() {
    log "Testing multiple failovers..."
    append_report ""
    append_report "## Multiple Failover Test"
    append_report ""
    
    local num_failovers=3
    local total_downtime=0
    local success_count=0
    
    for i in $(seq 1 $num_failovers); do
        log "Failover attempt $i/$num_failovers..."
        
        local leader=$(get_leader_pod)
        if [ -z "$leader" ]; then
            log_error "Could not determine leader pod for attempt $i"
            continue
        fi
        
        local start_time=$(date +%s.%N)
        kubectl --context "$CONTEXT" delete pod "$leader" -n "$NAMESPACE" --grace-period=0
        
        local max_wait=60
        local elapsed=0
        local new_leader=""
        
        while [ $elapsed -lt $max_wait ]; do
            sleep 1
            elapsed=$((elapsed + 1))
            
            new_leader=$(get_leader_pod)
            if [ -n "$new_leader" ] && [ "$new_leader" != "$leader" ]; then
                local end_time=$(date +%s.%N)
                local downtime=$(echo "$end_time - $start_time" | bc)
                total_downtime=$(echo "$total_downtime + $downtime" | bc)
                success_count=$((success_count + 1))
                
                log_success "Failover $i completed in ${downtime}s"
                append_report "**Failover $i:** ${downtime}s (from $leader to $new_leader)"
                
                # Wait a bit before next failover
                sleep 5
                break
            fi
        done
        
        if [ -z "$new_leader" ] || [ "$new_leader" = "$leader" ]; then
            log_error "Failover $i failed"
            append_report "**Failover $i:** ❌ Failed"
        fi
    done
    
    if [ $success_count -gt 0 ]; then
        local avg_downtime=$(echo "scale=2; $total_downtime / $success_count" | bc)
        log_success "Average failover time: ${avg_downtime}s (${success_count}/${num_failovers} successful)"
        append_report ""
        append_report "**Average Failover Time:** ${avg_downtime}s"
        append_report "**Success Rate:** ${success_count}/${num_failovers}"
    fi
}

cleanup() {
    log "Cleaning up test resources..."
    kubectl --context "$CONTEXT" delete deployment "$TEST_DEPLOYMENT" -n "$NAMESPACE" --ignore-not-found=true
    kubectl --context "$CONTEXT" delete service "$TEST_SERVICE" -n "$NAMESPACE" --ignore-not-found=true
    kubectl --context "$CONTEXT" delete service "${TEST_SERVICE}-leader" -n "$NAMESPACE" --ignore-not-found=true
    log_success "Cleanup complete"
}

# Main execution
main() {
    log "Starting zen-lead functional test"
    log "Context: $CONTEXT"
    log "Namespace: $NAMESPACE"
    log "Report: $REPORT_FILE"
    
    init_report
    
    # Check if bc is available for calculations
    if ! command -v bc &>/dev/null; then
        log_warn "bc not found, installing..."
        if command -v apt-get &>/dev/null; then
            sudo apt-get update && sudo apt-get install -y bc
        elif command -v yum &>/dev/null; then
            sudo yum install -y bc
        fi
    fi
    
    # Run tests
    test_install_zen_lead || exit 1
    sleep 10  # Give controller time to start
    
    test_create_test_deployment || exit 1
    sleep 15  # Give time for leader selection
    
    test_verify_leader_service || exit 1
    test_verify_endpointslice || exit 1
    
    test_failover || log_warn "Single failover test failed"
    sleep 10
    
    test_multiple_failovers || log_warn "Multiple failover test failed"
    
    # Final summary
    append_report ""
    append_report "## Test Conclusion"
    append_report ""
    append_report "**Test completed at:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    
    log_success "Test complete! Report saved to: $REPORT_FILE"
    cat "$REPORT_FILE"
}

# Trap cleanup on exit
trap cleanup EXIT

main "$@"

