# Versioning Strategy

Zen-Lead follows [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`

## Version Components

- **MAJOR**: Breaking API changes (CRD schema changes, incompatible changes)
- **MINOR**: New features, backward-compatible (new CRD fields, new features)
- **PATCH**: Bug fixes, backward-compatible (bug fixes, documentation)

## Current Version

**0.1.0-alpha**

- **0**: Initial development
- **1**: First feature-complete release
- **0**: Patch version
- **alpha**: Pre-release status

## Version Lifecycle

### Alpha (0.1.0-alpha)
- Initial development
- APIs may change
- Not production-ready
- For testing and evaluation

### Beta (0.1.0-beta)
- Feature-complete
- APIs stable
- Testing in production-like environments
- May have bugs

### Stable (0.1.0)
- Production-ready
- APIs stable
- Well-tested
- Recommended for production

## CRD Versioning

### API Version Strategy

- **v1alpha1**: Initial API version (current)
- **v1beta1**: Beta API version (future)
- **v1**: Stable API version (future)

### Migration Path

When moving from v1alpha1 to v1:
1. Support both versions during transition
2. Provide migration guide
3. Deprecate v1alpha1 with notice period
4. Remove v1alpha1 after deprecation period

## Image Tagging

- **Version tags**: `kubezen/zen-lead:0.1.0-alpha`
- **Latest tag**: `kubezen/zen-lead:latest` (points to latest stable)
- **Commit tags**: `kubezen/zen-lead:0.1.0-alpha-<commit-sha>`

## Helm Chart Versioning

Helm chart versions sync with application versions:
- Chart version: `0.1.0-alpha`
- App version: `0.1.0-alpha`

---

**See [CHANGELOG.md](CHANGELOG.md) for version history.**

