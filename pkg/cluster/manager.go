package cluster

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterManager struct {
	client.Client
	mutex    sync.RWMutex
	clusters map[string]*Cluster
	configs  map[string]*rest.Config
}

type Cluster struct {
	Name       string
	Namespace  string
	Status     string
	KubeConfig string
	Client     kubernetes.Interface
	Labels     map[string]string
}

type ClusterStatus string

const (
	ClusterStatusActive       ClusterStatus = "Active"
	ClusterStatusInactive     ClusterStatus = "Inactive"
	ClusterStatusConnecting   ClusterStatus = "Connecting"
	ClusterStatusDisconnected ClusterStatus = "Disconnected"
	ClusterStatusError        ClusterStatus = "Error"
)

func NewClusterManager(c client.Client) *ClusterManager {
	return &ClusterManager{
		Client:   c,
		clusters: make(map[string]*Cluster),
		configs:  make(map[string]*rest.Config),
	}
}

func (cm *ClusterManager) AddCluster(name, namespace string, kubeConfig string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	cm.clusters[key] = &Cluster{
		Name:       name,
		Namespace:  namespace,
		Status:     string(ClusterStatusActive),
		KubeConfig: kubeConfig,
		Client:     kubeClient,
		Labels:     make(map[string]string),
	}
	cm.configs[key] = config

	return nil
}

func (cm *ClusterManager) RemoveCluster(name, namespace string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	delete(cm.clusters, key)
	delete(cm.configs, key)

	return nil
}

func (cm *ClusterManager) GetCluster(name, namespace string) (*Cluster, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	cluster, exists := cm.clusters[key]
	if !exists {
		return nil, fmt.Errorf("cluster %s/%s not found", namespace, name)
	}

	return cluster, nil
}

func (cm *ClusterManager) ListClusters() []*Cluster {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	clusters := make([]*Cluster, 0, len(cm.clusters))
	for _, cluster := range cm.clusters {
		clusters = append(clusters, cluster)
	}

	return clusters
}

func (cm *ClusterManager) UpdateClusterStatus(name, namespace, status string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	cluster, exists := cm.clusters[key]
	if !exists {
		return fmt.Errorf("cluster %s/%s not found", namespace, name)
	}

	cluster.Status = status
	return nil
}

func (cm *ClusterManager) GetClusterClient(name, namespace string) (kubernetes.Interface, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	cluster, exists := cm.clusters[key]
	if !exists {
		return nil, fmt.Errorf("cluster %s/%s not found", namespace, name)
	}

	return cluster.Client, nil
}

func (cm *ClusterManager) GetClusterConfig(name, namespace string) (*rest.Config, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	config, exists := cm.configs[key]
	if !exists {
		return nil, fmt.Errorf("cluster config %s/%s not found", namespace, name)
	}

	return config, nil
}

func (cm *ClusterManager) SyncCluster(ctx context.Context, name, namespace string) error {
	cluster, err := cm.GetCluster(name, namespace)
	if err != nil {
		return err
	}

	_, err = cluster.Client.Discovery().ServerVersion()
	if err != nil {
		cm.UpdateClusterStatus(name, namespace, string(ClusterStatusError))
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	cm.UpdateClusterStatus(name, namespace, string(ClusterStatusActive))
	return nil
}

func (cm *ClusterManager) HealthCheck(ctx context.Context) map[string]bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	health := make(map[string]bool)

	for key, cluster := range cm.clusters {
		_, err := cluster.Client.Discovery().ServerVersion()
		health[key] = err == nil
	}

	return health
}

func (cm *ClusterManager) SetClusterLabels(name, namespace string, labels map[string]string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	cluster, exists := cm.clusters[key]
	if !exists {
		return fmt.Errorf("cluster %s/%s not found", namespace, name)
	}

	cluster.Labels = labels
	return nil
}

func (cm *ClusterManager) GetClustersByLabel(key, value string) []*Cluster {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var result []*Cluster
	for _, cluster := range cm.clusters {
		if cluster.Labels[key] == value {
			result = append(result, cluster)
		}
	}

	return result
}
