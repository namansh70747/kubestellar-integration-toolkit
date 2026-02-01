#!/bin/bash
set -e

echo "Setting up Kind clusters for KSIT..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "Error: kind is not installed. Please install kind first:"
    echo "  brew install kind  (macOS)"
    echo "  or visit: https://kind.sigs.k8s.io/docs/user/quick-start/"
    exit 1
fi

# Delete existing clusters if they exist
echo "Cleaning up existing clusters..."
kind delete cluster --name ksit-control 2>/dev/null || true
kind delete cluster --name cluster-1 2>/dev/null || true
kind delete cluster --name cluster-2 2>/dev/null || true

# Create control cluster
echo "Creating ksit-control cluster..."
cat <<EOF | kind create cluster --name ksit-control --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

# Create workload cluster-1
echo "Creating cluster-1..."
cat <<EOF | kind create cluster --name cluster-1 --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

# Create workload cluster-2
echo "Creating cluster-2..."
cat <<EOF | kind create cluster --name cluster-2 --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

# Get cluster IPs
echo "Getting cluster IPs..."
CLUSTER1_IP=$(docker inspect cluster-1-control-plane | grep '"IPAddress"' | head -1 | awk -F'"' '{print $4}')
CLUSTER2_IP=$(docker inspect cluster-2-control-plane | grep '"IPAddress"' | head -1 | awk -F'"' '{print $4}')

echo "Cluster IPs:"
echo "  cluster-1: $CLUSTER1_IP"
echo "  cluster-2: $CLUSTER2_IP"

# Create namespace on control cluster
kubectl config use-context kind-ksit-control
kubectl create namespace ksit-system --dry-run=client -o yaml | kubectl apply -f -

# Create kubeconfig secrets
echo "Creating kubeconfig secrets..."

# Cluster-1 kubeconfig
kind get kubeconfig --name cluster-1 | \
    sed "s|127.0.0.1:[0-9]*|${CLUSTER1_IP}:6443|g" | \
    kubectl create secret generic cluster-1-kubeconfig \
    --from-file=kubeconfig=/dev/stdin \
    -n ksit-system \
    --dry-run=client -o yaml | kubectl apply -f -

# Cluster-2 kubeconfig
kind get kubeconfig --name cluster-2 | \
    sed "s|127.0.0.1:[0-9]*|${CLUSTER2_IP}:6443|g" | \
    kubectl create secret generic cluster-2-kubeconfig \
    --from-file=kubeconfig=/dev/stdin \
    -n ksit-system \
    --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "Clusters created successfully!"
echo ""
echo "Next steps:"
echo "  1. Deploy KSIT controller: make deploy-local"
echo "  2. Install integrations: make install-integrations"
