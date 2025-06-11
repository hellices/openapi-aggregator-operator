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

Create an `OpenAPIAggregator` custom resource. This resource tells the operator how to discover services.

```bash
kubectl apply -f - <<EOF
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: openapi-aggregator
  # The namespace where this CR is created is important.
  # The generated openapi-specs ConfigMap will be created in this same namespace.
  namespace: default # Or any namespace where you want the ConfigMap
spec:
  # To watch services in the same namespace as this OpenAPIAggregator CR:
  # watchNamespaces: [] # or leave it undefined

  # To watch services in ALL namespaces (requires ClusterRole permissions for the operator):
  watchNamespaces: [""] # or ["*"]

  # Default values for service annotations if not specified on the service itself
  defaultPath: "/v2/api-docs"
  defaultPort: "8080"

  # Annotation keys used to discover and configure services
  swaggerAnnotation: "openapi.aggregator.io/swagger" # Service annotation to mark it for discovery
  pathAnnotation: "openapi.aggregator.io/path"         # Service annotation for custom OpenAPI path
  portAnnotation: "openapi.aggregator.io/port"         # Service annotation for custom OpenAPI port
  allowedMethodsAnnotation: "openapi.aggregator.io/allowed-methods" # Service annotation for allowed HTTP methods
EOF
```

**Note on `watchNamespaces`**:
*   If `watchNamespaces` is empty or not provided, the controller watches services in the same namespace as the `OpenAPIAggregator` CR.
*   If `watchNamespaces` is `[""]` or `["*"]`, the controller watches services in all namespaces. This requires the operator to have cluster-level RBAC permissions to list and watch services across all namespaces.
*   The `openapi-specs` ConfigMap, which stores the aggregated API information, is always created in the same namespace as the `OpenAPIAggregator` CR itself.

### 4. Create SwaggerServer Instance

To view the aggregated OpenAPI specifications, create a `SwaggerServer` custom resource. This will deploy a Swagger UI instance.

```bash
kubectl apply -f - <<EOF
apiVersion: observability.aggregator.io/v1alpha1
kind: SwaggerServer
metadata:
  name: swagger-server
  namespace: default # Should be the same namespace as the OpenAPIAggregator CR and the openapi-specs ConfigMap
spec:
  # image: ghcr.io/hellices/openapi-multi-swagger:latest # Optional: Defaults to this image
  # watchIntervalSeconds: 10 # Optional: How often to check for ConfigMap updates (default: 10)
  # logLevel: info # Optional: Log level for the Swagger UI server (default: info)
  # devMode: false # Optional: Enable dev mode for more verbose logging (default: false)
EOF
```

### 5. Access Swagger UI

Forward the port of the `SwaggerServer`'s service:

```bash
# The service name will be <SwaggerServer-CR-Name>-service
# Check the service name in the namespace where SwaggerServer CR was created.
# For example, if SwaggerServer CR is named 'swagger-server' in 'default' namespace:
kubectl port-forward -n default svc/swagger-server-service 9090:8080
```

Then open http://localhost:9090 in your browser.

## Features

- 🔍 **Flexible Service Discovery**: Discover services based on annotations within specified namespaces (CR's namespace, all namespaces, or a list of namespaces (future)).
- 🔄 **Real-time Updates**: The `OpenAPIAggregator` updates the `openapi-specs` ConfigMap with discovered API information.
- 📄 **Centralized Specs**: Aggregated API specifications are stored in a `ConfigMap`.
- 🎨 **Customizable Swagger UI**: The `SwaggerServer` deploys a pre-built Swagger UI (defaults to `ghcr.io/hellices/openapi-multi-swagger:latest`) that reads from the `openapi-specs` ConfigMap.

### 6. Ingress/Route Integration

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
