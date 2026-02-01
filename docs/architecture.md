# KSIT Architecture

This document explains how KSIT works under the hood. If you just want to use it, the getting-started guide is probably more useful. This is for people who want to understand the internals or contribute code.

## High-Level Overview

KSIT is a Kubernetes operator built with controller-runtime. It runs as a single deployment in your control cluster and uses the standard Kubernetes reconciliation loop pattern.

The architecture has three main layers:

1. **API Layer**: Custom Resource Definitions (CRDs) that define what to monitor
2. **Controller Layer**: Reconciliation logic that performs health checks
3. **Client Layer**: Code that talks to remote clusters and checks their resources

## Components

### Custom Resources

**IntegrationTarget**

- Represents a Kubernetes cluster you want to monitor
- Contains the cluster name and optional labels
- References a kubeconfig secret for authentication
- Status field shows if the cluster is reachable

**Integration**

- Defines which tool to monitor (argocd, flux, prometheus, istio)
- Lists target clusters to check
- Config map for tool-specific settings
- Status field aggregates health across all target clusters

### Controllers

**IntegrationTargetReconciler**

- Watches IntegrationTarget resources
- Reads the kubeconfig from the secret
- Registers the cluster with ClusterManager
- Tests connectivity and updates status
- Runs on every update and periodically (default: 30s)

**IntegrationReconciler**

- Watches Integration resources
- Gets cluster configs from ClusterManager
- Calls integration-specific health check logic
- Aggregates results across clusters
- Updates Integration status
- Runs on every update and periodically (default: 30s)

### ClusterManager

This is a shared in-memory cache that both reconcilers use:

```go
type ClusterManager struct {
    clusters map[string]*rest.Config  // namespace/name -> config
    mu       sync.RWMutex
}
```

When IntegrationTargetReconciler processes a target, it calls `ClusterManager.AddCluster()` to register the cluster config. Later, IntegrationReconciler calls `ClusterManager.GetCluster()` to retrieve it and perform health checks.

This design ensures both reconcilers see the same cluster configurations without duplicating kubeconfig parsing.

### Integration Clients

Each supported tool has its own health check implementation:

**ArgoCD Client** (`pkg/integrations/argocd/`)

- Checks for deployments: argocd-server, argocd-repo-server
- Verifies StatefulSet: argocd-application-controller
- Ensures services have endpoints
- Returns detailed status with component-level information

**Flux Client** (`pkg/integrations/flux/`)

- Looks for controllers in flux-system namespace
- Checks source-controller, kustomize-controller, helm-controller, notification-controller
- Counts how many are healthy vs total expected

**Prometheus Client** (`pkg/integrations/prometheus/`)

- Checks for prometheus-operator deployment
- Verifies prometheus StatefulSet
- Optionally checks grafana deployment
- Looks for alertmanager StatefulSet

**Istio Client** (`pkg/integrations/istio/`)

- Checks istiod deployment in istio-system
- Optionally verifies ingress gateway
- Can check for specific Istio CRDs

## Reconciliation Flow

Here's what happens when you create an Integration resource:

1. User creates Integration CR specifying type=argocd and targetClusters=["cluster-1", "cluster-2"]

2. IntegrationReconciler's Reconcile() method is triggered

3. Controller reads the Integration spec

4. For each target cluster, it:
   - Calls ClusterManager.GetCluster() to get the cluster config
   - Creates a Kubernetes client for that cluster
   - Calls the appropriate health check function (reconcileArgoCD)
   - Collects results

5. Health check function queries the remote cluster:
   - Lists deployments in argocd namespace
   - Checks replica counts and readiness
   - Verifies services and endpoints exist

6. Controller aggregates results:
   - If all checks pass on all clusters: Phase=Running, Message="Integration is running"
   - If any check fails: Phase=Failed, Message=detailed error

7. Controller updates Integration status

8. Reconciliation completes and will run again in 30 seconds

## Key Design Decisions

**Why not use dynamic client?**

We use typed clients (Clientset) because they provide type safety and better IDE support. The health checks are specific to known resource types, so there's no need for dynamic discovery.

**Why reconcile every 30 seconds?**

Kubernetes doesn't have a watch API for resources in remote clusters. We could use informers for each cluster, but that would be more complex and resource-intensive. Polling every 30 seconds is simple and adequate for health monitoring.

**Why store cluster configs in memory?**

Kubeconfig secrets can be large and parsing them is expensive. By caching the rest.Config objects, we avoid repeated parsing. The tradeoff is that config changes require controller restart, but this is acceptable since kubeconfigs rarely change.

**Why share ClusterManager between reconcilers?**

IntegrationTargetReconciler handles cluster registration. IntegrationReconciler uses those clusters. Sharing the manager ensures they see the same state without complex synchronization.

## Code Structure

```
pkg/
├── controller/
│   └── reconciler.go        # Both reconcilers in one file
├── cluster/
│   ├── manager.go           # ClusterManager implementation
│   └── inventory.go         # Cluster inventory tracking
└── integrations/
    ├── argocd/
    │   └── client.go        # ArgoCD health checks
    ├── flux/
    │   └── client.go        # Flux health checks
    ├── prometheus/
    │   └── client.go        # Prometheus health checks
    └── istio/
        └── client.go        # Istio health checks
```

The reconciler.go file contains:

- IntegrationReconciler struct and methods
- IntegrationTargetReconciler struct and methods
- reconcileArgoCD(), reconcileFlux(), reconcilePrometheus(), reconcileIstio() functions
- Helper functions for health checks

## Error Handling

The controller follows these principles:

1. **Transient errors** (network timeouts, API server temporarily unavailable) are logged but don't change the status to Failed. The next reconciliation might succeed.

2. **Permanent errors** (deployment doesn't exist, wrong namespace) immediately set status to Failed with a descriptive message.

3. **Partial failures** (2 out of 3 clusters healthy) show overall status as Failed but include per-cluster details in the status.

4. **Unknown states** (can't determine health) are treated as failures to be safe.

## Performance Considerations

**Network calls**: Each reconciliation makes API calls to each target cluster. With N integrations and M clusters, that's N*M calls every 30 seconds. For large deployments, consider increasing the reconciliation interval.

**Concurrent reconciliation**: The controller can reconcile multiple Integrations concurrently. The default is 1, but you can increase it with the --max-concurrent-reconciles flag.

**Resource usage**: The controller is lightweight, typically using <100MB memory and minimal CPU. Most of the work is waiting for API responses.

## Security Model

The controller needs:

- Read access to IntegrationTarget and Integration CRDs
- Read access to secrets (for kubeconfigs)
- Write access to update statuses

It does NOT need:

- Write access to remote clusters (read-only health checks)
- Cluster-admin permissions
- Access to application namespaces on remote clusters (only checks tool namespaces)

The kubeconfig secrets should use service accounts with minimal permissions on target clusters. For example, the ArgoCD health check only needs `get` and `list` permissions on deployments, services, and endpoints in the argocd namespace.

## Adding a New Integration Type

To add support for a new tool (e.g., Jenkins):

1. Add the integration type constant to api/v1alpha1/integration_types.go:

```go
const IntegrationTypeJenkins = "jenkins"
```

1. Create pkg/integrations/jenkins/client.go with health check logic

2. Add a case to IntegrationReconciler.Reconcile() in pkg/controller/reconciler.go:

```go
case ksitv1alpha1.IntegrationTypeJenkins:
    return r.reconcileJenkins(ctx, integration, config, cluster)
```

1. Implement reconcileJenkins() following the pattern of existing functions

2. Update CRD validation to accept the new type

3. Regenerate manifests: `make manifests`

## Testing

The test suite has three levels:

**Unit tests**: Test individual functions in isolation (controller logic, cluster manager, etc.)

**Integration tests**: Use envtest to run controllers against a fake API server

**E2E tests**: Run against real kind clusters

Run them with:

```bash
make test          # Unit tests only
make test-integration   # Integration tests
make test-e2e      # End-to-end tests
make test-all      # Everything
```

## Monitoring the Controller

The controller exposes metrics on :8080/metrics in Prometheus format:

- `controller_runtime_reconcile_total`: Number of reconciliations
- `controller_runtime_reconcile_errors_total`: Number of errors
- `controller_runtime_reconcile_time_seconds`: Time spent reconciling

You can scrape these with Prometheus and create dashboards showing:

- Reconciliation rate
- Error rate
- Health check latency
- Number of managed integrations/clusters

## Future Enhancements

Some ideas for improvement:

- Webhooks for validation (prevent creating Integration with unknown type)
- Support for integration-specific configuration (custom namespaces, etc.)
- Per-integration reconciliation intervals
- Events when health status changes
- Metrics for integration health (not just controller metrics)
- Support for more tools (Vault, Jenkins, Tekton, etc.)
