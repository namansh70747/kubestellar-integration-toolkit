package integration

import (
	"context"
	"encoding/base64"
	"fmt"
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
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = ksitv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create test namespace
	testNamespace = "default"
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
		},
	}
	err = k8sClient.Create(ctx, ns)
	if err != nil {
		// Namespace might already exist, ignore error
		logf.Log.Info("namespace may already exist", "namespace", testNamespace)
	}

	// ✅ CREATE KUBECONFIG SECRET FOR TEST CLUSTER
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
	err = k8sClient.Create(ctx, kubeconfigSecret)
	Expect(err).NotTo(HaveOccurred())
	logf.Log.Info("✅ created kubeconfig secret", "name", "default-kubeconfig")

	// ✅ CREATE INTEGRATIONTARGET FOR TEST CLUSTER
	integrationTarget := &ksitv1alpha1.IntegrationTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: testNamespace,
		},
		Spec: ksitv1alpha1.IntegrationTargetSpec{
			ClusterName: "default",
			Namespace:   testNamespace,
			Labels: map[string]string{
				"environment": "test",
			},
		},
	}
	err = k8sClient.Create(ctx, integrationTarget)
	Expect(err).NotTo(HaveOccurred())
	logf.Log.Info("✅ created IntegrationTarget", "name", "default")

	// Start manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// ✅ CREATE CLUSTER MANAGER AND REGISTER TEST CLUSTER
	clusterMgr = cluster.NewClusterManager(k8sManager.GetClient())

	// Register test cluster in ClusterManager
	err = clusterMgr.AddCluster("default", testNamespace, string(kubeconfigData))
	Expect(err).NotTo(HaveOccurred())
	logf.Log.Info("✅ registered cluster in ClusterManager", "cluster", "default", "namespace", testNamespace)

	// Verify cluster is registered
	registeredClusters := clusterMgr.ListClusters()
	Expect(len(registeredClusters)).To(Equal(1))
	Expect(registeredClusters[0].Name).To(Equal("default"))
	logf.Log.Info("✅ verified cluster registration", "clusters", len(registeredClusters))

	// Setup IntegrationTarget reconciler
	integrationTargetReconciler := &controller.IntegrationTargetReconciler{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		Log:            ctrl.Log.WithName("controllers").WithName("IntegrationTarget"),
		ClusterManager: clusterMgr,
	}
	err = integrationTargetReconciler.SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	// Setup Integration reconciler with ClusterManager
	integrationReconciler := &controller.IntegrationReconciler{
		Client:           k8sManager.GetClient(),
		Scheme:           k8sManager.GetScheme(),
		Log:              ctrl.Log.WithName("controllers").WithName("Integration"),
		ClusterManager:   clusterMgr,
		ClusterInventory: cluster.NewClusterInventory(),
	}
	err = integrationReconciler.SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()

	// Wait for manager to be ready
	time.Sleep(2 * time.Second)

	logf.Log.Info("✅ test environment setup complete")
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// ✅ Helper function to create kubeconfig from rest.Config
func createKubeconfigFromRestConfig(config *rest.Config) []byte {
	// Base64 encode certificate data
	caData := ""
	if len(config.CAData) > 0 {
		caData = base64.StdEncoding.EncodeToString(config.CAData)
	}

	certData := ""
	if len(config.CertData) > 0 {
		certData = base64.StdEncoding.EncodeToString(config.CertData)
	}

	keyData := ""
	if len(config.KeyData) > 0 {
		keyData = base64.StdEncoding.EncodeToString(config.KeyData)
	}

	// If no cert data, use token or other auth
	userAuth := ""
	if certData != "" && keyData != "" {
		userAuth = fmt.Sprintf(`    client-certificate-data: %s
    client-key-data: %s`, certData, keyData)
	} else if config.BearerToken != "" {
		userAuth = fmt.Sprintf(`    token: %s`, config.BearerToken)
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
%s
`, caData, config.Host, userAuth)

	return []byte(kubeconfig)
}
