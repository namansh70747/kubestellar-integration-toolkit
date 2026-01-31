# Multi-Cluster Flux Integration Example

This example shows how to use KSIT with Flux for GitOps-based multi-cluster deployments.

## Prerequisites

- Flux installed in the hub cluster (`flux-system` namespace)
- KSIT controller running
- Git repository with Kubernetes manifests
- Git credentials configured as secret `flux-git-credentials`

## Files

- **`integration.yaml`** - KSIT Integration for Flux
- **`bindingpolicy.yaml`** - BindingPolicy for Flux resources
- **`gitrepository.yaml`** - Flux GitRepository source
- **`flux-kustomization.yaml`** - Flux Kustomization for sync
- **`kustomization.yaml`** - Kustomize file to apply all resources

## Quick Start

### 1. Create Git Credentials Secret

```bash
# For HTTPS authentication
kubectl create secret generic flux-git-credentials \
  --namespace=flux-system \
  --from-literal=username=your-username \
  --from-literal=password=your-token

# Or for SSH authentication
kubectl create secret generic flux-git-credentials \
  --namespace=flux-system \
  --from-file=identity=./id_rsa \
  --from-file=known_hosts=./known_hosts
```

### 2. Apply All Resources

Using the kustomization file:

```bash
kubectl apply -k .
```

Or apply individually:

```bash
kubectl apply -f integration.yaml
kubectl apply -f gitrepository.yaml
kubectl apply -f flux-kustomization.yaml
kubectl apply -f bindingpolicy.yaml
```

### 3. Verify Deployment

```bash
# Check integration
kubectl get integration flux-multi-cluster -n flux-system

# Check GitRepository
flux get sources git -n flux-system

# Check Kustomization
flux get kustomizations -n flux-system

# View reconciliation status
flux reconcile source git ksit-multi-cluster-repo
```

### 4. Monitor Flux Operations

```bash
# Watch Flux logs
flux logs --follow

# Check specific kustomization
flux get kustomization ksit-multi-cluster-apps

# Trigger manual sync
flux reconcile kustomization ksit-multi-cluster-apps
```

## Configuration Details

### GitRepository

Points to your Git repository containing Kubernetes manifests:

- **url**: Git repository URL (HTTPS or SSH)
- **ref.branch**: Branch to track (default: `main`)
- **interval**: How often Flux checks for changes
- **secretRef**: Git credentials secret name

### Flux Kustomization

Defines how Flux applies manifests:

- **path**: Directory path in the repository (e.g., `./clusters`)
- **prune**: Delete resources removed from Git
- **validation**: Client-side validation before apply
- **healthChecks**: Resources to monitor for health

### BindingPolicy

Distributes Flux resources to target clusters:
- GitRepositories
- Kustomizations
- HelmReleases

## Repository Structure

Your Git repository should have this structure:

```
your-flux-config/
├── clusters/
│   ├── cluster1/
│   │   ├── kustomization.yaml
│   │   └── apps/
│   └── cluster2/
│       ├── kustomization.yaml
│       └── apps/
└── infrastructure/
    └── base/
```

## Customization

### Change Repository

Edit `gitrepository.yaml`:

```yaml
spec:
  url: https://github.com/your-org/your-repo
  ref:
    branch: main  # or: tag: v1.0.0
```

### Adjust Sync Interval

Edit `flux-kustomization.yaml`:

```yaml
spec:
  interval: 10m0s      # Check every 10 minutes
  retryInterval: 2m0s  # Retry failed sync after 2 minutes
```

### Add Path Filters

Edit `gitrepository.yaml` to exclude certain paths:

```yaml
spec:
  ignore: |
    # Exclude all
    /*
    # Include specific paths
    !/clusters/
    !/infrastructure/
    # Exclude specific files
    **/.git/
    **/README.md
```

## Troubleshooting

### GitRepository Not Ready

```bash
# Check GitRepository status
kubectl describe gitrepository ksit-multi-cluster-repo -n flux-system

# View Flux logs
flux logs --level=error

# Test Git connectivity
flux check
```

### Kustomization Failed

```bash
# Check kustomization status
flux get kustomization ksit-multi-cluster-apps

# View detailed error
kubectl describe kustomization ksit-multi-cluster-apps -n flux-system

# Force reconciliation
flux reconcile kustomization ksit-multi-cluster-apps --with-source
```

### Authentication Issues

```bash
# Verify secret exists
kubectl get secret flux-git-credentials -n flux-system

# Check secret data
kubectl get secret flux-git-credentials -n flux-system -o yaml

# Update secret
kubectl delete secret flux-git-credentials -n flux-system
kubectl create secret generic flux-git-credentials ...
```

### Sync Not Happening

```bash
# Check suspend status
flux get sources git

# Resume if suspended
flux resume source git ksit-multi-cluster-repo

# Check interval configuration
kubectl get gitrepository ksit-multi-cluster-repo -n flux-system -o yaml
```

## Health Checks

Monitor application health:

```bash
# Check all Flux resources
flux get all

# Check health of specific kustomization
flux get kustomization ksit-multi-cluster-apps

# View events
kubectl get events -n flux-system --sort-by='.lastTimestamp'
```

## Cleanup

```bash
# Delete using kustomize
kubectl delete -k .

# Or delete individually
kubectl delete -f flux-kustomization.yaml
kubectl delete -f gitrepository.yaml
kubectl delete -f bindingpolicy.yaml
kubectl delete -f integration.yaml

# Clean up secret
kubectl delete secret flux-git-credentials -n flux-system
```

## Best Practices

1. **Use Branch Protection**: Protect your main branch with pull request reviews
2. **Separate Environments**: Use different branches or paths for dev/staging/prod
3. **Encrypt Secrets**: Use Mozilla SOPS or Sealed Secrets for sensitive data
4. **Monitor Drift**: Enable automated sync to prevent configuration drift
5. **Version Tags**: Use Git tags for production deployments

## Next Steps

- Configure [Notification Controller](https://fluxcd.io/docs/components/notification/)
- Set up [Image Automation](https://fluxcd.io/docs/components/image/)
- Explore [Helm Releases](https://fluxcd.io/docs/components/helm/)

## References

- [Flux Documentation](https://fluxcd.io/docs/)
- [GitOps Toolkit](https://fluxcd.io/docs/components/)
- [KSIT Documentation](../../docs/)