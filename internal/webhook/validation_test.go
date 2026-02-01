package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ksitv1alpha1 "github.com/kubestellar/integration-toolkit/api/v1alpha1"
)

func TestValidateIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = ksitv1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	validator := NewIntegrationValidator(client)

	integration := &ksitv1alpha1.Integration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd",
			Namespace: "default",
		},
		Spec: ksitv1alpha1.IntegrationSpec{
			Type:           ksitv1alpha1.IntegrationTypeArgoCD,
			TargetClusters: []string{"cluster1"},
			Config: map[string]string{
				"serverURL": "https://argocd.example.com",
				"namespace": "argocd",
			},
		},
	}

	errors := validator.validateIntegration(integration)
	assert.Empty(t, errors)
}
