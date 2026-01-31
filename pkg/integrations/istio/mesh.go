package istio

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	peerAuthenticationGVK = schema.GroupVersionKind{
		Group:   "security.istio.io",
		Version: "v1beta1",
		Kind:    "PeerAuthentication",
	}
	authorizationPolicyGVK = schema.GroupVersionKind{
		Group:   "security.istio.io",
		Version: "v1beta1",
		Kind:    "AuthorizationPolicy",
	}
)

type MeshConfig struct {
	Name                   string
	Namespace              string
	EnableAutoMTLS         bool
	EnableTracing          bool
	EnableAccessLog        bool
	DefaultServiceExportTo []string
	OutboundTrafficPolicy  string
}

type PeerAuthentication struct {
	Name      string
	Namespace string
	Selector  map[string]string
	MTLSMode  string
	PortLevel map[uint32]string
}

type AuthorizationPolicy struct {
	Name      string
	Namespace string
	Selector  map[string]string
	Action    string
	Rules     []Rule
}

type Rule struct {
	From []From
	To   []To
	When []Condition
}

type From struct {
	Source *Source
}

type Source struct {
	Principals        []string
	Namespaces        []string
	IPBlocks          []string
	RemoteIPBlocks    []string
	RequestPrincipals []string
}

type To struct {
	Operation *Operation
}

type Operation struct {
	Hosts   []string
	Ports   []string
	Methods []string
	Paths   []string
}

type Condition struct {
	Key    string
	Values []string
}

type ServiceMesh struct {
	client.Client
}

func NewServiceMesh(c client.Client) *ServiceMesh {
	return &ServiceMesh{
		Client: c,
	}
}

func (sm *ServiceMesh) ConfigureMesh(ctx context.Context, config *MeshConfig) error {
	if config.EnableAutoMTLS {
		if err := sm.enableAutoMTLS(ctx, config.Namespace); err != nil {
			return fmt.Errorf("failed to enable auto mTLS: %w", err)
		}
	}

	return nil
}

func (sm *ServiceMesh) enableAutoMTLS(ctx context.Context, namespace string) error {
	peerAuth := &PeerAuthentication{
		Name:      "default",
		Namespace: namespace,
		MTLSMode:  "STRICT",
	}

	return sm.CreatePeerAuthentication(ctx, peerAuth)
}

func (sm *ServiceMesh) CreatePeerAuthentication(ctx context.Context, pa *PeerAuthentication) error {
	peerAuth := &unstructured.Unstructured{}
	peerAuth.SetGroupVersionKind(peerAuthenticationGVK)
	peerAuth.SetName(pa.Name)
	peerAuth.SetNamespace(pa.Namespace)

	spec := map[string]interface{}{
		"mtls": map[string]interface{}{
			"mode": pa.MTLSMode,
		},
	}

	if len(pa.Selector) > 0 {
		spec["selector"] = map[string]interface{}{
			"matchLabels": pa.Selector,
		}
	}

	if len(pa.PortLevel) > 0 {
		portLevelMTLS := make(map[string]interface{})
		for port, mode := range pa.PortLevel {
			portLevelMTLS[fmt.Sprintf("%d", port)] = map[string]interface{}{
				"mode": mode,
			}
		}
		spec["portLevelMtls"] = portLevelMTLS
	}

	if err := unstructured.SetNestedMap(peerAuth.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := sm.Create(ctx, peerAuth); err != nil {
		return fmt.Errorf("failed to create PeerAuthentication: %w", err)
	}

	return nil
}

func (sm *ServiceMesh) GetPeerAuthentication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	pa := &unstructured.Unstructured{}
	pa.SetGroupVersionKind(peerAuthenticationGVK)

	if err := sm.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, pa); err != nil {
		return nil, fmt.Errorf("failed to get PeerAuthentication: %w", err)
	}

	return pa, nil
}

func (sm *ServiceMesh) CreateAuthorizationPolicy(ctx context.Context, ap *AuthorizationPolicy) error {
	authPolicy := &unstructured.Unstructured{}
	authPolicy.SetGroupVersionKind(authorizationPolicyGVK)
	authPolicy.SetName(ap.Name)
	authPolicy.SetNamespace(ap.Namespace)

	spec := map[string]interface{}{
		"action": ap.Action,
	}

	if len(ap.Selector) > 0 {
		spec["selector"] = map[string]interface{}{
			"matchLabels": ap.Selector,
		}
	}

	if len(ap.Rules) > 0 {
		rules := make([]interface{}, 0, len(ap.Rules))
		for _, rule := range ap.Rules {
			ruleMap := make(map[string]interface{})

			// Add From clause
			if len(rule.From) > 0 {
				fromList := make([]interface{}, 0, len(rule.From))
				for _, from := range rule.From {
					fromMap := make(map[string]interface{})
					if from.Source != nil {
						source := make(map[string]interface{})
						if len(from.Source.Principals) > 0 {
							source["principals"] = from.Source.Principals
						}
						if len(from.Source.Namespaces) > 0 {
							source["namespaces"] = from.Source.Namespaces
						}
						if len(from.Source.IPBlocks) > 0 {
							source["ipBlocks"] = from.Source.IPBlocks
						}
						fromMap["source"] = source
					}
					fromList = append(fromList, fromMap)
				}
				ruleMap["from"] = fromList
			}

			// Add To clause
			if len(rule.To) > 0 {
				toList := make([]interface{}, 0, len(rule.To))
				for _, to := range rule.To {
					toMap := make(map[string]interface{})
					if to.Operation != nil {
						operation := make(map[string]interface{})
						if len(to.Operation.Hosts) > 0 {
							operation["hosts"] = to.Operation.Hosts
						}
						if len(to.Operation.Ports) > 0 {
							operation["ports"] = to.Operation.Ports
						}
						if len(to.Operation.Methods) > 0 {
							operation["methods"] = to.Operation.Methods
						}
						if len(to.Operation.Paths) > 0 {
							operation["paths"] = to.Operation.Paths
						}
						toMap["operation"] = operation
					}
					toList = append(toList, toMap)
				}
				ruleMap["to"] = toList
			}

			// Add When clause
			if len(rule.When) > 0 {
				whenList := make([]interface{}, 0, len(rule.When))
				for _, when := range rule.When {
					whenMap := map[string]interface{}{
						"key":    when.Key,
						"values": when.Values,
					}
					whenList = append(whenList, whenMap)
				}
				ruleMap["when"] = whenList
			}

			rules = append(rules, ruleMap)
		}
		spec["rules"] = rules
	}

	if err := unstructured.SetNestedMap(authPolicy.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set spec: %w", err)
	}

	if err := sm.Create(ctx, authPolicy); err != nil {
		return fmt.Errorf("failed to create AuthorizationPolicy: %w", err)
	}

	return nil
}

func (sm *ServiceMesh) GetAuthorizationPolicy(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	ap := &unstructured.Unstructured{}
	ap.SetGroupVersionKind(authorizationPolicyGVK)

	if err := sm.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, ap); err != nil {
		return nil, fmt.Errorf("failed to get AuthorizationPolicy: %w", err)
	}

	return ap, nil
}

func (sm *ServiceMesh) EnableMTLSForNamespace(ctx context.Context, namespace string, mode string) error {
	pa := &PeerAuthentication{
		Name:      "default",
		Namespace: namespace,
		MTLSMode:  mode,
	}

	return sm.CreatePeerAuthentication(ctx, pa)
}

func (sm *ServiceMesh) CreateDenyAllPolicy(ctx context.Context, namespace string) error {
	ap := &AuthorizationPolicy{
		Name:      "deny-all",
		Namespace: namespace,
		Action:    "DENY",
		Rules:     []Rule{},
	}

	return sm.CreateAuthorizationPolicy(ctx, ap)
}

func (sm *ServiceMesh) CreateAllowAllPolicy(ctx context.Context, namespace string) error {
	ap := &AuthorizationPolicy{
		Name:      "allow-all",
		Namespace: namespace,
		Action:    "ALLOW",
		Rules: []Rule{
			{
				From: []From{
					{
						Source: &Source{
							Principals: []string{"*"},
						},
					},
				},
			},
		},
	}

	return sm.CreateAuthorizationPolicy(ctx, ap)
}

func (sm *ServiceMesh) GetMeshStatus(ctx context.Context) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Count VirtualServices
	vsList := &unstructured.UnstructuredList{}
	vsList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "VirtualServiceList",
	})
	if err := sm.List(ctx, vsList); err == nil {
		status["virtualServices"] = len(vsList.Items)
	}

	// Count DestinationRules
	drList := &unstructured.UnstructuredList{}
	drList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "DestinationRuleList",
	})
	if err := sm.List(ctx, drList); err == nil {
		status["destinationRules"] = len(drList.Items)
	}

	// Count PeerAuthentications
	paList := &unstructured.UnstructuredList{}
	paList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "security.istio.io",
		Version: "v1beta1",
		Kind:    "PeerAuthenticationList",
	})
	if err := sm.List(ctx, paList); err == nil {
		status["peerAuthentications"] = len(paList.Items)
	}

	// Count Gateways
	gwList := &unstructured.UnstructuredList{}
	gwList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "GatewayList",
	})
	if err := sm.List(ctx, gwList); err == nil {
		status["gateways"] = len(gwList.Items)
	}

	status["healthy"] = true
	return status, nil
}

// ListVirtualServices lists all VirtualServices in a namespace
func (sm *ServiceMesh) ListVirtualServices(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	vsList := &unstructured.UnstructuredList{}
	vsList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "VirtualServiceList",
	})

	opts := &client.ListOptions{}
	if namespace != "" {
		opts.Namespace = namespace
	}

	if err := sm.List(ctx, vsList, opts); err != nil {
		return nil, fmt.Errorf("failed to list VirtualServices: %w", err)
	}

	return vsList.Items, nil
}

// ListDestinationRules lists all DestinationRules in a namespace
func (sm *ServiceMesh) ListDestinationRules(ctx context.Context, namespace string) ([]unstructured.Unstructured, error) {
	drList := &unstructured.UnstructuredList{}
	drList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "networking.istio.io",
		Version: "v1beta1",
		Kind:    "DestinationRuleList",
	})

	opts := &client.ListOptions{}
	if namespace != "" {
		opts.Namespace = namespace
	}

	if err := sm.List(ctx, drList, opts); err != nil {
		return nil, fmt.Errorf("failed to list DestinationRules: %w", err)
	}

	return drList.Items, nil
}

// ConfigureMultiClusterMesh configures mesh for multi-cluster setup
func (sm *ServiceMesh) ConfigureMultiClusterMesh(ctx context.Context, config *MeshConfig, clusters []string) error {
	// Enable mTLS for the namespace
	if config.EnableAutoMTLS {
		if err := sm.EnableMTLSForNamespace(ctx, config.Namespace, "STRICT"); err != nil {
			// Ignore if already exists
			if !isAlreadyExistsError(err) {
				return fmt.Errorf("failed to enable mTLS: %w", err)
			}
		}
	}

	return nil
}

// isAlreadyExistsError checks if error is an "already exists" error
func isAlreadyExistsError(err error) bool {
	return err != nil && (err.Error() == "already exists" ||
		(len(err.Error()) > 14 && err.Error()[len(err.Error())-14:] == "already exists"))
}
