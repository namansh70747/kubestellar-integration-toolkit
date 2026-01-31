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

// NewDefaultKubeStellarClient creates a new KubeStellar client with default config
func NewDefaultKubeStellarClient() (*KubeStellarClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	scheme := runtime.NewScheme()

	return NewKubeStellarClient(config, scheme)
}

// HealthCheck performs a health check on the KubeStellar API server
func (kc *KubeStellarClient) HealthCheck(ctx context.Context) error {
	_, err := kc.ListBindingPolicies(ctx, "")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// GetServerVersion retrieves the server version
func (kc *KubeStellarClient) GetServerVersion(ctx context.Context) (string, error) {
	return "v1alpha1", nil
}

// ValidateConnection validates the client connection
func (kc *KubeStellarClient) ValidateConnection(ctx context.Context) error {
	return kc.HealthCheck(ctx)
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
		clusterSelectors := make([]interface{}, 0, len(bp.ClusterSelectors))
		for _, cs := range bp.ClusterSelectors {
			selector := make(map[string]interface{})

			if len(cs.MatchLabels) > 0 {
				selector["matchLabels"] = cs.MatchLabels
			}

			if len(cs.MatchExpressions) > 0 {
				matchExpressions := make([]interface{}, 0, len(cs.MatchExpressions))
				for _, expr := range cs.MatchExpressions {
					matchExpressions = append(matchExpressions, map[string]interface{}{
						"key":      expr.Key,
						"operator": expr.Operator,
						"values":   expr.Values,
					})
				}
				selector["matchExpressions"] = matchExpressions
			}

			clusterSelectors = append(clusterSelectors, selector)
		}
		spec["clusterSelectors"] = clusterSelectors
	}

	// Set downsync rules
	if len(bp.DownSyncRules) > 0 {
		downSyncRules := make([]interface{}, 0, len(bp.DownSyncRules))
		for _, rule := range bp.DownSyncRules {
			ruleMap := make(map[string]interface{})

			if rule.APIGroup != "" {
				ruleMap["apiGroup"] = rule.APIGroup
			}

			if len(rule.Resources) > 0 {
				ruleMap["resources"] = rule.Resources
			}

			if len(rule.Namespaces) > 0 {
				ruleMap["namespaces"] = rule.Namespaces
			}

			if len(rule.ObjectNames) > 0 {
				ruleMap["objectNames"] = rule.ObjectNames
			}

			if len(rule.LabelSelectors) > 0 {
				labelSelectors := make([]interface{}, 0, len(rule.LabelSelectors))
				for _, ls := range rule.LabelSelectors {
					labelSelector := make(map[string]interface{})

					if len(ls.MatchLabels) > 0 {
						labelSelector["matchLabels"] = ls.MatchLabels
					}

					if len(ls.MatchExpressions) > 0 {
						matchExpressions := make([]interface{}, 0, len(ls.MatchExpressions))
						for _, expr := range ls.MatchExpressions {
							matchExpressions = append(matchExpressions, map[string]interface{}{
								"key":      expr.Key,
								"operator": string(expr.Operator),
								"values":   expr.Values,
							})
						}
						labelSelector["matchExpressions"] = matchExpressions
					}

					labelSelectors = append(labelSelectors, labelSelector)
				}
				ruleMap["labelSelectors"] = labelSelectors
			}

			downSyncRules = append(downSyncRules, ruleMap)
		}
		spec["downsync"] = downSyncRules
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
		clusterSelectors := make([]interface{}, 0, len(bp.ClusterSelectors))
		for _, cs := range bp.ClusterSelectors {
			selector := make(map[string]interface{})

			if len(cs.MatchLabels) > 0 {
				selector["matchLabels"] = cs.MatchLabels
			}

			if len(cs.MatchExpressions) > 0 {
				matchExpressions := make([]interface{}, 0, len(cs.MatchExpressions))
				for _, expr := range cs.MatchExpressions {
					matchExpressions = append(matchExpressions, map[string]interface{}{
						"key":      expr.Key,
						"operator": expr.Operator,
						"values":   expr.Values,
					})
				}
				selector["matchExpressions"] = matchExpressions
			}

			clusterSelectors = append(clusterSelectors, selector)
		}
		spec["clusterSelectors"] = clusterSelectors
	}

	// Update downsync rules
	if len(bp.DownSyncRules) > 0 {
		downSyncRules := make([]interface{}, 0, len(bp.DownSyncRules))
		for _, rule := range bp.DownSyncRules {
			ruleMap := make(map[string]interface{})

			if rule.APIGroup != "" {
				ruleMap["apiGroup"] = rule.APIGroup
			}
			if len(rule.Resources) > 0 {
				ruleMap["resources"] = rule.Resources
			}
			if len(rule.Namespaces) > 0 {
				ruleMap["namespaces"] = rule.Namespaces
			}
			if len(rule.ObjectNames) > 0 {
				ruleMap["objectNames"] = rule.ObjectNames
			}

			downSyncRules = append(downSyncRules, ruleMap)
		}
		spec["downsync"] = downSyncRules
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
	if err != nil || !found {
		return map[string]interface{}{}, nil
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
	if err != nil {
		return fmt.Errorf("failed to get spec: %w", err)
	}
	if !found {
		spec = make(map[string]interface{})
	}

	clusterSelectors, found, err := unstructured.NestedSlice(spec, "clusterSelectors")
	if err != nil {
		return fmt.Errorf("failed to get clusterSelectors: %w", err)
	}
	if !found {
		clusterSelectors = []interface{}{}
	}

	newSelector := make(map[string]interface{})
	if len(selector.MatchLabels) > 0 {
		newSelector["matchLabels"] = selector.MatchLabels
	}

	clusterSelectors = append(clusterSelectors, newSelector)
	spec["clusterSelectors"] = clusterSelectors

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

	listOpts := &client.ListOptions{
		Namespace: namespace,
	}

	if err := kc.List(ctx, bpList, listOpts); err != nil {
		return nil, fmt.Errorf("failed to list BindingPolicies: %w", err)
	}

	return bpList.Items, nil
}
