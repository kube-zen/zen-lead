# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy zen-sdk first (needed for latest logging code)
# Build context should be from parent directory (zen/)
COPY zen-sdk /workspace/zen-sdk

# Ensure zen-sdk dependencies are resolved
WORKDIR /workspace/zen-sdk
RUN go mod tidy && go mod download

# Back to workspace
WORKDIR /workspace

# Copy go mod files
COPY zen-lead/go.mod zen-lead/go.sum* ./

# Download dependencies (may fail for zen-sdk if tag not available, that's OK)
RUN go mod download || true

# Add replace directive to use local zen-sdk during build
RUN go mod edit -replace github.com/kube-zen/zen-sdk=./zen-sdk

# Download dependencies with local replace (updates go.sum without removing requires)
RUN go mod download

# Copy source code
    COPY zen-lead/cmd/ cmd/
    COPY zen-lead/pkg/ pkg/
    COPY zen-lead/Makefile Makefile

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

WORKDIR /

COPY --from=builder /workspace/zen-lead .

USER 65532:65532

ENTRYPOINT ["/zen-lead"]

