package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// HelmInstaller handles Helm-based installation of integrations
type HelmInstaller struct {
	integrationType string
	defaultConfig   *ksitv1alpha1.HelmInstallConfig
}

// Install installs the integration using Helm
func (h *HelmInstaller) Install(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error {
	helmConfig := integration.Spec.AutoInstall.HelmConfig
	if helmConfig == nil {
		helmConfig = h.defaultConfig
	}

	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = h.getDefaultNamespace()
	}

	settings := cli.New()

	// ✅ FIX: Write kubeconfig and keep it until Helm finishes
	kubeconfigFile, cleanup, err := writeKubeconfigToTempFile(config)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	// Cleanup AFTER all operations complete
	defer cleanup()

	settings.KubeConfig = kubeconfigFile

	// ✅ FIX: Extract repo name from URL, not chart name
	repoName := extractRepoNameFromURL(helmConfig.Repository)

	// Add Helm repository
	if err := h.addHelmRepo(ctx, helmConfig.Repository, repoName, settings); err != nil {
		return fmt.Errorf("failed to add helm repo: %w", err)
	}

	// Initialize action configuration
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Check if release exists
	listClient := action.NewList(actionConfig)
	releases, err := listClient.Run()
	if err == nil {
		for _, rel := range releases {
			if rel.Name == helmConfig.ReleaseName {
				// Upgrade existing release
				upgradeClient := action.NewUpgrade(actionConfig)
				upgradeClient.Namespace = namespace

				chartPath := fmt.Sprintf("%s/%s", repoName, helmConfig.Chart)
				chartRequested, err := upgradeClient.ChartPathOptions.LocateChart(chartPath, settings)
				if err != nil {
					return fmt.Errorf("failed to locate chart: %w", err)
				}

				loadedChart, err := loader.Load(chartRequested)
				if err != nil {
					return fmt.Errorf("failed to load chart: %w", err)
				}

				_, err = upgradeClient.Run(helmConfig.ReleaseName, loadedChart, convertValuesToMap(helmConfig.Values))
				return err
			}
		}
	}

	// Install new release
	installClient := action.NewInstall(actionConfig)
	installClient.Namespace = namespace
	installClient.CreateNamespace = true
	installClient.ReleaseName = helmConfig.ReleaseName

	chartPath := fmt.Sprintf("%s/%s", repoName, helmConfig.Chart)
	chartRequested, err := installClient.ChartPathOptions.LocateChart(chartPath, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	loadedChart, err := loader.Load(chartRequested)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	_, err = installClient.Run(loadedChart, convertValuesToMap(helmConfig.Values))
	return err
}

// ✅ ADD THIS NEW HELPER FUNCTION
func extractRepoNameFromURL(repoURL string) string {
	// Remove trailing slash
	repoURL = strings.TrimSuffix(repoURL, "/")

	// Get last part of URL
	// https://argoproj.github.io/argo-helm -> argo-helm
	parts := strings.Split(repoURL, "/")
	return parts[len(parts)-1]
}

// Uninstall removes the Helm release
func (h *HelmInstaller) Uninstall(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error {
	helmConfig := integration.Spec.AutoInstall.HelmConfig
	if helmConfig == nil {
		helmConfig = h.defaultConfig
	}

	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = h.getDefaultNamespace()
	}

	settings := cli.New()
	kubeconfigFile, cleanup, err := writeKubeconfigToTempFile(config)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	defer cleanup()

	settings.KubeConfig = kubeconfigFile

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(helmConfig.ReleaseName)
	return err
}

// IsInstalled checks if the Helm release exists
func (h *HelmInstaller) IsInstalled(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) (bool, error) {
	helmConfig := integration.Spec.AutoInstall.HelmConfig
	if helmConfig == nil {
		helmConfig = h.defaultConfig
	}

	namespace := integration.Spec.Config["namespace"]
	if namespace == "" {
		namespace = h.getDefaultNamespace()
	}

	settings := cli.New()
	kubeconfigFile, cleanup, err := writeKubeconfigToTempFile(config)
	if err != nil {
		return false, fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	defer cleanup()

	settings.KubeConfig = kubeconfigFile

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "secret", func(format string, v ...interface{}) {}); err != nil {
		return false, fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	listClient := action.NewList(actionConfig)
	releases, err := listClient.Run()
	if err != nil {
		return false, err
	}

	for _, rel := range releases {
		if rel.Name == helmConfig.ReleaseName {
			return true, nil
		}
	}

	return false, nil
}

// addHelmRepo adds a Helm repository
func (h *HelmInstaller) addHelmRepo(ctx context.Context, repoURL, repoName string, settings *cli.EnvSettings) error {
	// ✅ FIX: Ensure writable paths under /tmp for container environments
	if settings.RepositoryConfig == "" {
		settings.RepositoryConfig = filepath.Join("/tmp", "helm", "repositories.yaml")
	}
	if settings.RepositoryCache == "" {
		settings.RepositoryCache = filepath.Join("/tmp", "helm", "cache")
	}

	repoFile := settings.RepositoryConfig

	// Ensure directory exists
	repoDir := filepath.Dir(repoFile)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create repo dir: %w", err)
	}

	// Also create cache directory
	cacheDir := settings.RepositoryCache
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Load existing repos
	b, err := os.ReadFile(repoFile)
	var repoFileContent repo.File
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		repoFileContent = repo.File{
			APIVersion: "v1",
			Generated:  time.Now(),
		}
	} else {
		if err := yaml.Unmarshal(b, &repoFileContent); err != nil {
			return err
		}
	}

	// Check if repo already exists
	var repoEntry *repo.Entry
	for _, r := range repoFileContent.Repositories {
		if r.Name == repoName {
			repoEntry = r
			break
		}
	}

	// Add new repo if not exists
	if repoEntry == nil {
		repoEntry = &repo.Entry{
			Name: repoName,
			URL:  repoURL,
		}
		repoFileContent.Repositories = append(repoFileContent.Repositories, repoEntry)

		// Write updated file
		data, err := yaml.Marshal(&repoFileContent)
		if err != nil {
			return fmt.Errorf("failed to marshal repo file: %w", err)
		}
		if err := os.WriteFile(repoFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write repo file: %w", err)
		}

		// ✅ FIX: Download repo index after adding new repository
		chartRepo, err := repo.NewChartRepository(repoEntry, getter.All(settings))
		if err != nil {
			return fmt.Errorf("failed to create chart repository: %w", err)
		}
		chartRepo.CachePath = cacheDir
		if _, err := chartRepo.DownloadIndexFile(); err != nil {
			return fmt.Errorf("failed to download repo index: %w", err)
		}
	}

	return nil
}

// writeKubeconfigToTempFile writes kubeconfig to temp file and returns path + cleanup func
func writeKubeconfigToTempFile(config *rest.Config) (string, func(), error) {
	// Create temp kubeconfig
	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {
				Server:                   config.Host,
				CertificateAuthorityData: config.CAData,
				InsecureSkipTLSVerify:    config.Insecure,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "default",
				AuthInfo: "default",
			},
		},
		CurrentContext: "default",
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"default": {
				ClientCertificateData: config.CertData,
				ClientKeyData:         config.KeyData,
				Token:                 config.BearerToken,
			},
		},
	}

	kubeconfigBytes, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	// Write to temp file under /tmp for container compatibility
	tmpFile, err := os.CreateTemp("/tmp", "kubeconfig-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(kubeconfigBytes); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Return path and cleanup function
	cleanup := func() {
		os.Remove(tmpFile.Name())
	}

	return tmpFile.Name(), cleanup, nil
}

// convertValuesToMap converts string map to interface map
func convertValuesToMap(values map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range values {
		result[k] = v
	}
	return result
}

// getDefaultNamespace returns the default namespace for the integration type
func (h *HelmInstaller) getDefaultNamespace() string {
	switch h.integrationType {
	case ksitv1alpha1.IntegrationTypeArgoCD:
		return "argocd"
	case ksitv1alpha1.IntegrationTypeFlux:
		return "flux-system"
	case ksitv1alpha1.IntegrationTypePrometheus:
		return "monitoring"
	case ksitv1alpha1.IntegrationTypeIstio:
		return "istio-system"
	default:
		return "default"
	}
}
