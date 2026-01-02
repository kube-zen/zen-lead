#!/bin/bash
# Copyright 2025 Kube-ZEN Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

CLUSTER_NAME="${CLUSTER_NAME:-zen-lead-e2e}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-${HOME}/.kube/${CLUSTER_NAME}-config}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Install from https://kind.sigs.k8s.io/"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

create_cluster() {
    log_info "Creating kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_warn "Cluster $CLUSTER_NAME already exists"
        read -p "Delete existing cluster? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kind delete cluster --name "$CLUSTER_NAME"
        else
            log_info "Using existing cluster"
            return
        fi
    fi
    
    # Create cluster config
    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
  - containerPort: 8081
    hostPort: 8081
    protocol: TCP
EOF
    
    log_info "Cluster created successfully"
}

setup_kubeconfig() {
    log_info "Setting up kubeconfig..."
    export KUBECONFIG="$KUBECONFIG_PATH"
    kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_PATH"
    chmod 600 "$KUBECONFIG_PATH"
    log_info "Kubeconfig saved to $KUBECONFIG_PATH"
}

install_crds() {
    log_info "Installing CRDs..."
    # zen-lead is CRD-free, but we may need to install other dependencies
    # For now, skip CRD installation
    log_info "No CRDs to install (zen-lead is CRD-free)"
}

deploy_controller() {
    log_info "Building and deploying zen-lead controller..."
    
    # Build controller image
    cd "$(dirname "$0")/../.."
    docker build -t zen-lead:test -f Dockerfile .
    
    # Load image into kind
    kind load docker-image zen-lead:test --name "$CLUSTER_NAME"
    
    # Create namespace
    kubectl create namespace zen-lead-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply RBAC
    kubectl apply -f config/rbac/
    
    # Create deployment (simplified for E2E)
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-lead-controller-manager
  namespace: zen-lead-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zen-lead
  template:
    metadata:
      labels:
        app: zen-lead
    spec:
      serviceAccountName: zen-lead-role
      containers:
      - name: manager
        image: zen-lead:test
        imagePullPolicy: Never
        args:
        - --metrics-bind-address=:8080
        - --health-probe-bind-address=:8081
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        ports:
        - containerPort: 8080
          name: metrics
        - containerPort: 8081
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
EOF
    
    log_info "Waiting for controller to be ready..."
    kubectl wait --for=condition=ready pod -l app=zen-lead -n zen-lead-system --timeout=120s
    log_info "Controller deployed successfully"
}

delete_cluster() {
    log_info "Deleting kind cluster: $CLUSTER_NAME"
    kind delete cluster --name "$CLUSTER_NAME" || true
    log_info "Cluster deleted"
}

get_kubeconfig() {
    echo "$KUBECONFIG_PATH"
}

main() {
    case "${1:-}" in
        create)
            check_prerequisites
            create_cluster
            setup_kubeconfig
            install_crds
            deploy_controller
            log_info "✅ E2E test environment ready"
            log_info "Export kubeconfig: export KUBECONFIG=$KUBECONFIG_PATH"
            ;;
        delete)
            delete_cluster
            ;;
        kubeconfig)
            get_kubeconfig
            ;;
        *)
            echo "Usage: $0 {create|delete|kubeconfig}"
            exit 1
            ;;
    esac
}

main "$@"

