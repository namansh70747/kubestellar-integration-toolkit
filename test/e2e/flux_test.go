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

var _ = Describe("Flux Integration E2E Tests", func() {
	const (
		integrationName      = "test-flux-integration"
		integrationNamespace = "ksit-e2e-flux"
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

	Context("When creating a Flux integration", func() {
		It("Should create Integration resource successfully", func() {
			By("creating Integration")
			integration := &ksitv1alpha1.Integration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      integrationName,
					Namespace: testNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":      "flux-integration",
						"app.kubernetes.io/component": "integration",
					},
				},
				Spec: ksitv1alpha1.IntegrationSpec{
					Type:    ksitv1alpha1.IntegrationTypeFlux,
					Enabled: true,
					TargetClusters: []string{
						"cluster1",
						"cluster2",
					},
					Config: map[string]string{
						"namespace": "flux-system",
						"interval":  "5m",
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

			Expect(createdIntegration.Spec.Type).Should(Equal(ksitv1alpha1.IntegrationTypeFlux))
			Expect(createdIntegration.Spec.Enabled).Should(BeTrue())
			Expect(createdIntegration.Spec.Config["namespace"]).Should(Equal("flux-system"))
		})

		It("Should reconcile Flux resources", func() {
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
	})

	Context("When managing Flux resources", func() {
		It("Should create GitRepository resources", func() {
			Skip("Requires Flux CRDs to be installed")
		})

		It("Should create Kustomization resources", func() {
			Skip("Requires Flux CRDs to be installed")
		})
	})

	Context("When deleting a Flux integration", func() {
		It("Should cleanup properly", func() {
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
})
