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
    openapi.aggregator.io/allowed-methods: "get,post"  # Optional: Filter allowed HTTP methods
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
- 🔒 **Secure API Access**: All API requests from Swagger UI are proxied through the aggregator server instead of direct service access

### 5. Ingress/Route Integration

You can expose the Swagger UI through Ingress or OpenShift Route. 

#### Using Kubernetes Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: swagger-ui
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /$2
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /swagger-ui(/|$)(.*)
        pathType: Prefix
        backend:
          service:
            name: openapi-aggregator-openapi-aggregator-swagger-ui
            port:
              number: 9090
```

And set the environment variable in the deployment:
```yaml
env:
- name: SWAGGER_BASE_PATH
  value: /swagger-ui
```

#### Using OpenShift Route

```yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: swagger-ui
spec:
  to:
    kind: Service
    name: openapi-aggregator-openapi-aggregator-swagger-ui
  port:
    targetPort: swagger-ui
```

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
   - Acts as a proxy for all API requests, enhancing security by preventing direct access to services

### Request Flow

1. User accesses Swagger UI and selects an API endpoint
2. API request is sent to the Swagger UI Server
3. Server proxies the request to the target service
4. Response is returned through the proxy to Swagger UI

This proxy architecture provides several benefits:
- Enhanced security by preventing direct access to services
- Consistent request routing and handling
- Ability to add request/response transformations
- Centralized access control and monitoring

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

### Development Setup

### Prerequisites

- Go 1.19 or higher
- Kubernetes cluster (local or remote)
- kubectl
- kustomize
- controller-gen

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/hellices/openapi-aggregator-operator.git
cd openapi-aggregator-operator
```

2. Install dependencies:
```bash
make install-tools
```

3. Run the operator locally:
```bash
make run
```

### Building and Testing

- Build the operator:
```bash
make build
```

- Run tests:
```bash
make test
```

- Build docker image:
```bash
make docker-build
```

## Configuration

### Annotation Options

| Annotation | Description | Default | Required |
|------------|-------------|---------|----------|
| openapi.aggregator.io/swagger | Enable swagger aggregation | - | Yes |
| openapi.aggregator.io/path | Path to OpenAPI/Swagger endpoint | /v2/api-docs | No |
| openapi.aggregator.io/port | Port for OpenAPI/Swagger endpoint | 8080 | No |
| openapi.aggregator.io/allowed-methods | Comma-separated list of allowed HTTP methods | All methods | No |

### OpenAPIAggregator CR Options

```yaml
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: openapi-aggregator
spec:
  labelSelector:
    matchLabels:
      app: myapp  # Optional: Filter services by labels
  updateInterval: 10s  # Optional: Specification update interval
```

## Troubleshooting

### Common Issues

1. **Services not being discovered**
   - Verify service annotations are correct
   - Check if service is in the same namespace as the operator
   - Ensure service endpoints are accessible

2. **Swagger UI not loading**
   - Verify port-forward is running correctly
   - Check if swagger-ui service is deployed
   - Ensure OpenAPI specifications are valid

3. **API endpoints not accessible**
   - Verify allowed-methods annotation
   - Check if service is running and healthy
   - Ensure network policies allow access

### Logs

To check operator logs:
```bash
kubectl logs -n openapi-aggregator-system deployment/openapi-aggregator-controller-manager -c manager
```

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
