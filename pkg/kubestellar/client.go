package kubestellar

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// bindingPolicyGVK is the GroupVersionKind for BindingPolicy
var bindingPolicyGVK = schema.GroupVersionKind{
	Group:   "control.kubestellar.io",
	Version: "v1alpha1",
	Kind:    "BindingPolicy",
}

// KubeStellarClient wraps a controller-runtime client for KubeStellar resources
type KubeStellarClient struct {
	client.Client
	Config *rest.Config
	Scheme *runtime.Scheme
}

// NewKubeStellarClient creates a new KubeStellar client
func NewKubeStellarClient(config *rest.Config, scheme *runtime.Scheme) (*KubeStellarClient, error) {
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &KubeStellarClient{
		Client: c,
		Config: config,
		Scheme: scheme,
	}, nil
}

// BindingPolicy represents a KubeStellar BindingPolicy
type BindingPolicy struct {
	Name             string
	Namespace        string
	Labels           map[string]string
	Annotations      map[string]string
	ClusterSelectors []ClusterSelector
	DownSyncRules    []DownSyncRule
}

// ClusterSelector defines cluster selection criteria
type ClusterSelector struct {
	MatchLabels      map[string]string
	MatchExpressions []SelectorRequirement
}

// SelectorRequirement defines a label selector requirement
type SelectorRequirement struct {
	Key      string
	Operator string
	Values   []string
}

// DownSyncRule defines what to sync down to clusters
type DownSyncRule struct {
	APIGroup       string
	Resources      []string
	Namespaces     []string
	ObjectNames    []string
	LabelSelectors []metav1.LabelSelector
}

// CreateBindingPolicy creates a new BindingPolicy
func (kc *KubeStellarClient) CreateBindingPolicy(ctx context.Context, bp *BindingPolicy) error {
	bindingPolicy := &unstructured.Unstructured{}
	bindingPolicy.SetGroupVersionKind(bindingPolicyGVK)
	bindingPolicy.SetName(bp.Name)
	bindingPolicy.SetNamespace(bp.Namespace)

	if bp.Labels != nil {
		bindingPolicy.SetLabels(bp.Labels)
	}
	if bp.Annotations != nil {
		bindingPolicy.SetAnnotations(bp.Annotations)
	}

	spec := make(map[string]interface{})

	// Set cluster selectors
	if len(bp.ClusterSelectors) > 0 {
		selectors := make([]interface{}, 0, len(bp.ClusterSelectors))
		for _, selector := range bp.ClusterSelectors {
			s := make(map[string]interface{})
			if len(selector.MatchLabels) > 0 {
				s["matchLabels"] = selector.MatchLabels
			}
			if len(selector.MatchExpressions) > 0 {
				expressions := make([]interface{}, 0, len(selector.MatchExpressions))
				for _, expr := range selector.MatchExpressions {
					expressions = append(expressions, map[string]interface{}{
						"key":      expr.Key,
						"operator": expr.Operator,
						"values":   expr.Values,
					})
				}
				s["matchExpressions"] = expressions
			}
			selectors = append(selectors, s)
		}
		spec["clusterSelectors"] = selectors
	}

	// Set downsync rules
	if len(bp.DownSyncRules) > 0 {
		rules := make([]interface{}, 0, len(bp.DownSyncRules))
		for _, rule := range bp.DownSyncRules {
			r := map[string]interface{}{
				"apiGroup":  rule.APIGroup,
				"resources": rule.Resources,
			}
			if len(rule.Namespaces) > 0 {
				r["namespaces"] = rule.Namespaces
			}
			if len(rule.ObjectNames) > 0 {
				r["objectNames"] = rule.ObjectNames
			}
			rules = append(rules, r)
		}
		spec["downsync"] = rules
	}

	if err := unstructured.SetNestedMap(bindingPolicy.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := kc.Create(ctx, bindingPolicy); err != nil {
		return fmt.Errorf("failed to create BindingPolicy: %w", err)
	}

	return nil
}

// GetBindingPolicy retrieves a BindingPolicy
func (kc *KubeStellarClient) GetBindingPolicy(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	bp := &unstructured.Unstructured{}
	bp.SetGroupVersionKind(bindingPolicyGVK)

	if err := kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, bp); err != nil {
		return nil, fmt.Errorf("failed to get BindingPolicy: %w", err)
	}

	return bp, nil
}

// UpdateBindingPolicy updates an existing BindingPolicy
func (kc *KubeStellarClient) UpdateBindingPolicy(ctx context.Context, bp *BindingPolicy) error {
	existing, err := kc.GetBindingPolicy(ctx, bp.Name, bp.Namespace)
	if err != nil {
		return err
	}

	if bp.Labels != nil {
		existing.SetLabels(bp.Labels)
	}
	if bp.Annotations != nil {
		existing.SetAnnotations(bp.Annotations)
	}

	spec := make(map[string]interface{})

	// Update cluster selectors
	if len(bp.ClusterSelectors) > 0 {
		selectors := make([]interface{}, 0, len(bp.ClusterSelectors))
		for _, selector := range bp.ClusterSelectors {
			s := make(map[string]interface{})
			if len(selector.MatchLabels) > 0 {
				s["matchLabels"] = selector.MatchLabels
			}
			selectors = append(selectors, s)
		}
		spec["clusterSelectors"] = selectors
	}

	// Update downsync rules
	if len(bp.DownSyncRules) > 0 {
		rules := make([]interface{}, 0, len(bp.DownSyncRules))
		for _, rule := range bp.DownSyncRules {
			r := map[string]interface{}{
				"apiGroup":  rule.APIGroup,
				"resources": rule.Resources,
			}
			if len(rule.Namespaces) > 0 {
				r["namespaces"] = rule.Namespaces
			}
			rules = append(rules, r)
		}
		spec["downsync"] = rules
	}

	if err := unstructured.SetNestedMap(existing.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := kc.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update BindingPolicy: %w", err)
	}

	return nil
}

// DeleteBindingPolicy deletes a BindingPolicy
func (kc *KubeStellarClient) DeleteBindingPolicy(ctx context.Context, name, namespace string) error {
	bp := &unstructured.Unstructured{}
	bp.SetGroupVersionKind(bindingPolicyGVK)
	bp.SetName(name)
	bp.SetNamespace(namespace)

	if err := kc.Delete(ctx, bp); err != nil {
		return fmt.Errorf("failed to delete BindingPolicy: %w", err)
	}

	return nil
}

// GetBindingPolicyStatus retrieves the status of a BindingPolicy
func (kc *KubeStellarClient) GetBindingPolicyStatus(ctx context.Context, name, namespace string) (map[string]interface{}, error) {
	bp, err := kc.GetBindingPolicy(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	status, found, err := unstructured.NestedMap(bp.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	if !found {
		return make(map[string]interface{}), nil
	}

	return status, nil
}

// AddClusterSelector adds a cluster selector to a BindingPolicy
func (kc *KubeStellarClient) AddClusterSelector(ctx context.Context, name, namespace string, selector ClusterSelector) error {
	bp, err := kc.GetBindingPolicy(ctx, name, namespace)
	if err != nil {
		return err
	}

	spec, found, err := unstructured.NestedMap(bp.Object, "spec")
	if err != nil || !found {
		spec = make(map[string]interface{})
	}

	selectors, _, _ := unstructured.NestedSlice(spec, "clusterSelectors")

	newSelector := make(map[string]interface{})
	if len(selector.MatchLabels) > 0 {
		newSelector["matchLabels"] = selector.MatchLabels
	}

	selectors = append(selectors, newSelector)
	spec["clusterSelectors"] = selectors

	if err := unstructured.SetNestedMap(bp.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := kc.Update(ctx, bp); err != nil {
		return fmt.Errorf("failed to update BindingPolicy: %w", err)
	}

	return nil
}

// ListBindingPolicies lists all BindingPolicies in a namespace
func (kc *KubeStellarClient) ListBindingPolicies(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	bpList := &unstructured.UnstructuredList{}
	bpList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "control.kubestellar.io",
		Version: "v1alpha1",
		Kind:    "BindingPolicyList",
	})

	opts := &client.ListOptions{}
	if namespace != "" {
		opts.Namespace = namespace
	}

	if err := kc.List(ctx, bpList, opts); err != nil {
		return nil, fmt.Errorf("failed to list BindingPolicies: %w", err)
	}

	return bpList.Items, nil
}
