package installer

import (
	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// NewArgoCDInstaller creates a new ArgoCD installer with default configuration
func NewArgoCDInstaller() *HelmInstaller {
	return &HelmInstaller{
		integrationType: ksitv1alpha1.IntegrationTypeArgoCD,
		defaultConfig: &ksitv1alpha1.HelmInstallConfig{
			Repository:  "https://argoproj.github.io/argo-helm",
			Chart:       "argo-cd",
			Version:     "5.51.6",
			ReleaseName: "argocd",
			Values: map[string]string{
				"server.service.type": "ClusterIP",
				"server.insecure":     "true",
			},
		},
	}
}
