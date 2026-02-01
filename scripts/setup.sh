#!/bin/bash
set -e

echo "🚀 Setting up KubeStellar Integration Toolkit..."

# Get project root (macOS compatible)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "📂 Project root: $PROJECT_ROOT"

# Create directory structure
echo "📁 Creating directory structure..."
mkdir -p config/crd/bases
mkdir -p config/samples
mkdir -p config/manager
mkdir -p config/rbac
mkdir -p config/webhook
mkdir -p config/default
mkdir -p pkg/controller
mkdir -p pkg/cluster
mkdir -p pkg/config
mkdir -p pkg/kubestellar
mkdir -p pkg/integrations/{argocd,flux,prometheus,istio}
mkdir -p internal/{utils,webhook}
mkdir -p deploy/helm/ksit/templates
mkdir -p deploy/kustomize/{base,overlays/{dev,prod}}
mkdir -p test/{e2e,integration}
mkdir -p docs/integrations
mkdir -p examples/multi-cluster-{argocd,flux,mesh,observability}
mkdir -p cmd/ksit
mkdir -p api/v1alpha1
mkdir -p bin

echo "✅ Directory structure created"

# Check Go version
echo "🔍 Checking Go version..."
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go version: $GO_VERSION"

# Initialize go module if not exists
if [ ! -f "go.mod" ]; then
    echo "📦 Initializing Go module..."
    go mod init github.com/kubestellar/integration-toolkit
fi

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy
go mod download

# Install required tools
echo "🛠️  Installing required tools..."

# Install controller-gen
if ! command -v controller-gen &> /dev/null; then
    echo "Installing controller-gen..."
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
fi

# Install kustomize
if ! command -v kustomize &> /dev/null; then
    echo "Installing kustomize..."
    go install sigs.k8s.io/kustomize/kustomize/v5@latest
fi

# Install golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

# Add GOPATH/bin to PATH if not already
export PATH="$PATH:$(go env GOPATH)/bin"

echo "✅ Tools installed"

# Generate code
echo "🔧 Generating code..."

# Generate CRDs
echo "Generating CRDs..."
controller-gen crd paths="./api/..." output:crd:artifacts:config=config/crd/bases

# Generate deep copy functions
echo "Generating deepcopy functions..."
controller-gen object paths="./api/..."

# Generate RBAC
echo "Generating RBAC manifests..."
controller-gen rbac:roleName=manager-role paths="./..." output:rbac:artifacts:config=config/rbac

echo "✅ Code generation complete"

# Copy CRDs to deployment locations
echo "📋 Copying CRDs to deployment locations..."
cp config/crd/bases/*.yaml deploy/kustomize/base/ 2>/dev/null || true
cp config/crd/bases/*.yaml deploy/helm/ksit/templates/ 2>/dev/null || true

# Create .env file if not exists
if [ ! -f ".env" ]; then
    echo "📝 Creating .env file..."
    cat > .env <<EOF
# KubeStellar Integration Toolkit Environment Variables
KUBECONFIG=${HOME}/.kube/config
LOG_LEVEL=info
METRICS_BIND_ADDRESS=:8080
HEALTH_PROBE_BIND_ADDRESS=:8081
LEADER_ELECT=false
ENABLE_WEBHOOK=false
WEBHOOK_PORT=9443
WEBHOOK_CERT_DIR=/tmp/k8s-webhook-server/serving-certs
EOF
    echo "✅ .env file created"
fi

# Build the project
echo "🔨 Building project..."
if go build -o bin/ksit ./cmd/ksit/main.go; then
    echo "✅ Build successful: bin/ksit"
else
    echo "⚠️  Build failed, but setup can continue"
fi

# Run tests
echo "🧪 Running tests..."
if go test ./... -v -short 2>&1 | tee test-output.log; then
    echo "✅ Tests passed"
else
    echo "⚠️  Some tests failed, check test-output.log"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "📚 Next steps:"
echo "  1. Review generated CRDs: ls -la config/crd/bases/"
echo "  2. Build the project: make build"
echo "  3. Run tests: make test"
echo "  4. Run locally: make run"
echo "  5. Deploy to cluster: make deploy"
echo ""
echo "🔧 Available commands:"
echo "  make build       - Build the binary"
echo "  make run         - Run controller locally"
echo "  make test        - Run all tests"
echo "  make deploy      - Deploy to Kubernetes"
echo "  make install     - Install CRDs"
echo ""
echo "📖 Documentation: docs/"
echo "📝 Examples: examples/"