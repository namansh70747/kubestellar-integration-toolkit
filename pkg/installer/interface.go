package installer

import (
	"context"

	"k8s.io/client-go/rest"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

// Installer handles automatic installation of integrations
type Installer interface {
	// Install installs the integration on the target cluster
	Install(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error
	// Uninstall removes the integration from the target cluster
	Uninstall(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) error
	// IsInstalled checks if the integration is already installed
	IsInstalled(ctx context.Context, config *rest.Config, integration *ksitv1alpha1.Integration) (bool, error)
}

// InstallerFactory creates appropriate installer based on integration type
type InstallerFactory struct {
	installers map[string]Installer
}

// NewInstallerFactory creates a new installer factory
func NewInstallerFactory() *InstallerFactory {
	return &InstallerFactory{
		installers: map[string]Installer{
			ksitv1alpha1.IntegrationTypeArgoCD:     NewArgoCDInstaller(),
			ksitv1alpha1.IntegrationTypeFlux:       NewFluxInstaller(),
			ksitv1alpha1.IntegrationTypePrometheus: NewPrometheusInstaller(),
			ksitv1alpha1.IntegrationTypeIstio:      NewIstioInstaller(),
		},
	}
}

// GetInstaller returns the appropriate installer for the given integration type
func (f *InstallerFactory) GetInstaller(integrationType string) (Installer, error) {
	installer, ok := f.installers[integrationType]
	if !ok {
		return nil, nil
	}
	return installer, nil
}
