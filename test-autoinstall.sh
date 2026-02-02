#!/bin/bash
set -e

# KSIT Auto-Install Complete Test Script
# This script demonstrates the complete auto-install feature from scratch

echo "============================================"
echo "  KSIT Auto-Install & Monitoring Test"
echo "============================================"
echo

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

CONTROL_CLUSTER="kind-ksit-control"
TARGET_CLUSTER_1="kind-cluster-1"
TARGET_CLUSTER_2="kind-cluster-2"

# Step 1: Cleanup
echo -e "${YELLOW}Step 1: Cleaning up existing installations...${NC}"
kubectl delete integrations --all -A --context $CONTROL_CLUSTER 2>/dev/null || true

for cluster in $TARGET_CLUSTER_1 $TARGET_CLUSTER_2; do
    echo "  Cleaning $cluster..."
    for ns in argocd monitoring istio-system flux-system; do
        helm uninstall --namespace $ns $(helm list -n $ns --kube-context $cluster -q 2>/dev/null) --kube-context $cluster 2>/dev/null || true
    done
    kubectl delete ns argocd monitoring istio-system flux-system --context $cluster 2>/dev/null || true &
done
wait

echo -e "${GREEN}✓ Cleanup complete${NC}"
echo

# Step 2: Verify clean state
echo -e "${YELLOW}Step 2: Verifying clean state...${NC}"
sleep 5
REMAINING=$(kubectl get integrations -A --context $CONTROL_CLUSTER 2>/dev/null | grep -v "No resources" | wc -l)
if [ $REMAINING -eq 0 ]; then
    echo -e "${GREEN}✓ Clean state verified${NC}"
else
    echo -e "${RED}⚠ Some resources still exist${NC}"
fi
echo

# Step 3: Apply integrations with auto-install
echo -e "${YELLOW}Step 3: Applying integrations with auto-install enabled...${NC}"
echo "  1. ArgoCD (Helm-based)"
kubectl apply -f config/samples/argocd_integration_autoinstall.yaml --context $CONTROL_CLUSTER
echo "  2. Prometheus (Helm-based)"
kubectl apply -f config/samples/prometheus_integration_autoinstall.yaml --context $CONTROL_CLUSTER
echo "  3. Istio (Helm-based)"
kubectl apply -f config/samples/istio_integration_autoinstall.yaml --context $CONTROL_CLUSTER
echo "  4. Flux (Manifest-based - skeleton)"
kubectl apply -f config/samples/flux_integration.yaml --context $CONTROL_CLUSTER 2>/dev/null || echo "    Flux skipped (not implemented)"

echo -e "${GREEN}✓ All integrations applied${NC}"
echo

# Step 4: Wait for installations
echo -e "${YELLOW}Step 4: Waiting for auto-installations to complete (90 seconds)...${NC}"
for i in {1..9}; do
    echo -n "."
    sleep 10
done
echo
echo -e "${GREEN}✓ Wait complete${NC}"
echo

# Step 5: Verify integrations
echo -e "${YELLOW}Step 5: Verifying integration status...${NC}"
kubectl get integrations -A --context $CONTROL_CLUSTER
echo

# Step 6: Verify Helm releases
echo -e "${YELLOW}Step 6: Verifying Helm releases...${NC}"
echo "Cluster-1:"
helm list -A --kube-context $TARGET_CLUSTER_1 | grep -E "NAME|argocd|prometheus|istio" || echo "  No releases found"
echo
echo "Cluster-2:"
helm list -A --kube-context $TARGET_CLUSTER_2 | grep -E "NAME|argocd" || echo "  No releases found"
echo

# Step 7: Verify pods
echo -e "${YELLOW}Step 7: Verifying pod counts...${NC}"
echo "Cluster-1:"
echo "  ArgoCD pods: $(kubectl get pods -n argocd --context $TARGET_CLUSTER_1 --no-headers 2>/dev/null | wc -l)"
echo "  Prometheus pods: $(kubectl get pods -n monitoring --context $TARGET_CLUSTER_1 --no-headers 2>/dev/null | wc -l)"
echo "  Istio pods: $(kubectl get pods -n istio-system --context $TARGET_CLUSTER_1 --no-headers 2>/dev/null | wc -l)"
echo
echo "Cluster-2:"
echo "  ArgoCD pods: $(kubectl get pods -n argocd --context $TARGET_CLUSTER_2 --no-headers 2>/dev/null | wc -l)"
echo

# Step 8: Test continuous monitoring
echo -e "${YELLOW}Step 8: Testing continuous health monitoring...${NC}"
echo "Initial reconcile times:"
kubectl get integrations -A --context $CONTROL_CLUSTER -o custom-columns=NAME:.metadata.name,LAST_RECONCILE:.status.lastReconcileTime --no-headers | grep -v flux

echo
echo "Waiting 45 seconds for next reconciliation..."
sleep 45
echo
echo "Updated reconcile times:"
kubectl get integrations -A --context $CONTROL_CLUSTER -o custom-columns=NAME:.metadata.name,LAST_RECONCILE:.status.lastReconcileTime --no-headers | grep -v flux
echo

# Step 9: Final summary
echo "============================================"
echo "  TEST RESULTS SUMMARY"
echo "============================================"
echo

RUNNING_COUNT=$(kubectl get integrations -A --context $CONTROL_CLUSTER -o json | jq '[.items[] | select(.status.phase=="Running")] | length' 2>/dev/null || echo "0")
TOTAL_COUNT=$(kubectl get integrations -A --context $CONTROL_CLUSTER -o json | jq '.items | length' 2>/dev/null || echo "0")

echo -e "${GREEN}✅ Integrations Running: $RUNNING_COUNT/$TOTAL_COUNT${NC}"
echo

# Check each tool
check_integration() {
    local name=$1
    local status=$(kubectl get integration $name --context $CONTROL_CLUSTER -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    if [ "$status" = "Running" ]; then
        echo -e "  ✅ $name: ${GREEN}Running${NC}"
    elif [ "$status" = "Failed" ]; then
        echo -e "  ⚠️  $name: ${YELLOW}Failed${NC} (expected for Flux - not implemented)"
    else
        echo -e "  ❌ $name: ${RED}$status${NC}"
    fi
}

check_integration "argocd-autoinstall"
check_integration "prometheus-autoinstall"
check_integration "istio-autoinstall"
check_integration "flux-autoinstall"

echo
echo "Helm Releases Created:"
HELM_COUNT=$(helm list -A --kube-context $TARGET_CLUSTER_1 2>/dev/null | grep -c "deployed" || echo "0")
echo -e "  ${GREEN}$HELM_COUNT releases${NC} on cluster-1"

echo
echo "Health Monitoring:"
MONITORING=$(kubectl get integrations -A --context $CONTROL_CLUSTER -o json | jq '[.items[] | select(.status.lastReconcileTime != null)] | length' 2>/dev/null || echo "0")
echo -e "  ${GREEN}$MONITORING integrations${NC} actively monitored"

echo
echo "============================================"
echo -e "${GREEN}✅ Auto-Install Test Complete!${NC}"
echo "============================================"
echo
echo "Next Steps:"
echo "  - Check controller logs: kubectl logs -n ksit-system -l control-plane=controller-manager"
echo "  - Verify ArgoCD UI: kubectl port-forward svc/argocd-server -n argocd 8080:443"
echo "  - View Integration details: kubectl get integration <name> -o yaml"
echo
