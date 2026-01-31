package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Manager struct {
	mgr    manager.Manager
	log    logr.Logger
	client client.Client
	scheme *runtime.Scheme
}

func NewManager(mgr manager.Manager, log logr.Logger) *Manager {
	return &Manager{
		mgr:    mgr,
		log:    log,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	m.log.Info("starting controller manager")

	if err := m.setupControllers(); err != nil {
		return fmt.Errorf("failed to setup controllers: %w", err)
	}

	return m.mgr.Start(ctx)
}

func (m *Manager) setupControllers() error {
	integrationReconciler := &IntegrationReconciler{
		Client: m.client,
		Scheme: m.scheme,
		Log:    m.log.WithName("IntegrationReconciler"),
	}

	if err := integrationReconciler.SetupWithManager(m.mgr); err != nil {
		return fmt.Errorf("failed to setup integration reconciler: %w", err)
	}

	targetReconciler := &IntegrationTargetReconciler{
		Client: m.client,
		Scheme: m.scheme,
		Log:    m.log.WithName("IntegrationTargetReconciler"),
	}

	if err := targetReconciler.SetupWithManager(m.mgr); err != nil {
		return fmt.Errorf("failed to setup integration target reconciler: %w", err)
	}

	return nil
}

func (m *Manager) GetClient() client.Client {
	return m.client
}

func (m *Manager) GetScheme() *runtime.Scheme {
	return m.scheme
}
