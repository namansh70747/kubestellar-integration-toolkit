# KubeStellar Integration Toolkit (KSIT)

[![Go Report Card](https://goreportcard.com/badge/github.com/kubestellar/integration-toolkit)](https://goreportcard.com/report/github.com/kubestellar/integration-toolkit)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/kubestellar/integration-toolkit)](https://github.com/kubestellar/integration-toolkit/releases)

The **KubeStellar Integration Toolkit (KSIT)** is a powerful framework designed to facilitate **integration patterns** between multi-cluster management platforms (like KubeStellar) and popular Kubernetes ecosystem tools including **ArgoCD**, **Flux**, **Prometheus**, and **Istio**.

## 🎯 **Key Features**

- **🔄 Multi-Cluster GitOps**: Seamless integration with ArgoCD and Flux
- **📊 Unified Observability**: Centralized monitoring with Prometheus and Grafana
- **🌐 Service Mesh Integration**: Advanced traffic management with Istio
- **🎛️ Declarative Configuration**: Kubernetes-native CRDs for managing integrations
- **🔐 Enterprise-Ready**: Built with security, scalability, and reliability in mind
- **🛠️ Extensible Architecture**: Plugin-based design for custom integrations

---

## 📋 **Table of Contents**

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Usage Examples](#usage-examples)
- [Configuration](#configuration)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## 🏗️ **Architecture**

KSIT follows a **controller-based architecture** leveraging the Kubernetes operator pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                    KubeStellar Control Plane                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              KSIT Controller Manager                  │   │
│  │  ┌────────────────┐  ┌────────────────────────────┐  │   │
│  │  │  Integration   │  │  IntegrationTarget         │  │   │
│  │  │  Reconciler    │  │  Reconciler                │  │   │
│  │  └────────┬───────┘  └────────────┬───────────────┘  │   │
│  └───────────┼──────────────────────┼────────────────────┘   │
│              │                      │                        │
└──────────────┼──────────────────────┼────────────────────────┘
               │                      │
       ┌───────┴──────┬───────────────┴────────┬────────────┐
       │              │                        │            │
   ┌───▼───┐     ┌───▼───┐              ┌─────▼──┐    ┌───▼────┐
   │ ArgoCD│     │ Flux  │              │Promethe│    │ Istio  │
   │       │     │       │              │us      │    │        │
   └───┬───┘     └───┬───┘              └────┬───┘    └───┬────┘
       │             │                       │            │
       └─────────────┴───────────┬───────────┴────────────┘
                                 │
                    ┌────────────▼──────────────┐
                    │    Managed Clusters       │
                    │  (cluster1, cluster2, ...) │
                    └───────────────────────────┘
```

**Core Components:**

1. **Custom Resource Definitions (CRDs)**
   - `Integration`: Defines integration configuration
   - `IntegrationTarget`: Specifies target clusters

2. **Controllers**
   - `IntegrationReconciler`: Manages integration lifecycle
   - `IntegrationTargetReconciler`: Handles cluster connections

3. **Integration Clients**
   - ArgoCD client for GitOps management
   - Flux client for continuous delivery
   - Prometheus client for metrics collection
   - Istio client for service mesh configuration

---

## 📦 **Prerequisites**

### **Required**

- **Go**: 1.21+ ([Download](https://go.dev/dl/))
- **Kubernetes Cluster**: 1.27+ ([Kind](https://kind.sigs.k8s.io/), [Minikube](https://minikube.sigs.k8s.io/), or cloud provider)
- **kubectl**: 1.27+ ([Install](https://kubernetes.io/docs/tasks/tools/))
- **Docker**: 20.10+ ([Install](https://docs.docker.com/get-docker/))

### **Optional (for specific integrations)**

- **ArgoCD**: 2.8+ ([Install](https://argo-cd.readthedocs.io/en/stable/getting_started/))
- **Flux**: 2.0+ ([Install](https://fluxcd.io/flux/installation/))
- **Prometheus Operator**: 0.68+ ([Install](https://github.com/prometheus-operator/prometheus-operator))
- **Istio**: 1.19+ ([Install](https://istio.io/latest/docs/setup/getting-started/))
- **KubeStellar**: 0.20+ ([Install](https://docs.kubestellar.io/))

---

## 🚀 **Quick Start**

### **1. Clone the Repository**

```bash
git clone https://github.com/kubestellar/integration-toolkit.git
cd integration-toolkit
```

### **2. Install Dependencies**

```bash
make tools          # Install required tools
make install-deps   # Install Go dependencies
```

### **3. Run Locally**

```bash
# Generate CRDs and code
make generate manifests

# Install CRDs
make install

# Run controller
make run
```

### **4. Deploy Sample Integration**

```bash
# Deploy ArgoCD integration example
kubectl apply -f examples/multi-cluster-argocd/integration.yaml
kubectl apply -f examples/multi-cluster-argocd/bindingpolicy.yaml
kubectl apply -f examples/multi-cluster-argocd/applicationset.yaml

# Verify integration
kubectl get integration -n argocd
kubectl describe integration argocd-multi-cluster -n argocd
```

---

## 📥 **Installation**

### **Option 1: Using Kustomize**

```bash
# Install CRDs
kubectl apply -k config/crd

# Deploy controller
kubectl apply -k config/default

# Verify deployment
kubectl get pods -n ksit-system
```

### **Option 2: Using Helm**

```bash
# Add Helm repository (when published)
helm repo add ksit https://kubestellar.github.io/integration-toolkit

# Install chart
helm install ksit ksit/ksit \
  --namespace ksit-system \
  --create-namespace \
  --set image.tag=v0.1.0

# Verify installation
helm status ksit -n ksit-system
```

### **Option 3: Using Make**

```bash
# Build and deploy
make docker-build
make deploy

# Or deploy with Helm
make deploy-helm
```

---

## 💡 **Usage Examples**

### **ArgoCD Integration**

Create an integration to manage ArgoCD applications across clusters:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: argocd-multi-cluster
  namespace: argocd
spec:
  type: argocd
  enabled: true
  targetClusters:
    - cluster1
    - cluster2
  config:
    serverURL: "https://argocd-server.argocd.svc.cluster.local"
    namespace: "argocd"
    insecure: "false"
```

```bash
kubectl apply -f config/samples/argocd_integration.yaml
```

### **Flux Integration**

Configure Flux for GitOps across multiple clusters:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: flux-multi-cluster
  namespace: flux-system
spec:
  type: flux
  enabled: true
  targetClusters:
    - cluster1
    - cluster2
  config:
    namespace: "flux-system"
    interval: "5m"
```

```bash
kubectl apply -f config/samples/flux_integration.yaml
```

### **Prometheus Integration**

Set up centralized monitoring:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: prometheus-monitoring
  namespace: monitoring
spec:
  type: prometheus
  enabled: true
  targetClusters:
    - cluster1
    - cluster2
  config:
    url: "http://prometheus.monitoring.svc.cluster.local:9090"
    scrapeInterval: "30s"
```

```bash
kubectl apply -f config/samples/prometheus_integration.yaml
```

### **Istio Integration**

Configure service mesh across clusters:

```yaml
apiVersion: ksit.io/v1alpha1
kind: Integration
metadata:
  name: istio-mesh
  namespace: istio-system
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

```bash
kubectl apply -f config/samples/istio_integration.yaml
```

---

## ⚙️ **Configuration**

### **Environment Variables**

| Variable | Description | Default |
|----------|-------------|---------|
| `KUBECONFIG` | Path to kubeconfig file | `~/.kube/config` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `METRICS_BIND_ADDRESS` | Metrics server address | `:8080` |
| `HEALTH_PROBE_BIND_ADDRESS` | Health probe address | `:8081` |
| `LEADER_ELECT` | Enable leader election | `false` |
| `ENABLE_WEBHOOK` | Enable validating webhooks | `false` |

### **Controller Flags**

```bash
./bin/ksit \
  --metrics-bind-address=:8080 \
  --health-probe-bind-address=:8081 \
  --leader-elect=true \
  --enable-webhook=true \
  --webhook-port=9443
```

---

## 🛠️ **Development**

### **Project Structure**

```
.
├── api/v1alpha1/           # API definitions
├── cmd/ksit/               # Main application entry point
├── config/                 # Kubernetes manifests
│   ├── crd/               # Custom Resource Definitions
│   ├── manager/           # Controller deployment
│   ├── rbac/              # RBAC configurations
│   ├── samples/           # Sample integrations
│   └── webhook/           # Webhook configurations
├── deploy/                # Deployment configurations
│   ├── helm/             # Helm charts
│   └── kustomize/        # Kustomize overlays
├── docs/                  # Documentation
├── examples/              # Example integrations
├── internal/              # Internal packages
│   ├── utils/            # Utility functions
│   └── webhook/          # Webhook validators
├── pkg/                   # Public packages
│   ├── cluster/          # Cluster management
│   ├── config/           # Configuration handling
│   ├── controller/       # Controller logic
│   ├── integrations/     # Integration clients
│   └── kubestellar/      # KubeStellar integration
├── scripts/               # Build and utility scripts
└── test/                  # Tests
    ├── e2e/              # End-to-end tests
    └── integration/      # Integration tests
```

### **Building from Source**

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test

# Run linters
make lint
```

### **Running Tests**

```bash
# Unit tests
make test

# Integration tests
make test-integration

# E2E tests (requires cluster)
make test-e2e

# All tests with coverage
make test-all
go tool cover -html=coverage.out
```

---

## 🚢 **Deployment**

### **Development Environment**

```bash
# Deploy to local cluster
make deploy

# Deploy samples
make deploy-samples

# Watch logs
make logs
```

### **Production Environment**

```bash
# Build production image
make docker-build IMG=your-registry/ksit:v0.1.0

# Push image
make docker-push IMG=your-registry/ksit:v0.1.0

# Deploy with custom image
make deploy IMG=your-registry/ksit:v0.1.0
```

### **Using Helm (Recommended for Production)**

```bash
# Install with custom values
helm install ksit deploy/helm/ksit \
  --namespace ksit-system \
  --create-namespace \
  --values your-values.yaml

# Upgrade
helm upgrade ksit deploy/helm/ksit -n ksit-system

# Uninstall
helm uninstall ksit -n ksit-system
```

---

## 🔍 **Troubleshooting**

### **Common Issues**

#### **1. Controller Not Starting**

```bash
# Check controller logs
kubectl logs -n ksit-system -l control-plane=controller-manager

# Check CRDs are installed
kubectl get crd | grep ksit.io

# Verify RBAC permissions
kubectl auth can-i list integrations --as=system:serviceaccount:ksit-system:ksit-controller
```

#### **2. Integration Not Reconciling**

```bash
# Check integration status
kubectl describe integration <name> -n <namespace>

# Check controller logs
kubectl logs -n ksit-system -l control-plane=controller-manager | grep <integration-name>

# Force reconciliation
kubectl annotate integration <name> -n <namespace> reconcile=true --overwrite
```

#### **3. Webhook Validation Errors**

```bash
# Check webhook certificates
kubectl get secret ksit-webhook-server-cert -n ksit-system

# Regenerate certificates
./scripts/generate-webhook-certs.sh

# Verify webhook configuration
kubectl get validatingwebhookconfiguration ksit-validating-webhook-configuration
```

### **Debug Mode**

Run controller with verbose logging:

```bash
kubectl set env deployment/ksit-controller-manager -n ksit-system LOG_LEVEL=debug
```

---

## 📚 **Documentation**

- **[Architecture Guide](docs/architecture.md)**: Detailed system architecture
- **[Getting Started](docs/getting-started.md)**: Step-by-step setup guide
- **[API Reference](docs/api-reference.md)**: CRD specifications
- **[Integration Guides](docs/integrations/)**: Tool-specific documentation
  - [ArgoCD Integration](docs/integrations/argocd.md)
  - [Flux Integration](docs/integrations/flux.md)
  - [Prometheus Integration](docs/integrations/prometheus.md)
  - [Istio Integration](docs/integrations/istio.md)
- **[Troubleshooting Guide](docs/troubleshooting.md)**: Common issues and solutions

---

## 🤝 **Contributing**

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### **Development Workflow**

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test lint`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### **Code of Conduct**

This project adheres to the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).

---

## 📄 **License**

This project is licensed under the **Apache License 2.0** - see the [LICENSE](LICENSE) file for details.

---

## 🌟 **Acknowledgments**

- Built on top of [KubeStellar](https://docs.kubestellar.io/)
- Powered by [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- Inspired by the Kubernetes operator pattern

---

## 📞 **Support & Community**

- **Issues**: [GitHub Issues](https://github.com/kubestellar/integration-toolkit/issues)
- **Discussions**: [GitHub Discussions](https://github.com/kubestellar/integration-toolkit/discussions)
- **Slack**: [#kubestellar on Kubernetes Slack](https://kubernetes.slack.com/messages/kubestellar)

---

## 🗺️ **Roadmap**

- [ ] Support for additional integrations (Tekton, Kyverno)
- [ ] Enhanced multi-tenancy support
- [ ] Automated disaster recovery
- [ ] Advanced policy enforcement
- [ ] GUI dashboard for management

---

**Made with ❤️ by the KubeStellar Community**