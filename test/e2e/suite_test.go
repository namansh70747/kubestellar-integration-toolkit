package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

var (
	cfg        *rest.Config
	k8sClient  client.Client
	kubeClient *kubernetes.Clientset
	ctx        context.Context
	cancel     context.CancelFunc
	scheme     *runtime.Scheme
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	By("setting up kubernetes client")

	// Get kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())
		kubeconfig = home + "/.kube/config"
	}

	// Build config
	var err error
	cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Create kubernetes clientset
	kubeClient, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(kubeClient).NotTo(BeNil())

	// Create scheme
	scheme = runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(ksitv1alpha1.AddToScheme(scheme)).To(Succeed())

	// Create controller-runtime client
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By("verifying cluster connection")
	_, err = kubeClient.Discovery().ServerVersion()
	Expect(err).NotTo(HaveOccurred())

	logf.Log.Info("E2E test suite initialized successfully")
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
})

// Helper functions

// CreateNamespace creates a test namespace
func CreateNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return k8sClient.Create(ctx, ns)
}

// DeleteNamespace deletes a test namespace
func DeleteNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return k8sClient.Delete(ctx, ns)
}

// WaitForIntegrationReady waits for integration to be ready
func WaitForIntegrationReady(name, namespace string, timeout time.Duration) error {
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()
		case <-ticker.C:
			integration := &ksitv1alpha1.Integration{}
			if err := k8sClient.Get(ctx, client.ObjectKey{
				Name:      name,
				Namespace: namespace,
			}, integration); err != nil {
				continue
			}

			if integration.Status.Phase == ksitv1alpha1.PhaseRunning {
				return nil
			}
		}
	}
}
