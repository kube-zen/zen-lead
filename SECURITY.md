# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Security Model

Zen-Lead follows Kubernetes security best practices:

### Non-Invasive Design

- **No Pod Mutation:** Controller never patches or updates pods
- **Read-Only Pod Access:** Controller only reads pod status
- **Least-Privilege RBAC:** Minimal permissions required

### RBAC Permissions

**Day-0 Permissions (default):**
- `pods`: `get`, `list`, `watch` (read-only)
- `services`: `get`, `list`, `watch`, `create`, `update`, `patch`, `delete`
- `endpointslices`: `get`, `list`, `watch`, `create`, `update`, `patch`, `delete`
- `events`: `create`, `patch`

**No Permissions For:**
- `pods/patch` or `pods/update` (no pod mutation)
- `coordination.kube-zen.io/leaderpolicies` (not used - CRD-free)

**Required Permissions:**
- `coordination.k8s.io/leases` (required for controller-runtime leader election)

### Container Security

- **Non-Root Execution:** Runs as UID 65534 (nobody)
- **Read-Only Root Filesystem:** Enabled by default
- **No Privilege Escalation:** `allowPrivilegeEscalation: false`
- **Dropped Capabilities:** All capabilities dropped
- **Seccomp Profile:** RuntimeDefault

### Resource Isolation

- **Ownership:** All generated resources owned by source Service
- **Labels:** Clear labeling for identification (`app.kubernetes.io/managed-by: zen-lead`)
- **Garbage Collection:** Automatic cleanup via owner references

## Reporting a Vulnerability

If you discover a security vulnerability, please report it to: security@kube-zen.io

**Do not** open a public GitHub issue for security vulnerabilities.

## Security Considerations

### Network-Level Only

Zen-Lead provides network-level single-active routing. It does NOT:
- Guarantee application-level correctness
- Provide distributed consensus
- Handle application state coordination
- Prevent split-brain at application level

**Use Case:** Suitable for stateless applications or applications that handle their own state coordination.

### Failover Security

Failover is bounded by:
- Pod readiness transition latency
- Controller reconciliation latency (~1-2 seconds)
- kube-proxy EndpointSlice update latency (~1-2 seconds)

**Total:** Typically 2-5 seconds for complete failover.

During failover, there may be a brief period where no leader is selected (clean failure mode).
