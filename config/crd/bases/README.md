# CRD Bases

This directory would contain generated CRD manifests if CRDs were used.

## Current Status

**Zen-Lead does NOT use CRDs for day-0 functionality.**

Zen-Lead uses a **Service-annotation opt-in** approach:
- Services opt-in via `zen-lead.io/enabled: "true"` annotation
- No CRDs required
- No pod mutation
- Non-invasive design

## Historical Note

Previous versions of zen-lead used a `LeaderPolicy` CRD, but this has been removed in favor of the Service-annotation approach for better community adoption and non-invasive operation.

## Future CRDs

Future versions may introduce optional CRDs for advanced features, but day-0 functionality will always remain CRD-free.
