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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
)

// OpenAPIAggregatorReconciler reconciles a OpenAPIAggregator object
type OpenAPIAggregatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch

// Reconcile handles the reconciliation loop for OpenAPIAggregator resources
func (r *OpenAPIAggregatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).V(1) // 기본 로그 레벨을 1로 설정

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

	if err := r.List(ctx, &services, labelSelector); err != nil {
		logger.Error(err, "Failed to list services")
		return ctrl.Result{}, err
	}

	// Process each service and collect OpenAPI specs
	var collectedAPIs []observabilityv1alpha1.APIInfo
	for _, service := range services.Items {
		if apiInfo := r.processService(ctx, service, instance); apiInfo != nil {
			logger.V(1).Info("Collected API info", "service", service.Name, "url", apiInfo.URL)
			collectedAPIs = append(collectedAPIs, *apiInfo)
		}
	}

	// Update status with retry on conflict
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version
		latest := &observabilityv1alpha1.OpenAPIAggregator{}
		if err := r.Get(ctx, req.NamespacedName, latest); err != nil {
			return err
		}

		// Update status
		latest.Status.CollectedAPIs = collectedAPIs
		return r.Status().Update(ctx, latest)
	})

	if retryErr != nil {
		logger.Error(retryErr, "Failed to update OpenAPIAggregator status")
		return ctrl.Result{}, retryErr
	}

	// Create or update ConfigMap with collected specs
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openapi-specs",
			Namespace: req.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: instance.APIVersion,
					Kind:       instance.Kind,
					Name:       instance.Name,
					UID:        instance.UID,
					Controller: &[]bool{true}[0],
				},
			},
		},
		Data: map[string]string{},
	}

	// Convert collected APIs to JSON and store in ConfigMap
	for _, api := range collectedAPIs {
		apiJSON, err := json.Marshal(api)
		if err != nil {
			logger.Error(err, "Failed to marshal API info", "api", api.Name)
			continue
		}
		cm.Data[api.Name] = string(apiJSON)
	}

	// Create or update ConfigMap
	err = r.Client.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, &corev1.ConfigMap{})
	if err != nil {
		if errors.IsNotFound(err) {
			if err = r.Client.Create(ctx, cm); err != nil {
				logger.Error(err, "Failed to create ConfigMap")
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get ConfigMap")
			return ctrl.Result{}, err
		}
	} else {
		if err = r.Client.Update(ctx, cm); err != nil {
			logger.Error(err, "Failed to update ConfigMap")
			return ctrl.Result{}, err
		}
	}

	logger.V(1).Info("Reconciliation completed", "collectedAPIs", len(collectedAPIs))

	// Requeue after 10 seconds
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

// processService processes a single service and returns its API info if valid
func (r *OpenAPIAggregatorReconciler) processService(ctx context.Context, svc corev1.Service, instance *observabilityv1alpha1.OpenAPIAggregator) *observabilityv1alpha1.APIInfo {
	logger := log.FromContext(ctx).V(1)

	// Check if the service has the required swagger annotation
	if svc.Annotations[instance.Spec.SwaggerAnnotation] != "true" {
		logger.V(2).Info("Skipping service - missing swagger annotation",
			"service", svc.Name,
			"namespace", svc.Namespace,
			"requiredAnnotation", instance.Spec.SwaggerAnnotation)
		return nil
	}

	// Get path and port from annotations or defaults
	path := svc.Annotations[instance.Spec.PathAnnotation]
	if path == "" {
		path = instance.Spec.DefaultPath
	}

	port := svc.Annotations[instance.Spec.PortAnnotation]
	if port == "" {
		port = instance.Spec.DefaultPort
	}

	// Process allowed methods
	allowedMethods := make([]string, 0)
	methodsStr := svc.Annotations[instance.Spec.AllowedMethodsAnnotation]

	if methodsStr != "" {
		// Split the string by comma and trim spaces
		for _, method := range strings.Split(methodsStr, ",") {
			method = strings.ToLower(strings.TrimSpace(method))
			// Validate method
			switch method {
			case "get", "put", "post", "delete", "options", "head", "patch", "trace":
				allowedMethods = append(allowedMethods, method)
			}
		}
	}

	// Ensure path starts with "/"
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Create API info
	apiInfo := &observabilityv1alpha1.APIInfo{
		Name:           svc.Name,
		ResourceName:   svc.Name,
		ResourceType:   "Service",
		Namespace:      svc.Namespace,
		Path:           path,
		Port:           port,
		URL:            fmt.Sprintf("http://%s.%s.svc.cluster.local:%s%s", svc.Name, svc.Namespace, port, path),
		LastUpdated:    time.Now().Format(time.RFC3339),
		Annotations:    svc.Annotations,
		AllowedMethods: allowedMethods,
	}

	// Enable health check to validate accessibility
	// TODO: Uncomment this line for production use
	// r.checkAPIHealth(ctx, apiInfo)

	return apiInfo
}

// // checkAPIHealth verifies if the OpenAPI endpoint is accessible
// func (r *OpenAPIAggregatorReconciler) checkAPIHealth(ctx context.Context, apiInfo *observabilityv1alpha1.APIInfo) {
// 	logger := log.FromContext(ctx).V(1)

// 	client := &http.Client{Timeout: 5 * time.Second}
// 	resp, err := client.Get(apiInfo.URL)
// 	if err != nil {
// 		logger.V(1).Info("API health check failed", "name", apiInfo.Name, "error", err)
// 		apiInfo.Error = fmt.Sprintf("Failed to access OpenAPI endpoint: %v", err)
// 		return
// 	}
// 	defer func() {
// 		if err := resp.Body.Close(); err != nil {
// 			logger.Error(err, "Failed to close response body")
// 		}
// 	}()

// 	if resp.StatusCode != http.StatusOK {
// 		logger.V(1).Info("API health check failed",
// 			"name", apiInfo.Name,
// 			"statusCode", resp.StatusCode)
// 		apiInfo.Error = fmt.Sprintf("OpenAPI endpoint returned non-200 status: %d", resp.StatusCode)
// 		return
// 	}

// 	logger.V(2).Info("API health check successful", "name", apiInfo.Name)
// 	apiInfo.Error = ""
// }

// SetupWithManager sets up the controller with the Manager.
func (r *OpenAPIAggregatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.OpenAPIAggregator{}).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
				svc := obj.(*corev1.Service)
				// 서비스에 swagger 관련 어노테이션이 있는 경우에만 리컨실레이션 트리거
				if val, ok := svc.Annotations["openapi.aggregator.io/swagger"]; ok && val == "true" {
					return []ctrl.Request{
						{NamespacedName: types.NamespacedName{
							Name:      "openapi-aggregator",
							Namespace: svc.Namespace,
						}},
					}
				}
				return nil
			}),
		).
		Complete(r)
}
