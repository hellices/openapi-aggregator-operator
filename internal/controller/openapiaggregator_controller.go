/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
	"github.com/hellices/openapi-aggregator-operator/pkg/swagger"
)

// OpenAPIAggregatorReconciler reconciles a OpenAPIAggregator object
type OpenAPIAggregatorReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	swaggerServer *swagger.Server
	TestMode      bool
}

//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch

// Reconcile handles the reconciliation loop for OpenAPIAggregator resources
func (r *OpenAPIAggregatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting reconciliation", "namespace", req.Namespace, "name", req.Name)

	// Fetch the OpenAPIAggregator instance
	instance := &observabilityv1alpha1.OpenAPIAggregator{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// List all services based on label selector
	var services corev1.ServiceList
	labelSelector := client.MatchingLabels(instance.Spec.LabelSelector)
	logger.Info("Listing services", "labelSelector", instance.Spec.LabelSelector)

	if err := r.List(ctx, &services, labelSelector); err != nil {
		logger.Error(err, "Failed to list services")
		return ctrl.Result{}, err
	}
	logger.Info("Found services", "count", len(services.Items))

	// Process each service and collect OpenAPI specs
	var collectedAPIs []observabilityv1alpha1.APIInfo
	for _, service := range services.Items {
		logger.Info("Processing service", "name", service.Name, "namespace", service.Namespace)

		if apiInfo := r.processService(ctx, service, instance); apiInfo != nil {
			logger.Info("Collected API info", "service", service.Name, "url", apiInfo.URL)
			collectedAPIs = append(collectedAPIs, *apiInfo)
		} else {
			logger.V(1).Info("Service skipped", "name", service.Name, "namespace", service.Namespace)
		}
	}

	// Simply update the status with the new list
	// Any removed services will naturally be excluded
	instance.Status.CollectedAPIs = collectedAPIs
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "Failed to update OpenAPIAggregator status")
		return ctrl.Result{}, err
	}

	// Update Swagger UI with collected specs
	logger.Info("Updating Swagger UI specs", "count", len(collectedAPIs))
	r.swaggerServer.UpdateSpecs(collectedAPIs)

	logger.Info("Reconciliation completed", "collectedAPIs", len(collectedAPIs))

	// Requeue after 10 seconds
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// processService processes a single service and returns its API info if valid
func (r *OpenAPIAggregatorReconciler) processService(ctx context.Context, svc corev1.Service, instance *observabilityv1alpha1.OpenAPIAggregator) *observabilityv1alpha1.APIInfo {
	logger := log.FromContext(ctx).V(1)

	// Check if the service has the required swagger annotation
	if svc.Annotations[instance.Spec.SwaggerAnnotation] != "true" {
		logger.Info("Skipping service - missing swagger annotation",
			"service", svc.Name,
			"namespace", svc.Namespace,
			"requiredAnnotation", instance.Spec.SwaggerAnnotation)
		return nil
	}

	// Get path and port from annotations or defaults
	path := svc.Annotations[instance.Spec.PathAnnotation]
	if path == "" {
		logger.Info("Using default path", "service", svc.Name, "defaultPath", instance.Spec.DefaultPath)
		path = instance.Spec.DefaultPath
	}

	port := svc.Annotations[instance.Spec.PortAnnotation]
	if port == "" {
		logger.Info("Using default port", "service", svc.Name, "defaultPort", instance.Spec.DefaultPort)
		port = instance.Spec.DefaultPort
	}

	// Create API info
	apiInfo := &observabilityv1alpha1.APIInfo{
		Name:         svc.Name,
		ResourceName: svc.Name,
		ResourceType: "Service",
		Namespace:    svc.Namespace,
		Path:         path,
		Port:         port,
		URL:          fmt.Sprintf("http://%s.%s.svc.cluster.local:%s%s", svc.Name, svc.Namespace, port, path),
		LastUpdated:  time.Now().Format(time.RFC3339),
		Annotations:  svc.Annotations,
	}

	// Enable health check to validate accessibility
	// TODO: Uncomment this line for production use
	// r.checkAPIHealth(ctx, apiInfo)

	return apiInfo
}

// checkAPIHealth verifies if the OpenAPI endpoint is accessible
func (r *OpenAPIAggregatorReconciler) checkAPIHealth(ctx context.Context, apiInfo *observabilityv1alpha1.APIInfo) {
	logger := log.FromContext(ctx).V(1)

	logger.Info("Checking API health", "name", apiInfo.Name, "url", apiInfo.URL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiInfo.URL)
	if err != nil {
		logger.Info("API health check failed", "name", apiInfo.Name, "error", err)
		apiInfo.Error = fmt.Sprintf("Failed to access OpenAPI endpoint: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Info("API health check failed",
			"name", apiInfo.Name,
			"statusCode", resp.StatusCode,
			"headers", resp.Header)
		apiInfo.Error = fmt.Sprintf("OpenAPI endpoint returned non-200 status: %d", resp.StatusCode)
		return
	}

	logger.Info("API health check successful", "name", apiInfo.Name)
	apiInfo.Error = ""
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenAPIAggregatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize Swagger UI server
	if r.swaggerServer == nil && !r.TestMode {
		r.swaggerServer = swagger.NewServer()
		go func() {
			log.Log.Info("Starting Swagger UI server on HTTP port 9090")
			if err := r.swaggerServer.Start(9090); err != nil {
				log.Log.Error(err, "Failed to start Swagger UI server")
			}
		}()
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.OpenAPIAggregator{}).
		Watches(&corev1.Service{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
