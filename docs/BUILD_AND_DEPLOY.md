# Build and Deployment Guide

## Default Build Configuration

**As of 2025-01-02:** Default images are GA-only (no experimental features). Experimental features are opt-in.

**Rationale:** GA-only is the safe default. Experimental features provide 15-25% performance improvement but are opt-in.

## Building Images

### Default Build (GA-Only)

```bash
# Standard build - GA-only (default)
make docker-build

# Or directly
docker build -t kubezen/zen-lead:latest .
```

**Result:** GA-only image (no experimental features).

### Build With Experimental Features (Opt-In)

To build with experimental features for better performance:

```bash
# Build experimental image
make docker-build-experimental

# Or directly
docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc -t kubezen/zen-lead:experimental .
```

**Use Case:** Performance-critical deployments where you want to opt-in to 15-25% performance improvement.

## Helm Chart Configuration

### Choose Image Variant at Deployment Time

**Key:** You choose which image variant to use at deployment/startup time via Helm values. Both variants must be pre-built.

### Default Deployment (Experimental Features)

```yaml
# values.yaml (default)
image:
  repository: kubezen/zen-lead
  tag: "0.1.0"  # Base tag
  variant: "experimental"  # Uses <tag>-experimental image

experimental:
  jsonv2:
    enabled: true  # Informational
  greenteagc:
    enabled: true  # Informational
```

**Deploy:**
```bash
helm install zen-lead ./helm-charts/charts/zen-lead
```

**Result:** Uses `kubezen/zen-lead:0.1.0-experimental` (or `latest` if tag is `latest`)

### Using GA-Only Variant

Choose GA-only at deployment time:

```yaml
# values.yaml
image:
  repository: kubezen/zen-lead
  tag: "0.1.0"  # Base tag
  variant: "ga-only"  # Uses <tag>-ga-only image

experimental:
  jsonv2:
    enabled: false  # Informational
  greenteagc:
    enabled: false  # Informational
```

**Deploy:**
```bash
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.variant=ga-only
```

**Result:** Uses `kubezen/zen-lead:0.1.0-ga-only`

### Quick Selection Examples

```bash
# Use experimental (default, better performance)
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.variant=experimental

# Use GA-only (conservative)
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.variant=ga-only
```

## Image Variants

Both variants are built by default with `make docker-build`:

| Variant | Image Tag Pattern | GOEXPERIMENT | Performance | Stability | Use Case |
|---------|-------------------|--------------|-------------|-----------|----------|
| `ga-only` (default) | `<tag>-ga-only` or `latest` | (none) | Baseline | ✅ Stable | Recommended (default) |
| `experimental` | `<tag>-experimental` | `jsonv2,greenteagc` | ✅ 15-25% better | ✅ Stable | Opt-in for performance |

**Selection:** Choose via `image.variant` in Helm values at deployment time.

## Performance Comparison

Based on integration tests:

| Metric | GA-Only | With Experimental | Improvement |
|--------|---------|-------------------|-------------|
| Reconciliation Latency | Baseline | -15-20% | ✅ Significant |
| Failover Latency | Baseline | -5-15% | ✅ Moderate |
| API Call Latency | Baseline | -15-25% | ✅ Significant |
| GC Pause Times | Baseline | -10-40% | ✅ Significant |

## Recommendation

**Use default images (with experimental features)** unless you have specific requirements for GA-only builds.

The experimental features have shown:
- ✅ Performance improvements
- ✅ No stability issues
- ✅ Safe for production use

## Migration Guide

### Switching Between Variants

**Both variants must be pre-built.** Then switch at deployment time:

#### From GA-Only to Experimental

1. **Ensure both variants are built:**
   ```bash
   make docker-build  # Builds both variants
   ```

2. **Update Helm values:**
   ```yaml
   image:
     tag: "0.1.0"
     variant: "experimental"  # Switch to experimental
   ```

3. **Deploy:**
   ```bash
   helm upgrade zen-lead ./helm-charts/charts/zen-lead \
     --set image.variant=experimental
   ```

4. **Monitor:** Watch for performance improvements and verify stability.

#### From Experimental to GA-Only

1. **Ensure both variants are built:**
   ```bash
   make docker-build  # Builds both variants
   ```

2. **Update Helm values:**
   ```yaml
   image:
     tag: "0.1.0"
     variant: "ga-only"  # Switch to GA-only
   ```

3. **Deploy:**
   ```bash
   helm upgrade zen-lead ./helm-charts/charts/zen-lead \
     --set image.variant=ga-only
   ```

## CI/CD Integration

### GitHub Actions

Default builds will include experimental features. To build GA-only in CI:

```yaml
- name: Build GA-only image
  run: |
    docker build --build-arg GOEXPERIMENT="" \
      -t kubezen/zen-lead:${{ github.ref_name }}-ga-only .
```

### Custom Build Scripts

Update your build scripts to use experimental features by default:

```bash
#!/bin/bash
# Build with experimental features (default)
docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc \
  -t kubezen/zen-lead:latest .
```

## Verification

### Check if Image Includes Experimental Features

```bash
# Run container and check env var
docker run --rm kubezen/zen-lead:latest env | grep GOEXPERIMENT_INFO

# Should show: GOEXPERIMENT_INFO=jsonv2,greenteagc
```

### Monitor Performance

After deploying, monitor these metrics:
- `zen_lead_reconciliation_duration_seconds` - Should be lower
- `zen_lead_failover_latency_seconds` - Should be lower
- Error rates - Should be same or better

## Troubleshooting

### Performance Not Improved

**Cause:** Image may not include experimental features

**Solution:**
1. Verify image was built with GOEXPERIMENT: `docker inspect <image>`
2. Check GOEXPERIMENT_INFO env var in pod
3. Rebuild image with experimental features

### Want to Disable Experimental Features

**Solution:** Build GA-only image and use that tag in Helm values.

## References

- [Experimental Features Guide](EXPERIMENTAL_FEATURES.md)
- [Performance Comparison](EXPERIMENTAL_FEATURES_RECOMMENDATION.md)
- [Integration Tests](EXPERIMENTAL_TESTING_GUIDE.md)

