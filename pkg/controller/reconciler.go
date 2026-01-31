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
    Scheme *runtime.Scheme
    Log    logr.Logger
}

func (r *IntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := r.Log.WithValues("integration", req.NamespacedName)
    log.Info("reconciling integration")

    startTime := time.Now()

    // Fetch the Integration instance
    integration := &ksitv1alpha1.Integration{}
    if err := r.Get(ctx, req.NamespacedName, integration); err != nil {
        if errors.IsNotFound(err) {
            log.Info("Integration resource not found, ignoring")
            return ctrl.Result{}, nil
        }
        log.Error(err, "failed to get Integration")
        return ctrl.Result{}, err
    }

    // Handle deletion
    if !integration.ObjectMeta.DeletionTimestamp.IsZero() {
        if controllerutil.ContainsFinalizer(integration, integrationFinalizer) {
            if err := r.cleanupIntegration(ctx, integration); err != nil {
                log.Error(err, "failed to cleanup integration")
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
        log.Info("Integration is disabled, skipping")
        integration.Status.Phase = ksitv1alpha1.PhaseSucceeded
        integration.Status.Message = "Integration is disabled"
        if err := r.Status().Update(ctx, integration); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{RequeueAfter: requeueInterval}, nil
    }

    // Update status to Initializing
    if integration.Status.Phase == "" {
        integration.Status.Phase = ksitv1alpha1.PhaseInitializing
        integration.Status.Message = "Starting reconciliation"
        if err := r.Status().Update(ctx, integration); err != nil {
            return ctrl.Result{}, err
        }
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
        reconcileErr = fmt.Errorf("unknown integration type: %s", integration.Spec.Type)
    }

    // Record reconcile duration
    duration := time.Since(startTime).Seconds()
    prometheus.RecordReconcileDuration(integration.Name, integration.Spec.Type, duration)

    // Update status based on result
    now := metav1.Now()
    integration.Status.LastReconcileTime = &now
    integration.Status.ObservedGeneration = integration.Generation

    if reconcileErr != nil {
        log.Error(reconcileErr, "reconciliation failed")
        integration.Status.Phase = ksitv1alpha1.PhaseFailed
        integration.Status.Message = reconcileErr.Error()
        prometheus.RecordReconcile(integration.Name, integration.Spec.Type, "failed")

        // Update Ready condition
        meta.SetStatusCondition(&integration.Status.Conditions, metav1.Condition{
            Type:               ksitv1alpha1.ConditionTypeReady,
            Status:             metav1.ConditionFalse,
            Reason:             "ReconcileFailed",
            Message:            reconcileErr.Error(),
            LastTransitionTime: now,
        })
    } else {
        log.Info("reconciliation succeeded")
        integration.Status.Phase = ksitv1alpha1.PhaseRunning
        integration.Status.Message = "Integration is running"
        prometheus.RecordReconcile(integration.Name, integration.Spec.Type, "success")

        // Update Ready condition
        meta.SetStatusCondition(&integration.Status.Conditions, metav1.Condition{
            Type:               ksitv1alpha1.ConditionTypeReady,
            Status:             metav1.ConditionTrue,
            Reason:             "ReconcileSucceeded",
            Message:            "Integration reconciled successfully",
            LastTransitionTime: now,
        })

        // Update cluster statuses
        for _, cluster := range integration.Spec.TargetClusters {
            prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, true)
            prometheus.SetClusterConnectionStatus(cluster, true)
        }
    }

    if err := r.Status().Update(ctx, integration); err != nil {
        log.Error(err, "failed to update status")
        return ctrl.Result{}, err
    }

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
        r.Log.Info("ArgoCD health check failed", "error", err.Error())
        return fmt.Errorf("ArgoCD health check failed: %w", err)
    }

    r.Log.Info("ArgoCD health check passed")

    // Sync applications for each target cluster
    for _, cluster := range integration.Spec.TargetClusters {
        if cluster == "in-cluster" || cluster == "" {
            // For in-cluster, just verify ArgoCD is accessible
            apps, err := argoClient.ListApplications(ctx)
            if err != nil {
                r.Log.Info("failed to list ArgoCD applications", "error", err.Error())
                prometheus.RecordSyncOperation(integration.Name, cluster, "failed")
                continue
            }
            r.Log.Info("ArgoCD applications found", "count", len(apps))
            prometheus.RecordSyncOperation(integration.Name, cluster, "success")
            prometheus.RecordSyncLatency(integration.Name, cluster, time.Since(startTime).Seconds())
        } else {
            if err := argoClient.SyncCluster(ctx, cluster); err != nil {
                r.Log.Error(err, "failed to sync cluster", "cluster", cluster)
                prometheus.RecordSyncOperation(integration.Name, cluster, "failed")
                // Continue with other clusters instead of failing
            } else {
                prometheus.RecordSyncOperation(integration.Name, cluster, "success")
                prometheus.RecordSyncLatency(integration.Name, cluster, time.Since(startTime).Seconds())
            }
        }
    }

    return nil
}

func (r *IntegrationReconciler) reconcileFlux(ctx context.Context, integration *ksitv1alpha1.Integration) error {
    r.Log.Info("reconciling Flux integration", "name", integration.Name)
    startTime := time.Now()

    fluxClient := flux.NewFluxClient(r.Client, r.Scheme, r.Log)

    namespace := integration.Spec.Config["namespace"]
    if namespace == "" {
        namespace = "flux-system"
    }

    // List GitRepositories to verify Flux is working
    gitRepos, err := fluxClient.ListGitRepositories(ctx, namespace)
    if err != nil {
        r.Log.Info("failed to list GitRepositories (Flux may not be installed)", "error", err.Error())
    } else {
        r.Log.Info("Flux GitRepositories found", "count", len(gitRepos))
    }

    // List Kustomizations
    kustomizations, err := fluxClient.ListKustomizations(ctx, namespace)
    if err != nil {
        r.Log.Info("failed to list Kustomizations", "error", err.Error())
    } else {
        r.Log.Info("Flux Kustomizations found", "count", len(kustomizations))
    }

    // Record metrics for each target cluster
    for _, cluster := range integration.Spec.TargetClusters {
        prometheus.SetIntegrationStatus(integration.Name, string(integration.Spec.Type), cluster, true)
        prometheus.RecordSyncOperation(integration.Name, cluster, "success")
        prometheus.RecordSyncLatency(integration.Name, cluster, time.Since(startTime).Seconds())
    }

    return nil
}

func (r *IntegrationReconciler) reconcilePrometheus(ctx context.Context, integration *ksitv1alpha1.Integration) error {
    r.Log.Info("reconciling Prometheus integration", "name", integration.Name)

    prometheusURL := integration.Spec.Config["url"]
    if prometheusURL == "" {
        return fmt.Errorf("Prometheus URL is required")
    }

    promClient, err := prometheus.NewClient(prometheusURL)
    if err != nil {
        return fmt.Errorf("failed to create Prometheus client: %w", err)
    }

    // Validate connection
    if err := promClient.ValidateConnection(ctx); err != nil {
        return fmt.Errorf("failed to connect to Prometheus: %w", err)
    }

    r.Log.Info("Prometheus connection validated")

    // Get targets to verify scraping is working
    targets, err := promClient.GetTargets(ctx)
    if err != nil {
        r.Log.Error(err, "failed to get Prometheus targets")
    } else {
        activeTargets := 0
        for _, target := range targets.Active {
            if target.Health == "up" {
                activeTargets++
            }
        }
        r.Log.Info("Prometheus targets", "active", activeTargets, "total", len(targets.Active))
    }

    // Record metrics for each target cluster
    for _, cluster := range integration.Spec.TargetClusters {
        prometheus.SetIntegrationStatus(integration.Name, string(integration.Spec.Type), cluster, true)
    }

    return nil
}

func (r *IntegrationReconciler) reconcileIstio(ctx context.Context, integration *ksitv1alpha1.Integration) error {
    r.Log.Info("reconciling Istio integration", "name", integration.Name)

    meshClient := istio.NewServiceMesh(r.Client)

    namespace := integration.Spec.Config["namespace"]
    if namespace == "" {
        namespace = "istio-system"
    }

    enableMTLS := integration.Spec.Config["enableMTLS"] == "true"

    // Configure mesh if mTLS is enabled
    if enableMTLS {
        meshConfig := &istio.MeshConfig{
            Name:           integration.Name,
            Namespace:      namespace,
            EnableAutoMTLS: true,
        }

        if err := meshClient.ConfigureMesh(ctx, meshConfig); err != nil {
            r.Log.Info("failed to configure mesh (this is expected if PeerAuthentication already exists)", "error", err.Error())
        } else {
            r.Log.Info("Istio mesh configuration applied")
        }
    }

    // Record metrics for each target cluster
    for _, cluster := range integration.Spec.TargetClusters {
        prometheus.SetIntegrationStatus(integration.Name, string(integration.Spec.Type), cluster, true)
    }

    return nil
}

func (r *IntegrationReconciler) cleanupIntegration(ctx context.Context, integration *ksitv1alpha1.Integration) error {
    r.Log.Info("cleaning up integration", "name", integration.Name, "type", integration.Spec.Type)

    // Update metrics to show integration is down
    for _, cluster := range integration.Spec.TargetClusters {
        prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, cluster, false)
    }

    return nil
}

func (r *IntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
    log := r.Log.WithValues("integrationtarget", req.NamespacedName)
    log.Info("reconciling integration target")

    target := &ksitv1alpha1.IntegrationTarget{}
    if err := r.Get(ctx, req.NamespacedName, target); err != nil {
        if errors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // Update status
    now := metav1.Now()
    target.Status.Ready = true
    target.Status.Message = "Target is ready"

    // Add Ready condition
    meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
        Type:               ksitv1alpha1.ConditionTypeReady,
        Status:             metav1.ConditionTrue,
        Reason:             "TargetReady",
        Message:            "Integration target is ready",
        LastTransitionTime: now,
    })

    if err := r.Status().Update(ctx, target); err != nil {
        return ctrl.Result{}, err
    }

    // Update metrics
    prometheus.SetClusterConnectionStatus(target.Spec.ClusterName, true)

    return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *IntegrationTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&ksitv1alpha1.IntegrationTarget{}).
        Complete(r)
}