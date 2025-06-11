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

// ConfigMapReadyCondition indicates whether the ConfigMap is ready.
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

	instance := &observabilityv1alpha1.SwaggerServer{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	defer func() {
		if updateErr := r.Status().Update(ctx, instance); updateErr != nil {
			logger.Error(updateErr, "Failed to update SwaggerServer status")
		}
	}()

	if err := r.ensureConfigMapReady(ctx, instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.ensureDeployment(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.Ready = true
	instance.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", instance.Name, instance.Namespace, instance.Spec.Port)

	return ctrl.Result{}, nil
}

func (r *SwaggerServerReconciler) ensureConfigMapReady(ctx context.Context, instance *observabilityv1alpha1.SwaggerServer) error {
	logger := log.FromContext(ctx)
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.ConfigMapName, Namespace: instance.Namespace}, cm)

	configMapCondition := metav1.Condition{
		Type:               ConfigMapReadyCondition,
		ObservedGeneration: instance.Generation,
	}

	if err != nil {
		configMapCondition.Status = metav1.ConditionFalse
		if errors.IsNotFound(err) {
			logger.Info("ConfigMap not found", "configmap", instance.Spec.ConfigMapName, "namespace", instance.Namespace)
			configMapCondition.Reason = "ConfigMapNotFound"
			configMapCondition.Message = fmt.Sprintf("ConfigMap %s not found in namespace %s", instance.Spec.ConfigMapName, instance.Namespace)
		} else {
			logger.Error(err, "Failed to get ConfigMap", "configmap", instance.Spec.ConfigMapName)
			configMapCondition.Reason = "GetConfigMapFailed"
			configMapCondition.Message = fmt.Sprintf("Failed to get ConfigMap %s: %v", instance.Spec.ConfigMapName, err)
		}
		apimeta.SetStatusCondition(&instance.Status.Conditions, configMapCondition)
		instance.Status.Ready = false
		return err
	}

	configMapCondition.Status = metav1.ConditionTrue
	configMapCondition.Reason = "ConfigMapFound"
	configMapCondition.Message = fmt.Sprintf("ConfigMap %s found", instance.Spec.ConfigMapName)
	apimeta.SetStatusCondition(&instance.Status.Conditions, configMapCondition)
	return nil
}

func (r *SwaggerServerReconciler) ensureDeployment(ctx context.Context, instance *observabilityv1alpha1.SwaggerServer) error {
	logger := log.FromContext(ctx)
	image := instance.Spec.Image
	if image == "" {
		image = "ghcr.io/hellices/openapi-multi-swagger:latest"
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Spec = appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": instance.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": instance.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "swagger-ui",
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(instance.Spec.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{ContainerPort: instance.Spec.Port, Protocol: corev1.ProtocolTCP},
							},
							Env: []corev1.EnvVar{
								{Name: "CONFIGMAP_NAME", Value: instance.Spec.ConfigMapName},
								{Name: "NAMESPACE", Value: instance.Namespace},
								{Name: "PORT", Value: fmt.Sprintf("%d", instance.Spec.Port)},
								{Name: "WATCH_INTERVAL_SECONDS", Value: getValueOrDefault(instance.Spec.WatchIntervalSeconds, "10")},
								{Name: "LOG_LEVEL", Value: getValueOrDefault(instance.Spec.LogLevel, "info")},
								{Name: "DEV_MODE", Value: getValueOrDefault(instance.Spec.DevMode, "false")},
							},
							Resources: corev1.ResourceRequirements{
								Limits:   resourceListToK8s(instance.Spec.Resources.Limits),
								Requests: resourceListToK8s(instance.Spec.Resources.Requests),
							},
						},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(instance, deploy, r.Scheme)
	})

	if err != nil {
		logger.Error(err, "Failed to ensure Deployment")
		return err
	}
	return nil
}

func (r *SwaggerServerReconciler) ensureService(ctx context.Context, instance *observabilityv1alpha1.SwaggerServer) error {
	logger := log.FromContext(ctx)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec = corev1.ServiceSpec{
			Selector: map[string]string{"app": instance.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       instance.Spec.Port,
					TargetPort: intstr.FromInt(int(instance.Spec.Port)),
				},
			},
		}
		return controllerutil.SetControllerReference(instance, svc, r.Scheme)
	})

	if err != nil {
		logger.Error(err, "Failed to ensure Service")
		return err
	}
	return nil
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
