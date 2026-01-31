# Getting Started with KubeStellar Integration Toolkit

Welcome to the KubeStellar Integration Toolkit! This guide will help you set up and run the project, as well as provide an overview of its components and functionalities.

## Prerequisites

- Go 1.16 or later
- Kubernetes cluster (local or remote)
- kubectl installed and configured to access your cluster

## Getting Started

### Step 1: Create the Project

Run the following commands to create the project directory and initialize the Go module:

mkdir kubestellar-integration-toolkit && cd kubestellar-integration-toolkit
go mod init github.com/<your-username>/kubestellar-integration-toolkit

### Step 2: Install Dependencies

Install the necessary dependencies using the following commands:

go get sigs.k8s.io/controller-runtime@v0.18.0
go get k8s.io/client-go@v0.30.0
go get k8s.io/apimachinery@v0.30.0

### Step 3: Install Controller-Gen

Install the controller-gen tool, which is used for generating CRDs and other code:

go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

### Step 4: Generate Code

Generate the necessary code for your CRDs and objects:

controller-gen object paths="./api/..."
controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases

### Step 5: Build the Project

Build the project using the Makefile:

make build

### Step 6: Run the Controller Locally

You can run the controller locally with the following command:

make run

### Step 7: Apply a Sample ClusterDeploymentStatus

To test the integration, apply a sample `ClusterDeploymentStatus` resource:

kubectl apply -f config/samples/sample-clusterstatus.yaml

## Documentation Overview

- **Architecture**: Detailed description of the project architecture and components.
- **Getting Started**: Instructions for setting up and running the project.
- **Integrations**: Information on integrating with tools like ArgoCD, Flux, Prometheus, and Istio.
- **API Reference**: Documentation for the REST API endpoints.
- **Troubleshooting**: Common issues and their solutions.

## License

This project is licensed under the Apache 2.0 License.