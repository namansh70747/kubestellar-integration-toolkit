package installer

import (
	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// NewIstioInstaller creates a new Istio installer with default configuration
func NewIstioInstaller() *HelmInstaller {
	return &HelmInstaller{
		integrationType: ksitv1alpha1.IntegrationTypeIstio,
		defaultConfig: &ksitv1alpha1.HelmInstallConfig{
			Repository:  "https://istio-release.storage.googleapis.com/charts",
			Chart:       "istiod",
			Version:     "1.20.2",
			ReleaseName: "istio",
			Values: map[string]string{
				"global.proxy.resources.requests.cpu":    "10m",
				"global.proxy.resources.requests.memory": "128Mi",
			},
		},
	}
}
