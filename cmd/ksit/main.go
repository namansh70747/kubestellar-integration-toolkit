package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
	internalwebhook "github.com/kubestellar/integration-toolkit/internal/webhook"
	"github.com/kubestellar/integration-toolkit/pkg/cluster"
	"github.com/kubestellar/integration-toolkit/pkg/config"
	"github.com/kubestellar/integration-toolkit/pkg/controller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ksitv1alpha1.AddToScheme(scheme))
}

func main() {
	var configFile string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableWebhook bool
	var webhookPort int
	var certDir string

	flag.StringVar(&configFile, "config", "", "Path to configuration file")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.BoolVar(&enableWebhook, "enable-webhook", false, "Enable validating webhooks.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "Webhook server port.")
	flag.StringVar(&certDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs", "Webhook certificate directory.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Load config
	var cfg *config.Config
	if configFile != "" {
		var err error
		cfg, err = config.LoadConfig(configFile)
		if err != nil {
			setupLog.Error(err, "failed to load config file")
			cfg = config.NewDefaultConfig()
		}
	} else {
		cfg = config.NewDefaultConfig()
	}

	// Use config values
	if cfg.MetricsAddr != "" {
		metricsAddr = cfg.MetricsAddr
	}
	if cfg.ProbeAddr != "" {
		probeAddr = cfg.ProbeAddr
	}
	enableLeaderElection = cfg.LeaderElection

	webhookServerOptions := webhook.Options{
		Port:    webhookPort,
		CertDir: certDir,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ksit-leader-election",
		WebhookServer:          webhook.NewServer(webhookServerOptions),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// ✅ OPTION A: Use Manager Wrapper (Optional)
	// Uncomment these lines to use the wrapper:
	/*
	   ctrlManager := controller.NewManager(mgr, ctrl.Log)
	   // Manager wrapper will setup controllers internally
	   // No need to call SetupWithManager directly
	*/

	// ✅ OPTION B: Direct Setup (Current approach)
	// Create shared ClusterManager and ClusterInventory
	clusterMgr := cluster.NewClusterManager(mgr.GetClient())
	clusterInv := cluster.NewClusterInventory()

	if err = (&controller.IntegrationReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		Log:              ctrl.Log.WithName("controllers").WithName("Integration"),
		ClusterManager:   clusterMgr,
		ClusterInventory: clusterInv,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Integration")
		os.Exit(1)
	}

	if err = (&controller.IntegrationTargetReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            ctrl.Log.WithName("controllers").WithName("IntegrationTarget"),
		ClusterManager: clusterMgr,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IntegrationTarget")
		os.Exit(1)
	}

	// Setup webhooks if enabled
	if enableWebhook {
		setupLog.Info("setting up webhooks")
		if err := internalwebhook.SetupWebhookServer(mgr); err != nil {
			setupLog.Error(err, "unable to setup webhooks")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
