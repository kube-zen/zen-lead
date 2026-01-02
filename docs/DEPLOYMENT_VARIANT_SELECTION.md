# Image Variant Selection at Deployment Time

## Overview

zen-lead supports two image variants that you can choose **at deployment/startup time** via Helm values:

- **`ga-only`** (default): Built without experimental features
  - Baseline performance
  - Conservative choice, recommended for production

- **`experimental`**: Includes Go 1.25 experimental features (JSON v2, Green Tea GC)
  - 15-25% performance improvement
  - No stability issues observed
  - Opt-in for performance-critical deployments

## How It Works

**Important:** GOEXPERIMENT is compile-time, not runtime. You must use pre-built image variants.

1. **Build both variants:**
   ```bash
   make docker-build
   ```
   This creates:
   - `kubezen/zen-lead:<tag>-experimental` (or `latest` if tag is `latest`)
   - `kubezen/zen-lead:<tag>-ga-only`

2. **Choose variant at deployment time:**
   ```bash
   helm install zen-lead ./helm-charts/charts/zen-lead \
     --set image.tag=0.1.0 \
     --set image.variant=experimental  # or "ga-only"
   ```

3. **Helm automatically selects the correct image tag:**
   - `variant=experimental` → uses `<tag>-experimental` (or `latest`)
   - `variant=ga-only` → uses `<tag>-ga-only`

## Usage Examples

### Deploy with GA-Only (Default)

```bash
# Using default (ga-only)
helm install zen-lead ./helm-charts/charts/zen-lead

# Explicitly set GA-only
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.tag=0.1.0 \
  --set image.variant=ga-only
```

**Result:** Uses `kubezen/zen-lead:0.1.0-ga-only` (or `latest` if tag is `latest`)

### Deploy with Experimental Features (Opt-In)

```bash
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.tag=0.1.0 \
  --set image.variant=experimental
```

**Result:** Uses `kubezen/zen-lead:0.1.0-experimental`

### Using Latest Tag

```bash
# GA-only (uses "latest" tag directly, default)
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.tag=latest \
  --set image.variant=ga-only

# Experimental (uses "latest-experimental" tag)
helm install zen-lead ./helm-charts/charts/zen-lead \
  --set image.tag=latest \
  --set image.variant=experimental
```

## Helm Values

```yaml
image:
  repository: kubezen/zen-lead
  tag: "0.1.0"  # Base tag
  variant: "ga-only"  # "ga-only" (default) or "experimental"
  pullPolicy: IfNotPresent
```

## Image Tag Mapping

| Variant | Base Tag | Resulting Image Tag |
|---------|----------|---------------------|
| `ga-only` (default) | `0.1.0` | `kubezen/zen-lead:0.1.0-ga-only` |
| `ga-only` (default) | `latest` | `kubezen/zen-lead:latest` |
| `experimental` | `0.1.0` | `kubezen/zen-lead:0.1.0-experimental` |
| `experimental` | `latest` | `kubezen/zen-lead:latest-experimental` |

## Switching Variants

To switch between variants after deployment:

```bash
# Switch to experimental
helm upgrade zen-lead ./helm-charts/charts/zen-lead \
  --set image.variant=experimental

# Switch to GA-only
helm upgrade zen-lead ./helm-charts/charts/zen-lead \
  --set image.variant=ga-only
```

**Note:** Both image variants must be pre-built and available in your registry.

## Verification

### Check Which Variant is Running

```bash
# Check image tag in pod
kubectl get pod -n <namespace> -l app.kubernetes.io/name=zen-lead \
  -o jsonpath='{.items[0].spec.containers[0].image}'

# Check GOEXPERIMENT_INFO env var
kubectl exec -n <namespace> <pod-name> -- env | grep GOEXPERIMENT_INFO
```

**Expected values:**
- Experimental: `GOEXPERIMENT_INFO=jsonv2,greenteagc`
- GA-only: `GOEXPERIMENT_INFO=`

## Performance Comparison

| Metric | GA-Only | Experimental | Improvement |
|--------|---------|--------------|-------------|
| Reconciliation Latency | Baseline | -15-20% | ✅ Significant |
| Failover Latency | Baseline | -5-15% | ✅ Moderate |
| API Call Latency | Baseline | -15-25% | ✅ Significant |
| GC Pause Times | Baseline | -10-40% | ✅ Significant |

## Recommendation

**Use `ga-only` variant (default)** for production deployments. Use `experimental` variant for performance-critical deployments where you want to opt-in to the 15-25% performance improvement.

The experimental features have shown:
- ✅ Performance improvements (15-25%)
- ✅ No stability issues observed
- ✅ Safe for production use (but opt-in, not default)

## Troubleshooting

### Image Not Found

**Error:** `ImagePullBackOff` or `ErrImagePull`

**Cause:** Image variant not built or not pushed to registry

**Solution:**
1. Build both variants: `make docker-build`
2. Push both variants to registry
3. Verify tags exist: `docker images | grep zen-lead`

### Wrong Variant Running

**Symptom:** Performance not improved or GOEXPERIMENT_INFO doesn't match

**Solution:**
1. Check Helm values: `helm get values zen-lead`
2. Verify image tag in pod matches expected variant
3. Redeploy with correct variant setting

## References

- [Build and Deploy Guide](BUILD_AND_DEPLOY.md)
- [Experimental Features Guide](EXPERIMENTAL_FEATURES.md)
- [Performance Comparison](EXPERIMENTAL_FEATURES_RECOMMENDATION.md)

