# KubeStellar Integration Toolkit Makefile

# Project variables
BINARY_NAME=ksit
VERSION?=0.1.0
IMG?=kubestellar/integration-toolkit:$(VERSION)
IMG_LATEST=kubestellar/integration-toolkit:latest

# Tool versions
CONTROLLER_GEN_VERSION=v0.14.0
KUSTOMIZE_VERSION=v5.0.0
GOLANGCI_LINT_VERSION=v1.55.2
ENVTEST_VERSION=latest

# Tool binaries
CONTROLLER_GEN=$(shell which controller-gen)
KUSTOMIZE=$(shell which kustomize)
GOLANGCI_LINT=$(shell which golangci-lint)
ENVTEST=$(shell which setup-envtest)

# Go variables
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")
GO_TEST_FILES=$(shell find . -name '*_test.go' -not -path "./vendor/*")
GOPATH=$(shell go env GOPATH)
GOBIN=$(GOPATH)/bin

# Kubernetes variables
KUBECONFIG?=~/.kube/config
NAMESPACE?=ksit-system

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(BLUE)<target>$(NC)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: build ## Build everything

.PHONY: quickstart
quickstart: setup-clusters build-controller deploy-local install-integrations deploy-samples ## Complete setup from scratch

##@ Cluster Setup

.PHONY: setup-clusters
setup-clusters: ## Create kind clusters and configure kubeconfigs
	@echo "$(BLUE)Creating kind clusters...$(NC)"
	@chmod +x ./scripts/setup-clusters.sh
	@./scripts/setup-clusters.sh

.PHONY: install-integrations
install-integrations: ## Install ArgoCD, Flux, Prometheus, Istio on clusters
	@echo "$(BLUE)Installing DevOps tools...$(NC)"
	@chmod +x ./scripts/install-integrations.sh
	@./scripts/install-integrations.sh

.PHONY: cleanup
cleanup: ## Delete all kind clusters and resources
	@echo "$(BLUE)Cleaning up environment...$(NC)"
	@chmod +x ./scripts/cleanup.sh
	@./scripts/cleanup.sh

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code
	@echo "$(GREEN)Running go fmt...$(NC)"
	@go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "$(GREEN)Running golangci-lint...$(NC)"
	@if ! command -v golangci-lint &> /dev/null; then \
	go install github.com/golangci-lint-lint/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@golangci-lint run

.PHONY: test
test: generate fmt vet ## Run unit tests
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v -short ./pkg/...

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	@go test -v -coverprofile=coverage.out ./pkg/...
	@go tool cover -html=coverage.out -o coverage.html

.PHONY: test-e2e
test-e2e: ## Run e2e tests (requires real cluster)
	@echo "$(GREEN)Running e2e tests...$(NC)"
	@go test ./test/e2e/... -v -ginkgo.v

.PHONY: test-all
test-all: test test-integration test-e2e ## Run all tests

##@ Build

.PHONY: generate
generate: controller-gen ## Generate code
	@echo "$(GREEN)Generating code...$(NC)"
	@$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate manifests (CRDs, RBAC, etc.)
	@echo "$(GREEN)Generating manifests...$(NC)"
	@$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: build
build: generate fmt vet ## Build manager binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@go build -o bin/$(BINARY_NAME) ./cmd/ksit/main.go

.PHONY: build-controller
build-controller: ## Build controller Docker image
	@echo "$(GREEN)Building controller image ksit-controller:latest...$(NC)"
	@docker build -t ksit-controller:latest -f Dockerfile .
	@docker tag ksit-controller:latest ksit-controller:v$$(cat VERSION 2>/dev/null || echo "12")

.PHONY: deploy-local
deploy-local: build-controller ## Deploy controller to kind clusters using Helm
	@echo "$(GREEN)Loading image into kind clusters...$(NC)"
	@kind load docker-image ksit-controller:latest --name ksit-control
	@echo "$(GREEN)Deploying controller via Helm...$(NC)"
	@kubectl config use-context kind-ksit-control
	@helm upgrade --install ksit ./deploy/helm/ksit \
		--namespace ksit-system \
		--create-namespace \
		--set image.repository=ksit-controller \
		--set image.tag=latest \
		--set image.pullPolicy=IfNotPresent
	@echo "$(GREEN)Waiting for controller to be ready...$(NC)"
	@kubectl wait --for=condition=ready --timeout=120s pod -l control-plane=controller-manager -n ksit-system

.PHONY: build-local
build-local: generate fmt vet ## Build for local OS
	@echo "$(GREEN)Building for local OS...$(NC)"
	@CGO_ENABLED=0 go build -o bin/$(BINARY_NAME) ./cmd/ksit/main.go

.PHONY: run
run: generate fmt vet ## Run controller locally
	@echo "$(GREEN)Running controller...$(NC)"
	@go run ./cmd/ksit/main.go

.PHONY: run-webhook
run-webhook: generate fmt vet ## Run controller with webhooks enabled
	@echo "$(GREEN)Running controller with webhooks...$(NC)"
	@go run ./cmd/ksit/main.go --enable-webhook=true

##@ Docker

.PHONY: docker-build
docker-build: ## Build docker image
	@echo "$(GREEN)Building docker image $(IMG)...$(NC)"
	@docker build -t $(IMG) -t $(IMG_LATEST) .

.PHONY: docker-push
docker-push: ## Push docker image
	@echo "$(GREEN)Pushing docker image $(IMG)...$(NC)"
	@docker push $(IMG)
	@docker push $(IMG_LATEST)

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push docker image

##@ Deployment

.PHONY: install
install: manifests kustomize ## Install CRDs into cluster
	@echo "$(GREEN)Installing CRDs...$(NC)"
	@$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from cluster
	@echo "$(GREEN)Uninstalling CRDs...$(NC)"
	@$(KUSTOMIZE) build config/crd | kubectl delete -f -

.PHONY: deploy
deploy: manifests ## Deploy controller to cluster using Helm
	@echo "$(GREEN)Deploying controller via Helm...$(NC)"
	@helm upgrade --install ksit ./deploy/helm/ksit \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=ksit-controller \
		--set image.tag=latest \
		--set image.pullPolicy=IfNotPresent

.PHONY: deploy-helm
deploy-helm: ## Deploy using Helm
	@echo "$(GREEN)Deploying with Helm...$(NC)"
	@helm upgrade --install ksit ./deploy/helm/ksit \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=ksit-controller \
		--set image.tag=latest \
		--set image.pullPolicy=IfNotPresent

.PHONY: undeploy-helm
undeploy-helm: ## Undeploy using Helm
	@echo "$(GREEN)Uninstalling Helm release...$(NC)"
	@helm uninstall ksit --namespace $(NAMESPACE)

.PHONY: undeploy
undeploy: undeploy-helm ## Undeploy controller from cluster (alias for undeploy-helm)

.PHONY: deploy-samples
deploy-samples: ## Deploy sample integrations
	@echo "$(GREEN)Deploying samples...$(NC)"
	@kubectl apply -f config/samples/

##@ Tools

.PHONY: controller-gen
controller-gen: ## Download controller-gen if not present
	@if ! command -v controller-gen &> /dev/null; then \
	echo "$(YELLOW)Installing controller-gen...$(NC)"; \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION); \
	fi

.PHONY: kustomize
kustomize: ## Download kustomize if not present
	@if ! command -v kustomize &> /dev/null; then \
	echo "$(YELLOW)Installing kustomize...$(NC)"; \
	go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION); \
	fi

.PHONY: envtest
envtest: ## Download envtest if not present
	@if ! command -v setup-envtest &> /dev/null; then \
	echo "$(YELLOW)Installing envtest...$(NC)"; \
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION); \
	fi

.PHONY: test-integration
test-integration: envtest ## Run integration tests
	@echo "$(GREEN)Setting up envtest binaries...$(NC)"
	@./scripts/setup-test-env.sh
	@echo "$(GREEN)Running integration tests...$(NC)"
	@# ✅ FIX: Use absolute path for KUBEBUILDER_ASSETS
	@KUBEBUILDER_ASSETS=$$(cd "$$(pwd)/bin/k8s/k8s/1.29.5-darwin-arm64" 2>/dev/null && pwd || cd "$$(pwd)/bin/k8s/current" && pwd) \
		go test ./test/integration/... -v -ginkgo.v -timeout=10m

.PHONY: test-integration-debug
test-integration-debug: envtest ## Run integration tests with verbose output
	@./scripts/setup-test-env.sh
	@KUBEBUILDER_ASSETS=$$(cd "$$(pwd)/bin/k8s/k8s/1.29.5-darwin-arm64" 2>/dev/null && pwd || cd "$$(pwd)/bin/k8s/current" && pwd) \
		go test ./test/integration/... -v -ginkgo.v -ginkgo.trace -timeout=10m 2>&1 | tee integration-test-debug.log

.PHONY: tools
tools: controller-gen kustomize envtest ## Install all required tools
	@echo "$(GREEN)All tools installed$(NC)"

##@ Scripts

.PHONY: setup
setup: tools ## Run initial setup
	@echo "$(GREEN)Running setup...$(NC)"
	@./scripts/setup.sh

.PHONY: generate-crds
generate-crds: ## Generate CRDs using script
	@echo "$(GREEN)Generating CRDs...$(NC)"
	@./scripts/generate-crds.sh

.PHONY: generate-webhook-certs
generate-webhook-certs: ## Generate webhook certificates
	@echo "$(GREEN)Generating webhook certificates...$(NC)"
	@./scripts/generate-webhook-certs.sh

.PHONY: install-deps
install-deps: ## Install Go dependencies
	@echo "$(GREEN)Installing dependencies...$(NC)"
	@./scripts/install-deps.sh

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	@rm -rf bin/ coverage.out coverage.html

.PHONY: clean-all
clean-all: clean ## Clean everything including dependencies
	@echo "$(GREEN)Cleaning everything...$(NC)"
	@go clean -modcache

##@ Examples

.PHONY: validate-examples
validate-examples: ## Validate example configurations
	@echo "$(GREEN)Validating examples...$(NC)"
	@for file in config/samples/*.yaml; do \
	echo "Validating $$file"; \
	kubectl apply --dry-run=client -f $$file; \
	done

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(NC)"
	@echo "Documentation generation not implemented yet"

##@ Version

.PHONY: version
version: ## Show version
	@echo "$(GREEN)Version: $(VERSION)$(NC)"

##@ Status

.PHONY: status
status: ## Show deployment status
	@echo "$(GREEN)Checking deployment status...$(NC)"
	@kubectl get deployments -n $(NAMESPACE)
	@kubectl get pods -n $(NAMESPACE)

.PHONY: logs
logs: ## Show controller logs
	@echo "$(GREEN)Showing controller logs...$(NC)"
	@kubectl logs -n $(NAMESPACE) -l control-plane=controller-manager -f

##@ Auto-Install Feature

.PHONY: deploy-autoinstall
deploy-autoinstall: ## Deploy controller with auto-install feature (v13)
	@echo "$(GREEN)Deploying KSIT with auto-install feature...$(NC)"
	@echo "Step 1: Building controller image v13..."
	@docker build -t ksit-controller:v13 .
	@echo "Step 2: Loading image into kind cluster..."
	@kind load docker-image ksit-controller:v13 --name ksit-control || echo "Warning: Failed to load image, continuing..."
	@echo "Step 3: Updating CRDs..."
	@kubectl apply -f config/crd/bases/ksit.io_integrations.yaml --context kind-ksit-control
	@echo "Step 4: Upgrading Helm release..."
	@helm upgrade ksit ./deploy/helm/ksit \
		--namespace $(NAMESPACE) \
		--set image.repository=ksit-controller \
		--set image.tag=v13 \
		--set image.pullPolicy=Always \
		--kube-context kind-ksit-control
	@echo "Step 5: Waiting for controller to be ready..."
	@kubectl rollout status deployment/ksit-controller-manager -n $(NAMESPACE) --context kind-ksit-control --timeout=2m
	@echo "$(GREEN)✓ Auto-install feature deployed!$(NC)"

.PHONY: test-autoinstall
test-autoinstall: ## Test auto-install feature
	@echo "$(GREEN)Testing auto-install feature...$(NC)"
	@./test-autoinstall.sh

.PHONY: demo-autoinstall-argocd
demo-autoinstall-argocd: ## Demo: Auto-install ArgoCD
	@echo "$(GREEN)Creating Integration with auto-install for ArgoCD...$(NC)"
	@kubectl apply -f config/samples/argocd_integration_autoinstall.yaml --context kind-ksit-control
	@echo "Watch progress with: kubectl get integration argocd-autoinstall -n $(NAMESPACE) -w --context kind-ksit-control"

.PHONY: demo-autoinstall-prometheus
demo-autoinstall-prometheus: ## Demo: Auto-install Prometheus
	@echo "$(GREEN)Creating Integration with auto-install for Prometheus...$(NC)"
	@kubectl apply -f config/samples/prometheus_integration_autoinstall.yaml --context kind-ksit-control
	@echo "Watch progress with: kubectl get integration prometheus-autoinstall -n $(NAMESPACE) -w --context kind-ksit-control"

.PHONY: demo-autoinstall-istio
demo-autoinstall-istio: ## Demo: Auto-install Istio
	@echo "$(GREEN)Creating Integration with auto-install for Istio...$(NC)"
	@kubectl apply -f config/samples/istio_integration_autoinstall.yaml --context kind-ksit-control
	@echo "Watch progress with: kubectl get integration istio-autoinstall -n $(NAMESPACE) -w --context kind-ksit-control"

##@ Release

.PHONY: release
release: test lint build docker-build-push ## Build and push a release
	@echo "$(GREEN)Release $(VERSION) complete!$(NC)"

.PHONY: release-local
release-local: test lint build ## Build a local release
	@echo "$(GREEN)Local release $(VERSION) complete!$(NC)"