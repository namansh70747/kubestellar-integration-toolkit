package flux

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	helmReleaseGVK = schema.GroupVersionKind{
		Group:   "helm.toolkit.fluxcd.io",
		Version: "v2beta1",
		Kind:    "HelmRelease",
	}
)

// SyncStatus represents the synchronization status
type SyncStatus struct {
	Ready      bool
	Message    string
	LastUpdate time.Time
	Conditions []Condition
}

// Condition represents a status condition
type Condition struct {
	Type               string
	Status             string
	LastTransitionTime time.Time
	Reason             string
	Message            string
}

// SyncGitRepository synchronizes a GitRepository resource
func (f *FluxClient) SyncGitRepository(ctx context.Context, name, namespace string) error {
	f.Log.Info("syncing GitRepository", "name", name, "namespace", namespace)

	gitRepo, err := f.GetGitRepository(ctx, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to get GitRepository: %w", err)
	}

	// Trigger reconciliation by adding annotation
	annotations := gitRepo.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["reconcile.fluxcd.io/requestedAt"] = time.Now().Format(time.RFC3339)
	gitRepo.SetAnnotations(annotations)

	if err := f.Update(ctx, gitRepo); err != nil {
		return fmt.Errorf("failed to update GitRepository: %w", err)
	}

	f.Log.Info("GitRepository sync triggered", "name", name)
	return nil
}

// SyncKustomization synchronizes a Kustomization resource
func (f *FluxClient) SyncKustomization(ctx context.Context, name, namespace string) error {
	f.Log.Info("syncing Kustomization", "name", name, "namespace", namespace)

	kustomization := &unstructured.Unstructured{}
	kustomization.SetGroupVersionKind(kustomizationGVK)

	if err := f.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, kustomization); err != nil {
		return fmt.Errorf("failed to get Kustomization: %w", err)
	}

	// Trigger reconciliation
	annotations := kustomization.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["reconcile.fluxcd.io/requestedAt"] = time.Now().Format(time.RFC3339)
	kustomization.SetAnnotations(annotations)

	if err := f.Update(ctx, kustomization); err != nil {
		return fmt.Errorf("failed to update Kustomization: %w", err)
	}

	f.Log.Info("Kustomization sync triggered", "name", name)
	return nil
}

// GetGitRepositoryStatus retrieves the status of a GitRepository
func (f *FluxClient) GetGitRepositoryStatus(ctx context.Context, name, namespace string) (*SyncStatus, error) {
	gitRepo, err := f.GetGitRepository(ctx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitRepository: %w", err)
	}

	status := &SyncStatus{
		LastUpdate: time.Now(),
		Conditions: []Condition{},
	}

	// Extract status from the resource
	statusMap, found, err := unstructured.NestedMap(gitRepo.Object, "status")
	if err != nil || !found {
		return status, nil
	}

	// Check ready condition
	conditions, found, err := unstructured.NestedSlice(statusMap, "conditions")
	if err == nil && found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condMap, "type")
			condStatus, _, _ := unstructured.NestedString(condMap, "status")
			reason, _, _ := unstructured.NestedString(condMap, "reason")
			message, _, _ := unstructured.NestedString(condMap, "message")

			if condType == "Ready" && condStatus == "True" {
				status.Ready = true
			}

			status.Conditions = append(status.Conditions, Condition{
				Type:    condType,
				Status:  condStatus,
				Reason:  reason,
				Message: message,
			})
		}
	}

	return status, nil
}

// GetKustomizationStatus retrieves the status of a Kustomization
func (f *FluxClient) GetKustomizationStatus(ctx context.Context, name, namespace string) (*SyncStatus, error) {
	kustomization := &unstructured.Unstructured{}
	kustomization.SetGroupVersionKind(kustomizationGVK)

	if err := f.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, kustomization); err != nil {
		return nil, fmt.Errorf("failed to get Kustomization: %w", err)
	}

	status := &SyncStatus{
		LastUpdate: time.Now(),
		Conditions: []Condition{},
	}

	statusMap, found, err := unstructured.NestedMap(kustomization.Object, "status")
	if err != nil || !found {
		return status, nil
	}

	conditions, found, err := unstructured.NestedSlice(statusMap, "conditions")
	if err == nil && found {
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condMap, "type")
			condStatus, _, _ := unstructured.NestedString(condMap, "status")
			reason, _, _ := unstructured.NestedString(condMap, "reason")
			message, _, _ := unstructured.NestedString(condMap, "message")

			if condType == "Ready" && condStatus == "True" {
				status.Ready = true
			}

			status.Conditions = append(status.Conditions, Condition{
				Type:    condType,
				Status:  condStatus,
				Reason:  reason,
				Message: message,
			})
		}
	}

	return status, nil
}

// WaitForGitRepositoryReady waits for a GitRepository to become ready
func (f *FluxClient) WaitForGitRepositoryReady(ctx context.Context, name, namespace string, timeout time.Duration) error {
	f.Log.Info("waiting for GitRepository to be ready", "name", name, "timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := f.GetGitRepositoryStatus(ctx, name, namespace)
			if err != nil {
				if errors.IsNotFound(err) {
					f.Log.Info("GitRepository not found yet", "name", name)
					continue
				}
				return err
			}

			if status.Ready {
				f.Log.Info("GitRepository is ready", "name", name)
				return nil
			}

			f.Log.Info("GitRepository not ready yet", "name", name, "message", status.Message)
		}
	}

	return fmt.Errorf("timeout waiting for GitRepository %s to be ready", name)
}

// WaitForKustomizationReady waits for a Kustomization to become ready
func (f *FluxClient) WaitForKustomizationReady(ctx context.Context, name, namespace string, timeout time.Duration) error {
	f.Log.Info("waiting for Kustomization to be ready", "name", name, "timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := f.GetKustomizationStatus(ctx, name, namespace)
			if err != nil {
				if errors.IsNotFound(err) {
					f.Log.Info("Kustomization not found yet", "name", name)
					continue
				}
				return err
			}

			if status.Ready {
				f.Log.Info("Kustomization is ready", "name", name)
				return nil
			}

			f.Log.Info("Kustomization not ready yet", "name", name, "message", status.Message)
		}
	}

	return fmt.Errorf("timeout waiting for Kustomization %s to be ready", name)
}

// SuspendKustomization suspends a Kustomization
func (f *FluxClient) SuspendKustomization(ctx context.Context, name, namespace string, suspend bool) error {
	kustomization := &unstructured.Unstructured{}
	kustomization.SetGroupVersionKind(kustomizationGVK)

	if err := f.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, kustomization); err != nil {
		return fmt.Errorf("failed to get Kustomization: %w", err)
	}

	if err := unstructured.SetNestedField(kustomization.Object, suspend, "spec", "suspend"); err != nil {
		return fmt.Errorf("failed to set suspend field: %w", err)
	}

	if err := f.Update(ctx, kustomization); err != nil {
		return fmt.Errorf("failed to update Kustomization: %w", err)
	}

	f.Log.Info("Kustomization suspend status updated", "name", name, "suspend", suspend)
	return nil
}
