# Prometheus Integration Documentation

## Overview

This document outlines the integration of Prometheus with the KubeStellar Integration Toolkit (KSIT). It provides guidance on how to set up monitoring for multi-cluster environments using Prometheus.

## Prerequisites

- A running instance of Prometheus.
- Access to Kubernetes clusters managed by KSIT.
- Basic understanding of Kubernetes and Prometheus.

## Integration Steps

1. **Install Prometheus**: Deploy Prometheus in your Kubernetes cluster. You can use the official Helm chart or a custom YAML configuration.

2. **ServiceMonitor Configuration**: Create a `ServiceMonitor` resource to allow Prometheus to scrape metrics from the KSIT components. Below is an example configuration:

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: ksit-monitor
     labels:
       app: ksit
   spec:
     selector:
       matchLabels:
         app: ksit
     endpoints:
       - port: metrics
         interval: 30s
   ```

3. **Expose Metrics Endpoint**: Ensure that your KSIT components expose a metrics endpoint. This can be done by adding the following to your component's deployment:

   ```yaml
   ports:
     - name: metrics
       containerPort: 8080
       protocol: TCP
   ```

4. **Prometheus Configuration**: Update your Prometheus configuration to include the `ServiceMonitor`:

   ```yaml
   scrape_configs:
     - job_name: 'ksit'
       kubernetes_sd_configs:
         - role: endpoints
       relabel_configs:
         - source_labels: [__meta_kubernetes_service_name]
           action: keep
           regex: ksit-monitor
   ```

5. **Visualizing Metrics**: Use Grafana or Prometheus UI to visualize the metrics collected from KSIT components. Create dashboards to monitor the health and performance of your multi-cluster setup.

## Example Metrics

- `ksit_reconcile_duration_seconds`: Duration of reconciliation loops.
- `ksit_cluster_status`: Status of clusters managed by KSIT.

## Troubleshooting

- Ensure that Prometheus can access the metrics endpoint.
- Check the logs of the KSIT components for any errors related to metrics exposure.
- Validate the `ServiceMonitor` and Prometheus configurations for correctness.

## Conclusion

Integrating Prometheus with the KubeStellar Integration Toolkit enhances observability across multi-cluster environments, allowing for better monitoring and management of Kubernetes resources.