# KubeStellar Integration Toolkit (KSIT)

KSIT monitors DevOps tools running across multiple Kubernetes clusters from a single control plane. Instead of manually checking if ArgoCD, Flux, Prometheus, or Istio are healthy on each cluster, KSIT does it automatically and reports back through standard Kubernetes resources.

## The Problem

Imagine you manage 10 Kubernetes clusters. Each cluster has ArgoCD for deployments, Prometheus for monitoring, and maybe Istio for service mesh. How do you know if they're all working?

Typically, you'd:
- Switch kubectl context 10 times
- Check pods in each namespace on each cluster
- Look for errors in logs
- Hope nothing broke while you weren't looking

This gets old fast.

## What KSIT Does

KSIT runs in one cluster (your control plane) and connects to all your other clusters. Every 30 seconds, it checks if the tools you care about are healthy:

- **ArgoCD**: Are the server, repo-server, and application controller running?
- **Flux**: Are all four main controllers operational?
- **Prometheus**: Is the operator and prometheus pods running?
- **Istio**: Is istiod responding?

You get a simple status for each tool on each cluster. One `kubectl get integrations` shows you everything.

## Why This Matters

**Single pane of glass**: Check all clusters from one place. No context switching.

**Catch problems early**: Know within 30 seconds when something breaks, before users complain.

**Kubernetes-native**: Uses standard CRDs and kubectl commands. No new tools to learn.

**Declarative**: Define what to monitor in YAML, just like everything else in Kubernetes.

**Lightweight**: Just API calls to check resources. No agents on workload clusters.

## How to Use It

KSIT monitors clusters through two main resources:

### 1. IntegrationTarget - Define Your Clusters

Tell KSIT about each cluster you want to monitor:

```yaml
apiVersion: ksit.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: production-east
  namespace: ksit-system
spec:
  clusterName: production-east
  labels:
    environment: production
    region: us-east
```

KSIT needs a kubeconfig to access each cluster:

```bash
kubectl create secret generic production-east-kubeconfig \
  --from-file=kubeconfig=/path/to/prod-east.kubeconfig \
  -n ksit-system
```

### 2. Integration - Define What to Monitor

Tell KSIT which tools to check on which clusters:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-all-prod
  namespace: ksit-system
spec:
  type: argocd
  enabled: true
  targetClusters:
    - production-east
    - production-west
  config:
    namespace: argocd
    healthCheckInterval: "30s"
```

### 3. Check Status

See everything at a glance:

```bash
kubectl get integrations -n ksit-system
```

Output:
```
NAME              TYPE     PHASE     AGE
argocd-all-prod   argocd   Running   5m
flux-staging      flux     Failed    5m
prometheus-all    prometheus Running 5m
```

Get details on failures:

```bash
kubectl describe integration flux-staging -n ksit-system
```

## Installation

### Prerequisites

- **kubectl** - You'll use this to interact with KSIT
- **Helm 3** - For installing KSIT (recommended)
- **Docker** - If building from source
- **kind** - For local testing (optional)

### Quick Start with Helm

The fastest way to install KSIT:

```bash
# 1. Clone the repository
git clone https://github.com/namansh70747/kubestellar-integration-toolkit.git
cd kubestellar-integration-toolkit

# 2. Build the controller image
docker build -t ksit-controller:v12 .

# 3. Load image into your cluster (if using kind)
kind load docker-image ksit-controller:v12 --name your-control-cluster

# 4. Install via Helm
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.repository=ksit-controller \
  --set image.tag=v12

# 5. Verify it's running
kubectl get pods -n ksit-system
```

That's the entire installation. No complex configuration required.

### Quick Demo with Sample Clusters

Want to see it working immediately? Use the automated demo setup:

```bash
# Creates 3 kind clusters, installs KSIT, and sets up sample integrations
make quickstart

# Check status
kubectl get integrations -n ksit-system
kubectl get integrationtargets -n ksit-system
```

The demo creates:
- Control cluster running KSIT
- Two workload clusters with ArgoCD, Flux, Prometheus, and Istio
- Sample Integration resources monitoring everything

## Real-World Examples

### Example 1: Monitor ArgoCD on All Production Clusters

You have 5 production clusters and want to know if ArgoCD is healthy on all of them:

```bash
# Create one Integration for all clusters
cat <<EOF | kubectl apply -f -
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-production
  namespace: ksit-system
spec:
  type: argocd
  enabled: true
  targetClusters:
    - prod-us-east
    - prod-us-west
    - prod-eu-west
    - prod-ap-south
    - prod-ap-northeast
  config:
    namespace: argocd
EOF
```

Now `kubectl get integration argocd-production` tells you the aggregated health across all 5 clusters.

### Example 2: Different Tools on Different Clusters

Some clusters run different tools:

```bash
# Staging clusters have Flux
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: flux-staging
  namespace: ksit-system
spec:
  type: flux
  targetClusters:
    - staging-1
    - staging-2
EOF

# Production clusters use ArgoCD
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-production
  namespace: ksit-system
spec:
  type: argocd
  targetClusters:
    - prod-1
    - prod-2
EOF
```

### Example 3: Alert on Failures

KSIT updates status conditions, which you can monitor:

```bash
# Watch for status changes
kubectl get integration argocd-production -w

# Or use a tool like kubewatch to send alerts when Phase changes to Failed
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

## Common Questions

**Q: Do I need to install anything on my workload clusters?**

No. KSIT only needs read access to check if pods and deployments exist. Nothing runs on your workload clusters.

**Q: What if my tool is in a different namespace?**

Currently, KSIT expects standard namespaces (argocd, flux-system, monitoring, istio-system). Custom namespace support is planned.

**Q: Can this work with GKE, EKS, AKS?**

Yes. KSIT works with any Kubernetes cluster. Just provide a valid kubeconfig.

**Q: How much load does this add?**

Very little. Every 30 seconds, KSIT makes a few API calls per cluster to check pod status. It's like running `kubectl get pods` periodically.

**Q: What happens if a cluster is temporarily unreachable?**

KSIT marks it as Failed and retries on the next reconciliation (30 seconds later). Once the cluster comes back, status returns to Running.

## Troubleshooting

**Integration shows Failed but tools are running**:
- Check if tools are in expected namespaces
- Run `kubectl describe integration <name> -n ksit-system` for details

**IntegrationTarget shows not ready**:
- Verify kubeconfig secret exists: `kubectl get secret <cluster>-kubeconfig -n ksit-system`
- Check if cluster is reachable from the control plane

**Controller pod crashlooping**:
- Ensure CRDs are installed: `kubectl get crd | grep ksit`
- Check logs: `kubectl logs -n ksit-system -l control-plane=controller-manager`

See [detailed troubleshooting guide](docs/troubleshooting.md) for more solutions.

## Updating and Uninstalling

### Upgrade KSIT

```bash
helm upgrade ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --set image.tag=v13
```

### Uninstall

```bash
# Remove KSIT
helm uninstall ksit -n ksit-system

# Optionally remove CRDs
kubectl delete crd integrations.ksit.io integrationtargets.ksit.io
```

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
