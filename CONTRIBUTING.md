# Contributing to Zen-Lead

Thank you for your interest in contributing to Zen-Lead!

## Development Setup

### Prerequisites

- Go 1.24+
- Kubernetes cluster (k3d, kind, or minikube)
- kubectl
- controller-gen (for CRD generation)

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

3. **Install controller-gen:**
   ```bash
   go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
   ```

4. **Generate CRDs:**
   ```bash
   make generate
   ```

5. **Build:**
   ```bash
   make build
   ```

6. **Run tests:**
   ```bash
   make test
   ```

## Development Workflow

### Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes

3. Run tests and linters:
   ```bash
   make all
   ```

4. Commit your changes:
   ```bash
   git commit -m "feat: your feature description"
   ```

5. Push and create a PR

### Code Standards

- **Go Format:** Use `gofmt` (enforced by `make fmt`)
- **Go Vet:** Use `go vet` (enforced by `make vet`)
- **Tests:** Write tests for new features
- **Documentation:** Update docs for user-facing changes

### Project Structure

- `cmd/manager/` - Main application entry point
- `pkg/apis/` - CRD type definitions
- `pkg/controller/` - Controller logic
- `pkg/election/` - Leader election wrapper
- `pkg/pool/` - Pool management
- `config/` - RBAC and CRD manifests
- `deploy/` - Deployment manifests
- `examples/` - Example configurations
- `docs/` - Documentation

## Testing

### Unit Tests

```bash
go test ./pkg/...
```

### Integration Tests

```bash
# Requires a Kubernetes cluster
go test ./test/integration/...
```

### E2E Tests

```bash
# Requires a Kubernetes cluster
make test-e2e
```

## Adding New Features

### Adding a New CRD Field

1. Update `pkg/apis/coordination.kube-zen.io/v1alpha1/leaderpolicy_types.go`
2. Add kubebuilder markers for validation
3. Regenerate CRDs: `make generate`
4. Update controller logic if needed
5. Add tests
6. Update documentation

### Adding a New Follower Mode

1. Add enum value to `FollowerMode` field
2. Implement logic in controller
3. Add tests
4. Update documentation

## Documentation

- Update `README.md` for user-facing changes
- Update `docs/` for detailed documentation
- Add examples to `examples/`
- Update `CHANGELOG.md` for releases

## Release Process

1. Update version in `go.mod` and `CHANGELOG.md`
2. Create git tag
3. Build and push Docker image
4. Update Helm chart (if applicable)

## Questions?

- Open an issue for bugs or feature requests
- Check existing documentation in `docs/`
- Review examples in `examples/`

---

Thank you for contributing to Zen-Lead! 🚀

