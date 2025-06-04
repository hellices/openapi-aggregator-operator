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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if ConfigMap exists
	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.ConfigMapName, Namespace: req.Namespace}, cm)
	if err != nil {
		logger.Error(err, "Failed to get ConfigMap", "configmap", instance.Spec.ConfigMapName)
		return ctrl.Result{}, err
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
							Image:           instance.Spec.Image,
							ImagePullPolicy: corev1.PullPolicy(instance.Spec.ImagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: instance.Spec.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SWAGGER_BASE_PATH",
									Value: instance.Spec.BasePath,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "openapi-specs",
									MountPath: "/specs",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits:   resourceListToK8s(instance.Spec.Resources.Limits),
								Requests: resourceListToK8s(instance.Spec.Resources.Requests),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "openapi-specs",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: instance.Spec.ConfigMapName,
									},
								},
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
	instance.Status.Ready = true
	instance.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", instance.Name, instance.Namespace, instance.Spec.Port)
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "Failed to update SwaggerServer status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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
