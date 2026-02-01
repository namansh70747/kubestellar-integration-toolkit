package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
	"github.com/kubestellar/integration-toolkit/pkg/cluster"
	"github.com/kubestellar/integration-toolkit/pkg/integrations/argocd"
	"github.com/kubestellar/integration-toolkit/pkg/integrations/flux"
	"github.com/kubestellar/integration-toolkit/pkg/integrations/istio"
	"github.com/kubestellar/integration-toolkit/pkg/integrations/prometheus"
)

const (
	integrationFinalizer = "ksit.io/finalizer"
	requeueInterval      = 30 * time.Second
)

type IntegrationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	ClusterManager *cluster.ClusterManager // ✅ ADD THIS
}

func (r *IntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("integration", req.NamespacedName)
	log.Info("reconciling integration")

	startTime := time.Now()

	integration := &ksitv1alpha1.Integration{}
	if err := r.Get(ctx, req.NamespacedName, integration); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !integration.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(integration, integrationFinalizer) {
			if err := r.cleanupIntegration(ctx, integration); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(integration, integrationFinalizer)
			if err := r.Update(ctx, integration); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(integration, integrationFinalizer) {
		controllerutil.AddFinalizer(integration, integrationFinalizer)
		if err := r.Update(ctx, integration); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Skip if disabled
	if !integration.Spec.Enabled {
		integration.Status.Phase = ksitv1alpha1.PhaseFailed
		integration.Status.Message = "Integration is disabled"
		r.Status().Update(ctx, integration)
		return ctrl.Result{}, nil
	}

	// Update status to Initializing
	if integration.Status.Phase == "" {
		integration.Status.Phase = ksitv1alpha1.PhaseInitializing
		r.Status().Update(ctx, integration)
	}

	// Reconcile based on type
	var reconcileErr error
	switch integration.Spec.Type {
	case ksitv1alpha1.IntegrationTypeArgoCD:
		reconcileErr = r.reconcileArgoCD(ctx, integration)
	case ksitv1alpha1.IntegrationTypeFlux:
		reconcileErr = r.reconcileFlux(ctx, integration)
	case ksitv1alpha1.IntegrationTypePrometheus:
		reconcileErr = r.reconcilePrometheus(ctx, integration)
	case ksitv1alpha1.IntegrationTypeIstio:
		reconcileErr = r.reconcileIstio(ctx, integration)
	default:
		reconcileErr = fmt.Errorf("unsupported integration type: %s", integration.Spec.Type)
	}

	// Record reconcile duration
	duration := time.Since(startTime).Seconds()
	prometheus.RecordReconcileDuration(integration.Name, integration.Spec.Type, duration)

	// Update status based on result
	now := metav1.Now()
	integration.Status.LastReconcileTime = &now
	integration.Status.ObservedGeneration = integration.Generation

	if reconcileErr != nil {
		integration.Status.Phase = ksitv1alpha1.PhaseFailed
		integration.Status.Message = reconcileErr.Error()
		prometheus.RecordReconcile(integration.Name, integration.Spec.Type, "failed")

		// Update Ready condition
		meta.SetStatusCondition(&integration.Status.Conditions, metav1.Condition{
			Type:    ksitv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "ReconcileFailed",
			Message: reconcileErr.Error(),
		})
	} else {
		integration.Status.Phase = ksitv1alpha1.PhaseRunning
		integration.Status.Message = "Integration is running"
		prometheus.RecordReconcile(integration.Name, integration.Spec.Type, "success")

		// Update Ready condition
		meta.SetStatusCondition(&integration.Status.Conditions, metav1.Condition{
			Type:    ksitv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  "ReconcileSucceeded",
			Message: "Integration is healthy",
		})

		// Update cluster statuses
		for _, clusterName := range integration.Spec.TargetClusters {
			prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, clusterName, true)
		}
	}

	r.Status().Update(ctx, integration)

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *IntegrationReconciler) reconcileArgoCD(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling ArgoCD integration", "name", integration.Name)
	startTime := time.Now()

	argoClient, err := argocd.NewClient(r.Client, integration.Spec.Config)
	if err != nil {
		return fmt.Errorf("failed to create ArgoCD client: %w", err)
	}

	// Perform health check first
	if err := argoClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("ArgoCD health check failed: %w", err)
	}
	r.Log.Info("ArgoCD health check passed")

	// Sync applications for each target cluster
	for _, cluster := range integration.Spec.TargetClusters {
		if err := argoClient.SyncCluster(ctx, cluster); err != nil {
			r.Log.Error(err, "failed to sync cluster", "cluster", cluster)
			prometheus.RecordSyncOperation(integration.Name, cluster, "failed")
		} else {
			prometheus.RecordSyncOperation(integration.Name, cluster, "success")
		}

		latency := time.Since(startTime).Seconds()
		prometheus.RecordSyncLatency(integration.Name, cluster, latency)
	}

	return nil
}

func (r *IntegrationReconciler) reconcileFlux(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Flux integration", "name", integration.Name)

	fluxClient := flux.NewFluxClient(r.Client, r.Scheme, r.Log)

	// List GitRepositories to verify Flux is working
	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = "flux-system"
	}

	gitRepos, err := fluxClient.ListGitRepositories(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to list GitRepositories: %w", err)
	}
	r.Log.Info("found GitRepositories", "count", len(gitRepos))

	// List Kustomizations
	kustomizations, err := fluxClient.ListKustomizations(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to list Kustomizations: %w", err)
	}
	r.Log.Info("found Kustomizations", "count", len(kustomizations))

	// Record metrics for each target cluster
	for _, cluster := range integration.Spec.TargetClusters {
		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, true)
	}

	return nil
}

func (r *IntegrationReconciler) reconcilePrometheus(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Prometheus integration", "name", integration.Name)

	promURL := integration.Spec.Config["url"]
	if promURL == "" {
		return fmt.Errorf("Prometheus URL not configured")
	}

	promClient, err := prometheus.NewClient(promURL)
	if err != nil {
		return fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	// Validate connection
	if err := promClient.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("Prometheus connection failed: %w", err)
	}

	// Get targets to verify scraping is working
	targets, err := promClient.GetTargets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Prometheus targets: %w", err)
	}
	r.Log.Info("Prometheus targets", "active", len(targets.Active), "dropped", len(targets.Dropped))

	// Record metrics for each target cluster
	for _, cluster := range integration.Spec.TargetClusters {
		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, true)
	}

	return nil
}

func (r *IntegrationReconciler) reconcileIstio(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Istio integration", "name", integration.Name)

	istioClient, err := istio.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Istio client: %w", err)
	}

	// Health check
	if err := istioClient.HealthCheck(); err != nil {
		return fmt.Errorf("Istio health check failed: %w", err)
	}

	// Configure mesh if mTLS is enabled
	if integration.Spec.Config["enableMTLS"] == "true" {
		mesh := istio.NewServiceMesh(r.Client)
		meshConfig := &istio.MeshConfig{
			Name:           integration.Name,
			Namespace:      integration.Spec.Config["namespace"],
			EnableAutoMTLS: true,
		}
		if err := mesh.ConfigureMesh(ctx, meshConfig); err != nil {
			r.Log.Error(err, "failed to configure mesh")
		}
	}

	// Record metrics for each target cluster
	for _, cluster := range integration.Spec.TargetClusters {
		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, true)
	}

	return nil
}

func (r *IntegrationReconciler) cleanupIntegration(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("cleaning up integration", "name", integration.Name)

	// Update metrics to show integration is down
	for _, cluster := range integration.Spec.TargetClusters {
		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, false)
	}

	// Type-specific cleanup
	switch integration.Spec.Type {
	case ksitv1alpha1.IntegrationTypeArgoCD:
		// ArgoCD cleanup if needed
	case ksitv1alpha1.IntegrationTypeFlux:
		// Flux cleanup if needed
	case ksitv1alpha1.IntegrationTypePrometheus:
		// Prometheus cleanup if needed
	case ksitv1alpha1.IntegrationTypeIstio:
		// Istio cleanup if needed
	}

	return nil
}

func (r *IntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// ✅ INITIALIZE ClusterManager
	r.ClusterManager = cluster.NewClusterManager(r.Client)

	return ctrl.NewControllerManagedBy(mgr).
		For(&ksitv1alpha1.Integration{}).
		Complete(r)
}

// IntegrationTargetReconciler reconciles IntegrationTarget objects
type IntegrationTargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

func (r *IntegrationTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ✅ FIXED: Use the log variable
	r.Log.Info("reconciling integration target", "name", req.NamespacedName)

	target := &ksitv1alpha1.IntegrationTarget{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "failed to get integration target")
		return ctrl.Result{}, err
	}

	// Update status
	target.Status.Ready = true
	target.Status.Message = "Target is ready"

	// Add Ready condition
	meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "TargetReady",
		Message: "IntegrationTarget is ready",
	})

	if err := r.Status().Update(ctx, target); err != nil {
		r.Log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	// Update metrics
	prometheus.SetClusterConnectionStatus(target.Spec.ClusterName, true)

	r.Log.Info("successfully reconciled integration target", "name", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *IntegrationTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ksitv1alpha1.IntegrationTarget{}).
		Complete(r)
}
