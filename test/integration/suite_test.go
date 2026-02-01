package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
	"github.com/kubestellar/integration-toolkit/pkg/cluster"
	"github.com/kubestellar/integration-toolkit/pkg/controller"
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

	// ✅ FIX: Get absolute path to project root
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	Expect(err).NotTo(HaveOccurred())

	// ✅ FIX: Check KUBEBUILDER_ASSETS environment variable first
	binaryAssetsDir := os.Getenv("KUBEBUILDER_ASSETS")

	if binaryAssetsDir == "" {
		// If not set, look for binaries in project bin directory
		possiblePaths := []string{
			filepath.Join(projectRoot, "bin", "k8s", "k8s", "1.29.5-darwin-arm64"),
			filepath.Join(projectRoot, "bin", "k8s", "1.29.0-darwin-arm64"),
		}

		for _, path := range possiblePaths {
			if _, statErr := os.Stat(filepath.Join(path, "etcd")); statErr == nil {
				binaryAssetsDir = path
				logf.Log.Info("Found envtest binaries", "path", binaryAssetsDir)
				break
			}
		}

		if binaryAssetsDir == "" {
			Fail("❌ Envtest binaries not found. Please run: make test-integration or export KUBEBUILDER_ASSETS=$(setup-envtest use 1.29.x --bin-dir ./bin/k8s -p path)")
		}
	} else {
		// ✅ FIX: Convert to absolute path if it's relative
		if !filepath.IsAbs(binaryAssetsDir) {
			binaryAssetsDir = filepath.Join(projectRoot, binaryAssetsDir)
		}
	}

	logf.Log.Info("Using envtest binaries", "path", binaryAssetsDir)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(projectRoot, "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: false,
		BinaryAssetsDirectory: binaryAssetsDir,
	}

	logf.Log.Info("Starting test environment", "binaryPath", binaryAssetsDir)

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sScheme = runtime.NewScheme()
	err = scheme.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	err = ksitv1alpha1.AddToScheme(k8sScheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: k8sScheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// ✅ START MANAGER WITH CONTROLLERS
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: k8sScheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable metrics server in tests
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// Setup Integration reconciler
	integrationReconciler := &controller.IntegrationReconciler{
		Client:           k8sManager.GetClient(),
		Scheme:           k8sManager.GetScheme(),
		Log:              ctrl.Log.WithName("controllers").WithName("Integration"),
		ClusterManager:   cluster.NewClusterManager(k8sManager.GetClient()),
		ClusterInventory: cluster.NewClusterInventory(),
	}
	err = integrationReconciler.SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	// Setup IntegrationTarget reconciler
	targetReconciler := &controller.IntegrationTargetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("IntegrationTarget"),
	}
	err = targetReconciler.SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	// Start manager in background
	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()

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
