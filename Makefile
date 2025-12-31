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
	go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✅ Tests complete$(NC)"

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

## deploy: Deploy controller
deploy:
	@echo "$(GREEN)Deploying controller...$(NC)"
	kubectl apply -f config/rbac/
	kubectl apply -f deploy/
	@echo "$(GREEN)✅ Controller deployed$(NC)"

## undeploy: Undeploy controller
undeploy:
	@echo "$(GREEN)Undeploying controller...$(NC)"
	kubectl delete -f deploy/ --ignore-not-found=true
	kubectl delete -f config/rbac/ --ignore-not-found=true
	@echo "$(GREEN)✅ Controller undeployed$(NC)"

## run: Run controller locally
run:
	@echo "$(GREEN)Running controller locally...$(NC)"
	go run ./cmd/manager/main.go

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		-f Dockerfile \
		.
	@echo "$(GREEN)✅ Docker image built: $(IMAGE_NAME):$(IMAGE_TAG)$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -f coverage.out
	go clean -cache -testcache
	@echo "$(GREEN)✅ Clean complete$(NC)"

## all: Run all checks (lint, test, build)
all: lint test build
	@echo ""
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"
	@echo "$(GREEN)✅ All checks passed!$(NC)"
	@echo "$(GREEN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(NC)"

.DEFAULT_GOAL := help

check:
	@scripts/ci/check.sh

test-race:
	@go test -v -race -timeout=15m ./...
