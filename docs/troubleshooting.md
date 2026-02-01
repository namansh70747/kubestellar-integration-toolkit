# Troubleshooting KSIT

Common problems and their solutions.

## Integration Shows "Failed" But Pods Are Running

**Symptom**: `kubectl get integration` shows Phase: Failed, but when you check the cluster, all pods are running fine.

**Likely Causes**:

1. **Wrong namespace**: KSIT expects tools in specific namespaces by default:
   - ArgoCD: `argocd`
   - Flux: `flux-system`
   - Prometheus: `monitoring`
   - Istio: `istio-system`

   If you installed in a different namespace, KSIT won't find it.

**Solution**: Either reinstall in the expected namespace, or modify the Integration spec to specify your namespace (note: current version doesn't support custom namespaces, so reinstalling is the only option for now).

1. **Incomplete installation**: Some components might be missing.

**Solution**: Check controller logs to see what's missing:

```bash
kubectl logs deployment/ksit-controller-manager -n ksit-system | grep -i error
```

## IntegrationTarget Shows "Ready: false"

**Symptom**: `kubectl get integrationtargets` shows your cluster as not ready.

**Causes**:

1. **Kubeconfig secret missing or wrong**

Check if the secret exists:

```bash
kubectl get secret cluster-1-kubeconfig -n ksit-system
```

Verify the kubeconfig is valid:

```bash
kubectl get secret cluster-1-kubeconfig -n ksit-system -o jsonpath='{.data.kubeconfig}' | base64 -d | kubectl --kubeconfig=/dev/stdin get nodes
```

**Solution**: Recreate the secret with the correct kubeconfig.

1. **Wrong API server address**

If you're using kind clusters, the kubeconfig might have `127.0.0.1` which isn't reachable from inside the container.

**Solution**: Use the Docker bridge IP instead:

```bash
CLUSTER_IP=$(docker inspect cluster-1-control-plane | grep '"IPAddress"' | head -1 | awk -F'"' '{print $4}')
kind get kubeconfig --name cluster-1 | sed "s|127.0.0.1:[0-9]*|${CLUSTER_IP}:6443|g" > /tmp/cluster-1.kubeconfig
kubectl create secret generic cluster-1-kubeconfig --from-file=kubeconfig=/tmp/cluster-1.kubeconfig -n ksit-system --dry-run=client -o yaml | kubectl apply -f -
```

1. **Network connectivity issues**

The controller can't reach the cluster.

**Solution**: Check if you can reach the cluster from inside a pod:

```bash
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- sh
# Inside the pod, try to reach the cluster
curl -k https://<cluster-ip>:6443
```

## Controller Pod Is CrashLooping

**Symptom**: `kubectl get pods -n ksit-system` shows the controller pod restarting repeatedly.

**Causes**:

1. **CRDs not installed**

**Solution**:

```bash
kubectl apply -f config/crd/bases/
```

1. **RBAC permissions missing**

**Solution**:

```bash
kubectl apply -f config/rbac/
```

1. **Image not found (for kind clusters)**

If you built a new image but forgot to load it into kind.

**Solution**:

```bash
kind load docker-image ksit-controller:latest --name ksit-control
kubectl rollout restart deployment/ksit-controller-manager -n ksit-system
```

Check the actual error:

```bash
kubectl logs deployment/ksit-controller-manager -n ksit-system
```

## Integration Stuck in "Pending"

**Symptom**: Integration resource stays in Pending phase and never moves to Running or Failed.

**Cause**: The IntegrationTarget for one of the specified clusters doesn't exist.

Check if all target clusters exist:

```bash
kubectl get integrationtargets -n ksit-system
```

**Solution**: Create the missing IntegrationTarget resources.

## "Context Deadline Exceeded" Errors in Logs

**Symptom**: Controller logs show timeout errors when trying to check remote clusters.

**Causes**:

1. **Slow cluster responses**: The cluster is overloaded or slow.

**Solution**: Increase the timeout in the controller code (requires rebuilding).

1. **Network latency**: High latency to remote cluster.

**Solution**: If checking cloud clusters from a local controller, this might be expected. Consider running the controller closer to the clusters.

## Changes to Kubeconfig Not Taking Effect

**Symptom**: You updated the kubeconfig secret but the controller still can't connect.

**Cause**: The controller caches cluster configs in memory. It doesn't watch for secret changes.

**Solution**: Restart the controller:

```bash
kubectl rollout restart deployment/ksit-controller-manager -n ksit-system
```

## "No Such Host" or DNS Resolution Errors

**Symptom**: Controller logs show DNS resolution failures when trying to reach cluster API servers.

**Cause**: The API server hostname in the kubeconfig doesn't resolve from inside the pod.

**Solution**: Use IP addresses instead of hostnames in kubeconfigs, or ensure DNS is properly configured in your cluster.

## Integration Type Not Recognized

**Symptom**: Integration stuck in Pending with no error message.

**Cause**: You specified an unsupported integration type (e.g., typo: "argcd" instead of "argocd").

Check the spec:

```bash
kubectl get integration my-integration -o yaml
```

**Solution**: Fix the type to one of: argocd, flux, prometheus, istio

## Prometheus Shows Failed But It's Running

**Specific to Prometheus Integration**

KSIT looks for specific components:

- prometheus-operator deployment
- prometheus StatefulSet (not deployment)
- alertmanager StatefulSet

If you installed Prometheus without the operator (e.g., standalone Prometheus), KSIT won't recognize it.

**Solution**: Install the full kube-prometheus-stack which includes the operator.

## Flux Shows Healthy But Some Controllers Are Missing

KSIT checks for four main controllers:

- source-controller
- kustomize-controller
- helm-controller
- notification-controller

If you installed Flux with `--components` flag selecting only some controllers, KSIT might report partial health.

**Solution**: Install all controllers or modify the health check logic to expect fewer controllers.

## Getting More Debug Information

Enable verbose logging:

```bash
kubectl set env deployment/ksit-controller-manager -n ksit-system LOG_LEVEL=debug
```

Watch logs in real-time:

```bash
kubectl logs -f deployment/ksit-controller-manager -n ksit-system
```

Filter for specific integration:

```bash
kubectl logs deployment/ksit-controller-manager -n ksit-system | grep argocd
```

## Still Stuck?

If none of these solutions help:

1. Check the GitHub issues: <https://github.com/kubestellar/integration-toolkit/issues>
2. File a new issue with:
   - Output of `kubectl get integrations -n ksit-system -o yaml`
   - Output of `kubectl get integrationtargets -n ksit-system -o yaml`
   - Controller logs
   - Description of what you expected vs what happened
3. Join the KubeStellar community Slack for help
