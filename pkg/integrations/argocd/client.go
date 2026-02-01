package argocd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client represents an ArgoCD client
type Client struct {
	client.Client
	serverURL  string
	authToken  string
	httpClient *http.Client
	namespace  string
	secretName string
	secretKey  string
}

// NewClient creates a new ArgoCD client with secret-based token support
func NewClient(c client.Client, config map[string]string) (*Client, error) {
	serverURL := config["serverURL"]
	if serverURL == "" {
		return nil, fmt.Errorf("serverURL is required")
	}

	namespace := config["namespace"]
	if namespace == "" {
		namespace = "argocd"
	}

	insecure := config["insecure"] == "true"

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}

	client := &Client{
		Client:     c,
		serverURL:  serverURL,
		httpClient: httpClient,
		namespace:  namespace,
		secretName: config["secretName"],
		secretKey:  config["secretKey"],
		authToken:  config["token"],
	}

	return client, nil
}

// GetToken retrieves the auth token, either from config or from a Kubernetes Secret
func (c *Client) GetToken(ctx context.Context) (string, error) {
	// If token is directly provided, use it
	if c.authToken != "" {
		return c.authToken, nil
	}

	// If secretName is provided, fetch token from secret
	if c.secretName != "" {
		secret := &corev1.Secret{}
		err := c.Get(ctx, types.NamespacedName{
			Name:      c.secretName,
			Namespace: c.namespace,
		}, secret)
		if err != nil {
			return "", fmt.Errorf("failed to get secret %s: %w", c.secretName, err)
		}

		key := c.secretKey
		if key == "" {
			key = "token"
		}

		token, ok := secret.Data[key]
		if !ok {
			return "", fmt.Errorf("key %s not found in secret %s", key, c.secretName)
		}

		return string(token), nil
	}

	return "", fmt.Errorf("no auth token configured")
}

// Application represents an ArgoCD Application
type Application struct {
	Metadata ApplicationMetadata `json:"metadata"`
	Spec     ApplicationSpec     `json:"spec"`
	Status   ApplicationStatus   `json:"status,omitempty"`
}

// ApplicationMetadata contains application metadata
type ApplicationMetadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ApplicationSpec defines the application spec
type ApplicationSpec struct {
	Source      ApplicationSource      `json:"source"`
	Destination ApplicationDestination `json:"destination"`
	Project     string                 `json:"project"`
	SyncPolicy  *SyncPolicy            `json:"syncPolicy,omitempty"`
}

// ApplicationSource defines the source
type ApplicationSource struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path,omitempty"`
	TargetRevision string `json:"targetRevision"`
}

// ApplicationDestination defines the destination
type ApplicationDestination struct {
	Server    string `json:"server,omitempty"`
	Namespace string `json:"namespace"`
	Name      string `json:"name,omitempty"`
}

// SyncPolicy defines sync policy
type SyncPolicy struct {
	Automated   *AutomatedSyncPolicy `json:"automated,omitempty"`
	SyncOptions []string             `json:"syncOptions,omitempty"`
}

// AutomatedSyncPolicy defines automated sync options
type AutomatedSyncPolicy struct {
	Prune    bool `json:"prune,omitempty"`
	SelfHeal bool `json:"selfHeal,omitempty"`
}

// ApplicationStatus represents application status
type ApplicationStatus struct {
	Health HealthStatus `json:"health,omitempty"`
	Sync   SyncStatus   `json:"sync,omitempty"`
}

// HealthStatus represents health status
type HealthStatus struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// SyncStatus represents sync status
type SyncStatus struct {
	Status   string `json:"status,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// GetApplication retrieves an application
func (c *Client) GetApplication(ctx context.Context, namespace, name string) (*Application, error) {
	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s", c.serverURL, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get application: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get application, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var app Application
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, err
	}

	return &app, nil
}

// ListApplications lists all applications
func (c *Client) ListApplications(ctx context.Context) ([]Application, error) {
	token, err := c.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/applications", c.serverURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list applications, status: %d", resp.StatusCode)
	}

	var result struct {
		Items []Application `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// SyncApplication syncs an application
func (c *Client) SyncApplication(ctx context.Context, name string) error {
	token, err := c.GetToken(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/applications/%s/sync", c.serverURL, name)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader("{}"))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync application: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to sync application, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// HealthCheck checks ArgoCD health
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", c.serverURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// SyncCluster syncs all applications for a given cluster
func (c *Client) SyncCluster(ctx context.Context, clusterName string) error {
	apps, err := c.ListApplications(ctx)
	if err != nil {
		return err
	}

	for _, app := range apps {
		if err := c.SyncApplication(ctx, app.Metadata.Name); err != nil {
			return fmt.Errorf("failed to sync app %s: %w", app.Metadata.Name, err)
		}
	}

	return nil
}

// ReconcileCluster reconciles ArgoCD for a cluster
func (c *Client) ReconcileCluster(ctx context.Context, clusterName string) error {
	// Health check first
	if err := c.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Sync all applications for this cluster
	return c.SyncCluster(ctx, clusterName)
}
