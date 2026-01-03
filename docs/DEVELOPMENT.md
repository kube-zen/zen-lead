# Development Guide

This guide covers development setup, workflows, and best practices for {{ .projectName }}.

## Prerequisites

- Go 1.25 (see [Go Toolchain](#go-toolchain) section)
- kubectl configured to access a Kubernetes cluster
- Docker (for building images)
- Make

## Installation

```bash
git clone https://github.com/kube-zen/{{ .projectName }}.git
cd {{ .projectName }}
go mod download
```

## Quick Start

```bash
# Run all checks
make check

# Run tests
go test ./...

# Build
go build ./cmd/{{ .projectName }}
```

## Development Workflow

1. Create a feature branch from `main`
2. Make your changes
3. Run `make check` to ensure all checks pass
4. Commit and push your changes
5. Open a pull request

## Testing

```bash
# Run unit tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run specific test
go test -v ./pkg/...
```

## Building

```bash
# Build binary
make build

# Build Docker image
make build-image
```

## Code Standards

- Follow Go best practices
- Run `go fmt` before committing
- Ensure all tests pass
- Add tests for new functionality

## Go Toolchain (S133)

### Version

- **Go 1.25** is the standard for zen-lead
- Specified in `go.mod`: `go 1.25.0`
- Toolchain directive: Either use `toolchain go1.25.0` everywhere or nowhere (OSS consistency)

### go.mod Requirements

- Run `go mod tidy` regularly
- No `replace` directives in main branch (unless explicitly required for local dev)
- Pin dependency versions (no pseudo-versions in production)

### Standard Commands

```bash
# Test
go test ./...

# Test with race detector
go test -race ./...

# Build
go build ./...

# Format
gofmt -s -w .
goimports -w .

# Lint
golangci-lint run
```

## Local Development Overrides

**Note**: Currently, `zen-lead` uses a `replace` directive for `zen-sdk` because `zen-sdk` has not yet been published to a public Go module repository. This is a temporary measure until `zen-sdk` is published with proper version tags.

Once `zen-sdk` is published, the `replace` directive will be removed and `zen-lead` will use a proper version reference (e.g., `github.com/kube-zen/zen-sdk v0.1.2-alpha`).

For local development with other dependencies, you can use `go.work` (Go workspaces) or temporary `replace` directives in your local `go.mod`. **Do not commit `replace` directives for dependencies that are available in public repositories** - they make builds non-reproducible.

Example (local only, do not commit):
```bash
# In go.mod (local only)
replace github.com/kube-zen/zen-sdk => ../zen-sdk
```

For production builds, all dependencies must be resolved from public repositories with tagged versions.

## Go 1.25 Features

zen-lead uses Go 1.25 and benefits from several GA features:

### Automatic Features (Already Enabled)

1. **Container-Aware GOMAXPROCS** ✅
   - Automatically adjusts CPU utilization based on container limits
   - No configuration needed
   - Monitor CPU utilization metrics to verify optimization

2. **DWARF5 Debug Information** ✅
   - Reduced debug information size
   - Faster linking time
   - Enabled by default in Go 1.25

3. **Performance Optimizations** ✅
   - Faster map operations (cache lookups)
   - Improved slice operations (pod/service lists)
   - Better compiler optimizations
   - Automatic benefits, no code changes required

4. **Improved Error Handling** ✅
   - Better error wrapping and context
   - Improved error messages
   - Automatic benefits

### Available Features (Consider During Refactoring)

1. **WaitGroup.Go Method** ⚠️
   - Simplifies goroutine management
   - Consider when refactoring concurrent operations
   - Use for parallel cache updates or parallel API calls
   - Low priority - consider during refactoring

**Example:**
```go
// Before (sequential)
for _, ns := range namespaces {
    r.updateOptedInServicesCache(ctx, ns, logger)
}

// After (parallel with WaitGroup.Go)
var wg sync.WaitGroup
for _, ns := range namespaces {
    wg.Go(func() {
        r.updateOptedInServicesCache(ctx, ns, logger)
    })
}
wg.Wait()
```

### Experimental Features (Opt-In)

Experimental Go 1.25 features (JSON v2, Green Tea GC) are available as opt-in for better performance. See [EXPERIMENTAL_FEATURES.md](EXPERIMENTAL_FEATURES.md) for details.

**Status:**
- ✅ Performance improvements observed (15-25%)
- ✅ No stability regressions
- ⚠️ Experimental - not production-ready
- ✅ Safe for staging/testing environments

## Release Process

See [RELEASE.md](RELEASE.md) for the release process.

