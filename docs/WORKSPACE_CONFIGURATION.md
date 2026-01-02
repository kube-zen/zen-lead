# Workspace Configuration

## Issue

When running tests from the parent directory (`/home/neves/zen`), Go picks up the workspace configuration (`go.work`) which can cause conflicts:

```
go: conflicting replacements for github.com/kube-zen/shared:
	/home/neves/zen/zen-platform/src/shared
	/home/neves/zen/zen-platform/src/cluster/shared
```

## Solution

**zen-lead is independent** and should only use `zen-sdk` via GitHub URL (no local replacements).

### Running Tests

To run tests independently without workspace conflicts, use:

```bash
cd zen-lead
GOWORK=off go test ./...
```

Or set the environment variable:

```bash
export GOWORK=off
cd zen-lead
go test ./...
```

### CI/CD

In CI/CD pipelines, ensure `GOWORK=off` is set when building/testing zen-lead:

```bash
GOWORK=off go test ./... -cover
GOWORK=off go build ./...
```

### Verification

zen-lead's `go.mod` correctly uses zen-sdk via GitHub:

```go
require (
    github.com/kube-zen/zen-sdk v0.2.7-alpha
    // ... other dependencies
)
```

No `replace` directives are used, ensuring zen-lead is fully independent.

## Current Status

✅ **zen-lead is independent** - uses zen-sdk via GitHub URL only  
✅ **Tests pass** - with `GOWORK=off`  
✅ **Coverage**: 70.9% overall, 71.0% for pkg/director

