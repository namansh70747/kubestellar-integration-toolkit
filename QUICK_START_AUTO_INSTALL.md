# 🚀 Quick Start: Auto-Install Feature

## Overview

KSIT can now automatically install DevOps tools (ArgoCD, Flux, Prometheus, Istio) on your target Kubernetes clusters. Simply set `autoInstall.enabled: true` in your Integration resource!

## Prerequisites

- KSIT controller deployed (v13+)
- IntegrationTargets configured for your clusters
- Helm 3.x installed on controller node

## Usage

### 1. Install ArgoCD Automatically

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-autoinstall
  namespace: default
spec:
  type: argocd
  clusterSelector:
    matchLabels:
      environment: production
  autoInstall:
    enabled: true
    # Optional: specify version
    version: "5.51.6"
    # Optional: custom Helm values
    valuesConfig:
      server:
        replicas: 2
```

**Apply:**

```bash
kubectl apply -f config/samples/argocd_integration_autoinstall.yaml
```

**Verify:**

```bash
# Check Integration status
kubectl get integration argocd-autoinstall

# Verify Helm release
helm list -n argocd --kube-context <target-cluster>

# Check pods
kubectl get pods -n argocd --context <target-cluster>
```

### 2. Install Prometheus Automatically

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: prometheus-autoinstall
spec:
  type: prometheus
  clusterSelector:
    matchLabels:
      monitoring: enabled
  autoInstall:
    enabled: true
```

**Apply:**

```bash
kubectl apply -f config/samples/prometheus_integration_autoinstall.yaml
```

### 3. Install Istio Automatically

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: istio-autoinstall
spec:
  type: istio
  clusterSelector:
    matchLabels:
      service-mesh: required
  autoInstall:
    enabled: true
```

**Apply:**

```bash
kubectl apply -f config/samples/istio_integration_autoinstall.yaml
```

## Supported Tools

| Tool | Helm Chart | Default Version | Namespace |
|------|-----------|----------------|-----------|
| **ArgoCD** | argo/argo-cd | v5.51.6 | argocd |
| **Prometheus** | prometheus-community/kube-prometheus-stack | latest | monitoring |
| **Istio** | istio/istiod | latest | istio-system |
| **Flux** | (manifest-based) | v2.x | flux-system |

## How It Works

1. **Create Integration** with `autoInstall.enabled: true`
2. **Controller detects** the configuration
3. **Installer executes**:
   - Gets cluster kubeconfig
   - Adds Helm repository
   - Installs/upgrades chart
4. **Status updates** to Running/Failed
5. **Health checks** continue automatically

## Monitoring Progress

### Watch Controller Logs

```bash
kubectl logs -n ksit-system -l control-plane=controller-manager -f
```

**Expected output:**

```
INFO controllers.Integration auto-install enabled, checking installation status
INFO controllers.Integration installing integration {"type": "argocd", "cluster": "cluster-1"}
INFO controllers.Integration auto-install completed successfully
```

### Check Integration Status

```bash
kubectl get integration <name> -o yaml
```

**Status indicators:**

- `phase: Installing` - Installation in progress
- `phase: Running` - Installation successful, tool is healthy
- `phase: Failed` - Installation failed (check `.status.message`)

## Custom Configuration

### Override Helm Chart Version

```yaml
spec:
  autoInstall:
    enabled: true
    version: "6.0.0"  # Specific chart version
```

### Custom Helm Values

```yaml
spec:
  autoInstall:
    enabled: true
    valuesConfig:
      server:
        replicas: 3
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
```

### Select Specific Clusters

```yaml
spec:
  clusterSelector:
    matchLabels:
      environment: production
      region: us-west
  autoInstall:
    enabled: true
```

## Troubleshooting

### Installation Fails

**Check Integration status:**

```bash
kubectl get integration <name> -o jsonpath='{.status.message}'
```

**Common issues:**

1. **Existing resources**: Clean up manually-installed tools first
2. **Cluster access**: Verify IntegrationTarget is configured
3. **Helm repo**: Check network connectivity to chart repositories

### Clean Up Existing Installation

**Before auto-install, remove existing installation:**

```bash
# Delete namespace
kubectl delete ns argocd --context <cluster>

# Clean cluster-scoped resources
kubectl delete clusterrole,clusterrolebinding -l app.kubernetes.io/name=argocd
kubectl delete crd -l app.kubernetes.io/part-of=argocd
```

### View Detailed Logs

```bash
# Full controller logs
kubectl logs -n ksit-system -l control-plane=controller-manager --tail=200

# Filter auto-install logs
kubectl logs -n ksit-system -l control-plane=controller-manager | grep auto-install
```

## Examples

### Production Setup: All Tools

```yaml
---
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-prod
spec:
  type: argocd
  clusterSelector:
    matchLabels:
      environment: production
  autoInstall:
    enabled: true
    version: "5.51.6"
---
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: prometheus-prod
spec:
  type: prometheus
  clusterSelector:
    matchLabels:
      environment: production
  autoInstall:
    enabled: true
---
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: istio-prod
spec:
  type: istio
  clusterSelector:
    matchLabels:
      environment: production
  autoInstall:
    enabled: true
```

**Deploy all:**

```bash
kubectl apply -f production-integrations.yaml
```

## Benefits

✅ **Zero manual installation** - No kubectl apply or helm install needed  
✅ **Consistent deployments** - Same configuration across clusters  
✅ **Automatic health checks** - Integration monitors tool health  
✅ **Easy upgrades** - Change version field to upgrade  
✅ **Multi-cluster** - Install on multiple clusters simultaneously  
✅ **GitOps ready** - Declare in Git, KSIT handles installation  

## Next Steps

1. ✅ Create IntegrationTargets for your clusters
2. ✅ Apply Integration with `autoInstall.enabled: true`
3. ✅ Watch installation progress in controller logs
4. ✅ Verify Helm release and pods are running
5. ✅ Access your tools (ArgoCD UI, Grafana, etc.)

## Support

- **Documentation**: See [AUTO_INSTALL_SUCCESS.md](./AUTO_INSTALL_SUCCESS.md)
- **Examples**: Check [config/samples/](./config/samples/)
- **Issues**: Report on GitHub

---

**Happy Auto-Installing! 🚀**
