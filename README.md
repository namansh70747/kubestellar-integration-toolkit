# KubeStellar Integration Toolkit (KSIT)

A Kubernetes controller that manages DevOps tools across multiple clusters from one place.

KSIT monitors and installs tools like ArgoCD, Prometheus, Istio, and Flux across your Kubernetes clusters. Instead of manually installing and checking these tools on each cluster, you define what you want in a YAML file and KSIT handles the rest.

## What Works

| Integration | Auto-Install | Health Checks | Status |
|-------------|-------------|---------------|--------|
| ArgoCD | Yes | Yes | Works well |
| Prometheus | Yes | Yes | Works well |
| Istio | Partial* | Yes | Needs registry access |
| Flux | Not yet | Yes | In progress |

*Istio works fine in GKE/EKS/AKS. Kind requires pre-loading images since it doesn't have internet access.

## Why This Exists

If you manage multiple Kubernetes clusters, you know the pain:

- Installing the same tools (ArgoCD, Prometheus, etc.) on every cluster manually
- Not knowing which cluster has a broken deployment until something fails
- Constantly switching contexts to check if everything is still running

KSIT fixes this. You declare what tools should run where, and it handles installation and monitoring.

```bash
kubectl get integrations  # See status across all clusters
```

The controller checks health every 30 seconds and updates the Integration status, so you always know what's running and what's broken.

## Key Features

**🚀 Auto-Install**: KSIT installs tools automatically using Helm charts. No manual `helm install` on each cluster.

**💚 Health Monitoring**: Every 30 seconds, KSIT checks if your tools are running correctly and reports status.

**🔄 Multi-Cluster**: One Integration resource can target multiple clusters simultaneously.

**🎯 Kubernetes-Native**: Uses CRDs, kubectl, and standard Kubernetes patterns. No proprietary tooling.

**📦 Helm Packaging**: Enterprise-ready installation with customizable values.

**🔐 Secure**: Non-root container, read-only filesystem, minimal RBAC permissions.

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured
- Helm 3.x installed
- Docker (for building image)

### Installation

**Step 1: Build the Controller Image**
Features

- Auto-installs tools via Helm charts
- Monitors health and reports status every 30 seconds
- One Integration can target multiple clusters
- Standard Kubernetes CRDs (no weird custom APIs)
- Runs as non-root with minimal permissions

# Minikube

minikube image load ksit-controller:v1.0.0

```

**Step 3: Install via Helm**

```bash
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.repository=ksit-controller \
  --set image.tag=v1.0.0
```

**Step 4: Verify Installation**

```bash
kubectl get pods -n ksit-system
# Expected output:
# NAME                                  READY   STATUS    RESTARTS   AGE
# ksit-controller-manager-xxxxx-xxxxx   1/1     Running   0          30s
```

### Your First Integration

**Create an IntegrationTarget** (your workload cluster):

```bash
# First, get the kubeconfig for your workload cluster
kubectl config view --flatten --minify > /tmp/cluster-1.kubeconfig

# Create the secret
kubectl create secret generic cluster-1-kubeconfig \
  --from-file=kubeconfig=/tmp/cluster-1.kubeconfig \
  -n ksit-system

# Create the target
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: IntegrationTarget
metadata:
  name: cluster-1
  namespace: ksit-system
spec:
  clusterName: cluster-1
EOF
```

**Install ArgoCD Automatically**:

```bash
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-autoinstall
  namespace: ksit-system
spec:
  type: argocd
  enabled: true
  targetClusters:
    - cluster-1
  
  autoInstall:
    enabled: true
    method: helm
    helmConfig:
      repository: https://argoproj.github.io/argo-helm
      chart: argo-cd
      version: "5.51.6"
  
  config:
    namespace: argocd
    healthCheckInterval: "30s"
EOF
```

**Check Status**:

```bash
kubectl get integration argocd-autoinstall -n ksit-system

# Expected output after 2-3 minutes:
# NAME                 TYPE     PHASE     AGE
# argocd-autoinstall   argocd   Running   3m
```

**View Detailed Status**:

```bash
kubectl describe integration argocd-autoinstall -n ksit-system
```

## Installation

### Prerequisites

- **Kubernetes**: v1.24+ (works with GKE, EKS, AKS, Kind, Minikube)
- **kubectl**: Configured and connected to your control cluster
- **Helm 3**: Version 3.x or higher
- **Docker**: For building the controller image

### Production Installation with Helm

**Step 1: Build Controller Image**

```bash
git clone https://github.com/namansh70747/kubestellar-integration-toolkit.git
cd kubestellar-integration-toolkit

# Build the production image
docker build -t ksit-controller:v1.0.0 .
```

**Step 2: Push to Registry (Production)**

```bash
# Tag for your registry
docker tag ksit-controller:v1.0.0 your-registry.io/ksit-controller:v1.0.0

# Push to registry
docker push your-registry.io/ksit-controller:v1.0.0
```

**Step 3: Install via Helm**

```bash
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.repository=your-registry.io/ksit-controller \
  --set image.tag=v1.0.0
```

**Step 4: Verify Installation**

```bash
kubectl get pods -n ksit-system
kubectl get crd | grep ksit

# Expected CRDs:
# integrations.ksit.io
# integrationtargets.ksit.io
```

### Local Development Installation (Kind/Minikube)

```bash
# Build image
docker build -t ksit-controller:v1.0.0 .

# Load into Kind
kind load docker-image ksit-controller:v1.0.0 --name <your-control-cluster>

# Or load into Minikube
minikube image load ksit-controller:v1.0.0

# Install with Helm
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.repository=ksit-controller \
  --set image.tag=v1.0.0 \
  --set image.pullPolicy=IfNotPresent
```

### Helm Configuration Options

```yaml
# values.yaml customization
image:
  repository: ksit-controller
  tag: v1.0.0
  pullPolicy: IfNotPresent

replicaCount: 1

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

# Additional Helm values
serviceAccount:
  create: true
  name: ksit-controller

rbac:
  create: true
```

Install with custom values:

```bash
helm install ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --values custom-values.yaml
```

### Quick Demo Setup

Want to see KSIT in action immediately?

```bash
# Automated 3-cluster Kind setup with sample integrations
make quickstart

# This creates:
# - ksit-control (control plane with KSIT)
# - cluster-1 (workload cluster)
# - cluster-2 (workload cluster)
# - Pre-configured IntegrationTargets
# - Sample ArgoCD and Prometheus Integrations

# Check status
kubectl get integrations -n ksit-system
kubectl get integrationtargets -n ksit-system
```

## Production Examples

### Example 1: ArgoCD on All Production Clusters ✅ PRODUCTION READY

```yaml
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
    - prod-eu-central
    - prod-ap-southeast
  
  autoInstall:
    enabled: true
    method: helm
    helmConfig:
      repository: https://argoproj.github.io/argo-helm
      chart: argo-cd
      version: "5.51.6"
  
  config:
    namespace: argocd
    healthCheckInterval: "30s"
```

**Result**: ArgoCD automatically installed and monitored on all 4 clusters. Single status view.

### Example 2: Prometheus Multi-Cluster Monitoring ✅ PRODUCTION READY

```yaml
apiVersion: ksit.io/v1alpha1
This installs ArgoCD on all 4 clusters and monitors their health from one place
metadata:
  name: prometheus-stack
  namespace: ksit-system
spec:
  type: prometheus
  enabled: true
  targetClusters:
    - cluster-1
    - cluster-2
    - cluster-3
  
  autoInstall:
    enabled: true
    method: helm
    helmConfig:
      repository: https://prometheus-community.github.io/helm-charts
      chart: kube-prometheus-stack
      version: "55.5.0"
  
  config:
    namespace: monitoring
```

This deploys the full Prometheus stack on all three clusters.

### Example 3: Istio Service Mesh ⚠️ CLOUD READY

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: istio-mesh
  namespace: ksit-system
spec:
  type: istio
  enabled: true
  targetClusters:
    - prod-cluster-1
    - prod-cluster-2
  
  autoInstall:
    enabled: true
    method: helm
    helmConfig:
      repository: https://istio-release.storage.googleapis.com/charts
      chart: istiod
      version: "1.20.2"
  
  config:
    namespace: istio-system
```

Works fine in cloud environments. Kind needs manual image loading since it can't pull from the internet.

### Example 4: Different Tools Per Environment

```yaml
# Staging uses Flux (monitoring only, installation under development)
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: flux-staging-monitor
  namespace: ksit-system
spec:
  type: flux
  enabled: true
  targetClusters:
    - staging-1
    - staging-2
  config:
    namespace: flux-system
    # Note: autoInstall not enabled - Flux installation is under active development
---
# Production uses ArgoCD (fully supported)
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-production
  namespace: ksit-system
spec:
  type: argocd
  enabled: true
  targetClusters:
    - prod-1
    - prod-2
  autoInstall:
    enabled: true
    method: helm
```

## Supported Integrations

### ✅ ArgoCD - **PRODUCTION READY v1.0.0**

ArgoCD

Works well. Auto-install and monitoring both functional.

- Installs via Helm
- Monitors 7 pods per cluster (server, repo-server, controller, redis, dex, notifications, applicationset)
- Tested across multiple clusters
**Helm Configuration**:

```yaml
helmConfig:
  repository: https://argoproj.github.io/argo-helm
  chart: argo-cd
  version: "5.51.6"
```

**Recommended For**: GitOps deployments, CD pipelines, application delivery

---

### Prometheus

Works well. Deploys the full kube-prometheus-stack.

- Installs via Helm
- Monitors 6 pods per cluster (prometheus, alertmanager, grafana, operator, kube-state-metrics, node-exporter)
- Takes 3-5 minutes for StatefulSets to come up

```yaml
helmConfig:
  repository: https://prometheus-community.github.io/helm-charts
  chart: kube-prometheus-stack
  version: "55.5.0"
```

**Recommended For**: Cluster monitoring, metrics collection, alerting

---

### ⚠️ Istio - **CLOUD PRODUCTION READY**

**Status**: Functional in cloud environments, Kind requires additional setup

- **Auto-Install**: ⚠️ Works with registry access

### Istio

Mostly works, with caveats.

- Auto-install works in cloud (GKE, EKS, AKS)
- Local Kind needs image pre-loading
- Monitors istiod control plane

**Known Limitation**: Local Kind clusters don't have internet access to pull images from `docker.io/istio/*`. This is **testing-only limitation**. In production cloud environments with registry access, Istio works perfectly.

**Kind Workaround**:

```bash
docker pull docker.io/istio/pilot:1.28.3
docker pull docker.io/istio/proxyv2:1.28.3
kind load docker-image docker.io/istio/pilot:1.28.3 --name <cluster-name>
kind load docker-image docker.io/istio/proxyv2:1.28.3 --name <cluster-name>
```

**Recommended For**: Service mesh, traffic management, security policies (cloud deployments)

---

Kind doesn't have internet access, so you need to pre-load Istio images:

ng works, auto-install requires additional engineering

- **Auto-Install**: ❌ CRD installation issue under investigation
- **Health Monitoring**: ✅ Working (all 6 controllers)
- **Multi-Cluster**: ✅ Supported
- **Current Issue**: CustomResourceDefinitions not being applied correctly during manifest-based installation
- **Pods Expected**: 6 controllers (source, kustomize, helm, notification, image-automation, image-reflector)

### Flux

Still working on this one.

- Monitoring works fine (checks 6 controllers)
- Auto-install has a CRD timing issue we're fixing
- For now, install Flux manually and use KSIT just for monitoring
flux install --namespace=flux-system

# Then create KSIT Integration without autoInstall.enabled

```

## Roadmap

### v1.0.0 (Current - Production Ready)

If you need Flux now, install it manual
- ✅ Flux monitoring (manual install required)
- ✅What's Next

Things we're planning to add:

- Fix Flux auto-install (CRD timing issue)
- Custom namespace support
- Configurable health check intervals
- More integrations (Tekton, Vault, Cert-Manager)
- Better webhook validation
- Maybe a UI at some point  # Scheme registration
├── cmd/ksit/
│   └── main.go                # Controller entry point
├── pkg/
│   ├── controller/            # Reconciliation logic
│   │   ├── reconciler.go      # Main reconcile loop
│   │   └── manager.go         # Controller manager setup
│   ├── cluster/               # Cluster management
│   │   ├── manager.go         # Multi-cluster client handling
│   │   └── inventory.go       # Cluster inventory tracking
│   ├── installer/             # Auto-install implementations
│   │   ├── argocd.go          # ArgoCD Helm installer ✅
│   │   ├── prometheus.go      # Prometheus Helm installer ✅
│   │   ├── istio.go           # Istio Helm installer ⚠️
│   │   └── flux.go            # Flux manifest installer ❌
│   └── integrations/          # Integration-specific health checks
│       ├── argocd/            # ArgoCD client ✅
│       ├── prometheus/        # Prometheus client ✅
│       ├── istio/             # Istio client ⚠️
│       └── flux/              # Flux client ✅
├── config/
│   ├── crd/bases/             # Generated CRD manifests
│   ├── samples/               # Example Integration resources
│   │   ├── argocd_integration_autoinstall.yaml
│   │   ├── prometheus_integration_autoinstall.yaml
│   │   ├── istio_integration.yaml
│   │   └── flux_integration.yaml
│   ├── rbac/                  # RBAC manifests
│   └── manager/               # Controller deployment
├── deploy/helm/ksit/          # Helm chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── scripts/                   # Helper scripts
│   ├── setup.sh               # Automated cluster setup
│   └── cleanup.sh             # Cleanup resources
├── test/
│   ├── e2e/                   # End-to-end tests
│   └── integration/           # Integration tests
├── docs/
│   ├── PRODUCTION_READINESS_REPORT.md  # Production validation
│   ├── COMPREHENSIVE_TEST_REPORT.md     # Test results
│   └── PRODUCTION_PACKAGE_SUMMARY.md    # Package summary
└── Makefile                   # Build and deployment targets
```

## How It Works Internally

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Control Plane Cluster                     │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │            KSIT Controller Manager                      │ │
│  │                                                          │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │ │
│  │  │ Integration  │  │   Cluster    │  │  Installer  │  │ │
│  │  │ Reconciler   │◄─┤   Manager    │  │   Manager   │  │ │
│  │  └──────────────┘  └──────────────┘  └─────────────┘  │ │
│  │         │                  │                  │         │ │
│  └─────────┼──────────────────┼──────────────────┼─────────┘ │
│            │                  │                  │           │
│            ▼                  ▼                  ▼           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │  Integration    │  │ Integration     │  │   Helm      │ │
│  │  CRD Resources  │  │ Target CRDs     │  │   Client    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
└───────────────────────┬─────────────────────────────────────┘
                        │ Kubeconfig Secrets
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│  Cluster 1    │ │  Cluster 2    │ │  Cluster N    │
│               │ │               │ │               │
│  ArgoCD ✅    │ │  Prometheus✅ │ │  Istio ⚠️     │
│  Prometheus✅ │ │  ArgoCD ✅    │ │  Flux ❌      │
└───────────────┘ └───────────────┘ └───────────────┘
```

### Reconciliation Flow

1. **Watch Integration Resources**: Controller watches for Integration CRD changes
2. **Load Cluster Clients**: For each `targetCluster`, load kubeconfig from secret
3. **Check Existing Installation**: Query workload cluster for tool presence
4. **Install if Needed**: If `autoInstall.enabled=true` and tool missing, install via Helm
5. **Health Check**: Query specific pods/deployments based on integration type
6. **Update Status**: Set Integration phase (Initializing/Running/Failed) and conditions
7. **Requeue**: Wait 30 seconds (or configured interval), repeat

### Health Check Logic

**ArgoCD**:

```go
// Checks 3 core components
deployments := []string{
    "argocd-server",           // Must have ≥1 ready replica
    "argocd-repo-server",      // Must have ≥1 ready replica
}
statefulsets := []string{
    "argocd-application-controller",  // Must have ≥1 ready replica
}
// Status: Running if all healthy, Failed otherwise
```

**Prometheus**:

```go
// Checks operator and prometheus StatefulSet
deployments := []string{
    "prometheus-kube-prometheus-operator",  // Must be ready
}
statefulsets := []string{
    "prometheus-kube-prometheus-prometheus", // Must have ≥1 ready replica
}
// Optional: grafana, alertmanager validation
```

**Istio**:

```go
// Checks control plane
deployments := []string{
    "istiod",  // Must have ≥1 ready replica
}
namespace := "istio-system"
```

**Flux**:

```go
// Checks all 6 controllers
deployments := []string{
    "source-controller",
    "kustomize-controller",
    "helm-controller",
    "notification-controller",
    "image-automation-controller",
    "image-reflector-controller",
}
// All must be running in flux-system namespace
```

## Troubleshooting

### ArgoCD Issues

**Problem**: Integration shows "Failed" but ArgoCD pods are running

```bash
# Check exact error
kubectl describe integration <name> -n ksit-system

# Common causes:
# - Pods still initializing (wait 2-3 minutes)
# - Wrong namespace (default: argocd)
# - Missing RBAC permissions on workload cluster
```

**Solution**: Verify ArgoCD is in the correct namespace:

```bash
kubectl get pods -n argocd --context <cluster-context>
```

### Prometheus Issues

**Problem**: Integration shows "Initializing" for a long time

```bash
# Prometheus StatefulSets take 3-5 minutes to become ready
kubectl get pods -n monitoring --context <cluster-context>

# Wait for StatefulSets:
# - alertmanager-* (0/2 -> 2/2 ready)
# - prometheus-* (0/2 -> 2/2 ready)
```

 This is normal. StatefulSets require persistent volumes and take time to initialize.

### Istio Issues

**Problem**: `ImagePullBackOff` in Kind clusters

```bash
# This is expected in Kind - no internet access
kubectl get pods -n istio-system --context <cluster-context>
# NAME                      READY   STATUS             RESTARTS   AGE
# istiod-xxx-xxx            0/1     ImagePullBackOff   0          2m
```

Check if ArgoCD is in the righ
Pre-load the images:
Integration stuck on "Initializing"

```bash
# Pre-load images
docker pull docker.io/istio/pilot:1.28.3
docker pull docker.io/istio/proxyv2:1.28.3
kind load docker-image docker.io/istio/pilot:1.28.3 --name <cluster-name>
kind load docker-image docker.io/istio/proxyv2:1.28.3 --name <cluster-name>

# Restart pods
This is normal. Prometheus uses StatefulSets which take a few minutes to start up
```

This only affects Kind. Cloud clusters work fine since they have internet access.
ImagePullBackOff in Kind

### Flux Issues

Controllers in CrashLoopBackOff

```bash
kubectl get pods -n flux-system --context <cluster-context>
# All 6 controllers showing CrashLoopBackOff
```

Auto-install doesn't work yet. Install Flux manually for now:

```bash
# Install Flux CLI
flux install --namespace=flux-system --context <cluster-context>

# Create KSIT Integration without autoInstall
kubectl apply -f - <<EOF
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: flux-monitor
  namespace: ksit-system
spec:
  type: flux
  enabled: true
  targetClusters:
    - your-cluster
  # Note: autoInstall not enabled
  config:
    namespace: flux-system
EOF
```

### IntegrationTarget Not Ready

**Problem**: Target shows "NotReady" status

Target stuck on NotReady
kubectl get integrationtargets -n ksit-system

# NAME        READY   AGE

# cluster-1   False   5m

```

**Solution**: Check kubeconfig secret:

Check the
# Verify secret exists
kubectl get secret cluster-1-kubeconfig -n ksit-system

# Check secret content
kubectl get secret cluster-1-kubeconfig -n ksit-system -o yaml

# Recreate if needed
kubectl delete secret cluster-1-kubeconfig -n ksit-system
kubectl create secret generic cluster-1-kubeconfig \
  --from-file=kubeconfig=/path/to/cluster-1.kubeconfig \
  -n ksit-system
```

### Controller Pod Issues

**Problem**: ControCrashing

Controller won't start
kubectl get pods -n ksit-system

# NAME                                  READY   STATUS             RESTARTS   AGE

# ksit-controller-manager-xxx-xxx       0/1     CrashLoopBackOff   5          5m

```

**Solution**: Check logs and CRDs:

Check logs and verify
# Check controller logs
kubectl logs -n ksit-system -l control-plane=controller-manager

# Verify CRDs are installed
kubectl get crd | grep ksit
# Should show: integrations.ksit.io, integrationtargets.ksit.io

# Reinstall CRDs if missing
kubectl apply -f config/crd/bases/
```

### Network Connectivity Issues

**Problem**:Issues

# Check controller logs

kubectl logs -n ksit-system -l control-plane=controller-manager | grep "connection refused"

```

**Solution**: Verify network connectivity and kubeconfig:

Test connectivity
# Test connectivity from control cluster
kubectl run debug --rm -it --image=nicolaka/netshoot -- /bin/bash
# Inside pod: curl -k https://<workload-cluster-api-server>

# Ensure kubeconfig has correct API server URL
kubectl get secret <cluster>-kubeconfig -n ksit-system -o jsonpath='{.data.kubeconfig}' | base64 -d
```

### Performance Issues

**PrHigh Resource Usage

Controller using too much CPU/memory
kubectl top pods -n ksit-system

```

**Solution**: Adjust resource limits and health check intervals:

Increase health check intervals or bump resource
# In Integration spec
config:
  healthCheckInterval: "60s"  # Increase from default 30s
```

```bash
# Update Helm values for more resources
helm upgrade ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=1Gi
```

### Common Questions

**Integration shows "Failed" right after creation**

Wait a minute or two. The first health check takes time.

**Does this work with GKE/EKS/AKS?**

Yes, any Kubernetes cluster works. Just need a valid kubeconfig.

**Do I install anything on the workload clusters?**

No, KSIT only needs read access to check pod status.

**What RBAC does it need on workload clusters?**

Read-only access to deployments, statefulsets, pods in the tool's namespace.

**Can I use custom namespaces?**

Not yet. Right now it expects standard namespaces (argocd, flux-system, monitoring, istio-system).

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/namansh70747/kubestellar-integration-toolkit.git
cd kubestellar-integration-toolkit

# Build binary
make build

# Build Docker image
docker build -t ksit-controller:dev .
```

### Running Tests

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run e2e tests (requires Kind)
make test-e2e

# All tests
make test-all
```

### Local Development

Run controller outside the cluster:

```bash
# Install CRDs
make install

# Run controller locally
make run

# Controller will use your current kubectl context
```

### Modifying CRDs

After changing API types in `api/v1alpha1/`:

```bash
# Regenerate CRD manifests and deepcopy
make manifests generate

# Apply updated CRDs
make install
```

### Adding New Integrations

1. Create installer in `pkg/installer/<tool>.go`:

```go
type MyToolInstaller struct {
    helmClient *helm.Client
}

func (m *MyToolInstaller) Install(ctx context.Context, cluster string) error {
    // Implementation
}
```

1. Create health check in `pkg/integrations/<tool>/client.go`:

```go
func (c *MyToolClient) CheckHealth(ctx context.Context) (bool, string, error) {
    // Check deployments/pods
}
```

1. Register in controller reconciler
2. Add sample in `config/samples/`
3. Add tests in `test/`

### Debugging

Enable verbose logging:

```bash
kubectl set env deployment/ksit-controller-manager \
  -n ksit-system \
  LOG_LEVEL=debug
```

View controller logs:

```bash
kubectl logs -n ksit-system -l control-plane=controller-manager -f
```

## Maintenance and Operations

### Upgrading KSIT

```bash
# Pull latest code
git pull origin main

# Rebuild image
docker build -t ksit-controller:v1.1.0 .

# Upgrade via Helm
helm upgrade ksit ./deploy/helm/ksit \
  --namespace ksit-system \
  --set image.tag=v1.1.0
```

### Backup and Restore

**Backup Integrations**:

```bash
kubectl get integrations -n ksit-system -o yaml > integrations-backup.yaml
kubectl get integrationtargets -n ksit-system -o yaml > targets-backup.yaml
kubectl get secrets -n ksit-system -o yaml > secrets-backup.yaml
```

**Restore**:

```bash
kubectl apply -f integrations-backup.yaml
kubectl apply -f targets-backup.yaml
kubectl apply -f secrets-backup.yaml
```

### Monitoring KSIT

Expose controller metrics:

```bash
kubectl port-forward -n ksit-system svc/ksit-controller-manager-metrics-service 8080:8443
curl http://localhost:8080/metrics
```

Key metrics:

- `controller_runtime_reconcile_total` - Total reconciliations
- `controller_runtime_reconcile_errors_total` - Failed reconciliations
- `controller_runtime_reconcile_time_seconds` - Reconciliation duration

### Cleanup

**Remove specific Integration**:

```bash
kubectl delete integration <name> -n ksit-system
# Tool remains installed on clusters, only monitoring stops
```

**Uninstall KSIT completely**:

```bash
helm uninstall ksit -n ksit-system

# Optionally remove CRDs (deletes all Integration resources!)
kubectl delete crd integrations.ksit.io integrationtargets.ksit.io
```

**Full demo cleanup**:

```bash
make cleanup  # Deletes all Kind clusters and resources
```

## Contributing

Pull requests welcome. Fork it, make changes, add tests, submit PR.

```bash
make test-all  # Make sure this passes
```

## License

Apache License 2.0. See LICENSE file for details.

## Questions or Issues?

Open an issue on GitHub or reach out to the KubeStellar community.
2.0

## Questions?

Open an issue on GitHub
