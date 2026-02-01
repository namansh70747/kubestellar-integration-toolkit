package cluster

import (
    "context"
    "fmt"
    "sync"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

type ClusterInventory struct {
    mu       sync.RWMutex
    clusters map[string]*ClusterInfo
}

type ClusterInfo struct {
    Name         string
    Namespace    string
    Status       string
    Version      string
    NodeCount    int
    LastSeen     time.Time
    Labels       map[string]string
    Capabilities []string
}

func NewClusterInventory() *ClusterInventory {
    return &ClusterInventory{
        clusters: make(map[string]*ClusterInfo),
    }
}

func (ci *ClusterInventory) AddCluster(name, namespace, status string) {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    ci.clusters[name] = &ClusterInfo{
        Name:         name,
        Namespace:    namespace,
        Status:       status,
        LastSeen:     time.Now(),
        Labels:       make(map[string]string),
        Capabilities: []string{},
    }
}

func (ci *ClusterInventory) UpdateCluster(info *ClusterInfo) {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    info.LastSeen = time.Now()
    ci.clusters[info.Name] = info
}

func (ci *ClusterInventory) GetCluster(name string) (*ClusterInfo, error) {
    ci.mu.RLock()
    defer ci.mu.RUnlock()

    cluster, exists := ci.clusters[name]
    if !exists {
        return nil, fmt.Errorf("cluster %s not found", name)
    }

    return cluster, nil
}

func (ci *ClusterInventory) RemoveCluster(name string) {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    delete(ci.clusters, name)
}

func (ci *ClusterInventory) ListClusters() []*ClusterInfo {
    ci.mu.RLock()
    defer ci.mu.RUnlock()

    clusters := make([]*ClusterInfo, 0, len(ci.clusters))
    for _, cluster := range ci.clusters {
        clusters = append(clusters, cluster)
    }

    return clusters
}

func (ci *ClusterInventory) GetClustersByStatus(status string) []*ClusterInfo {
    ci.mu.RLock()
    defer ci.mu.RUnlock()

    var result []*ClusterInfo
    for _, cluster := range ci.clusters {
        if cluster.Status == status {
            result = append(result, cluster)
        }
    }

    return result
}

func (ci *ClusterInventory) GetClustersByLabel(key, value string) []*ClusterInfo {
    ci.mu.RLock()
    defer ci.mu.RUnlock()

    var result []*ClusterInfo
    for _, cluster := range ci.clusters {
        if cluster.Labels[key] == value {
            result = append(result, cluster)
        }
    }

    return result
}

func (ci *ClusterInventory) LoadClusters(kubeconfig string) error {
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        return fmt.Errorf("failed to build config: %w", err)
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return fmt.Errorf("failed to create clientset: %w", err)
    }

    ctx := context.Background()
    nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("failed to list nodes: %w", err)
    }

    version, err := clientset.Discovery().ServerVersion()
    if err != nil {
        return fmt.Errorf("failed to get server version: %w", err)
    }

    ci.mu.Lock()
    defer ci.mu.Unlock()

    clusterName := "default"
    ci.clusters[clusterName] = &ClusterInfo{
        Name:         clusterName,
        Namespace:    "default",
        Status:       string(ClusterStatusActive),
        Version:      version.String(),
        NodeCount:    len(nodes.Items),
        LastSeen:     time.Now(),
        Labels:       make(map[string]string),
        Capabilities: []string{},
    }

    return nil
}

func (ci *ClusterInventory) RefreshCluster(ctx context.Context, name string, client kubernetes.Interface) error {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    cluster, exists := ci.clusters[name]
    if !exists {
        return fmt.Errorf("cluster %s not found", name)
    }

    version, err := client.Discovery().ServerVersion()
    if err != nil {
        cluster.Status = string(ClusterStatusError)
        return fmt.Errorf("failed to get server version: %w", err)
    }

    nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        cluster.Status = string(ClusterStatusError)
        return fmt.Errorf("failed to list nodes: %w", err)
    }

    cluster.Version = version.String()
    cluster.NodeCount = len(nodes.Items)
    cluster.Status = string(ClusterStatusActive)
    cluster.LastSeen = time.Now()

    return nil
}

func (ci *ClusterInventory) CleanupStale(maxAge time.Duration) {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    cutoff := time.Now().Add(-maxAge)
    for name, cluster := range ci.clusters {
        if cluster.LastSeen.Before(cutoff) {
            delete(ci.clusters, name)
        }
    }
}

func (ci *ClusterInventory) Count() int {
    ci.mu.RLock()
    defer ci.mu.RUnlock()

    return len(ci.clusters)
}

func (ci *ClusterInventory) SetClusterLabels(name string, labels map[string]string) error {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    cluster, exists := ci.clusters[name]
    if !exists {
        return fmt.Errorf("cluster %s not found", name)
    }

    cluster.Labels = labels
    return nil
}

func (ci *ClusterInventory) AddClusterCapability(name, capability string) error {
    ci.mu.Lock()
    defer ci.mu.Unlock()

    cluster, exists := ci.clusters[name]
    if !exists {
        return fmt.Errorf("cluster %s not found", name)
    }

    // Check if capability already exists
    for _, cap := range cluster.Capabilities {
        if cap == capability {
            return nil
        }
    }

    cluster.Capabilities = append(cluster.Capabilities, capability)
    return nil
}