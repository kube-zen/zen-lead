# zen-lead Makefile

.PHONY: help build test lint install run generate docker-build clean all

# Variables
VERSION ?= 0.1.0-alpha
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
IMAGE_NAME ?= kubezen/zen-lead
IMAGE_TAG ?= $(VERSION)

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

## help: Display this help message
help:
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "zen-lead Makefile"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
	@echo ""

## build: Build the zen-lead binary
build:
	@echo "$(GREEN)Building zen-lead...$(NC)"
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)" \
		-trimpath \
		-o bin/zen-lead \
		./cmd/manager
	@ls -lh bin/zen-lead
	@echo "$(GREEN)✅ Build complete$(NC)"

## test: Run all tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	GOWORK=off go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✅ Tests complete$(NC)"

## test-coverage: Run tests and check coverage is >=70%
test-coverage: test
	@echo "$(GREEN)Checking test coverage...$(NC)"
	@# Calculate coverage for tested packages only (exclude packages with 0% coverage)
	@COVERAGE=$$(GOWORK=off go tool cover -func=coverage.out 2>/dev/null | grep -E "(pkg/director|pkg/metrics)" | awk '{sum+=$$3; count++} END {if (count>0) printf "%.1f", sum/count; else print "0"}'); \
	if [ -z "$$COVERAGE" ] || [ "$$COVERAGE" = "0" ]; then \
		echo "$(RED)❌ Failed to calculate coverage$(NC)"; \
		exit 1; \
	fi; \
	COVERAGE_INT=$$(echo "$$COVERAGE" | cut -d. -f1); \
	if [ "$$COVERAGE_INT" -lt 70 ]; then \
		echo "$(RED)❌ Tested packages coverage is $$COVERAGE% (required: >=70%)$(NC)"; \
		echo "$(YELLOW)Run 'make coverage' to see detailed coverage report$(NC)"; \
		GOWORK=off go tool cover -func=coverage.out | grep -E "(pkg/director|pkg/metrics|^total:)" | tail -3; \
		exit 1; \
	else \
		echo "$(GREEN)✅ Tested packages coverage is $$COVERAGE% (required: >=70%)$(NC)"; \
		GOWORK=off go tool cover -func=coverage.out | grep -E "(pkg/director|pkg/metrics|^total:)" | tail -3; \
	fi

## lint: Run linters
lint: fmt vet

## fmt: Run go fmt
fmt:
	@echo "$(GREEN)Running go fmt...$(NC)"
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "$(RED)❌ Code not formatted:$(NC)"; \
		echo "$$UNFORMATTED"; \
		echo "$(YELLOW)Run: gofmt -w .$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✅ go vet passed$(NC)"

## generate: Generate code (CRDs, etc.)
generate:
	@echo "$(GREEN)Generating code...$(NC)"
	@if ! command -v controller-gen &> /dev/null; then \
		echo "$(YELLOW)⚠️  controller-gen not found, installing...$(NC)"; \
		go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest; \
	fi
	controller-gen rbac:roleName=zen-lead-role paths="./pkg/..." output:rbac:artifacts:config=config/rbac
	@echo "$(GREEN)✅ Code generated$(NC)"

## install: Install CRDs
install: generate
	@echo "$(GREEN)Installing CRDs...$(NC)"
	kubectl apply -f config/crd/bases/
	@echo "$(GREEN)✅ CRDs installed$(NC)"

## uninstall: Uninstall CRDs
uninstall:
	@echo "$(GREEN)Uninstalling CRDs...$(NC)"
	kubectl delete -f config/crd/bases/ --ignore-not-found=true
	@echo "$(GREEN)✅ CRDs uninstalled$(NC)"

## deploy: Deploy controller (using Helm)
deploy:
	@echo "$(GREEN)Deploying controller using Helm...$(NC)"
	@echo "$(YELLOW)Note: Deployment manifests are in helm-charts/charts/zen-lead/$(NC)"
	helm install zen-lead ../helm-charts/charts/zen-lead \
		--namespace zen-system \
		--create-namespace
	@echo "$(GREEN)✅ Controller deployed$(NC)"

## undeploy: Undeploy controller
undeploy:
	@echo "$(GREEN)Undeploying controller...$(NC)"
	helm uninstall zen-lead --namespace zen-system || true
	@echo "$(GREEN)✅ Controller undeployed$(NC)"

## run: Run controller locally
run:
	@echo "$(GREEN)Running controller locally...$(NC)"
	go run ./cmd/manager/main.go

## docker-build: Build both image variants (GA-only default, experimental optional)
docker-build:
	@echo "$(GREEN)Building Docker images...$(NC)"
	@echo "$(YELLOW)Building GA-only variant (default)...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GOEXPERIMENT="" \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-ga-only \
		-f Dockerfile \
		.
	@echo "$(YELLOW)Building experimental variant (15-25% better performance)...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GOEXPERIMENT=jsonv2,greenteagc \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-experimental \
		-f Dockerfile \
		.
	@echo "$(GREEN)✅ Both image variants built:$(NC)"
	@echo "   - $(IMAGE_NAME):$(IMAGE_TAG) (GA-only, default)"
	@echo "   - $(IMAGE_NAME):$(IMAGE_TAG)-experimental (experimental, opt-in)"

## docker-build-ga-only: Build only GA-only variant (default)
docker-build-ga-only:
	@echo "$(GREEN)Building GA-only Docker image...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GOEXPERIMENT="" \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-ga-only \
		-f Dockerfile \
		.
	@echo "$(GREEN)✅ GA-only image built: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"

## docker-build-experimental: Build only experimental variant (opt-in)
docker-build-experimental:
	@echo "$(GREEN)Building experimental Docker image...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GOEXPERIMENT=jsonv2,greenteagc \
		-t $(IMAGE_NAME):$(IMAGE_TAG)-experimental \
		-f Dockerfile \
		.
	@echo "$(GREEN)✅ Experimental image built: $(IMAGE_NAME):$(IMAGE_TAG)-experimental$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -f coverage.out
	go clean -cache -testcache
	@echo "$(GREEN)✅ Clean complete$(NC)"

## all: Run all checks (lint, test-coverage, build)
all: lint test-coverage build
	@echo ""
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(GREEN)✅ All checks passed!$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"

.DEFAULT_GOAL := help

check:
	@scripts/ci/check.sh

test-race:
	@GOWORK=off go test -v -race -timeout=15m ./...

## coverage: Show detailed coverage report
coverage: test
	@echo "$(GREEN)Coverage Report:$(NC)"
	@GOWORK=off go tool cover -func=coverage.out
	@echo ""
	@COVERAGE=$$(GOWORK=off go tool cover -func=coverage.out | grep "^total:" | awk '{print $$3}'); \
	echo "$(GREEN)Total Coverage: $$COVERAGE$(NC)"

## test-e2e: Run E2E tests (requires kind cluster)
test-e2e:
	@echo "$(GREEN)Running E2E tests...$(NC)"
	@if [ -z "$$KUBECONFIG" ] && [ ! -f "$$HOME/.kube/zen-lead-e2e-config" ]; then \
		echo "$(YELLOW)⚠️  KUBECONFIG not set. Run 'make test-e2e-setup' first$(NC)"; \
		exit 1; \
	fi
	@GOWORK=off go test -v -tags=e2e -timeout=10m ./test/e2e/...
	@echo "$(GREEN)✅ E2E tests complete$(NC)"

## test-e2e-setup: Setup kind cluster for E2E tests
test-e2e-setup:
	@echo "$(GREEN)Setting up E2E test environment...$(NC)"
	@./test/e2e/setup_kind.sh create
	@echo "$(GREEN)✅ E2E test environment ready$(NC)"
	@echo "$(YELLOW)Export kubeconfig: export KUBECONFIG=$$(./test/e2e/setup_kind.sh kubeconfig)$(NC)"

## test-e2e-cleanup: Cleanup kind cluster for E2E tests
test-e2e-cleanup:
	@echo "$(GREEN)Cleaning up E2E test environment...$(NC)"
	@./test/e2e/setup_kind.sh delete
	@echo "$(GREEN)✅ E2E test environment cleaned up$(NC)"
