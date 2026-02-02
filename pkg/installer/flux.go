package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// FluxInstaller handles Flux installation using manifests
type FluxInstaller struct{}

// NewFluxInstaller creates a new Flux installer
func NewFluxInstaller() *FluxInstaller {
	return &FluxInstaller{}
}

// Install installs Flux using official manifests
func (f *FluxInstaller) Install(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error {
	if integration.Spec.AutoInstall == nil || !integration.Spec.AutoInstall.Enabled {
		return nil
	}

	manifestURL := integration.Spec.AutoInstall.ManifestURL
	if manifestURL == "" {
		manifestURL = "https://github.com/fluxcd/flux2/releases/latest/download/install.yaml"
	}

	// log.Info("downloading Flux manifests", "url", manifestURL)

	// Download manifests
	resp, err := http.Get(manifestURL)
	if err != nil {
		return fmt.Errorf("failed to download Flux manifests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download manifests: HTTP %d", resp.StatusCode)
	}

	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read manifest content: %w", err)
	}

	// log.Info("downloaded Flux manifests", "size", len(manifestBytes))

	// Create clients
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Ensure flux-system namespace exists
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create flux-system namespace: %w", err)
	}

	// log.Info("flux-system namespace ready")

	// Parse and apply manifests
	manifestsStr := string(manifestBytes)
	docs := strings.Split(manifestsStr, "---")

	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	applied := 0
	skipped := 0

	// PHASE 1: Apply CRDs first
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || strings.HasPrefix(doc, "#") {
			continue
		}

		// Only process CRDs in this pass
		if !strings.Contains(doc, "kind: CustomResourceDefinition") {
			continue
		}

		// Decode YAML to unstructured
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode([]byte(doc), nil, obj)
		if err != nil {
			skipped++
			continue
		}

		// Get resource mapping
		gvr, err := getGVR(gvk)
		if err != nil {
			skipped++
			continue
		}

		// Apply CRD (CRDs are always cluster-scoped)
		_, applyErr := dynClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		if errors.IsAlreadyExists(applyErr) {
			_, applyErr = dynClient.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{})
		}

		if applyErr != nil {
			// Log but continue - some CRDs may be partially applied
			continue
		}

		applied++
	}

	// Wait briefly for CRDs to be established
	time.Sleep(3 * time.Second)

	// PHASE 2: Apply all other resources
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" || strings.HasPrefix(doc, "#") {
			continue
		}

		// Skip CRDs - already applied
		if strings.Contains(doc, "kind: CustomResourceDefinition") {
			continue
		}

		// Decode YAML to unstructured
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode([]byte(doc), nil, obj)
		if err != nil {
			skipped++
			continue
		}

		// Get resource mapping
		gvr, err := getGVR(gvk)
		if err != nil {
			skipped++
			continue
		}

		namespace := obj.GetNamespace()
		_ = obj.GetName() // name used for logging only

		// Apply resource
		var applyErr error
		if namespace != "" {
			_, applyErr = dynClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
			if errors.IsAlreadyExists(applyErr) {
				_, applyErr = dynClient.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
			}
		} else {
			_, applyErr = dynClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
			if errors.IsAlreadyExists(applyErr) {
				_, applyErr = dynClient.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{})
			}
		}

		if applyErr != nil {
			// Continue with other resources
			continue
		}

		applied++
	}

	// log.Info("applied Flux manifests", "applied", applied, "skipped", skipped)

	// Wait for Flux controllers to be ready
	// log.Info("waiting for Flux controllers to be ready")
	err = wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
		deployments := []string{
			"source-controller",
			"kustomize-controller",
			"helm-controller",
			"notification-controller",
		}

		for _, name := range deployments {
			deploy, err := clientset.AppsV1().Deployments("flux-system").Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if deploy.Status.ReadyReplicas < 1 {
				return false, nil
			}
		}
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for Flux controllers: %w", err)
	}

	// log.Info("Flux installation completed successfully")
	return nil
}

// Uninstall removes Flux from the cluster
func (f *FluxInstaller) Uninstall(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Delete flux-system namespace (cascade deletes all resources)
	err = clientset.CoreV1().Namespaces().Delete(ctx, "flux-system", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete flux-system namespace: %w", err)
	}

	// log.Info("Flux uninstalled successfully")
	return nil
}

// IsInstalled checks if Flux is installed by looking for flux-system namespace and deployments
func (f *FluxInstaller) IsInstalled(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) (bool, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Check if flux-system namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "flux-system", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check if source-controller deployment exists
	_, err = clientset.AppsV1().Deployments("flux-system").Get(ctx, "source-controller", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// getGVR converts GroupVersionKind to GroupVersionResource
func getGVR(gvk *schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Map common Kubernetes resources to their plural forms
	resourceMap := map[string]string{
		"Namespace":                "namespaces",
		"ServiceAccount":           "serviceaccounts",
		"ClusterRole":              "clusterroles",
		"ClusterRoleBinding":       "clusterrolebindings",
		"Role":                     "roles",
		"RoleBinding":              "rolebindings",
		"ConfigMap":                "configmaps",
		"Secret":                   "secrets",
		"Service":                  "services",
		"Deployment":               "deployments",
		"StatefulSet":              "statefulsets",
		"DaemonSet":                "daemonsets",
		"CustomResourceDefinition": "customresourcedefinitions",
		"NetworkPolicy":            "networkpolicies",
		"PriorityClass":            "priorityclasses",
		"ResourceQuota":            "resourcequotas",
		"LimitRange":               "limitranges",
	}

	resource, ok := resourceMap[gvk.Kind]
	if !ok {
		// Default: lowercase + s
		resource = strings.ToLower(gvk.Kind) + "s"
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}, nil
}
