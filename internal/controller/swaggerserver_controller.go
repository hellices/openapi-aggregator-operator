/*
Copyright 2023.

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
	"time" // Added for RequeueAfter

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta" // Added for SetStatusCondition
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
)

// Define a condition type for ConfigMap readiness
const ConfigMapReadyCondition = "ConfigMapReady"

// SwaggerServerReconciler reconciles a SwaggerServer object
type SwaggerServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=observability.aggregator.io,resources=swaggerservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=swaggerservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.aggregator.io,resources=swaggerservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile handles the reconciliation loop for SwaggerServer resources
func (r *SwaggerServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the SwaggerServer instance
	instance := &observabilityv1alpha1.SwaggerServer{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Defer status update to ensure it's always attempted.
	defer func() {
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "Failed to update SwaggerServer status")
		}
	}()

	// Check if ConfigMap exists
	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.ConfigMapName, Namespace: req.Namespace}, cm)
	if err != nil {
		configMapCondition := metav1.Condition{
			Type:               ConfigMapReadyCondition,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: instance.Generation,
		}
		if errors.IsNotFound(err) {
			logger.Info("ConfigMap not found, requeuing", "configmap", instance.Spec.ConfigMapName, "namespace", req.Namespace)
			configMapCondition.Reason = "ConfigMapNotFound"
			configMapCondition.Message = fmt.Sprintf("ConfigMap %s not found in namespace %s", instance.Spec.ConfigMapName, req.Namespace)
			apimeta.SetStatusCondition(&instance.Status.Conditions, configMapCondition)
			instance.Status.Ready = false
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Requeue after a delay, not an immediate error
		}
		// Other error fetching ConfigMap
		logger.Error(err, "Failed to get ConfigMap", "configmap", instance.Spec.ConfigMapName)
		configMapCondition.Reason = "GetConfigMapFailed"
		configMapCondition.Message = fmt.Sprintf("Failed to get ConfigMap %s: %v", instance.Spec.ConfigMapName, err)
		apimeta.SetStatusCondition(&instance.Status.Conditions, configMapCondition)
		instance.Status.Ready = false
		return ctrl.Result{}, err // Actual error
	}

	// ConfigMap found, set condition to True
	apimeta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConfigMapReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "ConfigMapFound",
		Message:            fmt.Sprintf("ConfigMap %s found", instance.Spec.ConfigMapName),
		ObservedGeneration: instance.Generation,
	})

	// Determine the image to use
	image := instance.Spec.Image
	if image == "" {
		image = "ghcr.io/hellices/openapi-multi-swagger:latest"
	}

	// Create or update the deployment
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": instance.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": instance.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "swagger-ui",
							Image:           image, // Use the determined image
							ImagePullPolicy: corev1.PullPolicy(instance.Spec.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: instance.Spec.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CONFIGMAP_NAME",
									Value: instance.Spec.ConfigMapName,
								},
								{
									Name:  "NAMESPACE",
									Value: instance.Namespace, // Namespace of the CR
								},
								{
									Name:  "PORT",
									Value: fmt.Sprintf("%d", instance.Spec.Port),
								},
								{
									Name:  "WATCH_INTERVAL_SECONDS",
									Value: getValueOrDefault(instance.Spec.WatchIntervalSeconds, "10"),
								},
								{
									Name:  "LOG_LEVEL",
									Value: getValueOrDefault(instance.Spec.LogLevel, "info"),
								},
								{
									Name:  "DEV_MODE",
									Value: getValueOrDefault(instance.Spec.DevMode, "false"),
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits:   resourceListToK8s(instance.Spec.Resources.Limits),
								Requests: resourceListToK8s(instance.Spec.Resources.Requests),
							},
						},
					},
				},
			},
		},
	}

	// Set the owner reference
	if err := controllerutil.SetControllerReference(instance, deploy, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference for Deployment")
		return ctrl.Result{}, err
	}

	// Create or update deployment
	err = r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, &appsv1.Deployment{})
	if err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, deploy); err != nil {
				logger.Error(err, "Failed to create Deployment")
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get Deployment")
			return ctrl.Result{}, err
		}
	} else {
		if err = r.Update(ctx, deploy); err != nil {
			logger.Error(err, "Failed to update Deployment")
			return ctrl.Result{}, err
		}
	}

	// Create or update the service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": instance.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       instance.Spec.Port,
					TargetPort: intstr.FromInt(int(instance.Spec.Port)),
				},
			},
		},
	}

	// Set the owner reference for the service
	if err := controllerutil.SetControllerReference(instance, svc, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference for Service")
		return ctrl.Result{}, err
	}

	// Create or update service
	err = r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &corev1.Service{})
	if err != nil {
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, svc); err != nil {
				logger.Error(err, "Failed to create Service")
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get Service")
			return ctrl.Result{}, err
		}
	} else {
		if err = r.Update(ctx, svc); err != nil {
			logger.Error(err, "Failed to update Service")
			return ctrl.Result{}, err
		}
	}

	// Update status
	// If all operations were successful, set Ready to true.
	// The ConfigMapReady condition is already set.
	// Additional conditions for Deployment and Service readiness could be added here.
	instance.Status.Ready = true
	instance.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", instance.Name, instance.Namespace, instance.Spec.Port)
	// Status update is handled by the deferred function call

	return ctrl.Result{}, nil
}

// getValueOrDefault returns the value if not empty, otherwise returns the default value.
func getValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwaggerServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.SwaggerServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// resourceListToK8s converts our ResourceList to k8s ResourceList
func resourceListToK8s(rl observabilityv1alpha1.ResourceList) corev1.ResourceList {
	result := corev1.ResourceList{}
	for k, v := range rl {
		result[corev1.ResourceName(k)] = resource.MustParse(v)
	}
	return result
}
