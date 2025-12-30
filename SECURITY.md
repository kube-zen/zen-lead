# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it to: security@kube-zen.io

**Please do not** report security vulnerabilities through public GitHub issues.

## Security Considerations

### RBAC Permissions

Zen-Lead requires the following permissions:

- **LeaderPolicy CRDs**: Full CRUD access
- **Lease Resources**: Full CRUD access (coordination.k8s.io)
- **Pods**: Read and update (for role annotations)

### Least Privilege

- Controller runs as non-root user (UID 65532)
- Read-only filesystem (where possible)
- Minimal required permissions

### Network Security

- No external network dependencies
- All communication via Kubernetes API server
- Uses mTLS for API server communication (Kubernetes default)

### Data Security

- No secrets stored in CRDs
- Pod annotations are visible to all users with pod read access
- Lease resources contain pod identities (not sensitive)

## Security Best Practices

1. **RBAC:** Use least-privilege RBAC
2. **Network Policies:** Restrict pod-to-pod communication
3. **Pod Security:** Use PodSecurity standards
4. **Secrets:** Never store secrets in annotations
5. **Monitoring:** Monitor for unauthorized access

## Known Limitations

- Pod annotations are visible to all users with pod read access
- Leader identity is stored in Lease resource (visible to all users)
- No encryption at rest for CRDs (uses etcd encryption if enabled)

## Security Updates

Security updates will be released as patch versions (e.g., 0.1.1).

---

**Last Updated:** 2025-01-XX

