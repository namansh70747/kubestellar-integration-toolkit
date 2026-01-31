# Multi-Cluster ArgoCD Integration Example

This example demonstrates how to use KSIT with ArgoCD for multi-cluster GitOps deployments.

## Prerequisites

- ArgoCD installed in the hub cluster (`argocd` namespace)
- KSIT controller running (`ksit-system` namespace)
- At least 2 target clusters with labels `environment=production` and `integration=argocd`

## Files

- **`integration.yaml`** - KSIT Integration resource connecting to ArgoCD
- **`bindingpolicy.yaml`** - KubeStellar BindingPolicy for distributing ArgoCD resources
- **`applicationset.yaml`** - ArgoCD ApplicationSet for multi-cluster app deployment
- **`workload.yaml`** - Alternative ApplicationSet example

## Quick Start

### 1. Apply the Integration

```bash
kubectl apply -f integration.yaml
```

Verify the integration:
```bash
kubectl get integration argocd-multi-cluster -n argocd
kubectl describe integration argocd-multi-cluster -n argocd
```

### 2. Apply the BindingPolicy

```bash
kubectl apply -f bindingpolicy.yaml
```

Check binding status:
```bash
kubectl get bindingpolicy argocd-multi-cluster-binding -n argocd
```

### 3. Deploy ApplicationSet

```bash
kubectl apply -f applicationset.yaml
```

Monitor the applications:
```bash
# List all applications
kubectl get applications -n argocd

# Check sync status
argocd app list

# View specific application
argocd app get cluster1-myapp
```

### 4. Verify Multi-Cluster Deployment

```bash
# Check applications across clusters
for cluster in cluster1 cluster2; do
  echo "=== Applications in $cluster ==="
  kubectl get applications -n argocd -l cluster=$cluster
done
```

## Configuration Details

### Integration Configuration

The Integration resource connects KSIT to your ArgoCD instance:

- **serverURL**: ArgoCD server endpoint
- **namespace**: ArgoCD installation namespace
- **insecure**: Set to "false" for TLS verification

### BindingPolicy

Distributes the following ArgoCD resources to target clusters:
- Applications
- ApplicationSets
- AppProjects

Target clusters must have labels:
- `environment: production`
- `integration: argocd`

### ApplicationSet

Deploys applications to multiple clusters using the **list generator**:

- Generates one Application per cluster
- Supports cluster-specific paths: `deployments/{{cluster}}`
- Enables automated sync with self-healing

## Customization

### Add More Clusters

Edit `applicationset.yaml` and add cluster entries:

```yaml
spec:
  generators:
    - list:
        elements:
          - cluster: cluster3
            url: https://cluster3.example.com
            name: cluster3
```

### Change Repository

Update the `repoURL` in `applicationset.yaml`:

```yaml
source:
  repoURL: "https://github.com/your-org/your-repo.git"
```

### Adjust Sync Policy

Modify sync options in `applicationset.yaml`:

```yaml
syncPolicy:
  automated:
    prune: true          # Delete resources not in Git
    selfHeal: true       # Force sync when drift detected
    allowEmpty: false    # Prevent empty applications
```

## Troubleshooting

### Integration Not Ready

```bash
# Check integration status
kubectl describe integration argocd-multi-cluster -n argocd

# View controller logs
kubectl logs -n ksit-system -l control-plane=controller-manager -f
```

### Applications Not Syncing

```bash
# Check application status
argocd app get <app-name>

# View application events
kubectl describe application <app-name> -n argocd

# Force sync
argocd app sync <app-name>
```

### BindingPolicy Issues

```bash
# Verify binding policy
kubectl get bindingpolicy -n argocd

# Check cluster labels
kubectl get clusters --show-labels
```

## Cleanup

```bash
# Delete resources in order
kubectl delete -f applicationset.yaml
kubectl delete -f bindingpolicy.yaml
kubectl delete -f integration.yaml

# Clean up applications
kubectl delete applications -n argocd --all
```

## Next Steps

- Explore [Flux Integration](../multi-cluster-flux/)
- Set up [Multi-Cluster Mesh](../multi-cluster-mesh/)
- Configure [Observability](../multi-cluster-observability/)

## References

- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [ApplicationSet Documentation](https://argo-cd.readthedocs.io/en/stable/user-guide/application-set/)
- [KSIT Documentation](../../docs/)