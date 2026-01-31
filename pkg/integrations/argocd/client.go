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

    argoClient := &Client{
        Client:     c,
        serverURL:  strings.TrimSuffix(serverURL, "/"),
        httpClient: httpClient,
        namespace:  namespace,
        secretName: config["secretName"],
        secretKey:  config["secretKey"],
        authToken:  config["token"], // Direct token if provided
    }

    return argoClient, nil
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
            return "", fmt.Errorf("failed to get ArgoCD token secret: %w", err)
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

    return "", fmt.Errorf("no ArgoCD token configured - set either 'token' or 'secretName' in config")
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
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to get application: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return nil, fmt.Errorf("application %s not found", name)
    }

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get application (status %d): %s", resp.StatusCode, string(body))
    }

    var app Application
    if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
        return nil, fmt.Errorf("failed to decode application: %w", err)
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
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to list applications: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to list applications (status %d): %s", resp.StatusCode, string(body))
    }

    var result struct {
        Items []Application `json:"items"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
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

    syncRequest := map[string]interface{}{
        "prune": true,
    }
    body, _ := json.Marshal(syncRequest)

    req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to sync application: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to sync application (status %d): %s", resp.StatusCode, string(body))
    }

    return nil
}

// HealthCheck checks ArgoCD health
func (c *Client) HealthCheck(ctx context.Context) error {
    token, err := c.GetToken(ctx)
    if err != nil {
        return fmt.Errorf("failed to get token: %w", err)
    }

    url := fmt.Sprintf("%s/api/v1/applications", c.serverURL)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("ArgoCD health check failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return fmt.Errorf("ArgoCD authentication failed - check token")
    }

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("ArgoCD health check failed with status: %d", resp.StatusCode)
    }

    return nil
}

// SyncCluster syncs all applications for a given cluster
func (c *Client) SyncCluster(ctx context.Context, clusterName string) error {
    apps, err := c.ListApplications(ctx)
    if err != nil {
        return fmt.Errorf("failed to list applications: %w", err)
    }

    for _, app := range apps {
        targetServer := app.Spec.Destination.Server
        targetName := app.Spec.Destination.Name

        isTargetCluster := false
        if clusterName == "in-cluster" && targetServer == "https://kubernetes.default.svc" {
            isTargetCluster = true
        } else if targetName == clusterName || targetServer == clusterName {
            isTargetCluster = true
        }

        if isTargetCluster {
            if err := c.SyncApplication(ctx, app.Metadata.Name); err != nil {
                // Log but continue with other apps
                continue
            }
        }
    }

    return nil
}

// ReconcileCluster reconciles ArgoCD for a cluster
func (c *Client) ReconcileCluster(ctx context.Context, clusterName string) error {
    return c.SyncCluster(ctx, clusterName)
}