# Multi-Cluster Istio Service Mesh Example

This example demonstrates how to configure Istio service mesh across multiple clusters using KSIT.

## Prerequisites

- Istio installed in all clusters (`istio-system` namespace)
- Multi-cluster mesh configured between clusters
- KSIT controller running
- Clusters labeled with `mesh: enabled` and `integration: istio`

## Files

- **`integration.yaml`** - KSIT Integration for Istio
- **`bindingpolicy.yaml`** - BindingPolicy for Istio resources
- **`virtualservice.yaml`** - VirtualService for traffic routing
- **`destinationrule.yaml`** - DestinationRule for traffic policies

## Quick Start

### 1. Verify Istio Installation

```bash
# Check Istio control plane
kubectl get pods -n istio-system

# Verify multi-cluster setup
istioctl remote-clusters

# Check mesh connectivity
istioctl proxy-status
```

### 2. Apply Integration

```bash
kubectl apply -f integration.yaml
```

Verify integration:
```bash
kubectl get integration istio-multi-cluster-mesh -n istio-system
kubectl describe integration istio-multi-cluster-mesh -n istio-system
```

### 3. Apply BindingPolicy

```bash
kubectl apply -f bindingpolicy.yaml
```

### 4. Deploy Traffic Rules

```bash
# Apply VirtualService
kubectl apply -f virtualservice.yaml

# Apply DestinationRule
kubectl apply -f destinationrule.yaml
```

### 5. Verify Configuration

```bash
# Check VirtualService
kubectl get virtualservice multi-cluster-virtualservice -n default

# Check DestinationRule
kubectl get destinationrule multi-cluster-destination-rule -n default

# Analyze configuration
istioctl analyze -n default
```

## Configuration Details

### VirtualService

Routes traffic to services based on URI paths:

- **`/api`** → routes to `api-service:8080`
- **`/web`** → routes to `web-service:80`

Features:
- Host-based routing (`myapp.example.com`)
- Gateway binding for external access
- Path-based routing rules

### DestinationRule

Defines traffic policies for the API service:

- **mTLS**: Istio mutual TLS enabled
- **Load Balancing**: Round-robin algorithm
- **Connection Pool**: Limits concurrent connections
- **Outlier Detection**: Removes unhealthy endpoints

### BindingPolicy

Distributes Istio configuration to clusters with labels:
- `mesh: enabled`
- `integration: istio`

Resources synchronized:
- VirtualServices
- DestinationRules
- Gateways
- ServiceEntries
- PeerAuthentications
- AuthorizationPolicies

## Testing Traffic Routing

### 1. Deploy Test Application

```bash
# Deploy API service
kubectl create deployment api-service --image=your-api-image:latest
kubectl expose deployment api-service --port=8080

# Deploy Web service
kubectl create deployment web-service --image=your-web-image:latest
kubectl expose deployment web-service --port=80
```

### 2. Create Gateway

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: multi-cluster-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "myapp.example.com"
```

Apply:
```bash
kubectl apply -f gateway.yaml
```

### 3. Test Routing

```bash
# Get Istio ingress gateway address
export INGRESS_HOST=$(kubectl get service istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Test API endpoint
curl -H "Host: myapp.example.com" http://$INGRESS_HOST/api/health

# Test Web endpoint
curl -H "Host: myapp.example.com" http://$INGRESS_HOST/web/
```

## Advanced Configurations

### Traffic Splitting

Modify `virtualservice.yaml` for canary deployments:

```yaml
http:
  - match:
      - uri:
          prefix: /api
    route:
      - destination:
          host: api-service-v1
        weight: 90
      - destination:
          host: api-service-v2
        weight: 10
```

### Fault Injection

Add fault injection for testing:

```yaml
http:
  - match:
      - uri:
          prefix: /api
    fault:
      delay:
        percentage:
          value: 10
        fixedDelay: 5s
      abort:
        percentage:
          value: 5
        httpStatus: 503
```

### Circuit Breaking

Update `destinationrule.yaml`:

```yaml
trafficPolicy:
  connectionPool:
    tcp:
      maxConnections: 100
    http:
      http1MaxPendingRequests: 50
      maxRequestsPerConnection: 2
  outlierDetection:
    consecutive5xxErrors: 5
    interval: 30s
    baseEjectionTime: 30s
```

## Monitoring and Observability

### View Metrics

```bash
# Prometheus metrics
kubectl port-forward -n istio-system svc/prometheus 9090:9090

# Grafana dashboards
kubectl port-forward -n istio-system svc/grafana 3000:3000

# Kiali service mesh visualization
kubectl port-forward -n istio-system svc/kiali 20001:20001
```

### Check Traffic Distribution

```bash
# View service graph
istioctl dashboard kiali

# Check proxy configuration
istioctl proxy-config routes <pod-name> -n default

# View endpoint distribution
istioctl proxy-config endpoints <pod-name> -n default
```

### Distributed Tracing

```bash
# Jaeger UI
kubectl port-forward -n istio-system svc/tracing 16686:80

# View traces in browser
open http://localhost:16686
```

## Troubleshooting

### VirtualService Not Working

```bash
# Validate configuration
istioctl analyze -n default

# Check VirtualService status
kubectl describe virtualservice multi-cluster-virtualservice -n default

# View Envoy configuration
istioctl proxy-config route <pod-name>.<namespace> --name 8080 -o json
```

### DestinationRule Issues

```bash
# Verify DestinationRule
kubectl get destinationrule -n default

# Check applied policies
istioctl proxy-config cluster <pod-name> -n default

# View outlier detection status
istioctl proxy-config endpoint <pod-name> -n default --cluster "outbound|8080||api-service.default.svc.cluster.local"
```

### mTLS Not Working

```bash
# Check mTLS status
istioctl authn tls-check <pod-name>.<namespace>

# View authentication policies
kubectl get peerauthentication -A

# Test mTLS connectivity
kubectl exec <pod-name> -n default -- curl -v https://api-service.default.svc.cluster.local:8080
```

### Cross-Cluster Communication Failing

```bash
# Verify multi-cluster setup
istioctl remote-clusters

# Check service entries
kubectl get serviceentry -n istio-system

# View cross-cluster endpoints
istioctl proxy-config endpoint <pod-name> -n default | grep "outbound"
```

## Cleanup

```bash
# Delete traffic rules
kubectl delete -f virtualservice.yaml
kubectl delete -f destinationrule.yaml

# Delete binding policy
kubectl delete -f bindingpolicy.yaml

# Delete integration
kubectl delete -f integration.yaml

# Clean up test applications
kubectl delete deployment api-service web-service
kubectl delete service api-service web-service
```

## Best Practices

1. **Start with Permissive mTLS**: Use `PERMISSIVE` mode before enforcing `STRICT`
2. **Use Namespace Isolation**: Deploy services in separate namespaces
3. **Monitor Circuit Breakers**: Watch outlier detection metrics
4. **Test Fault Injection**: Validate application resilience
5. **Implement Retry Logic**: Configure retry policies for transient failures

## Next Steps

- Configure [Authorization Policies](https://istio.io/latest/docs/tasks/security/authorization/)
- Set up [Request Authentication](https://istio.io/latest/docs/tasks/security/authentication/authn-policy/)
- Implement [Rate Limiting](https://istio.io/latest/docs/tasks/policy-enforcement/rate-limit/)

## References

- [Istio Documentation](https://istio.io/latest/docs/)
- [Multi-Cluster Installation](https://istio.io/latest/docs/setup/install/multicluster/)
- [Traffic Management](https://istio.io/latest/docs/tasks/traffic-management/)
- [KSIT Documentation](../../docs/)