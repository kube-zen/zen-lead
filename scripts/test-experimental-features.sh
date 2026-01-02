#!/bin/bash
# Test script for experimental Go 1.25 features
# This script runs integration tests comparing standard vs experimental builds

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_NUM_SERVICES="${TEST_NUM_SERVICES:-5}"
TEST_PODS_PER_SERVICE="${TEST_PODS_PER_SERVICE:-3}"
TEST_DURATION="${TEST_DURATION:-5m}"
TEST_FAILOVER_FREQUENCY="${TEST_FAILOVER_FREQUENCY:-10}"
STANDARD_NAMESPACE="${STANDARD_DEPLOYMENT_NAMESPACE:-zen-lead-standard}"
EXPERIMENTAL_NAMESPACE="${EXPERIMENTAL_DEPLOYMENT_NAMESPACE:-zen-lead-experimental}"
REPORT_FILE="${COMPARISON_REPORT_FILE:-./experimental_comparison_report.txt}"

echo -e "${GREEN}=== Experimental Features Integration Test ===${NC}"
echo ""
echo "Configuration:"
echo "  Services: $TEST_NUM_SERVICES"
echo "  Pods per Service: $TEST_PODS_PER_SERVICE"
echo "  Test Duration: $TEST_DURATION"
echo "  Failover Frequency: $TEST_FAILOVER_FREQUENCY"
echo "  Standard Namespace: $STANDARD_NAMESPACE"
echo "  Experimental Namespace: $EXPERIMENTAL_NAMESPACE"
echo "  Report File: $REPORT_FILE"
echo ""

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl not found${NC}"
    exit 1
fi

if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Kubernetes cluster not accessible${NC}"
    exit 1
fi

# Check if deployments exist
if ! kubectl get deployment -n "$STANDARD_NAMESPACE" zen-lead-standard &> /dev/null; then
    echo -e "${YELLOW}Warning: Standard deployment not found in namespace $STANDARD_NAMESPACE${NC}"
    echo "  Deploy with: helm install zen-lead-standard ./helm-charts/charts/zen-lead --namespace $STANDARD_NAMESPACE --create-namespace --set image.tag=standard"
fi

if ! kubectl get deployment -n "$EXPERIMENTAL_NAMESPACE" zen-lead-experimental &> /dev/null; then
    echo -e "${YELLOW}Warning: Experimental deployment not found in namespace $EXPERIMENTAL_NAMESPACE${NC}"
    echo "  Deploy with: helm install zen-lead-experimental ./helm-charts/charts/zen-lead --namespace $EXPERIMENTAL_NAMESPACE --create-namespace --set image.tag=experimental --set experimental.jsonv2.enabled=true --set experimental.greenteagc.enabled=true"
fi

echo ""
echo -e "${GREEN}Running integration tests...${NC}"
echo ""

# Run tests
cd "$PROJECT_ROOT"

export ENABLE_EXPERIMENTAL_TESTS=true
export TEST_NUM_SERVICES
export TEST_PODS_PER_SERVICE
export TEST_DURATION
export TEST_FAILOVER_FREQUENCY
export STANDARD_DEPLOYMENT_NAMESPACE="$STANDARD_NAMESPACE"
export EXPERIMENTAL_DEPLOYMENT_NAMESPACE="$EXPERIMENTAL_NAMESPACE"
export SAVE_COMPARISON_REPORT=true
export COMPARISON_REPORT_FILE="$REPORT_FILE"

if go test -tags=integration -v -timeout=30m ./test/integration/experimental_features_test.go; then
    echo ""
    echo -e "${GREEN}✅ Tests completed successfully!${NC}"
    echo ""
    if [ -f "$REPORT_FILE" ]; then
        echo -e "${GREEN}Comparison report saved to: $REPORT_FILE${NC}"
        echo ""
        echo "Report contents:"
        echo "---"
        cat "$REPORT_FILE"
        echo "---"
    fi
else
    echo ""
    echo -e "${RED}❌ Tests failed${NC}"
    exit 1
fi

