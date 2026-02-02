package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

var (
	labelKeyRegex   = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`)
	labelValueRegex = regexp.MustCompile(`^([a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?)?$`)
)

// ValidationRequest represents a validation request for HTTP endpoints
type ValidationRequest struct {
	ClusterName string `json:"clusterName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Type        string `json:"type,omitempty"`
}

// ValidationResponse represents a validation response for HTTP endpoints
type ValidationResponse struct {
	IsValid bool     `json:"isValid"`
	Errors  []string `json:"errors,omitempty"`
}

// IntegrationValidator validates Integration resources
type IntegrationValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// NewIntegrationValidator creates a new IntegrationValidator
func NewIntegrationValidator(c client.Client) *IntegrationValidator {
	return &IntegrationValidator{
		Client: c,
	}
}

// Handle validates Integration admission requests
func (v *IntegrationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	integration := &ksitv1alpha1.Integration{}

	err := v.decoder.Decode(req, integration)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	errors := v.validateIntegration(integration)
	if len(errors) > 0 {
		return admission.Denied(strings.Join(errors, "; "))
	}

	return admission.Allowed("")
}

// validateIntegration performs validation checks on Integration resource
func (v *IntegrationValidator) validateIntegration(integration *ksitv1alpha1.Integration) []string {
	var errors []string

	// Validate type
	validTypes := []string{
		ksitv1alpha1.IntegrationTypeArgoCD,
		ksitv1alpha1.IntegrationTypeFlux,
		ksitv1alpha1.IntegrationTypePrometheus,
		ksitv1alpha1.IntegrationTypeIstio,
	}

	isValidType := false
	for _, validType := range validTypes {
		if integration.Spec.Type == validType {
			isValidType = true
			break
		}
	}

	if !isValidType {
		errors = append(errors, fmt.Sprintf("invalid integration type: %s", integration.Spec.Type))
	}

	// Validate target clusters
	if len(integration.Spec.TargetClusters) == 0 {
		errors = append(errors, "targetClusters cannot be empty")
	}

	for _, cluster := range integration.Spec.TargetClusters {
		if cluster == "" {
			errors = append(errors, "cluster name cannot be empty")
		}
	}

	// Type-specific validation
	switch integration.Spec.Type {
	case ksitv1alpha1.IntegrationTypeArgoCD:
		if integration.Spec.Config["serverURL"] == "" {
			errors = append(errors, "ArgoCD integration requires serverURL in config")
		}
	case ksitv1alpha1.IntegrationTypeFlux:
		if integration.Spec.Config["namespace"] == "" {
			errors = append(errors, "Flux integration requires namespace in config")
		}
	case ksitv1alpha1.IntegrationTypePrometheus:
		if integration.Spec.Config["url"] == "" {
			errors = append(errors, "Prometheus integration requires url in config")
		}
	case ksitv1alpha1.IntegrationTypeIstio:
		if integration.Spec.Config["namespace"] == "" {
			errors = append(errors, "Istio integration requires namespace in config")
		}
	}

	// Validate name
	if integration.Name == "" {
		errors = append(errors, "integration name cannot be empty")
	}

	if len(integration.Name) > 253 {
		errors = append(errors, "integration name exceeds maximum length of 253")
	}

	return errors
}

// InjectDecoder injects the decoder
func (v *IntegrationValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// ValidateCreate implements admission.CustomValidator
func (v *IntegrationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	integration, ok := obj.(*ksitv1alpha1.Integration)
	if !ok {
		return nil, fmt.Errorf("expected Integration but got %T", obj)
	}

	errors := v.validateIntegration(integration)
	if len(errors) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator
func (v *IntegrationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newIntegration, ok := newObj.(*ksitv1alpha1.Integration)
	if !ok {
		return nil, fmt.Errorf("expected Integration but got %T", newObj)
	}

	errors := v.validateIntegration(newIntegration)
	if len(errors) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil, nil
}

// ValidateDelete implements admission.CustomValidator
func (v *IntegrationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// IntegrationTargetValidator validates IntegrationTarget resources
type IntegrationTargetValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// NewIntegrationTargetValidator creates a new IntegrationTargetValidator
func NewIntegrationTargetValidator(c client.Client) *IntegrationTargetValidator {
	return &IntegrationTargetValidator{
		Client: c,
	}
}

// Handle validates IntegrationTarget admission requests
func (v *IntegrationTargetValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	target := &ksitv1alpha1.IntegrationTarget{}

	err := v.decoder.Decode(req, target)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	errors := v.validateIntegrationTarget(target)
	if len(errors) > 0 {
		return admission.Denied(strings.Join(errors, "; "))
	}

	return admission.Allowed("")
}

// validateIntegrationTarget performs validation checks on IntegrationTarget resource
func (v *IntegrationTargetValidator) validateIntegrationTarget(target *ksitv1alpha1.IntegrationTarget) []string {
	var errors []string

	if target.Spec.ClusterName == "" {
		errors = append(errors, "clusterName is required")
	}

	if len(target.Spec.ClusterName) > 253 {
		errors = append(errors, "clusterName exceeds maximum length of 253")
	}

	if target.Spec.Namespace != "" && len(target.Spec.Namespace) > 63 {
		errors = append(errors, "namespace exceeds maximum length of 63")
	}

	// Validate labels
	if len(target.Spec.Labels) > 0 {
		for key, value := range target.Spec.Labels {
			if !isValidLabelKey(key) {
				errors = append(errors, fmt.Sprintf("invalid label key: %s", key))
			}
			if !isValidLabelValue(value) {
				errors = append(errors, fmt.Sprintf("invalid label value: %s", value))
			}
		}
	}

	return errors
}

// isValidLabelKey checks if a label key is valid
func isValidLabelKey(key string) bool {
	if key == "" || len(key) > 63 {
		return false
	}
	return labelKeyRegex.MatchString(key)
}

// isValidLabelValue checks if a label value is valid
func isValidLabelValue(value string) bool {
	if len(value) > 63 {
		return false
	}
	if value == "" {
		return true
	}
	return labelValueRegex.MatchString(value)
}

// InjectDecoder injects the decoder
func (v *IntegrationTargetValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// ValidateCreate implements admission.CustomValidator
func (v *IntegrationTargetValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	target, ok := obj.(*ksitv1alpha1.IntegrationTarget)
	if !ok {
		return nil, fmt.Errorf("expected IntegrationTarget but got %T", obj)
	}

	errors := v.validateIntegrationTarget(target)
	if len(errors) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator
func (v *IntegrationTargetValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newTarget, ok := newObj.(*ksitv1alpha1.IntegrationTarget)
	if !ok {
		return nil, fmt.Errorf("expected IntegrationTarget but got %T", newObj)
	}

	errors := v.validateIntegrationTarget(newTarget)
	if len(errors) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil, nil
}

// ValidateDelete implements admission.CustomValidator
func (v *IntegrationTargetValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// ValidateCluster validates a cluster via HTTP endpoint
func ValidateCluster(w http.ResponseWriter, r *http.Request) {
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := ValidationResponse{IsValid: true, Errors: []string{}}

	// Validate cluster name
	if req.ClusterName == "" {
		response.IsValid = false
		response.Errors = append(response.Errors, "clusterName is required")
	} else if len(req.ClusterName) > 253 {
		response.IsValid = false
		response.Errors = append(response.Errors, "clusterName exceeds maximum length")
	}

	// Validate namespace if provided
	if req.Namespace != "" && len(req.Namespace) > 63 {
		response.IsValid = false
		response.Errors = append(response.Errors, "namespace exceeds maximum length")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ValidateIntegrationType validates an integration type via HTTP endpoint
func ValidateIntegrationType(w http.ResponseWriter, r *http.Request) {
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := ValidationResponse{IsValid: true, Errors: []string{}}

	// Validate integration type
	validTypes := []string{
		ksitv1alpha1.IntegrationTypeArgoCD,
		ksitv1alpha1.IntegrationTypeFlux,
		ksitv1alpha1.IntegrationTypePrometheus,
		ksitv1alpha1.IntegrationTypeIstio,
	}

	isValid := false
	for _, validType := range validTypes {
		if req.Type == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		response.IsValid = false
		response.Errors = append(response.Errors, fmt.Sprintf("invalid integration type: %s", req.Type))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetupWebhookServer sets up the webhook server with validating webhooks
func SetupWebhookServer(mgr ctrl.Manager) error {
	// Register Integration validator
	integrationValidator := NewIntegrationValidator(mgr.GetClient())
	mgr.GetWebhookServer().Register("/validate-v1alpha1-integration", &webhook.Admission{Handler: integrationValidator})

	// Register IntegrationTarget validator
	targetValidator := NewIntegrationTargetValidator(mgr.GetClient())
	mgr.GetWebhookServer().Register("/validate-v1alpha1-integrationtarget", &webhook.Admission{Handler: targetValidator})

	return nil
}
