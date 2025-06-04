FROM golang:1.21 as builder

WORKDIR /workspace
# Copy the Go Module files and download dependencies
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the source code
COPY pkg/swagger/ pkg/swagger/
COPY api/ api/

# Build the swagger server
RUN CGO_ENABLED=0 GOOS=linux go build -o swagger-server pkg/swagger/cmd/main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/swagger-server .
USER 65532:65532

ENTRYPOINT ["/swagger-server"]
