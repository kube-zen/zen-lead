# Contributing to Zen-Lead

Thank you for your interest in contributing to Zen-Lead!

## Development Setup

### Prerequisites

- Go 1.24+
- Kubernetes cluster (k3d, kind, or minikube)
- kubectl
- Helm 3.0+ (for testing)

### Getting Started

1. **Clone the repository:**
   ```bash
   git clone https://github.com/kube-zen/zen-lead
   cd zen-lead
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Build:**
   ```bash
   make build
   ```

4. **Run tests:**
   ```bash
   make test
   ```

## Development Workflow

### Making Changes

1. Create a feature branch from `main`
2. Make your changes
3. Run tests: `make test`
4. Run linter: `make lint`
5. Build: `make build`
6. Submit a pull request

### Code Style

- Follow Go standard formatting (`go fmt`)
- Use `golangci-lint` for linting
- Write tests for new functionality
- Update documentation for user-facing changes

### Testing

- Unit tests: `go test ./pkg/...`
- Integration tests: `go test ./test/integration/...`
- E2E tests: See `test/e2e/` directory

## Architecture

Zen-Lead uses a **Service-annotation opt-in** approach:

- Services opt-in via `zen-lead.io/enabled: "true"` annotation
- Controller creates selector-less `<service>-leader` Service
- Controller creates EndpointSlice pointing to leader pod
- No CRDs required, no pod mutation

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture information.

## Project Structure

```
zen-lead/
├── cmd/
│   └── manager/              # Main controller binary
│       └── main.go           # Entry point
│
├── pkg/
│   ├── director/            # Service director (core controller)
│   │   ├── service_director.go
│   │   └── strategy.go
│   │
│   ├── metrics/             # Prometheus metrics
│   │   └── metrics.go
│   │
│   │   └── zenlead_validator.go
│   │
│   └── client/              # Client SDK (optional)
│       └── client.go
│
├── config/
│   └── rbac/                # RBAC manifests
│
├── deploy/                  # Deployment manifests
│   ├── prometheus/          # Prometheus alert rules
│   └── grafana/             # Grafana dashboard
│
├── examples/                # Example configurations
│
├── docs/                    # Documentation
│
└── test/                    # Tests
```

## Key Principles

1. **Non-Invasive:** No pod mutation, no changes to user resources
2. **Service-First:** Opt-in via Service annotation
3. **Zero CRDs:** No CRDs required for day-0 usage
4. **Least-Privilege:** Minimal RBAC permissions

## Pull Request Process

1. Update documentation if needed
2. Add tests for new functionality
3. Ensure all tests pass
4. Update CHANGELOG.md if applicable
5. Submit PR with clear description

## Questions?

- Check [README.md](README.md) for overview
- See [docs/](docs/) for detailed documentation
- Open an issue for questions or discussions
