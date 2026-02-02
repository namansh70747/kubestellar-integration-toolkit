package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/kubestellar/integration-toolkit/pkg/installer"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	testEnv       *envtest.Environment
	ctx           context.Context
	cancel        context.CancelFunc
	clusterMgr    *cluster.ClusterManager
	testNamespace string
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")

	// Get project root
	projectRoot, projectErr := filepath.Abs(filepath.Join("..", ".."))
	Expect(projectErr).NotTo(HaveOccurred())

	// Set KUBEBUILDER_ASSETS
	envtestPath := os.Getenv("KUBEBUILDER_ASSETS")
	if envtestPath == "" {
		envtestPath = filepath.Join(projectRoot, "bin", "k8s", "k8s", "1.29.5-darwin-arm64")
		if _, statErr := os.Stat(envtestPath); os.IsNotExist(statErr) {
			envtestPath = filepath.Join(projectRoot, "bin", "k8s", "k8s", "1.29.5-darwin-amd64")
		}
		os.Setenv("KUBEBUILDER_ASSETS", envtestPath)
	}
	logf.Log.Info("using KUBEBUILDER_ASSETS", "path", envtestPath)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(projectRoot, "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: envtestPath,
	}

	// Start test environment
	var startErr error
	cfg, startErr = testEnv.Start()
	Expect(startErr).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Add scheme
	schemeErr := ksitv1alpha1.AddToScheme(scheme.Scheme)
	Expect(schemeErr).NotTo(HaveOccurred())

	// Create client
	var clientErr error
	k8sClient, clientErr = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(clientErr).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create test namespace
	testNamespace = "default"
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	nsErr := k8sClient.Create(ctx, ns)
	if nsErr != nil {
		logf.Log.Info("namespace may already exist", "namespace", testNamespace, "error", nsErr)
	}

	// Create kubeconfig secret
	kubeconfigData := createKubeconfigFromRestConfig(cfg)
	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-kubeconfig",
			Namespace: testNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfigData,
		},
	}
	secretErr := k8sClient.Create(ctx, kubeconfigSecret)
	if secretErr != nil {
		logf.Log.Info("kubeconfig secret may already exist", "error", secretErr)
	}

	// Create IntegrationTarget
	integrationTarget := &ksitv1alpha1.IntegrationTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: testNamespace,
		},
		Spec: ksitv1alpha1.IntegrationTargetSpec{
			ClusterName: "default",
		},
	}
	targetErr := k8sClient.Create(ctx, integrationTarget)
	if targetErr != nil {
		logf.Log.Info("integration target may already exist", "error", targetErr)
	}

	// Start manager
	var mgrErr error
	k8sManager, mgrErr := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(mgrErr).NotTo(HaveOccurred())

	// Create cluster manager and register test cluster
	clusterMgr = cluster.NewClusterManager(k8sManager.GetClient())
	logf.Log.Info("✅ created cluster manager")

	registerErr := clusterMgr.AddCluster("default", testNamespace, string(kubeconfigData))
	Expect(registerErr).NotTo(HaveOccurred())
	logf.Log.Info("✅ registered test cluster", "cluster", "default", "namespace", testNamespace)

	// Verify cluster registration
	testCluster, getErr := clusterMgr.GetCluster("default", testNamespace)
	Expect(getErr).NotTo(HaveOccurred())
	Expect(testCluster).NotTo(BeNil())
	logf.Log.Info("✅ verified cluster registration", "cluster", testCluster.Name)

	// Setup IntegrationTarget reconciler
	integrationTargetReconciler := &controller.IntegrationTargetReconciler{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		Log:            ctrl.Log.WithName("IntegrationTarget"),
		ClusterManager: clusterMgr,
	}
	targetSetupErr := integrationTargetReconciler.SetupWithManager(k8sManager)
	Expect(targetSetupErr).NotTo(HaveOccurred())

	// Create installer factory
	installerFactory := installer.NewInstallerFactory()
	logf.Log.Info("✅ created installer factory")

	// Setup Integration reconciler
	integrationReconciler := &controller.IntegrationReconciler{
		Client:           k8sManager.GetClient(),
		Scheme:           k8sManager.GetScheme(),
		Log:              ctrl.Log.WithName("Integration"),
		ClusterManager:   clusterMgr,
		ClusterInventory: cluster.NewClusterInventory(),
		InstallerFactory: installerFactory,
	}
	integrationSetupErr := integrationReconciler.SetupWithManager(k8sManager)
	Expect(integrationSetupErr).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		startMgrErr := k8sManager.Start(ctx)
		Expect(startMgrErr).NotTo(HaveOccurred(), "failed to run manager")
	}()

	// Wait for manager to be ready
	time.Sleep(2 * time.Second)
	logf.Log.Info("✅ BeforeSuite completed successfully")
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	stopErr := testEnv.Stop()
	Expect(stopErr).NotTo(HaveOccurred())
})

func createKubeconfigFromRestConfig(config *rest.Config) []byte {
	var caData, certData, keyData string
	if len(config.CAData) > 0 {
		caData = base64.StdEncoding.EncodeToString(config.CAData)
	}
	if len(config.CertData) > 0 {
		certData = base64.StdEncoding.EncodeToString(config.CertData)
	}
	if len(config.KeyData) > 0 {
		keyData = base64.StdEncoding.EncodeToString(config.KeyData)
	}

	if caData == "" || certData == "" || keyData == "" {
		kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: %s
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`, config.Host)
		return []byte(kubeconfig)
	}

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    client-certificate-data: %s
    client-key-data: %s
`, caData, config.Host, certData, keyData)
	return []byte(kubeconfig)
}
