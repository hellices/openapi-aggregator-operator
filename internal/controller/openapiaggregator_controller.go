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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/yourname/openapi-aggregator-operator/api/v1alpha1"
	"github.com/yourname/openapi-aggregator-operator/pkg/swagger"
)

// OpenAPIAggregatorReconciler reconciles a OpenAPIAggregator object
type OpenAPIAggregatorReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	swaggerServer *swagger.Server
}

// +kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observability.aggregator.io,resources=openapiaggregators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenAPIAggregator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *OpenAPIAggregatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpenAPIAggregator instance
	instance := &observabilityv1alpha1.OpenAPIAggregator{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch OpenAPIAggregator")
		return ctrl.Result{}, err
	}

	// List deployments matching the label selector
	deployments := &appsv1.DeploymentList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(instance.Spec.LabelSelector),
	}

	if instance.Spec.NamespaceSelector != "" {
		listOpts = append(listOpts, client.InNamespace(instance.Spec.NamespaceSelector))
	}

	if err := r.List(ctx, deployments, listOpts...); err != nil {
		logger.Error(err, "failed to list deployments")
		return ctrl.Result{}, err
	}

	// Collect OpenAPI specs from each deployment
	var collectedAPIs []observabilityv1alpha1.APIInfo
	for _, deploy := range deployments.Items {
		// Skip if deployment is not ready
		if deploy.Status.ReadyReplicas == 0 {
			logger.Info("Skipping deployment as it has no ready replicas",
				"deployment", deploy.Name,
				"namespace", deploy.Namespace)
			continue
		}

		path, port := r.getAPIPathAndPort(deploy, instance)

		// Get or create service for the deployment
		svc := &corev1.Service{}
		svcName := deploy.Name
		svcNS := deploy.Namespace

		err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: svcNS}, svc)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Service not found for deployment",
					"deployment", deploy.Name,
					"namespace", deploy.Namespace)
			} else {
				logger.Error(err, "Failed to get service",
					"deployment", deploy.Name,
					"namespace", deploy.Namespace)
			}
			continue
		}

		// Get cluster IP of the service
		if svc.Spec.ClusterIP == "" {
			logger.Info("Service has no cluster IP",
				"service", svc.Name,
				"namespace", svc.Namespace)
			continue
		}

		apiInfo := observabilityv1alpha1.APIInfo{
			Name:         instance.Spec.DisplayNamePrefix + deploy.Name,
			URL:          fmt.Sprintf("http://%s:%s%s", svc.Spec.ClusterIP, port, path),
			LastUpdated:  metav1.Now().Format(time.RFC3339),
			ResourceType: "Deployment",
			ResourceName: deploy.Name,
			Namespace:    deploy.Namespace,
			Annotations:  deploy.Annotations,
		}

		// Check if URL is reachable
		resp, err := http.Get(apiInfo.URL)
		if err != nil {
			apiInfo.Error = fmt.Sprintf("Failed to reach service: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				apiInfo.Error = fmt.Sprintf("Service returned status code: %d", resp.StatusCode)
			}
		}

		collectedAPIs = append(collectedAPIs, apiInfo)
	}

	// Update status
	instance.Status.CollectedAPIs = collectedAPIs
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "failed to update OpenAPIAggregator status")
		return ctrl.Result{}, err
	}

	// Update Swagger UI specs
	r.swaggerServer.UpdateSpecs(collectedAPIs)

	// Requeue after 5 minutes
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenAPIAggregatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create and start the Swagger UI server
	r.swaggerServer = swagger.NewServer()
	go func() {
		if err := r.swaggerServer.Start(8080); err != nil {
			mgr.GetLogger().Error(err, "Failed to start Swagger UI server")
		}
	}()

	// Watch the OpenAPIAggregator resource and related workloads
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.OpenAPIAggregator{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

func (r *OpenAPIAggregatorReconciler) findObjectsForWorkload(ctx context.Context, obj client.Object) []reconcile.Request {
	// Get all OpenAPIAggregator instances
	aggregators := &observabilityv1alpha1.OpenAPIAggregatorList{}
	if err := r.List(context.Background(), aggregators); err != nil {
		return nil
	}

	var requests []reconcile.Request
	workloadLabels := obj.GetLabels()

	for _, agg := range aggregators.Items {
		// Check if the workload matches the label selector
		matches := true
		for k, v := range agg.Spec.LabelSelector {
			if workloadLabels[k] != v {
				matches = false
				break
			}
		}

		// Check namespace selector if specified
		if matches && agg.Spec.NamespaceSelector != "" {
			matches = obj.GetNamespace() == agg.Spec.NamespaceSelector
		}

		if matches {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      agg.Name,
					Namespace: agg.Namespace,
				},
			})
		}
	}

	return requests
}

func (r *OpenAPIAggregatorReconciler) getAPIPathAndPort(deploy appsv1.Deployment, instance *observabilityv1alpha1.OpenAPIAggregator) (string, string) {
	if instance.Spec.IgnoreAnnotations {
		return instance.Spec.DefaultPath, instance.Spec.DefaultPort
	}

	path := deploy.Annotations[instance.Spec.PathAnnotation]
	if path == "" {
		path = instance.Spec.DefaultPath
	}

	port := deploy.Annotations[instance.Spec.PortAnnotation]
	if port == "" {
		port = instance.Spec.DefaultPort
	}

	return path, port
}
