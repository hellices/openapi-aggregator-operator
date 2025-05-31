# OpenAPI Aggregator Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/hellices/openapi-aggregator-operator)](https://goreportcard.com/report/github.com/hellices/openapi-aggregator-operator)
[![GitHub License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hellices/openapi-aggregator-operator)](go.mod)

Kubernetes operator that discovers and aggregates OpenAPI/Swagger specifications from services running in your cluster. It provides a unified Swagger UI interface to browse and test all your APIs in one place.

## Quick Start

### 1. Installation

```bash
# Install the operator
kubectl apply -f https://raw.githubusercontent.com/hellices/openapi-aggregator-operator/main/install.yaml

# Verify the installation
kubectl get pods -n openapi-aggregator-system
```

### 2. Configure Services

Add these annotations to your services that expose OpenAPI/Swagger specs:

```yaml
metadata:
  annotations:
    openapi.aggregator.io/swagger: "true"      # Required
    openapi.aggregator.io/path: "/v2/api-docs" # Optional (default: /v2/api-docs)
    openapi.aggregator.io/port: "8080"         # Optional (default: 8080)
```

### 3. Create Aggregator Instance

```bash
kubectl apply -f - <<EOF
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: openapi-aggregator
spec:
  labelSelector: {}  # Optional: Filter services by labels
EOF
```

### 4. Access Swagger UI

```bash
kubectl port-forward -n openapi-aggregator-system svc/openapi-aggregator-openapi-aggregator-swagger-ui 9090:9090
```

Then open http://localhost:9090 in your browser.

## Features

- 🔍 **Auto-discovery**: Automatically finds services with OpenAPI specifications using annotations
- 🔄 **Real-time Updates**: Fetches specifications in real-time and updates every 10 seconds
- 🎯 **Configurable Endpoints**: Customize OpenAPI spec paths and ports through annotations
- 🌐 **Unified UI**: Single Swagger UI interface to browse all discovered APIs
- 📝 **Service Information**: Displays service metadata including namespace and resource type
- ⚡ **Zero-config Services**: Works with any service that exposes an OpenAPI/Swagger specification

## Project Architecture

### Components

1. **Controller**: 
   - Watches for services with OpenAPI annotations
   - Collects service metadata and OpenAPI spec URLs
   - Updates status every 10 seconds

2. **Swagger UI Server**: 
   - Serves unified Swagger UI interface
   - Fetches OpenAPI specs in real-time
   - Provides API selection and documentation

### Project Structure

```
.
├── api/             # API definitions and generated code
├── cmd/             # Main application entry point
├── config/          # Kubernetes manifests and kustomize configs
├── internal/        # Internal packages
│   └── controller/  # Operator controller logic
└── pkg/            
    ├── swagger/     # Swagger UI server implementation
    └── version/     # Version information
```

## Development Guide

### Prerequisites

- Go 1.22+
- Kubernetes 1.24+
- kubectl
- kustomize
- controller-gen

### Installation Methods

#### 1. Quick Install (For Users)

```bash
kubectl apply -f https://raw.githubusercontent.com/hellices/openapi-aggregator-operator/main/install.yaml
```

#### 2. Development Install

```bash
# Clone and build
git clone https://github.com/hellices/openapi-aggregator-operator
cd openapi-aggregator-operator
make install
make deploy
```

#### 3. OLM Install (Advanced)

For installation via Operator Lifecycle Manager, see detailed instructions in [OLM Installation Guide](docs/olm-install.md).

### Development Environment Setup

1. Deploy the test service:
```bash
# First, deploy the test service that provides OpenAPI specs
kubectl apply -f config/samples/test-service.yaml

# Port forward the test service to localhost:8080
kubectl port-forward svc/test-service 8080:8080
```

2. Run the operator in development mode:
```bash
# Run locally
make run

# Run tests
make test

# Build and push image
make docker-build docker-push

# Generate manifests
make manifests
```

Note: When running the operator in development mode with `make run`, ensure that the test service is running and port-forwarded to localhost:8080. This is required for the operator to properly fetch and display the OpenAPI specifications in the Swagger UI.

### Version Management

- Version is managed in `versions.txt`
- Used for Docker images, releases, and binary info
- Format: `ghcr.io/hellices/openapi-aggregator-operator:<version>`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
