# Build the manager binary
FROM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/controller/ internal/controller/
COPY pkg/ pkg/

# Build arguments for version info
ARG VERSION=unknown
ARG BUILD_DATE

# Build architecture-specific binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags="-s -w -X github.com/hellices/openapi-aggregator-operator/pkg/version.version=${VERSION} -X github.com/hellices/openapi-aggregator-operator/pkg/version.buildDate=${BUILD_DATE}" \
    -o manager_${TARGETARCH} cmd/main.go

# Get CA certificates from alpine for secure communication
FROM alpine:3.21 AS certificates
RUN apk --no-cache add ca-certificates

# Create minimal runtime image 
FROM scratch

ARG TARGETARCH

WORKDIR /

# Copy the certificates
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the architecture-specific binary
COPY --from=builder /workspace/manager_${TARGETARCH} manager

# Use non-root user
USER 65532:65532

ENTRYPOINT ["/manager"]
