# API Domain Migration: coordination.zen.io → coordination.kube-zen.io

**Date:** 2025-01-XX  
**Reason:** Use owned domain `kube-zen.io` for all APIs

## Migration Summary

All API references have been updated from `coordination.zen.io` to `coordination.kube-zen.io`.

## Changes Made

### Code Files Updated

1. **API Definitions:**
   - `pkg/apis/coordination.kube-zen.io/v1alpha1/groupversion_info.go`
   - `pkg/apis/coordination.kube-zen.io/v1alpha1/leaderpolicy_types.go`

2. **Controller:**
   - `pkg/controller/leaderpolicy_controller.go`
   - `pkg/controller/leaderpolicy_controller_test.go`

3. **Main Application:**
   - `cmd/manager/main.go`

4. **Tests:**
   - `test/integration/leader_election_test.go`

### Configuration Files Updated

1. **RBAC:**
   - `config/rbac/role.yaml`

2. **Examples:**
   - `examples/leaderpolicy.yaml`

### Documentation Updated

- `README.md`
- `docs/API.md`
- `docs/INTEGRATION.md`
- `docs/USE_CASES.md`
- `docs/ARCHITECTURE.md`
- `QUICKSTART.md`
- `CONTRIBUTING.md`
- `PROJECT_STRUCTURE.md`
- `PROJECT_SUMMARY.md`
- `PROJECT_STATUS.md`
- `COMPLETION_SUMMARY.md`
- `config/crd/bases/README.md`

## New API Group

**Old:** `coordination.zen.io/v1alpha1`  
**New:** `coordination.kube-zen.io/v1alpha1`

## CRD Resource Name

**Old:** `leaderpolicies.coordination.zen.io`  
**New:** `leaderpolicies.coordination.kube-zen.io`

## Verification

All references have been updated. To verify:

```bash
# Check for any remaining old references
grep -r "coordination.zen.io" . --exclude-dir=.git

# Should return no results (except this file)
```

## Impact

- ✅ **No breaking changes** - This is a new project (0.1.0-alpha)
- ✅ **All code updated** - Go imports, RBAC, examples
- ✅ **All docs updated** - Documentation reflects new domain
- ✅ **Committed and pushed** - Changes are in git

## Next Steps

1. Generate CRDs with new API group: `make generate`
2. Install CRDs: `make install`
3. Verify CRD: `kubectl get crd leaderpolicies.coordination.kube-zen.io`

---

**Migration Complete:** ✅  
**Status:** All APIs now use `kube-zen.io` domain

