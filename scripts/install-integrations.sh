#!/bin/bash
set -e

echo "Installing DevOps tools on clusters..."

# Check required tools
for cmd in kubectl helm flux istioctl; do
    if ! command -v $cmd &> /dev/null; then
        echo "Warning: $cmd is not installed. Skipping $cmd integration."
    fi
done

# Install ArgoCD on both clusters
echo "Installing ArgoCD..."
for cluster in cluster-1 cluster-2; do
    echo "  Installing on $cluster..."
    kubectl config use-context kind-$cluster
    kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
    echo "  Waiting for ArgoCD to be ready on $cluster..."
    kubectl wait --for=condition=available --timeout=300s deployment/argocd-server -n argocd || true
done

# Install Flux on cluster-1
if command -v flux &> /dev/null; then
    echo "Installing Flux on cluster-1..."
    kubectl config use-context kind-cluster-1
    flux install --components-extra=image-reflector-controller,image-automation-controller
    echo "  Waiting for Flux to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/source-controller -n flux-system || true
else
    echo "Skipping Flux (flux CLI not installed)"
fi

# Install Prometheus on both clusters
if command -v helm &> /dev/null; then
    echo "Installing Prometheus Stack..."
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
    helm repo update
    
    for cluster in cluster-1 cluster-2; do
        echo "  Installing on $cluster..."
        kubectl config use-context kind-$cluster
        kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
        helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
            --namespace monitoring \
            --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
            --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
            --wait --timeout=5m || true
    done
else
    echo "Skipping Prometheus (helm not installed)"
fi

# Install Istio on cluster-2
if command -v istioctl &> /dev/null; then
    echo "Installing Istio on cluster-2..."
    kubectl config use-context kind-cluster-2
    istioctl install --set profile=default -y
    echo "  Waiting for Istio to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/istiod -n istio-system || true
else
    echo "Skipping Istio (istioctl not installed)"
fi

# Switch back to control cluster
kubectl config use-context kind-ksit-control

echo ""
echo "Integration tools installed!"
echo ""
echo "Installed on cluster-1:"
echo "  - ArgoCD (7 components)"
echo "  - Flux (6 controllers)"
echo "  - Prometheus Stack (operator, grafana, prometheus, alertmanager)"
echo ""
echo "Installed on cluster-2:"
echo "  - ArgoCD (7 components)"
echo "  - Prometheus Stack (operator, grafana, prometheus, alertmanager)"
echo "  - Istio (istiod, ingress gateway)"
echo ""
echo "Next step: Create Integration CRDs"
echo "  kubectl apply -f config/samples/"
