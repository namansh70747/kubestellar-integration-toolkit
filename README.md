# KubeStellar Integration Toolkit

A Kubernetes operator that monitors the health of DevOps tools across multiple clusters. Think of it as a centralized dashboard that continuously checks if your ArgoCD, Flux, Prometheus, and Istio installations are running correctly across all your clusters.

## What Problem Does This Solve?

When managing multiple Kubernetes clusters, you typically install tools like ArgoCD for GitOps, Prometheus for monitoring, and Istio for service mesh. The problem? You have no easy way to know if these tools are healthy across all clusters without manually checking each one.

KSIT solves this by providing:

- A single API to check the status of all your DevOps tools across all clusters
- Automatic health monitoring every 30 seconds
- Kubernetes-native CRDs to declaratively define what should be monitored
- Centralized visibility without needing to switch contexts between clusters

## How It Works

KSIT runs as a controller in your control plane cluster. You tell it about your workload clusters (cluster-1, cluster-2, etc.) and which integrations to monitor (ArgoCD, Flux, Prometheus, Istio). It then:

1. Connects to each cluster using kubeconfig secrets
2. Checks if the specified tools are installed and healthy
3. Reports the aggregated status back to you
4. Continuously reconciles every 30 seconds

This means you can run a single command to see if ArgoCD is working on all 10 of your clusters, instead of checking each one manually.

## Architecture

The system has two main custom resources:

**IntegrationTarget**: Represents a cluster you want to monitor

```yaml
apiVersion: ksit.kubestellar.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: production-cluster
spec:
  clusterName: prod-us-east-1
```

**Integration**: Defines what tool to monitor on which clusters

```yaml
apiVersion: ksit.kubestellar.io/v1alpha1
kind: Integration
metadata:
  name: argocd-monitoring
spec:
  type: argocd
  targetClusters:
    - production-cluster
    - staging-cluster
```

The controller watches these resources and performs health checks on the target clusters.

## Prerequisites

You need the following installed on your machine:

- Go 1.21 or later
- Docker
- kubectl
- kind (for local testing)
- Optionally: helm, flux CLI, istioctl (if you want to install those tools)

## Quick Start

If you want to see KSIT in action quickly:

```bash
# 1. Clone the repository
git clone https://github.com/kubestellar/integration-toolkit.git
cd integration-toolkit

# 2. Run the complete setup (creates clusters, builds controller, installs tools)
make quickstart

# 3. Check integration status
kubectl get integrations -n ksit-system

# 4. See detailed status
kubectl describe integration argocd-multi-cluster -n ksit-system
```

That's it. The quickstart command will:

- Create 3 kind clusters (1 control plane, 2 workload clusters)
- Build and deploy the KSIT controller
- Install ArgoCD, Flux, Prometheus, and Istio on the workload clusters
- Create sample Integration resources

## Manual Setup

If you prefer to set things up step by step:

### Step 1: Create Clusters

```bash
make setup-clusters
```

This creates three kind clusters: `ksit-control`, `cluster-1`, and `cluster-2`.

### Step 2: Build and Deploy Controller

```bash
make build-controller
make deploy-local
```

This builds the Docker image and deploys it to your control cluster.

### Step 3: Install DevOps Tools

```bash
make install-integrations
```

This installs ArgoCD, Flux, Prometheus, and Istio on your workload clusters.

### Step 4: Create Integration Resources

```bash
kubectl apply -f config/samples/
```

This tells KSIT what to monitor.

## Configuration

### Adding a New Cluster

Create an IntegrationTarget and a kubeconfig secret:

```bash
# Create kubeconfig secret
kubectl create secret generic my-cluster-kubeconfig \
  --from-file=kubeconfig=/path/to/kubeconfig \
  -n ksit-system

# Create IntegrationTarget
kubectl apply -f - <<EOF
apiVersion: ksit.kubestellar.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: my-cluster
  namespace: ksit-system
spec:
  clusterName: my-cluster
EOF
```

### Monitoring a Tool on Specific Clusters

```bash
kubectl apply -f - <<EOF
apiVersion: ksit.kubestellar.io/v1alpha1
kind: Integration
metadata:
  name: argocd-prod
  namespace: ksit-system
spec:
  type: argocd
  targetClusters:
    - my-cluster
EOF
```

## Checking Health Status

View all integrations:

```bash
kubectl get integrations -n ksit-system
```

See detailed information:

```bash
kubectl describe integration argocd-prod -n ksit-system
```

Watch logs in real-time:

```bash
kubectl logs -f deployment/ksit-controller-manager -n ksit-system
```

## Supported Integrations

Currently supported:

- **ArgoCD**: Checks if argocd-server, argocd-repo-server, and argocd-application-controller are running
- **Flux**: Verifies source-controller, kustomize-controller, helm-controller, and notification-controller
- **Prometheus**: Monitors prometheus-operator, grafana, prometheus StatefulSet, and alertmanager
- **Istio**: Checks istiod and optionally istio-ingressgateway

## Development

Build from source:

```bash
make build
```

Run tests:

```bash
make test
```

Run controller locally (outside cluster):

```bash
make run
```

Generate CRDs after modifying API:

```bash
make manifests
```

## Cleanup

Remove everything:

```bash
make cleanup
```

This deletes all kind clusters and removes all resources.

## Project Structure

```
.
├── api/v1alpha1/              # CRD definitions
├── cmd/ksit/                  # Main entry point
├── pkg/
│   ├── controller/            # Reconciliation logic
│   ├── cluster/               # Cluster management
│   └── integrations/          # Integration-specific clients
├── config/
│   ├── crd/                   # Generated CRDs
│   ├── samples/               # Example resources
│   └── manager/               # Controller deployment
├── scripts/                   # Helper scripts
└── test/                      # Test suites
```

## How Health Checks Work

Each integration type has specific health check logic:

**ArgoCD**: Queries the argocd namespace for:

- Deployment: argocd-server (must have 1+ ready replicas)
- Deployment: argocd-repo-server (must have 1+ ready replicas)
- StatefulSet: argocd-application-controller (must have 1+ ready replicas)
- Service endpoints must exist

**Flux**: Checks flux-system namespace for running controllers

**Prometheus**: Looks for prometheus-operator deployment and prometheus StatefulSet

**Istio**: Verifies istiod deployment in istio-system namespace

If any component is missing or unhealthy, the Integration status changes to "Failed" with a descriptive message.

## Troubleshooting

**Problem**: Integration shows "Failed" but pods are running

Check if the namespace is correct. By default, KSIT expects:

- ArgoCD in `argocd` namespace
- Flux in `flux-system` namespace
- Prometheus in `monitoring` namespace
- Istio in `istio-system` namespace

**Problem**: "cluster not found" error

Make sure the IntegrationTarget exists and the kubeconfig secret is properly created with the correct cluster API server address.

**Problem**: Controller not starting

Check logs: `kubectl logs deployment/ksit-controller-manager -n ksit-system`

Common issues:

- CRDs not installed
- RBAC permissions missing
- Image not loaded into kind cluster

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

Make sure tests pass before submitting:

```bash
make test-all
```

## License

Apache License 2.0. See LICENSE file for details.

## Questions or Issues?

Open an issue on GitHub or reach out to the KubeStellar community.
