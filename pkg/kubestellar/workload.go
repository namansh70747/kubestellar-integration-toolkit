package kubestellar

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkloadType represents different types of workloads
type WorkloadType string

const (
	WorkloadTypeDeployment  WorkloadType = "Deployment"
	WorkloadTypeStatefulSet WorkloadType = "StatefulSet"
	WorkloadTypeDaemonSet   WorkloadType = "DaemonSet"
	WorkloadTypeJob         WorkloadType = "Job"
	WorkloadTypeCronJob     WorkloadType = "CronJob"
	WorkloadTypeService     WorkloadType = "Service"
	WorkloadTypeConfigMap   WorkloadType = "ConfigMap"
	WorkloadTypeSecret      WorkloadType = "Secret"
)

// Workload represents a Kubernetes workload to be distributed
type Workload struct {
	Name        string
	Namespace   string
	Type        WorkloadType
	Labels      map[string]string
	Annotations map[string]string
	Spec        map[string]interface{}
}

// CreateWorkload creates a workload resource
func (kc *KubeStellarClient) CreateWorkload(ctx context.Context, workload *Workload) error {
	gvk, err := getGVKForWorkloadType(workload.Type)
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(workload.Name)
	obj.SetNamespace(workload.Namespace)

	if workload.Labels != nil {
		obj.SetLabels(workload.Labels)
	}
	if workload.Annotations != nil {
		obj.SetAnnotations(workload.Annotations)
	}

	if workload.Spec != nil {
		if err := unstructured.SetNestedMap(obj.Object, workload.Spec, "spec"); err != nil {
			return fmt.Errorf("failed to set spec: %w", err)
		}
	}

	if err := kc.Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to create workload: %w", err)
	}

	return nil
}

// GetWorkload retrieves a workload resource
func (kc *KubeStellarClient) GetWorkload(ctx context.Context, workloadType WorkloadType, name, namespace string) (*unstructured.Unstructured, error) {
	gvk, err := getGVKForWorkloadType(workloadType)
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	if err := kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj); err != nil {
		return nil, fmt.Errorf("failed to get workload: %w", err)
	}

	return obj, nil
}

// UpdateWorkload updates a workload resource
func (kc *KubeStellarClient) UpdateWorkload(ctx context.Context, workload *Workload) error {
	existing, err := kc.GetWorkload(ctx, workload.Type, workload.Name, workload.Namespace)
	if err != nil {
		return err
	}

	if workload.Labels != nil {
		existing.SetLabels(workload.Labels)
	}
	if workload.Annotations != nil {
		existing.SetAnnotations(workload.Annotations)
	}

	if workload.Spec != nil {
		if err := unstructured.SetNestedMap(existing.Object, workload.Spec, "spec"); err != nil {
			return fmt.Errorf("failed to set spec: %w", err)
		}
	}

	if err := kc.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update workload: %w", err)
	}

	return nil
}

// DeleteWorkload deletes a workload resource
func (kc *KubeStellarClient) DeleteWorkload(ctx context.Context, workloadType WorkloadType, name, namespace string) error {
	gvk, err := getGVKForWorkloadType(workloadType)
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)

	if err := kc.Delete(ctx, obj); err != nil {
		return fmt.Errorf("failed to delete workload: %w", err)
	}

	return nil
}

// ListWorkloads lists workloads of a specific type in a namespace
func (kc *KubeStellarClient) ListWorkloads(ctx context.Context, workloadType WorkloadType, namespace string) ([]unstructured.Unstructured, error) {
	gvk, err := getGVKForWorkloadType(workloadType)
	if err != nil {
		return nil, err
	}

	listGVK := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	}

	objList := &unstructured.UnstructuredList{}
	objList.SetGroupVersionKind(listGVK)

	listOpts := &client.ListOptions{
		Namespace: namespace,
	}

	if err := kc.List(ctx, objList, listOpts); err != nil {
		return nil, fmt.Errorf("failed to list workloads: %w", err)
	}

	return objList.Items, nil
}

// ScaleWorkload scales a workload (for Deployments, StatefulSets, etc.)
func (kc *KubeStellarClient) ScaleWorkload(ctx context.Context, workloadType WorkloadType, name, namespace string, replicas int32) error {
	if workloadType != WorkloadTypeDeployment && workloadType != WorkloadTypeStatefulSet {
		return fmt.Errorf("scaling is only supported for Deployments and StatefulSets")
	}

	obj, err := kc.GetWorkload(ctx, workloadType, name, namespace)
	if err != nil {
		return err
	}

	if err := unstructured.SetNestedField(obj.Object, int64(replicas), "spec", "replicas"); err != nil {
		return fmt.Errorf("failed to set replicas: %w", err)
	}

	if err := kc.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to update workload: %w", err)
	}

	return nil
}

// getGVKForWorkloadType returns the GVK for a workload type
func getGVKForWorkloadType(workloadType WorkloadType) (schema.GroupVersionKind, error) {
	switch workloadType {
	case WorkloadTypeDeployment:
		return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, nil
	case WorkloadTypeStatefulSet:
		return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, nil
	case WorkloadTypeDaemonSet:
		return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, nil
	case WorkloadTypeJob:
		return schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, nil
	case WorkloadTypeCronJob:
		return schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}, nil
	case WorkloadTypeService:
		return schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}, nil
	case WorkloadTypeConfigMap:
		return schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, nil
	case WorkloadTypeSecret:
		return schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, nil
	default:
		return schema.GroupVersionKind{}, fmt.Errorf("unsupported workload type: %s", workloadType)
	}
}
