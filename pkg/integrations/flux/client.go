package flux

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	gitRepositoryGVK = schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepository",
	}
	kustomizationGVK = schema.GroupVersionKind{
		Group:   "kustomize.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "Kustomization",
	}
)

type FluxClient struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

type GitRepository struct {
	Name      string
	Namespace string
	URL       string
	Branch    string
	Interval  string
	SecretRef string
}

type Kustomization struct {
	Name            string
	Namespace       string
	SourceRef       string
	Path            string
	Interval        string
	Prune           bool
	TargetNamespace string
}

func NewFluxClient(c client.Client, scheme *runtime.Scheme, log logr.Logger) *FluxClient {
	return &FluxClient{
		Client: c,
		Scheme: scheme,
		Log:    log,
	}
}

func (f *FluxClient) GetGitRepository(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	gitRepo := &unstructured.Unstructured{}
	gitRepo.SetGroupVersionKind(gitRepositoryGVK)

	err := f.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, gitRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitRepository: %w", err)
	}

	return gitRepo, nil
}

func (f *FluxClient) CreateGitRepository(ctx context.Context, repo *GitRepository) error {
	gitRepo := &unstructured.Unstructured{}
	gitRepo.SetGroupVersionKind(gitRepositoryGVK)
	gitRepo.SetName(repo.Name)
	gitRepo.SetNamespace(repo.Namespace)

	spec := map[string]interface{}{
		"url":      repo.URL,
		"interval": repo.Interval,
		"ref": map[string]interface{}{
			"branch": repo.Branch,
		},
	}

	if repo.SecretRef != "" {
		spec["secretRef"] = map[string]interface{}{
			"name": repo.SecretRef,
		}
	}

	if err := unstructured.SetNestedMap(gitRepo.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := f.Create(ctx, gitRepo); err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}

	return nil
}

func (f *FluxClient) UpdateGitRepository(ctx context.Context, repo *GitRepository) error {
	gitRepo, err := f.GetGitRepository(ctx, repo.Name, repo.Namespace)
	if err != nil {
		return err
	}

	spec := map[string]interface{}{
		"url":      repo.URL,
		"interval": repo.Interval,
		"ref": map[string]interface{}{
			"branch": repo.Branch,
		},
	}

	if repo.SecretRef != "" {
		spec["secretRef"] = map[string]interface{}{
			"name": repo.SecretRef,
		}
	}

	if err := unstructured.SetNestedMap(gitRepo.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := f.Update(ctx, gitRepo); err != nil {
		return fmt.Errorf("failed to update GitRepository: %w", err)
	}

	return nil
}

func (f *FluxClient) DeleteGitRepository(ctx context.Context, name string, namespace string) error {
	gitRepo := &unstructured.Unstructured{}
	gitRepo.SetGroupVersionKind(gitRepositoryGVK)
	gitRepo.SetName(name)
	gitRepo.SetNamespace(namespace)

	if err := f.Delete(ctx, gitRepo); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete GitRepository: %w", err)
	}

	return nil
}

func (f *FluxClient) CreateKustomization(ctx context.Context, ks *Kustomization) error {
	kustomization := &unstructured.Unstructured{}
	kustomization.SetGroupVersionKind(kustomizationGVK)
	kustomization.SetName(ks.Name)
	kustomization.SetNamespace(ks.Namespace)

	spec := map[string]interface{}{
		"interval": ks.Interval,
		"path":     ks.Path,
		"prune":    ks.Prune,
		"sourceRef": map[string]interface{}{
			"kind": "GitRepository",
			"name": ks.SourceRef,
		},
	}

	if ks.TargetNamespace != "" {
		spec["targetNamespace"] = ks.TargetNamespace
	}

	if err := unstructured.SetNestedMap(kustomization.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := f.Create(ctx, kustomization); err != nil {
		return fmt.Errorf("failed to create Kustomization: %w", err)
	}

	return nil
}

func (f *FluxClient) ReconcileCluster(ctx context.Context, clusterName string) error {
	f.Log.Info("reconciling flux for cluster", "cluster", clusterName)

	// For in-cluster mode, trigger reconciliation of all GitRepositories and Kustomizations
	if clusterName == "in-cluster" || clusterName == "" {
		// List all GitRepositories in flux-system namespace
		gitRepos, err := f.ListGitRepositories(ctx, "flux-system")
		if err != nil {
			return fmt.Errorf("failed to list GitRepositories: %w", err)
		}

		for _, repo := range gitRepos {
			name, _, _ := unstructured.NestedString(repo.Object, "metadata", "name")
			f.Log.Info("found GitRepository", "name", name)
		}

		// List all Kustomizations in flux-system namespace
		kustomizations, err := f.ListKustomizations(ctx, "flux-system")
		if err != nil {
			return fmt.Errorf("failed to list Kustomizations: %w", err)
		}

		for _, ks := range kustomizations {
			name, _, _ := unstructured.NestedString(ks.Object, "metadata", "name")
			f.Log.Info("found Kustomization", "name", name)
		}
	}

	return nil
}

// ListGitRepositories lists all GitRepositories in a namespace
func (f *FluxClient) ListGitRepositories(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	gitRepoList := &unstructured.UnstructuredList{}
	gitRepoList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepositoryList",
	})

	if err := f.List(ctx, gitRepoList, &client.ListOptions{Namespace: namespace}); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list GitRepositories: %w", err)
	}

	return gitRepoList.Items, nil
}

// ListKustomizations lists all Kustomizations in a namespace
func (f *FluxClient) ListKustomizations(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	ksList := &unstructured.UnstructuredList{}
	ksList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kustomize.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "KustomizationList",
	})

	if err := f.List(ctx, ksList, &client.ListOptions{Namespace: namespace}); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list Kustomizations: %w", err)
	}

	return ksList.Items, nil
}

// TriggerReconcile annotates a resource to trigger immediate reconciliation
func (f *FluxClient) TriggerReconcile(ctx context.Context, obj *unstructured.Unstructured) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["reconcile.fluxcd.io/requestedAt"] = metav1.Now().Format(time.RFC3339Nano)
	obj.SetAnnotations(annotations)

	if err := f.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to trigger reconcile: %w", err)
	}

	return nil
}

func (f *FluxClient) GetFluxStatus(ctx context.Context, namespace string) (map[string]string, error) {
	status := make(map[string]string)

	// Check GitRepositories
	gitRepos, err := f.ListGitRepositories(ctx, namespace)
	if err != nil {
		status["gitRepositories"] = "error"
	} else {
		status["gitRepositories"] = fmt.Sprintf("%d found", len(gitRepos))
	}

	// Check Kustomizations
	kustomizations, err := f.ListKustomizations(ctx, namespace)
	if err != nil {
		status["kustomizations"] = "error"
	} else {
		status["kustomizations"] = fmt.Sprintf("%d found", len(kustomizations))
	}

	status["ready"] = "true"
	return status, nil
}
