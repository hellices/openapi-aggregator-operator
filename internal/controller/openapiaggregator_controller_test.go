package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	observabilityv1alpha1 "github.com/yourname/openapi-aggregator-operator/api/v1alpha1"
)

var _ = Describe("OpenAPIAggregator Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-aggregator"
			namespace    = "default"
		)

		var (
			ctx                context.Context
			typeNamespacedName types.NamespacedName
			reconciler         *OpenAPIAggregatorReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}
			reconciler = &OpenAPIAggregatorReconciler{
				Client:        k8sClient,
				Scheme:        k8sClient.Scheme(),
				swaggerServer: NewTestSwaggerServer(),
			}

			// Create the OpenAPIAggregator object
			aggregator := &observabilityv1alpha1.OpenAPIAggregator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: observabilityv1alpha1.OpenAPIAggregatorSpec{
					DisplayNamePrefix: "Test-",
					LabelSelector: map[string]string{
						"app": "test-api",
					},
					Path: "/v3/api-docs",
					Port: "8080",
				},
			}
			Expect(k8sClient.Create(ctx, aggregator)).To(Succeed())
		})

		AfterEach(func() {
			// Clean up resources
			By("Cleaning up the test resources")
			resources := []client.Object{
				&observabilityv1alpha1.OpenAPIAggregator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
					},
				},
			}

			for _, res := range resources {
				err := k8sClient.Delete(ctx, res)
				Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
			}
		})

		Context("With a valid deployment", func() {
			BeforeEach(func() {
				// Create a test deployment
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
						Labels: map[string]string{
							"app": "test-api",
						},
						Annotations: map[string]string{
							"openapi.path": "/v3/api-docs",
							"openapi.port": "8080",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-api",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-api",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "api",
										Image: "test-image",
										Ports: []corev1.ContainerPort{
											{
												ContainerPort: 8080,
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

				// Create a service for the deployment
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "test-api",
						},
						Ports: []corev1.ServicePort{
							{
								Port:       8080,
								TargetPort: intstr.FromInt(8080),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, service)).To(Succeed())
			})

			It("should collect API specs when deployment becomes ready", func() {
				By("Updating deployment status to ready")
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-api", Namespace: namespace}, deployment)).To(Succeed())

				deployment.Status = appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				}
				Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())

				By("Reconciling the aggregator")
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(5 * time.Minute))

				By("Verifying the aggregator status")
				aggregator := &observabilityv1alpha1.OpenAPIAggregator{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, aggregator)).To(Succeed())

				Expect(aggregator.Status.CollectedAPIs).To(HaveLen(1))
				api := aggregator.Status.CollectedAPIs[0]
				Expect(api.Name).To(Equal("Test-test-api"))
				Expect(api.ResourceType).To(Equal("Deployment"))
				Expect(api.Namespace).To(Equal(namespace))
				Expect(api.Annotations).To(HaveKeyWithValue("openapi.path", "/v3/api-docs"))
			})

			It("should handle deployment updates", func() {
				By("Initially setting deployment as ready")
				deployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-api", Namespace: namespace}, deployment)).To(Succeed())

				deployment.Status = appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				}
				Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())

				By("First reconciliation")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Updating deployment annotations")
				deployment = &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-api", Namespace: namespace}, deployment)).To(Succeed())

				deployment.Annotations["openapi.path"] = "/swagger/v3/api-docs"
				Expect(k8sClient.Update(ctx, deployment)).To(Succeed())

				By("Second reconciliation")
				_, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying updated aggregator status")
				aggregator := &observabilityv1alpha1.OpenAPIAggregator{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, aggregator)).To(Succeed())

				Expect(aggregator.Status.CollectedAPIs).To(HaveLen(1))
				api := aggregator.Status.CollectedAPIs[0]
				Expect(api.Annotations).To(HaveKeyWithValue("openapi.path", "/swagger/v3/api-docs"))
			})
		})

		Context("With invalid configurations", func() {
			It("should handle deployments without required annotations", func() {
				By("Creating deployment without annotations")
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
						Labels: map[string]string{
							"app": "test-api",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-api",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-api",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "api",
										Image: "test-image",
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

				By("Reconciling the aggregator")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the aggregator status")
				aggregator := &observabilityv1alpha1.OpenAPIAggregator{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, aggregator)).To(Succeed())
				Expect(aggregator.Status.CollectedAPIs).To(BeEmpty())
			})

			It("should handle non-existent service", func() {
				By("Creating deployment without service")
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-api",
						Namespace: namespace,
						Labels: map[string]string{
							"app": "test-api",
						},
						Annotations: map[string]string{
							"openapi.path": "/v3/api-docs",
							"openapi.port": "8080",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test-api",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test-api",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "api",
										Image: "test-image",
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

				By("Reconciling the aggregator")
				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying the aggregator status")
				aggregator := &observabilityv1alpha1.OpenAPIAggregator{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, aggregator)).To(Succeed())
				Expect(aggregator.Status.CollectedAPIs).To(BeEmpty())
			})
		})
	})
})
