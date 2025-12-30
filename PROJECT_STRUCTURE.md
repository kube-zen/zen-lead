# Zen-Lead Project Structure

```
zen-lead/
├── cmd/
│   └── manager/              # Main controller binary
│       └── main.go           # Entry point, controller setup
│
├── pkg/
│   ├── director/            # Service director (core controller)
│   │   ├── service_director.go      # Service-watching reconciler
│   │   └── strategy.go              # Leader selection strategies
│   │
│   ├── metrics/             # Prometheus metrics
│   │   ├── metrics.go       # Metrics definitions
│   │   └── metrics_test.go  # Metrics tests
│   │
│   ├── client/              # Client SDK (optional, for application integration)
│   │   └── client.go
│   │
│   └── election/            # Election utilities (reference implementation)
│       └── election.go
│
├── config/
│   └── rbac/                # RBAC manifests
│       ├── role.yaml       # ClusterRole (non-invasive minimum)
│       ├── role_binding.yaml
│       └── service_account.yaml
│
├── deploy/                  # Deployment manifests
│   ├── deployment.yaml      # Controller deployment
│   ├── prometheus/          # Prometheus alert rules
│   │   └── prometheus-rules.yaml
│   └── grafana/             # Grafana dashboard
│       └── dashboard.json
│
├── examples/                # Example configurations
│   ├── basic-service.yaml
│   ├── named-targetport.yaml
│   └── multi-port-service.yaml
│
├── docs/                    # Documentation
│   ├── ARCHITECTURE.md      # Architecture overview
│   ├── INTEGRATION.md       # Integration guide
│   ├── TROUBLESHOOTING.md   # Troubleshooting guide
│   └── USE_CASES.md         # Use case examples
│
├── test/                    # Tests
│   └── integration/         # Integration tests
│
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies
├── Makefile                 # Build automation
├── README.md                # Main documentation
└── LICENSE                  # Apache 2.0 license
```

## Key Components

### ServiceDirector (`pkg/director/service_director.go`)

**Core Controller:**
- Watches `corev1.Service` resources
- Detects `zen-lead.io/enabled: "true"` annotation
- Creates/updates selector-less leader Service
- Creates/updates EndpointSlice pointing to leader pod
- Resolves named targetPorts from pod container ports
- Handles failover when leader pod becomes NotReady

**Key Functions:**
- `Reconcile()` - Main reconciliation loop
- `selectLeaderPod()` - Leader selection (sticky + oldest Ready)
- `reconcileLeaderService()` - Create/update leader Service
- `reconcileEndpointSlice()` - Create/update EndpointSlice
- `resolveServicePorts()` - Resolve named targetPorts

### Metrics (`pkg/metrics/metrics.go`)

**Prometheus Metrics:**
- `zen_lead_leader_duration_seconds` - Leader duration
- `zen_lead_failover_count_total` - Failover count
- `zen_lead_reconciliation_duration_seconds` - Reconciliation duration
- `zen_lead_pods_available` - Ready pods count
- `zen_lead_port_resolution_failures_total` - Port resolution failures
- `zen_lead_reconciliation_errors_total` - Reconciliation errors

### Client SDK (`pkg/client/client.go`)

**Optional Feature:**
- Simple query API: "Am I the leader?"
- Uses Leases for leader status
- Caching for performance

## Build Targets

```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make docker-build   # Build Docker image
```

## Deployment

**Helm Chart:**
- `helm-charts/charts/zen-lead/` - Helm chart for deployment
- Default: namespace-scoped, non-invasive, metrics enabled

**Manifests:**
- `deploy/deployment.yaml` - Controller deployment
- `config/rbac/` - RBAC resources

## Testing

- **Unit Tests:** `pkg/*/*_test.go`
- **Integration Tests:** `test/integration/`
- **E2E Tests:** `test/e2e/` (future)

## Documentation

- **README.md** - Main documentation
- **docs/ARCHITECTURE.md** - Architecture details
- **docs/INTEGRATION.md** - Integration guide
- **docs/TROUBLESHOOTING.md** - Troubleshooting
- **docs/USE_CASES.md** - Use case examples
