# Getting Started with KSIT

This guide shows you how to install KSIT and start monitoring your clusters. You'll go from zero to having live health monitoring in about 15 minutes.

## What You Need

Before you start, install these on your machine:

- **kubectl** - For interacting with Kubernetes
- **Helm 3** - For installing KSIT
- **Docker** - For building the controller image
- **kind** (optional) - If you want to try it locally first

On macOS, install everything with Homebrew:

```bash
brew install kubectl helm kind
```

## Option 1: Quick Demo (Recommended for First Time)

Want to see KSIT working without setting up real clusters? Use our automated demo:

```bash
# Clone the repository
git clone https://github.com/namansh70747/kubestellar-integration-toolkit.git
cd kubestellar-integration-toolkit

# One command to set everything up
make quickstart
```

This creates three kind clusters, installs KSIT, deploys ArgoCD/Flux/Prometheus/Istio, and configures monitoring. Wait about 10 minutes for everything to install.

Check the results:

```bash
kubectl get integrations -n ksit-system
kubectl get integrationtargets -n ksit-system
```

You should see integrations showing "Running" status and targets showing "Ready".

## Option 2: Install on Your Own Clusters

If you already have Kubernetes clusters, here's how to install KSIT:

### Step 1: Build the Controller Image

```bash
cd kubestellar-integration-toolkit
docker build -t ksit-controller:v12 .
```

If your control cluster is kind, load the image:

```bash
kind load docker-image ksit-controller:v12 --name your-control-cluster
```

### Step 2: Install with Helm

```bash
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.repository=ksit-controller \
  --set image.tag=v12 \
  --set image.pullPolicy=IfNotPresent
```

Verify the controller is running:

```bash
kubectl get pods -n ksit-system
```

You should see `ksit-controller-manager` in Running state.

### Step 3: Add Your Clusters

For each cluster you want to monitor, create a kubeconfig secret and IntegrationTarget:

```bash
# Create kubeconfig secret
kubectl create secret generic prod-cluster-kubeconfig \
  --from-file=kubeconfig=/path/to/prod-kubeconfig.yaml \
  -n ksit-system

# Create IntegrationTarget
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: prod-cluster
  namespace: ksit-system
spec:
  clusterName: prod-cluster
  labels:
    environment: production
EOF
```

Check if the cluster connected:

```bash
kubectl get integrationtarget prod-cluster -n ksit-system
```

It should show `READY: true` after a few seconds.

### Step 4: Monitor Your Tools

Create Integration resources for the tools you want to monitor:

```bash
# Monitor ArgoCD
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-prod
  namespace: ksit-system
spec:
  type: argocd
  enabled: true
  targetClusters:
    - prod-cluster
  config:
    namespace: argocd
    healthCheckInterval: "30s"
EOF

# Monitor Flux
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: flux-prod
  namespace: ksit-system
spec:
  type: flux
  enabled: true
  targetClusters:
    - prod-cluster
  config:
    namespace: flux-system
    healthCheckInterval: "30s"
EOF
```

Check status:

```bash
kubectl get integrations -n ksit-system
```

## What Just Happened?

Let me explain what's running now:

**Control Cluster**: Runs the KSIT controller. This is where you installed Helm. The controller watches for Integration and IntegrationTarget resources.

**Your Workload Clusters**: These are the clusters you're monitoring. KSIT connects to them using the kubeconfig secrets you created. It checks if ArgoCD, Flux, Prometheus, or Istio are healthy.

**Health Checks**: Every 30 seconds, the controller connects to each target cluster and checks:

- Are the expected deployments running?
- Do they have healthy replicas?
- Are the services responding?

Results appear in the Integration status field, which you can see with `kubectl get integrations`.

## Try It Out

Now that everything is set up, experiment with it:

### See What the Controller Is Doing

Watch the controller logs in real-time:

```bash
kubectl logs -f -n ksit-system -l control-plane=controller-manager
```

You'll see messages like:

```
INFO  controllers.Integration  checking ArgoCD health on cluster  {"cluster": "prod-cluster"}
INFO  controllers.Integration  ArgoCD integration is healthy
```

### Break Something on Purpose

Let's see how KSIT detects failures. If you're using the demo setup:

```bash
# Scale down ArgoCD on cluster-1
kubectl scale deployment argocd-server --replicas=0 -n argocd --context kind-cluster-1

# Watch the integration status change
kubectl get integration -n ksit-system -w
```

Within 30 seconds, the ArgoCD integration will show Phase: Failed with a message explaining what's wrong.

Fix it:

```bash
kubectl scale deployment argocd-server --replicas=1 -n argocd --context kind-cluster-1
```

The status returns to Running automatically.

### Monitor Multiple Clusters

Add another cluster to an existing integration:

```bash
# Create kubeconfig secret for new cluster
kubectl create secret generic staging-kubeconfig \
  --from-file=kubeconfig=/path/to/staging.kubeconfig \
  -n ksit-system

# Create IntegrationTarget
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: staging-cluster
  namespace: ksit-system
spec:
  clusterName: staging-cluster
EOF

# Update integration to include new cluster
kubectl patch integration argocd-prod -n ksit-system --type=merge -p '
spec:
  targetClusters:
    - prod-cluster
    - staging-cluster
'
```

Now one Integration resource monitors ArgoCD across both clusters.

## Common Questions

**Q: How do I know if my cluster connected successfully?**

Check the IntegrationTarget status:

```bash
kubectl get integrationtarget <cluster-name> -n ksit-system
```

If it shows `READY: true`, you're good. If not, check the Message field for details.

**Q: Why does my integration show Failed when the tools are running?**

KSIT expects tools in specific namespaces:

- ArgoCD: `argocd`
- Flux: `flux-system`
- Prometheus: `monitoring`
- Istio: `istio-system`

If you installed in a different namespace, KSIT won't find it. Custom namespace support is coming soon.

**Q: Can I use this with cloud clusters like GKE or EKS?**

Absolutely. KSIT works with any Kubernetes cluster. Just create a kubeconfig secret pointing to your cloud cluster and add an IntegrationTarget for it.

**Q: Does KSIT need to install anything on my workload clusters?**

No. KSIT only reads data from your clusters. It doesn't deploy anything or modify resources. It just checks if things exist and are healthy.

**Q: What if a cluster goes offline?**

KSIT marks it as Failed and keeps trying every 30 seconds. When the cluster comes back online, the status automatically returns to Running.

## Cleaning Up

Done experimenting? Remove everything:

```bash
# If using the demo setup
make cleanup

# If using Helm on your own clusters
helm uninstall ksit -n ksit-system
kubectl delete crd integrations.ksit.io integrationtargets.ksit.io
```

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
