# Build stage
FROM golang:1.25-alpine AS builder

# Security: Run as non-root user in builder stage
RUN addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S nonroot -G nonroot

WORKDIR /workspace

# Copy go mod files
COPY zen-lead/go.mod zen-lead/go.sum* ./

# Copy zen-sdk (needed for latest HTTP client changes)
COPY zen-sdk/ ./zen-sdk/

# Download dependencies (with zen-sdk replace directive)
RUN go mod edit -replace github.com/kube-zen/zen-sdk=./zen-sdk && \
    go mod download

# Copy source code
COPY zen-lead/cmd/ cmd/
COPY zen-lead/pkg/ pkg/
COPY zen-lead/Makefile Makefile

# Change ownership to non-root user
RUN chown -R nonroot:nonroot /workspace

# Switch to non-root user for build
USER nonroot:nonroot

# Build
# Default: GA-only (no experimental features)
# To enable experimental features: docker build --build-arg GOEXPERIMENT=jsonv2,greenteagc
# Available experiments: jsonv2, greenteagc
# Experimental features provide 15-25% performance improvement but are opt-in
ARG GOEXPERIMENT=""
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    GOEXPERIMENT=${GOEXPERIMENT} \
    go build \
    -ldflags="-w -s" \
    -trimpath \
    -o zen-lead \
    ./cmd/manager

# Final stage
FROM gcr.io/distroless/static:nonroot

# Security: Explicitly set user (distroless already uses nonroot, but explicit is better)
USER 65532:65532

WORKDIR /

COPY --from=builder --chown=65532:65532 /workspace/zen-lead .

ENTRYPOINT ["/zen-lead"]

