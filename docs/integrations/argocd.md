# File: /kubestellar-integration-toolkit/kubestellar-integration-toolkit/docs/integrations/argocd.md

# ArgoCD Integration

The KubeStellar Integration Toolkit provides seamless integration with ArgoCD for GitOps-based multi-cluster deployments.

## Overview

The ArgoCD integration enables:

- Automatic synchronization of applications across multiple clusters
- Health monitoring of ArgoCD applications
- Sync status tracking and reporting
- Multi-cluster application deployment coordination

## Configuration

Create an Integration resource for ArgoCD:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-integration
spec:
  type: argocd
  enabled: true
  targetClusters:
    - cluster1
    - cluster2
  config:
    serverURL: "https://argocd-server.argocd.svc.cluster.local"
    insecure: "false"
```

## Features

### Application Synchronization

The integration monitors ArgoCD applications and ensures they are synchronized across target clusters.

### Health Monitoring

Continuous health checks are performed on ArgoCD applications to detect and report issues.

### Multi-Cluster Support

Applications can be deployed to multiple clusters simultaneously with consistent configuration.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│  KSIT Controller│────▶│  ArgoCD Server  │
└─────────────────┘     └─────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐     ┌─────────────────┐
│    Cluster 1    │     │    Cluster 2    │
└─────────────────┘     └─────────────────┘
```

## Troubleshooting

### Common Issues

1. **Connection Refused**: Ensure ArgoCD server URL is correct and accessible
2. **Authentication Failed**: Verify the ArgoCD token is valid
3. **Sync Failed**: Check application manifests for errors
