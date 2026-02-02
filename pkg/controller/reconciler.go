package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
	"github.com/kubestellar/integration-toolkit/pkg/cluster"
	"github.com/kubestellar/integration-toolkit/pkg/installer"
	"github.com/kubestellar/integration-toolkit/pkg/integrations/prometheus"
)

const (
	integrationFinalizer = "ksit.io/finalizer"
	requeueInterval      = 30 * time.Second
)

type IntegrationReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Log              logr.Logger
	ClusterManager   *cluster.ClusterManager
	ClusterInventory *cluster.ClusterInventory
	InstallerFactory *installer.InstallerFactory
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

	// ✅ USE CLUSTER INVENTORY: Track clusters
	for _, clusterName := range integration.Spec.TargetClusters {
		clusterInfo, err := r.ClusterInventory.GetCluster(clusterName)
		if err != nil {
			// Cluster not in inventory, add it
			r.ClusterInventory.AddCluster(clusterName, integration.Namespace, string(cluster.ClusterStatusActive))
			log.Info("added cluster to inventory", "cluster", clusterName)
		} else {
			// Update last seen time
			clusterInfo.LastSeen = time.Now()
			r.ClusterInventory.UpdateCluster(clusterInfo)
		}
	}

	// Handle deletion
	if !integration.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(integration, integrationFinalizer) {
			if err := r.cleanupIntegration(ctx, integration); err != nil {
				return ctrl.Result{}, err
			}

			// ✅ REMOVE CLUSTERS FROM INVENTORY
			for _, clusterName := range integration.Spec.TargetClusters {
				r.ClusterInventory.RemoveCluster(clusterName)
				log.Info("removed cluster from inventory", "cluster", clusterName)
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
		if err := r.Status().Update(ctx, integration); err != nil {
			r.Log.Error(err, "failed to update status for disabled integration")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Update status to Initializing
	if integration.Status.Phase == "" {
		integration.Status.Phase = ksitv1alpha1.PhaseInitializing
		if err := r.Status().Update(ctx, integration); err != nil {
			r.Log.Error(err, "failed to update status to Initializing")
			return ctrl.Result{}, err
		}
	}

	// Handle auto-installation if enabled
	if integration.Spec.AutoInstall != nil && integration.Spec.AutoInstall.Enabled {
		log.Info("auto-install enabled, checking installation status")

		installErr := r.handleAutoInstall(ctx, integration)
		if installErr != nil {
			log.Error(installErr, "auto-install failed")
			integration.Status.Phase = ksitv1alpha1.PhaseFailed
			integration.Status.Message = fmt.Sprintf("Auto-install failed: %v", installErr)
			if err := r.Status().Update(ctx, integration); err != nil {
				log.Error(err, "failed to update status after auto-install failure")
			}
			return ctrl.Result{RequeueAfter: requeueInterval}, installErr
		}
		log.Info("auto-install completed successfully")
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

		// ✅ UPDATE INVENTORY: Mark clusters as error
		for _, clusterName := range integration.Spec.TargetClusters {
			clusterInfo, _ := r.ClusterInventory.GetCluster(clusterName)
			if clusterInfo != nil {
				clusterInfo.Status = string(cluster.ClusterStatusError)
				r.ClusterInventory.UpdateCluster(clusterInfo)
			}
		}

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

		// ✅ UPDATE INVENTORY: Mark clusters as active
		for _, clusterName := range integration.Spec.TargetClusters {
			clusterInfo, _ := r.ClusterInventory.GetCluster(clusterName)
			if clusterInfo != nil {
				clusterInfo.Status = string(cluster.ClusterStatusActive)
				r.ClusterInventory.UpdateCluster(clusterInfo)
			}
			prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, clusterName, true)
		}

		meta.SetStatusCondition(&integration.Status.Conditions, metav1.Condition{
			Type:    ksitv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  "ReconcileSucceeded",
			Message: "Integration is healthy",
		})
	}

	if err := r.Status().Update(ctx, integration); err != nil {
		r.Log.Error(err, "failed to update integration status")
		return ctrl.Result{}, err
	}

	// ✅ CLEANUP STALE CLUSTERS FROM INVENTORY (every hour)
	go func() {
		time.Sleep(1 * time.Hour)
		r.ClusterInventory.CleanupStale(24 * time.Hour)
		log.Info("cleaned up stale clusters from inventory")
	}()

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *IntegrationReconciler) reconcileArgoCD(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling ArgoCD integration", "name", integration.Name)
	startTime := time.Now()

	// Get namespace from config or use default
	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = "argocd"
	}

	// Health check for each target cluster using Kubernetes API
	for _, clusterName := range integration.Spec.TargetClusters {
		r.Log.Info("checking ArgoCD health on cluster", "cluster", clusterName)

		// Get cluster configuration
		clusterConfig, err := r.ClusterManager.GetClusterConfig(clusterName, integration.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get cluster config for %s: %w", clusterName, err)
		}

		// Create clientset for target cluster
		clientset, err := kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create clientset for %s: %w", clusterName, err)
		}

		// ✅ Health Check 1: Namespace exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ArgoCD namespace %s not found on %s: %w", namespace, clusterName, err)
		}

		// ✅ Health Check 2: ArgoCD server deployment is healthy
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, "argocd-server", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ArgoCD server deployment not found on %s: %w", clusterName, err)
		}

		if deployment.Status.AvailableReplicas == 0 {
			return fmt.Errorf("ArgoCD server has 0 available replicas on %s", clusterName)
		}

		// ✅ Health Check 3: ArgoCD server service has endpoints
		endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(ctx, "argocd-server", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ArgoCD server endpoints not found on %s: %w", clusterName, err)
		}

		totalEndpoints := 0
		for _, subset := range endpoints.Subsets {
			totalEndpoints += len(subset.Addresses)
		}

		if totalEndpoints == 0 {
			return fmt.Errorf("ArgoCD server service has no endpoints on %s", clusterName)
		}

		// ✅ Health Check 4: Check critical ArgoCD components
		criticalComponents := []string{
			"argocd-server",
			"argocd-repo-server",
			"argocd-application-controller",
		}

		for _, componentName := range criticalComponents {
			deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, componentName, metav1.GetOptions{})
			if err != nil {
				r.Log.Info("ArgoCD component not found", "component", componentName, "cluster", clusterName)
				continue
			}

			if deploy.Status.AvailableReplicas > 0 {
				r.Log.Info("ArgoCD component is healthy",
					"component", componentName,
					"cluster", clusterName,
					"replicas", deploy.Status.AvailableReplicas)
			}
		}

		// ✅ Health Check 5: Verify ArgoCD pods are running
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name",
		})
		if err == nil {
			runningPods := 0
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}
			r.Log.Info("ArgoCD pods status",
				"cluster", clusterName,
				"total", len(pods.Items),
				"running", runningPods)

			if runningPods == 0 {
				return fmt.Errorf("no ArgoCD pods are running on %s", clusterName)
			}
		}

		latency := time.Since(startTime).Seconds()
		prometheus.RecordSyncLatency(integration.Name, clusterName, latency)
		prometheus.RecordSyncOperation(integration.Name, clusterName, "success")
		r.Log.Info("✅ ArgoCD integration is healthy", "cluster", clusterName)
	}

	return nil
}

func (r *IntegrationReconciler) reconcileFlux(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Flux integration", "name", integration.Name)

	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = "flux-system"
	}

	// Health check for each target cluster using Kubernetes API
	for _, clusterName := range integration.Spec.TargetClusters {
		r.Log.Info("checking Flux health on cluster", "cluster", clusterName)

		// Get cluster configuration
		clusterConfig, err := r.ClusterManager.GetClusterConfig(clusterName, integration.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get cluster config for %s: %w", clusterName, err)
		}

		// Create clientset for target cluster
		clientset, err := kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create clientset for %s: %w", clusterName, err)
		}

		// ✅ Health Check 1: Namespace exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Flux namespace %s not found on %s: %w", namespace, clusterName, err)
		}

		// ✅ Health Check 2: Flux controllers are running
		fluxControllers := []string{
			"source-controller",
			"kustomize-controller",
			"helm-controller",
			"notification-controller",
		}

		healthyControllers := 0
		for _, controllerName := range fluxControllers {
			deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, controllerName, metav1.GetOptions{})
			if err != nil {
				r.Log.Info("Flux controller not found", "controller", controllerName, "cluster", clusterName)
				continue
			}

			if deploy.Status.AvailableReplicas > 0 {
				healthyControllers++
				r.Log.Info("Flux controller is healthy",
					"controller", controllerName,
					"cluster", clusterName,
					"replicas", deploy.Status.AvailableReplicas)
			}
		}

		if healthyControllers == 0 {
			return fmt.Errorf("no Flux controllers are running on %s", clusterName)
		}

		// ✅ Health Check 3: Check Flux pods
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list Flux pods on %s: %w", clusterName, err)
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		r.Log.Info("Flux pods status",
			"cluster", clusterName,
			"total", len(pods.Items),
			"running", runningPods)

		if runningPods == 0 {
			return fmt.Errorf("no Flux pods are running on %s", clusterName)
		}

		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, clusterName, true)
		r.Log.Info("✅ Flux integration is healthy", "cluster", clusterName, "controllers", healthyControllers)
	}

	return nil
}

func (r *IntegrationReconciler) reconcilePrometheus(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Prometheus integration", "name", integration.Name)

	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = "monitoring"
	}

	// Health check for each target cluster using Kubernetes API
	for _, clusterName := range integration.Spec.TargetClusters {
		r.Log.Info("checking Prometheus health on cluster", "cluster", clusterName)

		// Get cluster configuration
		clusterConfig, err := r.ClusterManager.GetClusterConfig(clusterName, integration.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get cluster config for %s: %w", clusterName, err)
		}

		// Create clientset for target cluster
		clientset, err := kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create clientset for %s: %w", clusterName, err)
		}

		// ✅ Health Check 1: Namespace exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Prometheus namespace %s not found on %s: %w", namespace, clusterName, err)
		}

		// ✅ Health Check 2: Check Prometheus operator deployment
		deployments := []string{
			"prometheus-kube-prometheus-operator",
			"prometheus-grafana",
		}

		healthyComponents := 0
		for _, deployName := range deployments {
			deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
			if err != nil {
				r.Log.Info("Prometheus component not found", "component", deployName, "cluster", clusterName)
				continue
			}

			if deploy.Status.AvailableReplicas > 0 {
				healthyComponents++
				r.Log.Info("Prometheus component is healthy",
					"component", deployName,
					"cluster", clusterName,
					"replicas", deploy.Status.AvailableReplicas)
			}
		}

		// ✅ Health Check 3: Check StatefulSets (Prometheus, Alertmanager)
		statefulsets := []string{
			"prometheus-prometheus-kube-prometheus-prometheus",
			"alertmanager-prometheus-kube-prometheus-alertmanager",
		}

		for _, stsName := range statefulsets {
			sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
			if err != nil {
				r.Log.Info("StatefulSet not found", "statefulset", stsName, "cluster", clusterName)
				continue
			}

			if sts.Status.ReadyReplicas > 0 {
				healthyComponents++
				r.Log.Info("StatefulSet is healthy",
					"statefulset", stsName,
					"cluster", clusterName,
					"replicas", sts.Status.ReadyReplicas)
			}
		}

		// ✅ Health Check 4: Count running Prometheus pods
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list Prometheus pods on %s: %w", clusterName, err)
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		r.Log.Info("Prometheus pods status",
			"cluster", clusterName,
			"total", len(pods.Items),
			"running", runningPods)

		if runningPods == 0 {
			return fmt.Errorf("no Prometheus pods are running on %s", clusterName)
		}

		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, clusterName, true)
		r.Log.Info("✅ Prometheus integration is healthy", "cluster", clusterName)
	}

	return nil
}

func (r *IntegrationReconciler) reconcileIstio(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	r.Log.Info("reconciling Istio integration", "name", integration.Name)

	// Istio typically runs in istio-system namespace
	namespace := "istio-system"

	// Health check for each target cluster using Kubernetes API
	for _, clusterName := range integration.Spec.TargetClusters {
		r.Log.Info("checking Istio health on cluster", "cluster", clusterName)

		// Get cluster configuration
		clusterConfig, err := r.ClusterManager.GetClusterConfig(clusterName, integration.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get cluster config for %s: %w", clusterName, err)
		}

		// Create clientset for target cluster
		clientset, err := kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("failed to create clientset for %s: %w", clusterName, err)
		}

		// ✅ Health Check 1: Namespace exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Istio namespace %s not found on %s: %w", namespace, clusterName, err)
		}

		// ✅ Health Check 2: Istiod (control plane) is running
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, "istiod", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Istiod deployment not found on %s: %w", clusterName, err)
		}

		if deployment.Status.AvailableReplicas == 0 {
			return fmt.Errorf("Istiod has 0 available replicas on %s", clusterName)
		}

		r.Log.Info("Istiod is healthy",
			"cluster", clusterName,
			"replicas", deployment.Status.AvailableReplicas)

		// ✅ Health Check 3: Ingress gateway (if exists)
		ingressDeploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, "istio-ingressgateway", metav1.GetOptions{})
		if err == nil {
			r.Log.Info("Istio ingress gateway found",
				"cluster", clusterName,
				"replicas", ingressDeploy.Status.AvailableReplicas)
		} else {
			r.Log.Info("Istio ingress gateway not found (optional)", "cluster", clusterName)
		}

		// ✅ Health Check 4: Check Istio pods
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list Istio pods on %s: %w", clusterName, err)
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}

		r.Log.Info("Istio pods status",
			"cluster", clusterName,
			"total", len(pods.Items),
			"running", runningPods)

		if runningPods == 0 {
			return fmt.Errorf("no Istio pods are running on %s", clusterName)
		}

		prometheus.SetIntegrationStatus(integration.Name, integration.Spec.Type, clusterName, true)
		r.Log.Info("✅ Istio integration is healthy", "cluster", clusterName)
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
	// ClusterManager and ClusterInventory should be set before calling SetupWithManager
	// They are passed from main.go to ensure both reconcilers share the same instances

	return ctrl.NewControllerManagedBy(mgr).
		For(&ksitv1alpha1.Integration{}).
		Complete(r)
}

// IntegrationTargetReconciler reconciles IntegrationTarget objects
type IntegrationTargetReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	ClusterManager *cluster.ClusterManager
}

func (r *IntegrationTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("reconciling integration target", "name", req.NamespacedName)

	target := &ksitv1alpha1.IntegrationTarget{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		if errors.IsNotFound(err) {
			// Target was deleted - remove from cluster manager
			if r.ClusterManager != nil {
				_ = r.ClusterManager.RemoveCluster(req.Name, req.Namespace)
				r.Log.Info("removed cluster from manager", "cluster", req.Name)
			}
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "failed to get integration target")
		return ctrl.Result{}, err
	}

	// Get kubeconfig from secret
	secretName := target.Spec.ClusterName + "-kubeconfig"
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      secretName,
		Namespace: target.Namespace,
	}

	if err := r.Get(ctx, secretKey, secret); err != nil {
		r.Log.Error(err, "failed to get kubeconfig secret", "secret", secretName)
		target.Status.Ready = false
		target.Status.Message = fmt.Sprintf("Kubeconfig secret %s not found", secretName)

		meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "SecretNotFound",
			Message: fmt.Sprintf("Kubeconfig secret %s not found", secretName),
		})

		_ = r.Status().Update(ctx, target)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Extract kubeconfig from secret
	kubeconfigData, ok := secret.Data["kubeconfig"]
	if !ok {
		r.Log.Error(fmt.Errorf("kubeconfig key not found"), "secret missing kubeconfig key")
		target.Status.Ready = false
		target.Status.Message = "Secret missing 'kubeconfig' key"

		meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidSecret",
			Message: "Secret missing 'kubeconfig' key",
		})

		_ = r.Status().Update(ctx, target)
		return ctrl.Result{}, nil
	}

	// Register cluster with ClusterManager
	if r.ClusterManager != nil {
		if err := r.ClusterManager.AddCluster(
			target.Spec.ClusterName,
			target.Namespace,
			string(kubeconfigData),
		); err != nil {
			r.Log.Error(err, "failed to register cluster", "cluster", target.Spec.ClusterName)
			target.Status.Ready = false
			target.Status.Message = fmt.Sprintf("Failed to register cluster: %v", err)

			meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "RegistrationFailed",
				Message: fmt.Sprintf("Failed to register cluster: %v", err),
			})

			_ = r.Status().Update(ctx, target)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		r.Log.Info("successfully registered cluster",
			"cluster", target.Spec.ClusterName,
			"namespace", target.Namespace)

		// Test connection
		if err := r.ClusterManager.SyncCluster(ctx, target.Spec.ClusterName, target.Namespace); err != nil {
			r.Log.Error(err, "cluster connection test failed", "cluster", target.Spec.ClusterName)
			target.Status.Ready = false
			target.Status.Message = fmt.Sprintf("Connection test failed: %v", err)

			meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "ConnectionFailed",
				Message: fmt.Sprintf("Connection test failed: %v", err),
			})

			_ = r.Status().Update(ctx, target)
			prometheus.SetClusterConnectionStatus(target.Spec.ClusterName, false)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		r.Log.Info("cluster connection verified", "cluster", target.Spec.ClusterName)
	}

	// Update status - cluster is ready
	target.Status.Ready = true
	target.Status.Message = "Target cluster is connected and ready"
	now := metav1.Now()
	target.Status.LastSyncTime = &now

	meta.SetStatusCondition(&target.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "ClusterReady",
		Message: "Successfully connected to target cluster",
	})

	if err := r.Status().Update(ctx, target); err != nil {
		r.Log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	// Update metrics
	prometheus.SetClusterConnectionStatus(target.Spec.ClusterName, true)

	r.Log.Info("successfully reconciled integration target", "name", req.NamespacedName)
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *IntegrationTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ksitv1alpha1.IntegrationTarget{}).
		Complete(r)
}

// handleAutoInstall installs the integration tool on target clusters if not already installed
func (r *IntegrationReconciler) handleAutoInstall(ctx context.Context, integration *ksitv1alpha1.Integration) error {
	log := r.Log.WithValues("integration", integration.Name, "type", integration.Spec.Type)

	// Get the installer for this integration type
	inst, err := r.InstallerFactory.GetInstaller(integration.Spec.Type)
	if err != nil {
		return fmt.Errorf("failed to get installer: %w", err)
	}

	// Install on each target cluster
	for _, clusterName := range integration.Spec.TargetClusters {
		clusterLog := log.WithValues("cluster", clusterName)

		// Get cluster config from manager
		config, err := r.ClusterManager.GetClusterConfig(clusterName, integration.Namespace)
		if err != nil {
			clusterLog.Error(err, "failed to get cluster config")
			return fmt.Errorf("failed to get config for cluster %s: %w", clusterName, err)
		}

		// Check if already installed
		installed, err := inst.IsInstalled(ctx, config, integration)
		if err != nil {
			clusterLog.Error(err, "failed to check installation status")
			return fmt.Errorf("failed to check installation on cluster %s: %w", clusterName, err)
		}

		if installed {
			clusterLog.Info("integration already installed, skipping")
			continue
		}

		// Install the integration
		clusterLog.Info("installing integration")
		if err := inst.Install(ctx, config, integration); err != nil {
			clusterLog.Error(err, "installation failed")
			return fmt.Errorf("failed to install on cluster %s: %w", clusterName, err)
		}

		clusterLog.Info("installation completed successfully")
	}

	return nil
}
