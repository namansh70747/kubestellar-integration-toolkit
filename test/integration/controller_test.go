package integration

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
	"github.com/kubestellar/integration-toolkit/pkg/controller"
)

var _ = Describe("Integration Controller Tests", func() {
	const (
		IntegrationName      = "test-integration"
		IntegrationNamespace = "default"
		timeout              = time.Second * 30
		interval             = time.Millisecond * 250
	)

	Context("When reconciling an Integration", func() {
		It("Should create Integration successfully", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "ksit.io/v1alpha1",
					Kind:       "Integration",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      IntegrationName,
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeArgoCD,
					Enabled: true,
					TargetClusters: []string{
						"cluster1",
					},
					Config: map[string]string{
						"serverURL": "https://argocd.example.com",
						"namespace": "argocd",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())

			integrationLookupKey := types.NamespacedName{
				Name:      IntegrationName,
				Namespace: IntegrationNamespace,
			}
			createdIntegration := &ksitv1alpha1.Integration{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, integrationLookupKey, createdIntegration)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdIntegration.Spec.Type).Should(Equal(ksitv1alpha1.IntegrationTypeArgoCD))
			Expect(createdIntegration.Spec.Enabled).Should(BeTrue())
		})

		It("Should update Integration status", func() {
			ctx := context.Background()

			integrationLookupKey := types.NamespacedName{
				Name:      IntegrationName,
				Namespace: IntegrationNamespace,
			}
			integration := &ksitv1alpha1.Integration{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, integrationLookupKey, integration)
				if err != nil {
					return false
				}
				return integration.Status.Phase != ""
			}, timeout, interval).Should(BeTrue())
		})

		It("Should handle disabled Integration", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "disabled-integration",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeFlux,
					Enabled: false,
					Config: map[string]string{
						"namespace": "flux-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())
		})

		It("Should delete Integration successfully", func() {
			ctx := context.Background()

			integrationLookupKey := types.NamespacedName{
				Name:      IntegrationName,
				Namespace: IntegrationNamespace,
			}
			integration := &ksitv1alpha1.Integration{}

			Expect(k8sClient.Get(ctx, integrationLookupKey, integration)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, integration)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, integrationLookupKey, &ksitv1alpha1.Integration{})
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When reconciling an IntegrationTarget", func() {
		It("Should create IntegrationTarget successfully", func() {
			ctx := context.Background()

			target := &ksitv1alpha1.IntegrationTarget{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "ksit.io/v1alpha1",
					Kind:       "IntegrationTarget",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-target",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationTargetSpec{
					ClusterName: "test-cluster",
					Namespace:   "default",
					Labels: map[string]string{
						"environment": "test",
					},
				},
			}

			Expect(k8sClient.Create(ctx, target)).Should(Succeed())

			targetLookupKey := types.NamespacedName{
				Name:      "test-target",
				Namespace: IntegrationNamespace,
			}
			createdTarget := &ksitv1alpha1.IntegrationTarget{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, targetLookupKey, createdTarget)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdTarget.Spec.ClusterName).Should(Equal("test-cluster"))
		})

		It("Should update IntegrationTarget status", func() {
			ctx := context.Background()

			targetLookupKey := types.NamespacedName{
				Name:      "test-target",
				Namespace: IntegrationNamespace,
			}
			target := &ksitv1alpha1.IntegrationTarget{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, targetLookupKey, target)
				if err != nil {
					return false
				}
				return target.Status.Ready
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When testing reconciler setup", func() {
		It("Should setup IntegrationReconciler with manager", func() {
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: k8sScheme,
			})
			Expect(err).NotTo(HaveOccurred())

			reconciler := &controller.IntegrationReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				Log:    ctrl.Log.WithName("test"),
			}

			err = reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should setup IntegrationTargetReconciler with manager", func() {
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: k8sScheme,
			})
			Expect(err).NotTo(HaveOccurred())

			reconciler := &controller.IntegrationTargetReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				Log:    ctrl.Log.WithName("test"),
			}

			err = reconciler.SetupWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When testing multiple integration types", func() {
		It("Should create ArgoCD integration", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-integration",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeArgoCD,
					Enabled: true,
					Config: map[string]string{
						"serverURL": "https://argocd.example.com",
						"namespace": "argocd",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())
		})

		It("Should create Flux integration", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "flux-integration",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeFlux,
					Enabled: true,
					Config: map[string]string{
						"namespace": "flux-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())
		})

		It("Should create Prometheus integration", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prometheus-integration",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypePrometheus,
					Enabled: true,
					Config: map[string]string{
						"url": "http://prometheus.monitoring:9090",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())
		})

		It("Should create Istio integration", func() {
			ctx := context.Background()

			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-integration",
					Namespace: IntegrationNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeIstio,
					Enabled: true,
					Config: map[string]string{
						"namespace": "istio-system",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())
		})
	})
})
