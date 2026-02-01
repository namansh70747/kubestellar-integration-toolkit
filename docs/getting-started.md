# Getting Started with KSIT

This guide walks you through setting up KSIT from scratch. By the end, you'll have a working multi-cluster setup with health monitoring for ArgoCD, Flux, Prometheus, and Istio.

## What You'll Need

Before starting, make sure you have these tools installed:

- Docker (version 20.10 or later)
- kubectl
- kind (for creating local clusters)
- Go 1.21+ (if building from source)

Optional but recommended:

- helm (for Prometheus installation)
- flux CLI (for Flux installation)
- istioctl (for Istio installation)

You can install these on macOS with Homebrew:

```bash
brew install kind kubectl helm fluxcd/tap/flux istioctl
```

## Option 1: Automated Setup (Recommended)

The easiest way to get started is using the automated setup:

```bash
git clone https://github.com/kubestellar/integration-toolkit.git
cd integration-toolkit
make quickstart
```

This single command will:

1. Create three kind clusters
2. Build the KSIT controller
3. Deploy it to the control cluster
4. Install ArgoCD, Flux, Prometheus, and Istio
5. Configure monitoring for all tools

Wait about 5-10 minutes for everything to install, then check the status:

```bash
kubectl get integrations -n ksit-system
```

You should see four integrations all showing "Running" status.

## Option 2: Step-by-Step Setup

If you prefer understanding each step:

### 1. Create the Clusters

```bash
make setup-clusters
```

This creates:

- `ksit-control`: Control plane where the KSIT controller runs
- `cluster-1`: Workload cluster for ArgoCD, Flux, Prometheus
- `cluster-2`: Workload cluster for ArgoCD, Prometheus, Istio

The script automatically configures kubeconfig secrets so the controller can access the workload clusters.

### 2. Build and Deploy the Controller

```bash
make build-controller
make deploy-local
```

The first command builds a Docker image. The second loads it into the kind cluster and deploys the controller.

Verify it's running:

```bash
kubectl get pods -n ksit-system
```

You should see the `ksit-controller-manager` pod in Running state.

### 3. Install DevOps Tools

```bash
make install-integrations
```

This installs:

- ArgoCD on both clusters
- Flux on cluster-1
- Prometheus on both clusters
- Istio on cluster-2

The installation takes a few minutes. You can watch progress:

```bash
kubectl get pods -n argocd --context kind-cluster-1
kubectl get pods -n flux-system --context kind-cluster-1
```

### 4. Create Integration Resources

Tell KSIT what to monitor:

```bash
kubectl apply -f config/samples/
```

This creates:

- IntegrationTarget resources for cluster-1 and cluster-2
- Integration resources for argocd, flux, prometheus, and istio

### 5. Verify Everything Works

Check integration status:

```bash
kubectl get integrations -n ksit-system
```

All four should show Phase: Running. If any show Failed, check the controller logs:

```bash
kubectl logs deployment/ksit-controller-manager -n ksit-system
```

## What Just Happened?

Let me break down what's now running:

1. **Control Cluster (ksit-control)**: Runs the KSIT controller which monitors everything

2. **Cluster-1**: Has ArgoCD, Flux, and Prometheus installed. The controller connects to this cluster and checks if these tools are healthy.

3. **Cluster-2**: Has ArgoCD, Prometheus, and Istio installed. Same health monitoring applies here.

The controller reconciles every 30 seconds, checking deployments, pods, and services for each tool.

## Next Steps

Now that everything is running, try these exercises:

### Exercise 1: Add a New Cluster

Create a new kind cluster and add it to KSIT:

```bash
kind create cluster --name cluster-3
kind get kubeconfig --name cluster-3 > /tmp/cluster-3-kubeconfig

kubectl create secret generic cluster-3-kubeconfig \
  --from-file=kubeconfig=/tmp/cluster-3-kubeconfig \
  -n ksit-system

kubectl apply -f - <<EOF
apiVersion: ksit.kubestellar.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: cluster-3
  namespace: ksit-system
spec:
  clusterName: cluster-3
EOF
```

### Exercise 2: Break Something on Purpose

Delete ArgoCD from cluster-1:

```bash
kubectl delete namespace argocd --context kind-cluster-1
```

Watch the integration status change to Failed:

```bash
kubectl get integration argocd-multi-cluster -n ksit-system -w
```

The controller detects the issue within 30 seconds.

Reinstall ArgoCD:

```bash
kubectl create namespace argocd --context kind-cluster-1
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml --context kind-cluster-1
```

Watch the status change back to Running.

### Exercise 3: View Real-Time Logs

See the controller performing health checks:

```bash
kubectl logs -f deployment/ksit-controller-manager -n ksit-system | grep "healthy"
```

You'll see messages like:

```
INFO    controllers.Integration ArgoCD integration is healthy  {"cluster": "cluster-1"}
INFO    controllers.Integration Flux integration is healthy    {"cluster": "cluster-1"}
```

## Understanding the CRDs

KSIT uses two custom resources:

**IntegrationTarget**: Defines a cluster to monitor

- Contains the cluster name
- References a kubeconfig secret for access
- Can have labels for grouping

**Integration**: Defines what to monitor

- Specifies the tool type (argocd, flux, prometheus, istio)
- Lists which clusters to check
- Reports aggregated health status

View the CRD definitions:

```bash
kubectl get crd integrations.ksit.kubestellar.io -o yaml
kubectl get crd integrationtargets.ksit.kubestellar.io -o yaml
```

## Cleanup

When you're done experimenting:

```bash
make cleanup
```

This deletes all three kind clusters and removes all resources.

## Troubleshooting

**Problem**: Quickstart fails with "kind not found"

Install kind first: `brew install kind`

**Problem**: Controller pod is crashlooping

Check if CRDs are installed:

```bash
kubectl get crd | grep ksit
```

If missing, apply them:

```bash
kubectl apply -f config/crd/bases/
```

**Problem**: Integration stuck in "Pending"

Check if the IntegrationTarget exists and is Ready:

```bash
kubectl get integrationtargets -n ksit-system
```

If the target shows `ready: false`, check the kubeconfig secret and controller logs.

**Problem**: "Failed" status but pods are running

The controller might be looking in the wrong namespace. Check the Integration spec to ensure it matches where the tool is actually installed.

## Common Questions

**Q: Can I use this with real cloud clusters instead of kind?**

Yes. Just create kubeconfig secrets pointing to your real clusters and create IntegrationTarget resources for them. The controller doesn't care if it's kind, GKE, EKS, or AKS.

**Q: Does KSIT install the tools for me?**

No. KSIT only monitors tools that are already installed. You need to install ArgoCD, Flux, Prometheus, and Istio yourself (or use our `make install-integrations` helper for development).

**Q: Can I add monitoring for other tools besides the four supported ones?**

Currently no, but the code is extensible. You would need to modify the controller to add new integration types and implement their health check logic.

**Q: How much overhead does this add?**

Very little. The controller makes lightweight API calls to check resources. It's comparable to running `kubectl get deployments` periodically.

## What's Next?

Check out these guides:

- [Architecture](architecture.md) - Learn how the controller works internally
- [ArgoCD Integration](integrations/argocd.md) - Details on ArgoCD health checks
- [Flux Integration](integrations/flux.md) - Details on Flux health checks
- [Troubleshooting](troubleshooting.md) - Solutions to common issues
