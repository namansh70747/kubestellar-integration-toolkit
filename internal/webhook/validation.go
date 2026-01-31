package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
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
		errors = append(errors, fmt.Sprintf("invalid integration type: %s, must be one of: %s",
			integration.Spec.Type, strings.Join(validTypes, ", ")))
	}

	if len(integration.Spec.TargetClusters) == 0 {
		errors = append(errors, "at least one target cluster must be specified")
	}

	for _, cluster := range integration.Spec.TargetClusters {
		if cluster == "" {
			errors = append(errors, "cluster name cannot be empty")
		}
		if len(cluster) > 253 {
			errors = append(errors, fmt.Sprintf("cluster name %s exceeds maximum length of 253", cluster))
		}
	}

	switch integration.Spec.Type {
	case ksitv1alpha1.IntegrationTypeArgoCD:
		if integration.Spec.Config["serverURL"] == "" {
			errors = append(errors, "serverURL is required for ArgoCD integration")
		}
	case ksitv1alpha1.IntegrationTypeFlux:
		if integration.Spec.Config["namespace"] == "" {
			errors = append(errors, "namespace is required for Flux integration")
		}
	case ksitv1alpha1.IntegrationTypePrometheus:
		if integration.Spec.Config["url"] == "" {
			errors = append(errors, "url is required for Prometheus integration")
		}
	case ksitv1alpha1.IntegrationTypeIstio:
		if integration.Spec.Config["namespace"] == "" {
			errors = append(errors, "namespace is required for Istio integration")
		}
	}

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
	if !isAlphaNumeric(key[0]) || !isAlphaNumeric(key[len(key)-1]) {
		return false
	}
	return true
}

// isValidLabelValue checks if a label value is valid
func isValidLabelValue(value string) bool {
	if len(value) > 63 {
		return false
	}
	if value == "" {
		return true
	}
	if !isAlphaNumeric(value[0]) || !isAlphaNumeric(value[len(value)-1]) {
		return false
	}
	return true
}

// isAlphaNumeric checks if a byte is alphanumeric
func isAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// InjectDecoder injects the decoder
func (v *IntegrationTargetValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// ValidateCluster validates a cluster via HTTP endpoint
func ValidateCluster(w http.ResponseWriter, r *http.Request) {
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := ValidationResponse{IsValid: true}

	// Validate cluster name
	if req.ClusterName == "" {
		response.IsValid = false
		response.Errors = append(response.Errors, "clusterName is required")
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

	response := ValidationResponse{IsValid: true}

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
		response.Errors = append(response.Errors, fmt.Sprintf("invalid type: %s", req.Type))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetupWebhookServer sets up the webhook server with validating webhooks
func SetupWebhookServer(mgr ctrl.Manager) error {
	hookServer := mgr.GetWebhookServer()

	integrationValidator := NewIntegrationValidator(mgr.GetClient())
	hookServer.Register("/validate-v1alpha1-integration",
		&webhook.Admission{Handler: integrationValidator})

	targetValidator := NewIntegrationTargetValidator(mgr.GetClient())
	hookServer.Register("/validate-v1alpha1-integrationtarget",
		&webhook.Admission{Handler: targetValidator})

	return nil
}
