# KubeStellar Integration Toolkit - Complete Deployment Guide

## Overview

This guide walks you through deploying KSIT with real multi-cluster GitOps workloads.

## Prerequisites

✅ Kind clusters: ksit-hub, cluster1, cluster2
✅ ArgoCD installed in argocd namespace
✅ Flux installed in flux-system namespace
✅ Prometheus/Grafana installed in monitoring namespace
✅ Istio installed in istio-system namespace

## Phase 1: Delete Old Guestbook Application

```bash
# Remove the example guestbook app
kubectl delete application guestbook -n argocd
```

## Phase 2: Apply KSIT Demo Applications

```bash
# Navigate to project
cd /Users/namansharma/Kubestellar-demo/kubestellar-integration-toolkit

# Apply Prometheus ServiceMonitor (so Prometheus scrapes KSIT metrics)
kubectl apply -f config/samples/prometheus_servicemonitor.yaml

# Apply Flux GitRepository (watches your GitHub repo)
kubectl apply -f config/samples/flux_gitrepository.yaml

# Apply ArgoCD Applications (deploys to cluster1 and cluster2)
kubectl apply -f config/samples/argocd_applications.yaml

# Apply KubeStellar BindingPolicy (propagates workloads across clusters)
kubectl apply -f config/samples/kubestellar_binding.yaml
```

## Phase 3: Apply KSIT Integrations

```bash
# Apply KSIT integrations
kubectl apply -f config/samples/argocd_integration.yaml
kubectl apply -f config/samples/flux_integration.yaml
kubectl apply -f config/samples/prometheus_integration.yaml
kubectl apply -f config/samples/istio_integration.yaml
```

## Phase 4: Build and Run KSIT Controller

```bash
# Build the controller
go build -o bin/ksit ./cmd/ksit/main.go

# Run the controller
./bin/ksit --metrics-bind-address=:9090 --health-probe-bind-address=:8081
```

## Phase 5: Verify Everything Works

### Check ArgoCD Applications

```bash
# Should see ksit-demo-cluster1 and ksit-demo-cluster2
kubectl get applications -n argocd

# Check details
kubectl describe application ksit-demo-cluster1 -n argocd
kubectl describe application ksit-demo-cluster2 -n argocd
```

### Check Flux GitRepository

```bash
# Should see ksit-demo-apps
kubectl get gitrepositories -n flux-system

# Check Kustomizations
kubectl get kustomizations -n flux-system
```

### Check Demo Deployments

```bash
# Should see cluster1-demo-app (3 replicas) and cluster2-demo-app (2 replicas)
kubectl get deployments -n demo

# Check pods
kubectl get pods -n demo

# Check services
kubectl get svc -n demo
```

### Check Istio Resources

```bash
# Check VirtualServices
kubectl get virtualservices -n demo

# Check DestinationRules
kubectl get destinationrules -n demo

# Check Istio injection
kubectl get namespace demo --show-labels
```

### Check KSIT Integrations

```bash
# Should show all integrations in Running phase
kubectl get integrations -A

# Check detailed status
kubectl describe integration argocd-integration -n argocd
kubectl describe integration flux-integration -n flux-system
kubectl describe integration prometheus-integration -n monitoring
kubectl describe integration istio-integration -n istio-system
```

### Check Prometheus Metrics

```bash
# KSIT metrics endpoint
curl http://localhost:9090/metrics | grep ksit

# Or port-forward Prometheus
kubectl port-forward svc/prometheus-kube-prometheus-prometheus -n monitoring 9090:9090 &

# Open http://localhost:9090 and search for:
# - ksit_integration_reconcile_total
# - ksit_integration_status
# - ksit_cluster_connection_status
```

### View in ArgoCD UI

```bash
# Port forward ArgoCD
kubectl port-forward svc/argocd-server -n argocd 8080:443 &

# Login credentials:
# Username: admin
# Password: HfBvrEjw-SuYSQDX

# Open https://localhost:8080
# You should see:
# - ksit-demo-cluster1 (Synced, Healthy)
# - ksit-demo-cluster2 (Synced, Healthy)
```

### View in Grafana

```bash
# Port forward Grafana
kubectl port-forward svc/prometheus-grafana -n monitoring 3000:80 &

# Login credentials:
# Username: admin
# Password: acaWwQjZk8qvA7UqNLwZdRBokNQr6PWfLQWcispB

# Open http://localhost:3000
# Create dashboard to visualize KSIT metrics
```

### View in Kiali (Istio Dashboard)

```bash
# Start Kiali dashboard
istioctl dashboard kiali

# You should see:
# - demo namespace with istio-injection enabled
# - Traffic flow between cluster1 and cluster2 subsets
# - 50/50 traffic split visualization
```

## What Each Component Does

### ArgoCD Applications
- **ksit-demo-cluster1**: Deploys 3 replicas with cluster1 label, region: us-east
- **ksit-demo-cluster2**: Deploys 2 replicas with cluster2 label, region: us-west
- Both sync from your GitHub repo automatically

### Flux GitRepository
- Watches `https://github.com/namansh70747/kubestellar-integration-toolkit`
- Syncs `examples/multi-cluster-workloads/` directory
- Creates Kustomizations for deployment

### Prometheus ServiceMonitor
- Scrapes KSIT controller metrics from ksit-system namespace
- Exposes metrics on port 9090 at /metrics endpoint
- Tracks integration reconciliation, sync operations, cluster status

### KubeStellar BindingPolicy
- Propagates deployments, services, and configmaps from demo namespace
- Targets clusters with label `location-group: edge`
- Enables multi-cluster workload distribution

### Istio VirtualService & DestinationRule
- Routes traffic 50/50 between cluster1 and cluster2 subsets
- Enables header-based routing (cluster: cluster1 or cluster: cluster2)
- Enforces mTLS between services

### KSIT Integrations
- **ArgoCD Integration**: Monitors and can trigger syncs for all ArgoCD apps
- **Flux Integration**: Watches GitRepositories and Kustomizations
- **Prometheus Integration**: Validates connection and exports custom metrics
- **Istio Integration**: Configures mesh with mTLS enabled

## Architecture

```
GitHub Repo (this project)
         │
         ├──────────────────┬─────────────────────┐
         ▼                  ▼                     ▼
     ArgoCD              Flux            Prometheus/Istio
     (cluster1/2)        (demo)          (monitoring/mesh)
         │                  │                     │
         └──────────────────┼─────────────────────┘
                            ▼
                   KSIT Controller
                   (reconciles all)
                            │
                            ▼
                   KubeStellar
                   (multi-cluster)
                            │
              ┌─────────────┴─────────────┐
              ▼                           ▼
         Cluster 1                   Cluster 2
         3 replicas                  2 replicas
         us-east                     us-west
```

## Troubleshooting

### ArgoCD Applications Not Syncing
```bash
# Check ArgoCD server logs
kubectl logs -n argocd deployment/argocd-server --tail=50

# Check ArgoCD application controller
kubectl logs -n argocd deployment/argocd-application-controller --tail=50

# Manually trigger sync
argocd app sync ksit-demo-cluster1
argocd app sync ksit-demo-cluster2
```

### Flux Not Syncing
```bash
# Check Flux logs
kubectl logs -n flux-system deployment/source-controller --tail=50
kubectl logs -n flux-system deployment/kustomize-controller --tail=50

# Reconcile manually
flux reconcile source git ksit-demo-apps
flux reconcile kustomization ksit-demo-workloads
```

### KSIT Controller Not Starting
```bash
# Check for build errors
go build -o bin/ksit ./cmd/ksit/main.go

# Check CRDs are installed
kubectl get crd integrations.ksit.io
kubectl get crd integrationtargets.ksit.io

# Check logs when running
./bin/ksit --metrics-bind-address=:9090 --health-probe-bind-address=:8081
```

### Prometheus Not Scraping KSIT
```bash
# Check ServiceMonitor
kubectl get servicemonitor -n monitoring ksit-controller-metrics

# Check Prometheus targets
kubectl port-forward svc/prometheus-kube-prometheus-prometheus -n monitoring 9090:9090 &
# Open http://localhost:9090/targets and look for ksit
```

## Success Criteria

✅ ArgoCD UI shows 2 applications (cluster1, cluster2) - both Synced & Healthy
✅ `kubectl get pods -n demo` shows 5 total pods (3 from cluster1, 2 from cluster2)
✅ `kubectl get integrations -A` shows all 4 integrations in Running phase
✅ Prometheus shows KSIT metrics (search for `ksit_`)
✅ Kiali shows traffic flow in demo namespace with istio-injection
✅ Grafana displays KSIT dashboards with real data

## Next Steps

1. **Create Custom Grafana Dashboard**: Visualize KSIT metrics
2. **Add More Clusters**: Scale to cluster3, cluster4
3. **Implement CI/CD**: Automate deployments via GitHub Actions
4. **Add Alerts**: Configure Prometheus alerts for integration failures
5. **Extend KSIT**: Add more integration types (Jenkins, Tekton, etc.)
