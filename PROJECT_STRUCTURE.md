# Zen-Lead Project Structure

```
zen-lead/
├── cmd/
│   └── manager/              # Main controller binary
│       └── main.go           # Entry point
│
├── pkg/
│   ├── apis/                 # CRD API definitions
│   │   └── coordination.kube-zen.io/
│   │       └── v1alpha1/
│   │           ├── groupversion_info.go
│   │           └── leaderpolicy_types.go
│   │
│   ├── controller/           # Controller logic
│   │   ├── leaderpolicy_controller.go
│   │   └── pod_event_handler.go
│   │
│   ├── election/             # Leader election wrapper
│   │   └── election.go
│   │
│   └── pool/                 # Pool management
│       └── pool.go
│
├── config/
│   ├── crd/
│   │   └── bases/            # Generated CRD manifests
│   └── rbac/                 # RBAC manifests
│
├── deploy/                   # Deployment manifests
│
├── examples/                 # Example configurations
│   ├── leaderpolicy.yaml
│   ├── deployment-with-pool.yaml
│   └── cronjob-with-pool.yaml
│
├── go.mod                    # Go module definition
├── go.sum                    # Go dependencies
├── Makefile                  # Build automation
├── README.md                 # Main documentation
└── LICENSE                   # Apache 2.0 license
```

## Key Components

### CRD API (`pkg/apis/coordination.kube-zen.io/v1alpha1/`)

- **LeaderPolicy**: Defines a pool of candidates and election configuration
- **LeaderPolicySpec**: Configuration (lease duration, identity strategy, follower mode)
- **LeaderPolicyStatus**: Current state (phase, leader, candidates)

### Controller (`pkg/controller/`)

- **LeaderPolicyReconciler**: Main reconciliation logic
  - Watches LeaderPolicy resources
  - Finds candidates (pods with annotations)
  - Monitors Lease resources
  - Updates pod role annotations
  - Updates LeaderPolicy status

- **PodEventHandler**: Handles pod events
  - Triggers reconciliation when pods with pool annotations change

### Election (`pkg/election/`)

- **Election**: Wrapper around client-go leaderelection
  - Manages lease acquisition
  - Handles callbacks (onStarted, onStopped)
  - Provides IsLeader() check

### Pool (`pkg/pool/`)

- **Manager**: Manages pools of candidates
  - Finds pods participating in a pool
  - Updates pod role annotations
  - Helper functions for annotation management

## Design Decisions

1. **Uses controller-runtime**: Standard Kubernetes operator pattern
2. **Annotation-based**: Pods join pools via annotations (no code changes needed)
3. **Lease-based**: Uses Kubernetes Lease API (coordination.k8s.io)
4. **Status-driven**: LeaderPolicy status shows current leader and candidates

