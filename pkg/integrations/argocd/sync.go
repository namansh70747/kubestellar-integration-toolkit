package argocd

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Syncer handles ArgoCD sync operations
type Syncer struct {
	k8sClient  client.Client
	argoClient *Client
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Status    string
	Message   string
	Synced    bool
	Healthy   bool
	Timestamp time.Time
}

// NewSyncer creates a new ArgoCD syncer
func NewSyncer(c client.Client, argoClient *Client) *Syncer {
	return &Syncer{
		k8sClient:  c,
		argoClient: argoClient,
	}
}

// Sync triggers a sync for an application
func (s *Syncer) Sync(ctx context.Context, namespace string, name string) error {
	app, err := s.argoClient.GetApplication(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get application: %w", err)
	}

	if app.Status.Sync.Status == "Synced" {
		return nil
	}

	return s.argoClient.SyncApplication(ctx, name)
}

// GetSyncResult returns the sync status of an application
func (s *Syncer) GetSyncResult(ctx context.Context, namespace string, name string) (*SyncResult, error) {
	app, err := s.argoClient.GetApplication(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return &SyncResult{
		Status:    app.Status.Sync.Status,
		Synced:    app.Status.Sync.Status == "Synced",
		Healthy:   app.Status.Health.Status == "Healthy",
		Timestamp: time.Now(),
	}, nil
}

// WaitForSync waits for an application to be synced
func (s *Syncer) WaitForSync(ctx context.Context, namespace string, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			result, err := s.GetSyncResult(ctx, namespace, name)
			if err != nil {
				continue
			}
			if result.Synced {
				return nil
			}
		}
	}

	return fmt.Errorf("timeout waiting for sync")
}

// SyncAll syncs all applications
func (s *Syncer) SyncAll(ctx context.Context) error {
	apps, err := s.argoClient.ListApplications(ctx)
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	for _, app := range apps {
		if err := s.argoClient.SyncApplication(ctx, app.Metadata.Name); err != nil {
			return fmt.Errorf("failed to sync %s: %w", app.Metadata.Name, err)
		}
	}

	return nil
}

// Sync status constants
const (
	HealthStatusHealthy     = "Healthy"
	HealthStatusProgressing = "Progressing"
	HealthStatusDegraded    = "Degraded"
	HealthStatusSuspended   = "Suspended"
	HealthStatusMissing     = "Missing"
	HealthStatusUnknown     = "Unknown"

	SyncStatusCodeSynced    = "Synced"
	SyncStatusCodeOutOfSync = "OutOfSync"
	SyncStatusCodeUnknown   = "Unknown"

	OperationRunning     = "Running"
	OperationSucceeded   = "Succeeded"
	OperationFailed      = "Failed"
	OperationError       = "Error"
	OperationTerminating = "Terminating"
)

// SyncRequest represents a sync request for an application
type SyncRequest struct {
	Revision    string
	Prune       bool
	DryRun      bool
	SyncOptions []string
	Resources   []SyncResource
}

// SyncResource represents a specific resource to sync
type SyncResource struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
}

// WaitForHealthy waits for an application to be healthy
func (c *Client) WaitForHealthy(ctx context.Context, appName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			app, err := c.GetApplication(ctx, c.namespace, appName)
			if err != nil {
				continue
			}
			if app.Status.Health.Status == HealthStatusHealthy {
				return nil
			}
		}
	}

	return fmt.Errorf("timeout waiting for healthy status")
}

// RefreshApplication refreshes an application
func (c *Client) RefreshApplication(ctx context.Context, appName string) error {
	app, err := c.GetApplication(ctx, c.namespace, appName)
	if err != nil {
		return err
	}

	annotations := app.Metadata.Labels
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["argocd.argoproj.io/refresh"] = "true"

	return nil
}
