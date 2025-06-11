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

### 3. Deploy Sample Custom Resources

After the controller is running, you can deploy the sample `OpenAPIAggregator` and `SwaggerServer` custom resources to your cluster using the files in the `config/samples` directory:

```bash
kubectl apply -f config/samples/observability_v1alpha1_openapiaggregator.yaml
kubectl apply -f config/samples/swagger-server-sample.yaml
```

For a detailed explanation of these sample custom resources, see [config/samples/README.md](config/samples/README.md).

The `openapi-specs` ConfigMap, which stores the aggregated API information, is created in the same namespace as the `OpenAPIAggregator` CR (e.g., `default` if using the sample). The `SwaggerServer` should be deployed in the same namespace to access this ConfigMap.

### 4. Access Swagger UI

Forward the port of the `SwaggerServer`'s service:

```bash
# The service name will be <SwaggerServer-CR-Name>-service.
# For the sample, the SwaggerServer CR is named 'swagger-ui' in the 'default' namespace (see config/samples/swagger-server-sample.yaml).
# The service created will be 'swagger-ui' and it will listen on the port defined in the CR's spec.port (9090 for the sample).
kubectl port-forward -n default svc/swagger-ui 9090:9090
```

Then open http://localhost:9090 in your browser.

## Features

- 🔍 **Flexible Service Discovery**: Discover services based on annotations within specified namespaces (CR's namespace, all namespaces, or a list of namespaces (future)).
- 🔄 **Real-time Updates**: The `OpenAPIAggregator` updates the `openapi-specs` ConfigMap with discovered API information.
- 📄 **Centralized Specs**: Aggregated API specifications are stored in a `ConfigMap`.
- 🎨 **Customizable Swagger UI**: The `SwaggerServer` deploys a pre-built Swagger UI (defaults to `ghcr.io/hellices/openapi-multi-swagger:latest`) that reads from the `openapi-specs` ConfigMap.

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

1. **OpenAPIAggregator Controller**:
   - Watches for `OpenAPIAggregator` custom resources.
   - Based on the `watchNamespaces` field, lists and watches `Services` in the specified namespace(s).
   - Filters services based on the `swaggerAnnotation`.
   - Collects metadata (path, port, allowed methods) from service annotations or uses defaults from the `OpenAPIAggregator` spec.
   - Creates/Updates a `ConfigMap` named `openapi-specs` in the same namespace as the `OpenAPIAggregator` CR. This ConfigMap contains the JSON representation of the discovered API endpoints, keyed by `namespace.serviceName`.

2. **SwaggerServer Controller**:
   - Watches for `SwaggerServer` custom resources.
   - Deploys a `Deployment` and `Service` for a Swagger UI application (e.g., `ghcr.io/hellices/openapi-multi-swagger:latest`).
   - Configures the Swagger UI deployment to load API specifications from the `openapi-specs` ConfigMap created by an `OpenAPIAggregator` in the same namespace.
   - Manages the lifecycle of the Swagger UI deployment and service.

3. **Swagger UI Server (Pod)**:
   - Serves a unified Swagger UI interface.
   - Loads API definitions from the mounted `openapi-specs` ConfigMap.
   - Allows users to browse and interact with the aggregated APIs.
   - API requests are typically proxied by the Swagger UI itself or made directly from the browser, depending on the Swagger UI implementation.

### Request Flow (Simplified)

1.  **Discovery**: `OpenAPIAggregator` controller discovers services with the specified annotation in the configured `watchNamespaces`.
2.  **Aggregation**: It writes the API details (URL, path, etc.) into the `openapi-specs` ConfigMap in its own namespace.
3.  **Deployment**: `SwaggerServer` controller deploys a Swagger UI pod, mounting the `openapi-specs` ConfigMap.
4.  **UI Access**: User accesses the Swagger UI service.
5.  **Spec Loading**: Swagger UI reads the API list from the `openapi-specs` ConfigMap.
6.  **Interaction**: User selects an API; Swagger UI displays its documentation and allows interaction.

This setup decouples API discovery/aggregation from the UI presentation. The `OpenAPIAggregator` focuses on finding and preparing API specs, while the `SwaggerServer` focuses on presenting them.

### Project Structure

```
.
├── api/             # API definitions (CRDs for OpenAPIAggregator, SwaggerServer)
├── cmd/             # Main application entry point for the operator manager
├── config/          # Kubernetes manifests and kustomize configs
│   ├── crd/         # CRD definitions
│   ├── default/     # Default kustomize overlays
│   ├── manager/     # Manager (operator) deployment manifests
│   ├── rbac/        # RBAC configurations (Roles, RoleBindings, ClusterRoles)
│   └── samples/     # Sample CRs for OpenAPIAggregator and SwaggerServer
├── internal/        # Internal packages
│   └── controller/  # Operator controller logic for both CRDs
└── pkg/             # Shared packages (version, etc.)
# Removed pkg/swagger as the Swagger UI is now a separate Docker image
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
