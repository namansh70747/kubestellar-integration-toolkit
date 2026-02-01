package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	k8sScheme *runtime.Scheme
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)

	By("bootstrapping test environment")

	// Set up test environment
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: false,                                              // ✅ Don't fail if CRDs missing
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s", "current"), // ✅ ADD THIS
	}

	// Start test environment
	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Set up scheme
	k8sScheme = runtime.NewScheme()
	err = scheme.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	err = ksitv1alpha1.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	// Create client
	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sScheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	logf.Log.Info("Test environment started successfully")
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})
