package installer

import (
	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// NewPrometheusInstaller creates a new Prometheus installer with default configuration
func NewPrometheusInstaller() *HelmInstaller {
	return &HelmInstaller{
		integrationType: ksitv1alpha1.IntegrationTypePrometheus,
		defaultConfig: &ksitv1alpha1.HelmInstallConfig{
			Repository:  "https://prometheus-community.github.io/helm-charts",
			Chart:       "kube-prometheus-stack",
			Version:     "55.5.0",
			ReleaseName: "prometheus",
			Values: map[string]string{
				"prometheus.prometheusSpec.retention": "7d",
				"grafana.enabled":                     "true",
			},
		},
	}
}
