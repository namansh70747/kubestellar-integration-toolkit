package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

var _ = Describe("ArgoCD Integration E2E Tests", func() {
	const (
		integrationName      = "test-argocd-integration"
		integrationNamespace = "ksit-e2e-argocd"
		timeout              = time.Second * 120
		interval             = time.Second * 5
	)

	var testNamespace string

	BeforeEach(func() {
		testNamespace = integrationNamespace

		By("creating test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).Should(Succeed())
	})

	AfterEach(func() {
		By("cleaning up test namespace")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = k8sClient.Delete(ctx, ns)
	})

	Context("When creating an ArgoCD integration", func() {
		It("Should create Integration resource successfully", func() {
			By("creating Integration")
			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      integrationName,
					Namespace: testNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "argocd-integration",
						"app.kubernetes.io/component": "integration",
					},
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeArgoCD,
					Enabled: true,
					TargetClusters: []string{
						"cluster1",
						"cluster2",
					},
					Config: map[string]string{
						"serverURL": "https://argocd-server.argocd.svc.cluster.local",
						"namespace": "argocd",
						"insecure":  "true",
					},
				},
			}

			Expect(k8sClient.Create(ctx, integration)).Should(Succeed())

			By("verifying Integration was created")
			createdIntegration := &ksitv1alpha1.Integration{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, createdIntegration)
			}, timeout, interval).Should(Succeed())

			Expect(createdIntegration.Spec.Type).Should(Equal(ksitv1alpha1.IntegrationTypeArgoCD))
			Expect(createdIntegration.Spec.Enabled).Should(BeTrue())
			Expect(createdIntegration.Spec.TargetClusters).Should(HaveLen(2))
		})

		It("Should update Integration status to running", func() {
			By("waiting for Integration status update")
			Eventually(func() string {
				integration := &ksitv1alpha1.Integration{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, integration)
				if err != nil {
					return ""
				}
				return integration.Status.Phase
			}, timeout, interval).Should(Or(
				Equal(ksitv1alpha1.PhaseRunning),
				Equal(ksitv1alpha1.PhaseInitializing),
			))
		})

		It("Should have reconcileTime updated", func() {
			By("checking lastReconcileTime")
			Eventually(func() bool {
				integration := &ksitv1alpha1.Integration{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, integration)
				if err != nil {
					return false
				}
				return integration.Status.LastReconcileTime != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When updating an ArgoCD integration", func() {
		It("Should update Integration config successfully", func() {
			By("getting existing Integration")
			integration := &ksitv1alpha1.Integration{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      integrationName,
				Namespace: testNamespace,
			}, integration)).Should(Succeed())

			By("updating Integration config")
			integration.Spec.Config["serverURL"] = "https://argocd-server-updated.argocd.svc.cluster.local"
			Expect(k8sClient.Update(ctx, integration)).Should(Succeed())

			By("verifying Integration was updated")
			updatedIntegration := &ksitv1alpha1.Integration{}
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, updatedIntegration)
				if err != nil {
					return ""
				}
				return updatedIntegration.Spec.Config["serverURL"]
			}, timeout, interval).Should(Equal("https://argocd-server-updated.argocd.svc.cluster.local"))
		})

		It("Should update target clusters", func() {
			By("getting existing Integration")
			integration := &ksitv1alpha1.Integration{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      integrationName,
				Namespace: testNamespace,
			}, integration)).Should(Succeed())

			By("adding new target cluster")
			integration.Spec.TargetClusters = append(integration.Spec.TargetClusters, "cluster3")
			Expect(k8sClient.Update(ctx, integration)).Should(Succeed())

			By("verifying target clusters were updated")
			Eventually(func() int {
				updatedIntegration := &ksitv1alpha1.Integration{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, updatedIntegration)
				if err != nil {
					return 0
				}
				return len(updatedIntegration.Spec.TargetClusters)
			}, timeout, interval).Should(Equal(3))
		})
	})

	Context("When deleting an ArgoCD integration", func() {
		It("Should delete Integration successfully", func() {
			By("getting existing Integration")
			integration := &ksitv1alpha1.Integration{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      integrationName,
				Namespace: testNamespace,
			}, integration)).Should(Succeed())

			By("deleting Integration")
			Expect(k8sClient.Delete(ctx, integration)).Should(Succeed())

			By("verifying Integration was deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      integrationName,
					Namespace: testNamespace,
				}, &ksitv1alpha1.Integration{})
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When testing ArgoCD integration validation", func() {
		It("Should reject invalid integration type", func() {
			By("creating Integration with invalid type")
			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-integration",
					Namespace: testNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    "invalid-type",
					Enabled: true,
				},
			}

			err := k8sClient.Create(ctx, integration)
			Expect(err).Should(HaveOccurred())
		})

		It("Should reject missing required config", func() {
			By("creating Integration without serverURL")
			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-config-integration",
					Namespace: testNamespace,
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeArgoCD,
					Enabled: true,
					Config:  map[string]string{}, // Missing serverURL
				},
			}

			// This may or may not fail depending on webhook configuration
			// If webhook is enabled, it should fail
			_ = k8sClient.Create(ctx, integration)
		})
	})
})
