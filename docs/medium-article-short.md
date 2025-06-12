# Solving API Chaos in Kubernetes with One Simple Operator

*Stop juggling multiple Swagger UIs and discover all your APIs in one place*

If you've worked with microservices in Kubernetes, you've felt this pain: hunting down API documentation across dozens of services, each with its own Swagger UI endpoint, each requiring its own port-forward command. What if there was a better way?

## The Problem Every Kubernetes Team Faces

Your production cluster runs 20+ microservices. Each has its own API. When you need to test an integration or onboard a new developer, you find yourself:

- Digging through Kubernetes manifests to find API endpoints
- Running multiple `kubectl port-forward` commands
- Switching between browser tabs with different Swagger UIs
- Asking teammates "Where's the docs for the payment service?"

This isn't just inconvenient — it's a productivity killer.

## The Solution: OpenAPI Aggregator Operator

The OpenAPI Aggregator Operator automatically discovers APIs in your cluster and presents them in a unified Swagger UI. Think of it as having one dashboard for all your APIs, updated in real-time.

Here's how simple it is:

### 1. Install the Operator
```bash
kubectl apply -f https://raw.githubusercontent.com/hellices/openapi-aggregator-operator/main/install.yaml
```

### 2. Mark Your Services
Add one annotation to services that expose APIs:
```yaml
metadata:
  annotations:
    openapi.aggregator.io/swagger: "true"
```

### 3. Create the Aggregator
```yaml
apiVersion: observability.aggregator.io/v1alpha1
kind: OpenAPIAggregator
metadata:
  name: api-aggregator
spec:
  watchNamespaces: [""]  # Watch all namespaces
```

### 4. Deploy Swagger UI
```yaml
apiVersion: observability.aggregator.io/v1alpha1
kind: SwaggerServer
metadata:
  name: unified-apis
spec:
  port: 9090
```

### 5. Access Your APIs
```bash
kubectl port-forward svc/unified-apis 9090:9090
```

Open http://localhost:9090 — every API in your cluster, in one interface.

## Real Impact

**Before**: 30 minutes to find and test an API across 3 different services.
**After**: 2 minutes to browse, select, and test any API from one dashboard.

**Before**: New developers spend their first day figuring out where documentation lives.
**After**: One URL gives them access to every API in the system.

## Advanced Features

**Namespace Control**: Only watch specific namespaces for sensitive environments.

**Ingress Integration**: Share the dashboard with your entire team.

**Method Filtering**: Control which HTTP methods are exposed per service.

## Why This Matters

This isn't just about convenience. It's about:
- **Developer Velocity**: Less time hunting, more time building
- **Team Collaboration**: Self-service API discovery across teams
- **Quality**: Visible APIs tend to be better documented
- **Onboarding**: New team members become productive faster

## Try It Now

The operator is:
- ✅ Open source (Apache 2.0)
- ✅ Production-ready
- ✅ Easy to install and remove
- ✅ Compatible with existing setups

**Get started**: https://github.com/hellices/openapi-aggregator-operator

Your scattered API documentation problem can be solved in 15 minutes. The only question is: what are you waiting for?

---

**Resources:**
- [GitHub Repository](https://github.com/hellices/openapi-aggregator-operator)
- [Full Documentation](https://github.com/hellices/openapi-aggregator-operator/blob/main/README.md)
- [Quick Start Guide](https://github.com/hellices/openapi-aggregator-operator#quick-start)

*Follow for more Kubernetes solutions that solve real developer problems.*