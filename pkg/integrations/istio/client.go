package istio

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	virtualServiceGVK = schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "VirtualService",
	}
	destinationRuleGVK = schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "DestinationRule",
	}
)

// Client represents an Istio client
type Client struct {
	client.Client
	config    *rest.Config
	namespace string
}

// NewClient creates a new Istio client
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		Client:    c,
		config:    config,
		namespace: "istio-system",
	}, nil
}

// NewClientWithConfig creates a new Istio client with custom config
func NewClientWithConfig(config *rest.Config, namespace string) (*Client, error) {
	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		Client:    c,
		config:    config,
		namespace: namespace,
	}, nil
}

// HealthCheck performs a health check on Istio
func (c *Client) HealthCheck() error {
	vsList := &unstructured.UnstructuredList{}
	vsList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "VirtualServiceList",
	})

	if err := c.List(context.Background(), vsList, &client.ListOptions{Namespace: c.namespace}); err != nil {
		return fmt.Errorf("istio health check failed: %w", err)
	}

	return nil
}

// VirtualService represents an Istio VirtualService
type VirtualService struct {
	Name      string
	Namespace string
	Hosts     []string
	Gateways  []string
	HTTP      []HTTPRoute
}

// HTTPRoute represents an HTTP route
type HTTPRoute struct {
	Match []HTTPMatchRequest
	Route []HTTPRouteDestination
}

// HTTPMatchRequest represents match conditions
type HTTPMatchRequest struct {
	URI    *StringMatch
	Method *StringMatch
}

// StringMatch represents string matching
type StringMatch struct {
	Exact  string
	Prefix string
	Regex  string
}

// HTTPRouteDestination represents a route destination
type HTTPRouteDestination struct {
	Host   string
	Port   uint32
	Weight int32
}

// CreateVirtualService creates a VirtualService
func (c *Client) CreateVirtualService(ctx context.Context, vs *VirtualService) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(virtualServiceGVK)
	obj.SetName(vs.Name)
	obj.SetNamespace(vs.Namespace)

	spec := map[string]interface{}{
		"hosts": vs.Hosts,
	}

	if len(vs.Gateways) > 0 {
		spec["gateways"] = vs.Gateways
	}

	if len(vs.HTTP) > 0 {
		httpRoutes := make([]interface{}, 0, len(vs.HTTP))
		for _, route := range vs.HTTP {
			r := make(map[string]interface{})
			if len(route.Route) > 0 {
				destinations := make([]interface{}, 0, len(route.Route))
				for _, dest := range route.Route {
					d := map[string]interface{}{
						"destination": map[string]interface{}{
							"host": dest.Host,
							"port": map[string]interface{}{
								"number": dest.Port,
							},
						},
						"weight": dest.Weight,
					}
					destinations = append(destinations, d)
				}
				r["route"] = destinations
			}
			httpRoutes = append(httpRoutes, r)
		}
		spec["http"] = httpRoutes
	}

	if err := unstructured.SetNestedMap(obj.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := c.Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to create VirtualService: %w", err)
	}

	return nil
}

// GetVirtualService retrieves a VirtualService
func (c *Client) GetVirtualService(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	vs := &unstructured.Unstructured{}
	vs.SetGroupVersionKind(virtualServiceGVK)

	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vs); err != nil {
		return nil, fmt.Errorf("failed to get VirtualService: %w", err)
	}

	return vs, nil
}

// DeleteVirtualService deletes a VirtualService
func (c *Client) DeleteVirtualService(ctx context.Context, name, namespace string) error {
	vs := &unstructured.Unstructured{}
	vs.SetGroupVersionKind(virtualServiceGVK)
	vs.SetName(name)
	vs.SetNamespace(namespace)

	if err := c.Delete(ctx, vs); err != nil {
		return fmt.Errorf("failed to delete VirtualService: %w", err)
	}

	return nil
}

// ReconcileCluster reconciles Istio configuration for a cluster
func (c *Client) ReconcileCluster(ctx context.Context, clusterName string) error {
	return nil
}
