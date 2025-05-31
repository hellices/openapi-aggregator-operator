# OpenAPI Aggregator Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/hellices/openapi-aggregator-operator)](https://goreportcard.com/report/github.com/hellices/openapi-aggregator-operator)
[![GitHub License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hellices/openapi-aggregator-operator)](go.mod)
[![Docker Pulls](https://img.shields.io/docker/pulls/hellices/openapi-aggregator-operator)](https://hub.docker.com/r/hellices/openapi-aggregator-operator)
[![Release](https://img.shields.io/github/v/release/hellices/openapi-aggregator-operator)](https://github.com/hellices/openapi-aggregator-operator/releases)

Kubernetes operator that discovers and aggregates OpenAPI/Swagger specifications from services running in your cluster. It provides a unified Swagger UI interface to browse and test all your APIs in one place.

## Features

- 🔍 **Auto-discovery**: Automatically finds services with OpenAPI specifications using annotations
- 🔄 **Real-time Updates**: Fetches specifications in real-time and updates every 10 seconds
- 🎯 **Configurable Endpoints**: Customize OpenAPI spec paths and ports through annotations
- 🌐 **Unified UI**: Single Swagger UI interface to browse all discovered APIs
- 📝 **Service Information**: Displays service metadata including namespace and resource type
- ⚡ **Zero-config Services**: Works with any service that exposes an OpenAPI/Swagger specification

## Installation

```bash
# Clone the repository
git clone https://github.com/hellices/openapi-aggregator-operator
cd openapi-aggregator-operator

# Install the CRD
make install

# Deploy the operator
make deploy
```

## Usage

### 1. Add OpenAPI Annotations to Your Services

Add the following annotations to your Kubernetes services that expose OpenAPI/Swagger specifications:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: example-service
  annotations:
    openapi.aggregator.io/swagger: "true"                 # Required: Enable OpenAPI aggregation
    openapi.aggregator.io/path: "/v2/api-docs"           # Optional: Custom path to OpenAPI spec (default: /v2/api-docs)
    openapi.aggregator.io/port: "8080"                   # Optional: Port number (default: 8080)
spec:
  ports:
    - port: 8080
  selector:
    app: example-service
```

### 2. Create OpenAPIAggregator Resource

Create an instance of the OpenAPIAggregator custom resource:

```yaml
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: openapi-aggregator
spec:
  labelSelector: {}                                       # Optional: Filter services by labels
  swaggerAnnotation: "openapi.aggregator.io/swagger"     # Required: Annotation to identify OpenAPI services
  pathAnnotation: "openapi.aggregator.io/path"           # Optional: Annotation for custom paths
  portAnnotation: "openapi.aggregator.io/port"           # Optional: Annotation for custom ports
  defaultPath: "/v2/api-docs"                            # Default path if not specified in annotations
  defaultPort: "8080"                                    # Default port if not specified in annotations
```

### 3. Access the Swagger UI

The operator runs a Swagger UI server on port 9090. You can access it through:

```bash
# Port forward the operator's Swagger UI
kubectl port-forward deployment/openapi-aggregator-operator-controller-manager 9090:9090 -n openapi-aggregator-system

# Open in your browser
open http://localhost:9090
```

## Architecture

The operator consists of two main components:

1. **Controller**: 
   - Watches for services with OpenAPI annotations
   - Collects service metadata and OpenAPI spec URLs
   - Updates status every 10 seconds

2. **Swagger UI Server**: 
   - Serves unified Swagger UI interface
   - Fetches OpenAPI specs in real-time
   - Provides API selection and documentation

## Development

Requirements:
- Go 1.21+
- Kubernetes 1.24+
- kubectl
- kustomize
- controller-gen

```bash
# Run locally
make run

# Run tests
make test

# Build container image
make docker-build

# Generate manifests
make manifests
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
