package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Integration type constants
const (
    IntegrationTypeArgoCD     = "argocd"
    IntegrationTypeFlux       = "flux"
    IntegrationTypePrometheus = "prometheus"
    IntegrationTypeIstio      = "istio"
)

// Phase constants
const (
    PhaseInitializing = "Initializing"
    PhaseRunning      = "Running"
    PhaseFailed       = "Failed"
    PhaseSucceeded    = "Succeeded"
)

// Condition types
const (
    ConditionTypeReady       = "Ready"
    ConditionTypeProgressing = "Progressing"
    ConditionTypeDegraded    = "Degraded"
)

// IntegrationSpec defines the desired state of Integration
type IntegrationSpec struct {
    // Type specifies the integration type (argocd, flux, prometheus, istio)
    // +kubebuilder:validation:Enum=argocd;flux;prometheus;istio
    // +kubebuilder:validation:Required
    Type string `json:"type"`

    // Enabled determines if the integration is active
    // +kubebuilder:default=true
    Enabled bool `json:"enabled,omitempty"`

    // TargetClusters is the list of clusters to target
    TargetClusters []string `json:"targetClusters,omitempty"`

    // Config holds integration-specific configuration
    Config map[string]string `json:"config,omitempty"`
}

// ClusterStatus represents the status of a target cluster
type ClusterStatus struct {
    // Name of the cluster
    Name string `json:"name"`

    // Connected indicates if the cluster is reachable
    Connected bool `json:"connected"`

    // LastSeen is the last time the cluster was seen
    LastSeen metav1.Time `json:"lastSeen,omitempty"`

    // Message provides additional information
    Message string `json:"message,omitempty"`
}

// IntegrationStatus defines the observed state of Integration
type IntegrationStatus struct {
    // Phase represents the current phase of the integration
    // +kubebuilder:validation:Enum=Initializing;Running;Failed;Succeeded
    Phase string `json:"phase,omitempty"`

    // Message provides additional status information
    Message string `json:"message,omitempty"`

    // LastReconcileTime is the last time the integration was reconciled
    LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

    // ObservedGeneration is the generation observed by the controller
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // ClusterStatuses shows status per cluster
    ClusterStatuses []ClusterStatus `json:"clusterStatuses,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Integration is the Schema for the integrations API
type Integration struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   IntegrationSpec   `json:"spec,omitempty"`
    Status IntegrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IntegrationList contains a list of Integration
type IntegrationList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Integration `json:"items"`
}

// IntegrationTargetSpec defines the desired state of IntegrationTarget
type IntegrationTargetSpec struct {
    // ClusterName is the name of the target cluster
    // +kubebuilder:validation:Required
    ClusterName string `json:"clusterName"`

    // Namespace is the target namespace
    Namespace string `json:"namespace,omitempty"`

    // Labels are cluster labels for selection
    Labels map[string]string `json:"labels,omitempty"`

    // KubeConfig is the kubeconfig for connecting to the cluster
    KubeConfig string `json:"kubeConfig,omitempty"`
}

// IntegrationTargetStatus defines the observed state of IntegrationTarget
type IntegrationTargetStatus struct {
    // Ready indicates if the target is ready
    Ready bool `json:"ready,omitempty"`

    // Message provides additional status information
    Message string `json:"message,omitempty"`

    // Conditions represent the latest available observations
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IntegrationTarget is the Schema for the integrationtargets API
type IntegrationTarget struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   IntegrationTargetSpec   `json:"spec,omitempty"`
    Status IntegrationTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IntegrationTargetList contains a list of IntegrationTarget
type IntegrationTargetList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []IntegrationTarget `json:"items"`
}

func init() {
    SchemeBuilder.Register(&Integration{}, &IntegrationList{})
    SchemeBuilder.Register(&IntegrationTarget{}, &IntegrationTargetList{})
}