# Troubleshooting Guide for KubeStellar Integration Toolkit

## Common Issues

### 1. Controller Not Starting
- Ensure that all dependencies are installed correctly.
- Check the logs for any error messages using `kubectl logs <controller-pod-name>`.
- Verify that the Kubernetes cluster is running and accessible.

### 2. CRD Not Found
- Confirm that the Custom Resource Definitions (CRDs) are applied correctly.
- Use `kubectl get crd` to list all CRDs and ensure `ClusterDeploymentStatus` is present.
- If missing, re-run the CRD generation command.

### 3. Integration Issues with ArgoCD
- Ensure that the ArgoCD application is configured correctly.
- Check the application logs for any sync errors.
- Verify that the correct permissions are set in the RBAC configuration.

### 4. Metrics Not Showing in Prometheus
- Ensure that the Prometheus service is running and configured to scrape the metrics endpoint.
- Check the service monitor configuration for any misconfigurations.
- Use `kubectl port-forward` to access the Prometheus UI and check for targets.

### 5. Istio Configuration Errors
- Verify that the Istio control plane is installed and running.
- Check the virtual service and destination rule configurations for correctness.
- Use `istioctl analyze` to identify potential issues in the configuration.

## Debugging Steps

1. **Check Logs**: Always start by checking the logs of the controller and any related services.
2. **Validate Configurations**: Ensure that all YAML configurations are valid and correctly formatted.
3. **Network Policies**: If using network policies, ensure that they allow traffic between components.
4. **Resource Limits**: Check if any resource limits are causing components to crash or become unresponsive.
5. **Kubernetes Events**: Use `kubectl get events` to check for any warnings or errors in the cluster.

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/home/)
- [ArgoCD Documentation](https://argoproj.github.io/argo-cd/)
- [Prometheus Documentation](https://prometheus.io/docs/introduction/overview/)
- [Istio Documentation](https://istio.io/latest/docs/)

## Contact

For further assistance, please reach out to the KubeStellar community or open an issue in the project repository.