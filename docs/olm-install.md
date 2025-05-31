# OLM Installation Guide

This guide explains how to install the OpenAPI Aggregator Operator using the Operator Lifecycle Manager (OLM).

## Prerequisites

Ensure OLM is installed in your cluster:
```bash
operator-sdk olm install
```

## Installation Steps

1. Build and push the bundle image:
```bash
# Generate and validate bundle
make bundle

# Build the bundle image
make bundle-build BUNDLE_IMG=ghcr.io/hellices/openapi-aggregator-operator-bundle:0.1.0

# Push the bundle image
make bundle-push BUNDLE_IMG=ghcr.io/hellices/openapi-aggregator-operator-bundle:0.1.0
```

2. Build and push the catalog image:
```bash
# Build catalog
make catalog-build CATALOG_IMG=ghcr.io/hellices/openapi-aggregator-operator-catalog:0.1.0

# Push catalog
make catalog-push CATALOG_IMG=ghcr.io/hellices/openapi-aggregator-operator-catalog:0.1.0
```

3. Create a CatalogSource:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: openapi-aggregator-catalog
  namespace: openshift-marketplace  # or olm for vanilla Kubernetes
spec:
  sourceType: grpc
  image: ghcr.io/hellices/openapi-aggregator-operator-catalog:0.1.0
  displayName: OpenAPI Aggregator Operator
  publisher: hellices
```

4. Create a Subscription:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openapi-aggregator-operator
  namespace: openapi-aggregator-system
spec:
  channel: alpha
  name: openapi-aggregator-operator
  source: openapi-aggregator-catalog
  sourceNamespace: openshift-marketplace  # or olm for vanilla Kubernetes
```

The operator will be automatically installed and managed by OLM.
