# File: /kubestellar-integration-toolkit/kubestellar-integration-toolkit/docs/integrations/istio.md

# Istio Integration

The KubeStellar Integration Toolkit provides Istio integration for multi-cluster service mesh management.

## Overview

The Istio integration enables:

- VirtualService management across clusters
- DestinationRule synchronization
- Multi-cluster traffic management
- Service mesh configuration distribution

## Configuration

Create an Integration resource for Istio:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: istio-integration
spec:
  type: istio
  enabled: true
  targetClusters:
    - cluster1
    - cluster2
  config:
    namespace: "istio-system"
    enableMTLS: "true"
```

## Features

### VirtualService Management

Create and manage VirtualServices across multiple clusters with consistent routing rules.

### DestinationRule Sync

Synchronize DestinationRules for consistent traffic policies.

### Multi-Cluster Mesh

Configure cross-cluster service mesh communication.

## Architecture

```
┌─────────────────┐
│  KSIT Controller│
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────┐
│Cluster│ │Cluster│
│   1   │ │   2   │
│┌─────┐│ │┌─────┐│
││Istio││◀─▶│Istio││
│└─────┘│ │└─────┘│
└───────┘ └───────┘
```
