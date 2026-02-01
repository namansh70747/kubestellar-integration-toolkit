#!/bin/bash
set -e

echo "Cleaning up KSIT environment..."

# Delete Integration and IntegrationTarget resources
kubectl config use-context kind-ksit-control 2>/dev/null || true
kubectl delete integrations --all -n ksit-system 2>/dev/null || true
kubectl delete integrationtargets --all -n ksit-system 2>/dev/null || true

# Undeploy KSIT controller
kubectl delete deployment ksit-controller-manager -n ksit-system 2>/dev/null || true
kubectl delete namespace ksit-system 2>/dev/null || true

# Delete kind clusters
echo "Deleting kind clusters..."
kind delete cluster --name ksit-control 2>/dev/null || true
kind delete cluster --name cluster-1 2>/dev/null || true
kind delete cluster --name cluster-2 2>/dev/null || true

echo "Cleanup complete!"
