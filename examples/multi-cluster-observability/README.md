# Multi-Cluster Observability with Prometheus Example

This example demonstrates how to monitor KSIT and multi-cluster deployments using Prometheus.

## Prerequisites

- Prometheus Operator installed (`monitoring` namespace)
- KSIT controller running with metrics enabled
- Grafana (optional, for dashboards)

## Files

- **`integration.yaml`** - KSIT Integration for Prometheus
- **`bindingpolicy.yaml`** - BindingPolicy for monitoring resources
- **`servicemonitor.yaml`** - ServiceMonitor for KSIT metrics
- **`prometheus-rules.yaml`** - Alert rules for KSIT

## Quick Start

### 1. Verify Prometheus Installation

```bash
# Check Prometheus Operator
kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus-operator

# Check Prometheus instances
kubectl get prometheus -n monitoring

# Verify services
kubectl get svc -n monitoring
```

### 2. Apply Integration

```bash
kubectl apply -f integration.yaml
```

Verify:
```bash
kubectl get integration prometheus-multi-cluster -n monitoring
```

### 3. Deploy ServiceMonitor

```bash
kubectl apply -f servicemonitor.yaml
```

Verify scraping:
```bash
# Check ServiceMonitor
kubectl get servicemonitor -n monitoring

# View Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
open http://localhost:9090/targets
```

### 4. Deploy Alert Rules

```bash
kubectl apply -f prometheus-rules.yaml
```

Verify rules:
```bash
# Check PrometheusRule
kubectl get prometheusrule -n monitoring

# View in Prometheus UI
open http://localhost:9090/rules
```

### 5. Apply BindingPolicy

```bash
kubectl apply -f bindingpolicy.yaml
```

## Metrics Exposed

KSIT controller exposes the following metrics:

### Integration Metrics

- **`ksit_integration_status{integration, type, cluster}`**
  - Current status of integrations (1=running, 0=not running)
  
- **`ksit_integration_reconcile_total{integration, type, status}`**
  - Total number of integration reconciliations
  
- **`ksit_integration_reconcile_duration_seconds{integration, type}`**
  - Duration of integration reconciliation

### Cluster Metrics

- **`ksit_cluster_connection_status{cluster}`**
  - Cluster connection status (1=connected, 0=disconnected)

### Sync Metrics

- **`ksit_sync_operations_total{integration, cluster, status}`**
  - Total number of sync operations
  
- **`ksit_sync_latency_seconds{integration, cluster}`**
  - Sync operation latency

## Alert Rules

The following alerts are configured:

### IntegrationDown (Critical)

Fires when an integration is down for more than 5 minutes.

```promql
ksit_integration_status{status="running"} == 0
```

### HighSyncLatency (Warning)

Fires when sync latency exceeds 30 seconds for more than 10 minutes.

```promql
ksit_sync_latency_seconds > 30
```

### SyncFailures (Warning)

Fires when sync failure rate exceeds 0.1 per second.

```promql
rate(ksit_sync_operations_total{status="failed"}[5m]) > 0.1
```

### ClusterDisconnected (Critical)

Fires when a cluster is disconnected for more than 2 minutes.

```promql
ksit_cluster_connection_status == 0
```

## Viewing Metrics

### Access Prometheus UI

```bash
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
```

Open browser: http://localhost:9090

### Example Queries

**Check integration status:**
```promql
ksit_integration_status
```

**Reconciliation rate per integration:**
```promql
rate(ksit_integration_reconcile_total[5m])
```

**Average sync latency:**
```promql
avg(ksit_sync_latency_seconds) by (integration, cluster)
```

**Failed syncs in last hour:**
```promql
increase(ksit_sync_operations_total{status="failed"}[1h])
```

## Grafana Dashboards

### Install Grafana

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm install grafana grafana/grafana -n monitoring
```

Get admin password:
```bash
kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode
```

Access Grafana:
```bash
kubectl port-forward -n monitoring svc/grafana 3000:80
```

### Import Dashboard

Create a dashboard with these panels:

**Integration Status Panel:**
```promql
sum(ksit_integration_status) by (integration, type)
```

**Sync Latency Panel:**
```promql
histogram_quantile(0.95, sum(rate(ksit_sync_latency_seconds_bucket[5m])) by (le, integration))
```

**Reconciliation Rate Panel:**
```promql
sum(rate(ksit_integration_reconcile_total[5m])) by (integration, status)
```

**Cluster Connection Status:**
```promql
ksit_cluster_connection_status
```

## Alert Manager Configuration

Configure AlertManager to receive alerts:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: monitoring
data:
  alertmanager.yml: |
    global:
      resolve_timeout: 5m
    
    route:
      group_by: ['alertname', 'cluster']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 12h
      receiver: 'slack-notifications'
    
    receivers:
      - name: 'slack-notifications'
        slack_configs:
          - api_url: 'YOUR_SLACK_WEBHOOK_URL'
            channel: '#ksit-alerts'
            title: 'KSIT Alert: {{ .GroupLabels.alertname }}'
            text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

Apply:
```bash
kubectl apply -f alertmanager-config.yaml
```

## Testing Alerts

### Trigger IntegrationDown Alert

```bash
# Scale down KSIT controller
kubectl scale deployment ksit-controller-manager -n ksit-system --replicas=0

# Wait 5 minutes and check alerts
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
open http://localhost:9090/alerts
```

### Trigger SyncFailures Alert

```bash
# Delete a critical resource to cause sync failures
kubectl delete integration <integration-name> -n <namespace>
```

Restore:
```bash
kubectl scale deployment ksit-controller-manager -n ksit-system --replicas=1
```

## Troubleshooting

### ServiceMonitor Not Scraping

```bash
# Check ServiceMonitor selector
kubectl get servicemonitor ksit-controller-metrics -n monitoring -o yaml

# Verify service labels match
kubectl get svc -n ksit-system -l app.kubernetes.io/name=ksit --show-labels

# Check Prometheus configuration
kubectl get prometheus -n monitoring -o yaml | grep serviceMonitorSelector
```

### Metrics Not Appearing

```bash
# Check KSIT controller metrics endpoint
kubectl port-forward -n ksit-system svc/ksit-controller-manager-metrics 8080:8080 &
curl http://localhost:8080/metrics | grep ksit_

# Verify Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
open http://localhost:9090/targets
```

### Alerts Not Firing

```bash
# Check PrometheusRule
kubectl get prometheusrule ksit-multi-cluster-alerts -n monitoring

# Verify rule loading in Prometheus
open http://localhost:9090/rules

# Check AlertManager
kubectl logs -n monitoring alertmanager-<pod-name>
```

## Cleanup

```bash
# Delete monitoring resources
kubectl delete -f prometheus-rules.yaml
kubectl delete -f servicemonitor.yaml
kubectl delete -f bindingpolicy.yaml
kubectl delete -f integration.yaml
```

## Best Practices

1. **Set Appropriate Alert Thresholds**: Adjust based on your SLOs
2. **Use Recording Rules**: Pre-compute expensive queries
3. **Implement Alert Silencing**: During maintenance windows
4. **Monitor Resource Usage**: Track Prometheus memory/CPU
5. **Regular Retention Policy**: Configure appropriate data retention

## Next Steps

- Configure [Thanos](https://thanos.io/) for long-term storage
- Set up [Loki](https://grafana.com/oss/loki/) for log aggregation
- Implement [Jaeger](https://www.jaegertracing.io/) for distributed tracing

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
- [Grafana Documentation](https://grafana.com/docs/)
- [KSIT Documentation](../../docs/)