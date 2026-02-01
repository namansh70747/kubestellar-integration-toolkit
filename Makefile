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

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@go test -v ./test/integration/...

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	@echo "$(GREEN)Running e2e tests...$(NC)"
	@go test -v ./test/e2e/...

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
deploy: manifests kustomize ## Deploy controller to cluster
	@echo "$(GREEN)Deploying controller...$(NC)"
	@cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	@$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from cluster
	@echo "$(GREEN)Removing controller...$(NC)"
	@$(KUSTOMIZE) build config/default | kubectl delete -f -

.PHONY: deploy-helm
deploy-helm: ## Deploy using Helm
	@echo "$(GREEN)Deploying with Helm...$(NC)"
	@helm install ksit deploy/helm/ksit --namespace $(NAMESPACE) --create-namespace

.PHONY: undeploy-helm
undeploy-helm: ## Undeploy using Helm
	@echo "$(GREEN)Uninstalling Helm release...$(NC)"
	@helm uninstall ksit --namespace $(NAMESPACE)

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

##@ Release

.PHONY: release
release: test lint build docker-build-push ## Build and push a release
	@echo "$(GREEN)Release $(VERSION) complete!$(NC)"

.PHONY: release-local
release-local: test lint build ## Build a local release
	@echo "$(GREEN)Local release $(VERSION) complete!$(NC)"